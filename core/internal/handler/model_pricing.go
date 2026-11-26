package handler

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/auenger/MEPERECO/core/internal/metrics"
	"github.com/auenger/MEPERECO/core/internal/middleware"
)

type ModelRate struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cache_read"`
	CacheWrite float64 `json:"cache_write"`
}
type ModelPricingRule struct {
	Provider     string    `json:"provider"`
	Model        string    `json:"model"`
	Mode         string    `json:"mode"`
	ReferenceKey string    `json:"reference_key"`
	Rates        ModelRate `json:"rates"`
}
type ModelPricingEntry struct {
	ModelPricingRule
	Source         string     `json:"source"`
	EffectiveRates *ModelRate `json:"effective_rates"`
	Tokens         int64      `json:"tokens"`
	ActualUSD      float64    `json:"actual_usd"`
}
type ModelCost struct {
	ActualUSD       float64   `json:"actual_usd"`
	EstimatedUSD    float64   `json:"estimated_usd"`
	UnknownTokens   int64     `json:"unknown_tokens"`
	PricedTokens    int64     `json:"priced_tokens"`
	EstimatedTokens int64     `json:"estimated_tokens"`
	CacheSavingsUSD float64   `json:"cache_savings_usd"`
	Breakdown       ModelRate `json:"breakdown"`
}
type modelPricingKey struct{ provider, model string }
type modelPricingRules map[modelPricingKey]ModelPricingRule

func pricingKey(provider, model string) modelPricingKey {
	return modelPricingKey{strings.ToLower(strings.TrimSpace(provider)), strings.TrimSpace(model)}
}
func modelRate(r metrics.ReferenceRate) ModelRate {
	return ModelRate{r.Input, r.Output, r.CacheRead, r.CacheWrite}
}
func (rules modelPricingRules) rate(provider, model string) (ModelRate, string) {
	if rule, ok := rules[pricingKey(provider, model)]; ok {
		switch rule.Mode {
		case "custom":
			return rule.Rates, "custom"
		case "free":
			return ModelRate{}, "free"
		case "alias":
			if p, ok := metrics.ReferenceRateByKey(rule.ReferenceKey); ok {
				return modelRate(p), "alias"
			}
			return ModelRate{}, "unpriced"
		}
	}
	if p, ok := metrics.MatchReferenceRate(provider, model); ok {
		return modelRate(p), "reference"
	}
	return ModelRate{}, "unpriced"
}
func (rules modelPricingRules) cost(provider, model string, actual int64, allIn, allOut, allRead, allWrite, in, out, read, write int64) ModelCost {
	c := ModelCost{ActualUSD: float64(actual) / metrics.CostUSDTicksPerUSD}
	unpriced := in + out + read + write
	all := allIn + allOut + allRead + allWrite
	c.PricedTokens = all - unpriced
	rate, source := rules.rate(provider, model)
	if source == "unpriced" {
		c.UnknownTokens = unpriced
		c.Breakdown.Input = c.ActualUSD
		return c
	}
	c.PricedTokens = all
	c.EstimatedTokens = unpriced
	c.CacheSavingsUSD = float64(allRead) * math.Max(0, rate.Input-rate.CacheRead) / 1e6
	c.Breakdown = ModelRate{float64(in) * rate.Input / 1e6, float64(out) * rate.Output / 1e6, float64(read) * rate.CacheRead / 1e6, float64(write) * rate.CacheWrite / 1e6}
	c.EstimatedUSD = c.Breakdown.Input + c.Breakdown.Output + c.Breakdown.CacheRead + c.Breakdown.CacheWrite
	// Only the total actual spend is authoritative; apportion it across known
	// token categories using the reference rate weights for presentation.
	weights := ModelRate{float64(allIn-in) * rate.Input, float64(allOut-out) * rate.Output, float64(allRead-read) * rate.CacheRead, float64(allWrite-write) * rate.CacheWrite}
	sum := weights.Input + weights.Output + weights.CacheRead + weights.CacheWrite
	if sum > 0 {
		f := c.ActualUSD / sum
		c.Breakdown.Input += weights.Input * f
		c.Breakdown.Output += weights.Output * f
		c.Breakdown.CacheRead += weights.CacheRead * f
		c.Breakdown.CacheWrite += weights.CacheWrite * f
	} else {
		c.Breakdown.Input += c.ActualUSD
	}
	return c
}
func (c *ModelCost) add(v ModelCost) {
	c.ActualUSD += v.ActualUSD
	c.EstimatedUSD += v.EstimatedUSD
	c.UnknownTokens += v.UnknownTokens
	c.PricedTokens += v.PricedTokens
	c.EstimatedTokens += v.EstimatedTokens
	c.CacheSavingsUSD += v.CacheSavingsUSD
	c.Breakdown.Input += v.Breakdown.Input
	c.Breakdown.Output += v.Breakdown.Output
	c.Breakdown.CacheRead += v.Breakdown.CacheRead
	c.Breakdown.CacheWrite += v.Breakdown.CacheWrite
}
func (h *Handler) loadModelPricing(ctx context.Context, ws pgtype.UUID) (modelPricingRules, error) {
	rows, e := h.DB.Query(ctx, `SELECT provider,model,mode,reference_key,input_per_m,output_per_m,cache_read_per_m,cache_write_per_m FROM workspace_model_price WHERE workspace_id=$1`, ws)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := modelPricingRules{}
	for rows.Next() {
		var r ModelPricingRule
		if e = rows.Scan(&r.Provider, &r.Model, &r.Mode, &r.ReferenceKey, &r.Rates.Input, &r.Rates.Output, &r.Rates.CacheRead, &r.Rates.CacheWrite); e != nil {
			return nil, e
		}
		out[pricingKey(r.Provider, r.Model)] = r
	}
	return out, rows.Err()
}
func (h *Handler) GetWorkspaceModelPricing(w http.ResponseWriter, r *http.Request) {
	ws, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workspace id")
	if !ok {
		return
	}
	rules, e := h.loadModelPricing(r.Context(), ws)
	if e != nil {
		writeError(w, 500, "failed to load model pricing")
		return
	}
	rows, e := h.DB.Query(r.Context(), `WITH discovered AS (SELECT lower(trim(u.provider)) provider,trim(u.model) model, sum(u.input_tokens+u.output_tokens+u.cache_read_tokens+u.cache_write_tokens)::bigint tokens,coalesce(sum(u.cost_usd_ticks),0)::bigint ticks FROM agent_run_usage u JOIN agent_run a ON a.id=u.run_id JOIN agent ag ON ag.id=a.agent_id WHERE ag.workspace_id=$1 GROUP BY 1,2) SELECT coalesce(d.provider,p.provider),coalesce(d.model,p.model),coalesce(d.tokens,0),coalesce(d.ticks,0) FROM discovered d FULL JOIN (SELECT provider,model FROM workspace_model_price WHERE workspace_id=$1) p USING(provider,model) ORDER BY 3 DESC,1,2`, ws)
	if e != nil {
		writeError(w, 500, "failed to discover models")
		return
	}
	defer rows.Close()
	entries := []ModelPricingEntry{}
	for rows.Next() {
		var entry ModelPricingEntry
		var ticks int64
		if e = rows.Scan(&entry.Provider, &entry.Model, &entry.Tokens, &ticks); e != nil {
			writeError(w, 500, "failed to read models")
			return
		}
		entry.Mode = "default"
		if rule, ok := rules[pricingKey(entry.Provider, entry.Model)]; ok {
			entry.ModelPricingRule = rule
		}
		rate, source := rules.rate(entry.Provider, entry.Model)
		entry.Source = source
		if source != "unpriced" {
			entry.EffectiveRates = &rate
		}
		entry.ActualUSD = float64(ticks) / metrics.CostUSDTicksPerUSD
		entries = append(entries, entry)
	}
	if rows.Err() != nil {
		writeError(w, 500, "failed to read models")
		return
	}
	member, _ := middleware.MemberFromContext(r.Context())
	writeJSON(w, 200, map[string]any{"entries": entries, "reference_rates": metrics.ReferenceRates(), "can_manage": roleAllowed(member.Role, "owner", "admin")})
}
func validateModelPricing(rule ModelPricingRule) error {
	if len(rule.Provider) > 128 || len(rule.Model) > 512 || rule.Model == "" {
		return errors.New("invalid model identity")
	}
	switch rule.Mode {
	case "default", "free":
		return nil
	case "alias":
		if _, ok := metrics.ReferenceRateByKey(rule.ReferenceKey); !ok {
			return errors.New("unknown reference price")
		}
	case "custom":
		for _, v := range []float64{rule.Rates.Input, rule.Rates.Output, rule.Rates.CacheRead, rule.Rates.CacheWrite} {
			if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1e6 {
				return errors.New("invalid rate")
			}
		}
	default:
		return errors.New("invalid pricing mode")
	}
	return nil
}
func (h *Handler) SetWorkspaceModelPricing(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Actor-Source") == "agent_run_token" {
		writeError(w, 403, "task tokens cannot manage pricing")
		return
	}
	ws, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workspace id")
	if !ok {
		return
	}
	var req struct {
		ModelPricingRule
		RateCurrency string  `json:"rate_currency"`
		USDToCNY     float64 `json:"usd_to_cny"`
		Rates        *struct {
			Input      *float64 `json:"input"`
			Output     *float64 `json:"output"`
			CacheRead  *float64 `json:"cache_read"`
			CacheWrite *float64 `json:"cache_write"`
		} `json:"rates"`
	}
	if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&req); e != nil {
		writeError(w, 400, "invalid pricing request")
		return
	}
	rule := req.ModelPricingRule
	key := pricingKey(rule.Provider, rule.Model)
	rule.Provider = key.provider
	rule.Model = key.model
	if rule.Mode == "custom" {
		if req.Rates == nil || req.Rates.Input == nil || req.Rates.Output == nil || req.Rates.CacheRead == nil || req.Rates.CacheWrite == nil {
			writeError(w, 400, "rates required")
			return
		}
		rule.Rates = ModelRate{*req.Rates.Input, *req.Rates.Output, *req.Rates.CacheRead, *req.Rates.CacheWrite}
		if req.RateCurrency != "" && req.RateCurrency != "USD" {
			current, err := h.loadPricingCurrency(r.Context(), ws)
			if err != nil {
				writeError(w, 500, "failed to load pricing currency")
				return
			}
			rule.Rates, err = normalizeRateCurrency(rule.Rates, req.RateCurrency, req.USDToCNY, current)
			if err != nil {
				writeError(w, 400, err.Error())
				return
			}
		}
	}
	if rule.Mode != "custom" {
		rule.Rates = ModelRate{}
	}
	if rule.Mode != "alias" {
		rule.ReferenceKey = ""
	}
	if e := validateModelPricing(rule); e != nil {
		writeError(w, 400, e.Error())
		return
	}
	if rule.Mode == "default" {
		_, e := h.DB.Exec(r.Context(), `DELETE FROM workspace_model_price WHERE workspace_id=$1 AND provider=$2 AND model=$3`, ws, rule.Provider, rule.Model)
		if e != nil {
			writeError(w, 500, "failed to reset model price")
			return
		}
	} else {
		_, e := h.DB.Exec(r.Context(), `INSERT INTO workspace_model_price(workspace_id,provider,model,mode,reference_key,input_per_m,output_per_m,cache_read_per_m,cache_write_per_m) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(workspace_id,provider,model) DO UPDATE SET mode=excluded.mode,reference_key=excluded.reference_key,input_per_m=excluded.input_per_m,output_per_m=excluded.output_per_m,cache_read_per_m=excluded.cache_read_per_m,cache_write_per_m=excluded.cache_write_per_m,updated_at=now()`, ws, rule.Provider, rule.Model, rule.Mode, rule.ReferenceKey, rule.Rates.Input, rule.Rates.Output, rule.Rates.CacheRead, rule.Rates.CacheWrite)
		if e != nil {
			writeError(w, 500, "failed to save model price")
			return
		}
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}
