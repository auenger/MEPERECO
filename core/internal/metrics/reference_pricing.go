package metrics

import (
	"regexp"
	"sort"
	"strings"
)

type ReferenceRate struct {
	Key        string  `json:"key"`
	Provider   string  `json:"provider"`
	Model      string  `json:"model"`
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cache_read"`
	CacheWrite float64 `json:"cache_write"`
}

func referenceRate(key string, p ModelPrice) ReferenceRate {
	return ReferenceRate{key, p.Provider, p.Model, p.InputPerM, p.OutputPerM, p.CacheReadPerM, p.CacheWritePerM}
}
func ReferenceRates() []ReferenceRate {
	out := make([]ReferenceRate, 0, len(modelPrices))
	for key, p := range modelPrices {
		out = append(out, referenceRate(key, p))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}
func ReferenceRateByKey(key string) (ReferenceRate, bool) {
	p, ok := modelPrices[key]
	return referenceRate(key, p), ok
}

var claudeMinor = regexp.MustCompile(`-(\d+)-(\d+)$`)
var datedSnapshot = regexp.MustCompile(`-(\d{8}|\d{4}-\d{2}-\d{2}|latest)$`)

// Resolve exact public model identities only. An arbitrary gateway prefix is
// not stripped; admins must explicitly map a private or ambiguous model.
func MatchReferenceRate(provider, model string) (ReferenceRate, bool) {
	raw := strings.ToLower(strings.TrimSpace(model))
	provider = strings.ToLower(strings.TrimSpace(provider))
	if prefix, rest, ok := strings.Cut(raw, "/"); ok {
		switch prefix {
		case "openai", "anthropic", "deepseek", "google", "xai", "minimax", "moonshot", "zai":
			raw = rest
		case "cursor":
			if provider != "cursor" {
				return ReferenceRate{}, false
			}
			raw = rest
		default:
			return ReferenceRate{}, false
		}
	}
	if strings.HasPrefix(raw, "claude-") {
		raw = strings.TrimSuffix(raw, "[1m]")
		raw = datedSnapshot.ReplaceAllString(raw, "")
		raw = claudeMinor.ReplaceAllString(raw, `-$1.$2`)
	} else {
		raw = datedSnapshot.ReplaceAllString(raw, "")
	}
	switch raw {
	case "deepseek-v4-flash", "deepseek-chat", "deepseek-reasoner":
		return ReferenceRateByKey("deepseek:v4-flash")
	case "deepseek-v4-pro":
		return ReferenceRateByKey("deepseek:v4-pro")
	}
	if provider == "cursor" {
		if p, ok := ReferenceRateByKey("cursor:" + raw); ok {
			return p, true
		}
	}
	for _, p := range ReferenceRates() {
		if p.Provider != "cursor" && p.Model == raw && (strings.HasPrefix(raw, "gpt-") || strings.HasPrefix(raw, "claude-") || strings.HasPrefix(raw, "glm-") || strings.HasPrefix(raw, "kimi-") || strings.HasPrefix(raw, "gemini-") || strings.HasPrefix(raw, "grok-") || strings.HasPrefix(raw, "o3") || strings.HasPrefix(raw, "o4")) {
			return p, true
		}
	}
	return ReferenceRate{}, false
}
