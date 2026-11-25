package handler

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type PricingCurrency struct {
	Currency string  `json:"currency"`
	USDToCNY float64 `json:"usd_to_cny"`
}

func (h *Handler) loadPricingCurrency(ctx context.Context, ws pgtype.UUID) (PricingCurrency, error) {
	c := PricingCurrency{Currency: "USD"}
	err := h.DB.QueryRow(ctx, `SELECT currency,usd_to_cny FROM workspace_pricing_currency WHERE workspace_id=$1`, ws).Scan(&c.Currency, &c.USDToCNY)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, nil
	}
	return c, err
}
func validPricingCurrency(c PricingCurrency) bool {
	return (c.Currency == "USD" || c.Currency == "CNY") && !math.IsNaN(c.USDToCNY) && !math.IsInf(c.USDToCNY, 0) && c.USDToCNY >= 0 && c.USDToCNY <= 1000 && (c.Currency != "CNY" || c.USDToCNY > 0)
}
func (h *Handler) GetWorkspacePricingCurrency(w http.ResponseWriter, r *http.Request) {
	ws, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workspace id")
	if !ok {
		return
	}
	c, err := h.loadPricingCurrency(r.Context(), ws)
	if err != nil {
		writeError(w, 500, "failed to load pricing currency")
		return
	}
	writeJSON(w, 200, c)
}
func (h *Handler) SetWorkspacePricingCurrency(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Actor-Source") == "agent_run_token" {
		writeError(w, 403, "task tokens cannot manage pricing currency")
		return
	}
	ws, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "workspace id")
	if !ok {
		return
	}
	var c PricingCurrency
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&c); err != nil || !validPricingCurrency(c) {
		writeError(w, 400, "invalid currency or exchange rate")
		return
	}
	if _, err := h.DB.Exec(r.Context(), `INSERT INTO workspace_pricing_currency(workspace_id,currency,usd_to_cny) VALUES($1,$2,$3) ON CONFLICT(workspace_id) DO UPDATE SET currency=excluded.currency,usd_to_cny=excluded.usd_to_cny,updated_at=now()`, ws, c.Currency, c.USDToCNY); err != nil {
		writeError(w, 500, "failed to save pricing currency")
		return
	}
	writeJSON(w, 200, c)
}

// The rate table and provider-reported costs remain in USD. CNY edits carry
// the displayed exchange-rate snapshot; reject stale quotes instead of
// silently applying a different conversion after another admin changes it.
func normalizeRateCurrency(rate ModelRate, currency string, quote float64, current PricingCurrency) (ModelRate, error) {
	switch currency {
	case "", "USD":
		return rate, nil
	case "CNY":
		if current.Currency != "CNY" || current.USDToCNY <= 0 || quote != current.USDToCNY {
			return ModelRate{}, errors.New("pricing currency changed; reload and retry")
		}
		return ModelRate{rate.Input / quote, rate.Output / quote, rate.CacheRead / quote, rate.CacheWrite / quote}, nil
	default:
		return ModelRate{}, errors.New("unsupported pricing currency")
	}
}
