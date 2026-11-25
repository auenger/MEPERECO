package handler

import (
	"math"
	"testing"
)

func TestPricingCurrency(t *testing.T) {
	c := PricingCurrency{Currency: "CNY", USDToCNY: 7.2}
	if !validPricingCurrency(c) || !validPricingCurrency(PricingCurrency{Currency: "USD"}) {
		t.Fatal("valid currency rejected")
	}
	for _, bad := range []PricingCurrency{{Currency: "CNY"}, {Currency: "EUR", USDToCNY: 7.2}, {Currency: "CNY", USDToCNY: math.Inf(1)}} {
		if validPricingCurrency(bad) {
			t.Fatal("invalid currency accepted", bad)
		}
	}
	r, e := normalizeRateCurrency(ModelRate{Input: 7.2, Output: 14.4, CacheRead: .72, CacheWrite: 7.2}, "CNY", 7.2, c)
	if e != nil || math.Abs(r.Input-1) > 1e-10 || math.Abs(r.Output-2) > 1e-10 || math.Abs(r.CacheRead-.1) > 1e-10 {
		t.Fatalf("incorrect conversion: %+v %v", r, e)
	}
	if _, e = normalizeRateCurrency(ModelRate{Input: 7.2}, "CNY", 7.1, c); e == nil {
		t.Fatal("stale quote accepted")
	}
	r, e = normalizeRateCurrency(ModelRate{Input: 7.2}, "USD", 0, c)
	if e != nil || r.Input != 7.2 {
		t.Fatal("USD must remain unchanged")
	}
}
