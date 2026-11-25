package handler

import (
	"math"
	"testing"
)

func TestWorkspaceModelPricing(t *testing.T) {
	rules := modelPricingRules{}
	unknown := rules.cost("codex", "deepseek-flash", 0, 1000000, 2000000, 3000000, 4000000, 1000000, 2000000, 3000000, 4000000)
	if unknown.UnknownTokens != 10000000 || unknown.PricedTokens != 0 || unknown.EstimatedUSD != 0 {
		t.Fatalf("unknown model: %+v", unknown)
	}
	rule := ModelPricingRule{Provider: "codex", Model: "deepseek-flash", Mode: "custom", Rates: ModelRate{Input: 2, Output: 3, CacheRead: .5, CacheWrite: 1}}
	rules[pricingKey(rule.Provider, rule.Model)] = rule
	c := rules.cost("CODEX", "deepseek-flash", 10000000000, 2000000, 2000000, 3000000, 4000000, 1000000, 2000000, 3000000, 4000000)
	if c.ActualUSD != 1 || math.Abs(c.EstimatedUSD-13.5) > 1e-9 || c.UnknownTokens != 0 || c.EstimatedTokens != 10000000 || math.Abs(c.Breakdown.Input+c.Breakdown.Output+c.Breakdown.CacheRead+c.Breakdown.CacheWrite-14.5) > 1e-9 {
		t.Fatalf("mixed cost: %+v", c)
	}
	if v := rules.cost("claude", "deepseek-flash", 0, 1, 0, 0, 0, 1, 0, 0, 0); v.UnknownTokens != 1 {
		t.Fatalf("provider isolation: %+v", v)
	}
	if v := rules.cost("claude", "workbuddy/deepseek-v4.1-flash[1m]", 0, 1, 0, 0, 0, 1, 0, 0, 0); v.UnknownTokens != 1 {
		t.Fatalf("private model should not guess pricing: %+v", v)
	}
	rule.Mode = "free"
	rules[pricingKey(rule.Provider, rule.Model)] = rule
	if v := rules.cost("codex", "deepseek-flash", 0, 1, 0, 0, 0, 1, 0, 0, 0); v.UnknownTokens != 0 || v.PricedTokens != 1 || v.EstimatedUSD != 0 {
		t.Fatalf("explicit free: %+v", v)
	}
	rule.Mode = "alias"
	rule.ReferenceKey = "openai:gpt-5.4"
	rules[pricingKey(rule.Provider, rule.Model)] = rule
	if v := rules.cost("codex", "deepseek-flash", 0, 1000000, 0, 0, 0, 1000000, 0, 0, 0); v.EstimatedUSD != 2.5 {
		t.Fatalf("alias: %+v", v)
	}
	rule.Mode = "custom"
	rule.Rates.Input = -1
	if validateModelPricing(rule) == nil {
		t.Fatal("negative price accepted")
	}
}
