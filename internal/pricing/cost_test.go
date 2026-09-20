package pricing

import (
	"math"
	"testing"
)

const tol = 1e-9

func closeTo(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %.10f, want %.10f", label, got, want)
	}
}

func calc(t *testing.T) *Calculator {
	t.Helper()
	tbl, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	return NewCalculator(tbl)
}

// One million of every class, so each figure reads as the rate itself.
func oneMillionEach() Tokens {
	return Tokens{Input: 1e6, Output: 1e6, CacheWrite5m: 1e6, CacheWrite1h: 1e6, CacheRead: 1e6}
}

func TestCostAllFiveColumns(t *testing.T) {
	c := calc(t)
	got := c.Cost(Request{Model: "claude-opus-5", Tokens: oneMillionEach()})
	if !got.Priced {
		t.Fatal("opus-5 should be priced")
	}
	closeTo(t, "input", got.Input, 5)
	closeTo(t, "output", got.Output, 25)
	closeTo(t, "cache write 5m", got.CacheWrite5m, 6.25)
	closeTo(t, "cache write 1h", got.CacheWrite1h, 10)
	closeTo(t, "cache read", got.CacheRead, 0.50)
	closeTo(t, "total", got.Total, 46.75)
}

func TestCostPerModel(t *testing.T) {
	c := calc(t)
	tests := map[string]float64{
		"claude-opus-5":     46.75,  // 5 + 25 + 6.25 + 10 + 0.50
		"claude-sonnet-5":   18.70,  // 2 + 10 + 2.50 + 4 + 0.20
		"claude-sonnet-4-6": 28.05,  // 3 + 15 + 3.75 + 6 + 0.30
		"claude-haiku-4-5":  9.35,   // 1 + 5 + 1.25 + 2 + 0.10
		"claude-fable-5-1":  92.75,  // 10 + 50 + 12.50 + 20 + 0.25
		"claude-fable-5":    93.50,  // same, but cache read is 1.00
		"claude-opus-4-1":   140.25, // 15 + 75 + 18.75 + 30 + 1.50
	}
	for model, want := range tests {
		got := c.Cost(Request{Model: model, Tokens: oneMillionEach()})
		closeTo(t, model, got.Total, want)
	}
}

// Sonnet 4.6 is 50% dearer than Sonnet 5; collapsing them understates by a third.
func TestCostSonnetGenerationsDiffer(t *testing.T) {
	c := calc(t)
	five := c.Cost(Request{Model: "claude-sonnet-5", Tokens: oneMillionEach()})
	fourSix := c.Cost(Request{Model: "claude-sonnet-4-6", Tokens: oneMillionEach()})
	if fourSix.Total <= five.Total {
		t.Fatalf("sonnet-4.6 (%v) should cost more than sonnet-5 (%v)", fourSix.Total, five.Total)
	}
	closeTo(t, "ratio", fourSix.Total/five.Total, 1.5)
}

// Cache reads dominate real usage, and this pair differs 4x on that column
// alone while matching everywhere else.
func TestCostFableGenerationsDifferOnlyOnCacheRead(t *testing.T) {
	c := calc(t)
	readOnly := Tokens{CacheRead: 1e6}
	five := c.Cost(Request{Model: "claude-fable-5", Tokens: readOnly})
	fiveOne := c.Cost(Request{Model: "claude-fable-5-1", Tokens: readOnly})
	closeTo(t, "fable-5 cache read", five.Total, 1.00)
	closeTo(t, "fable-5.1 cache read", fiveOne.Total, 0.25)
	closeTo(t, "ratio", five.Total/fiveOne.Total, 4)

	noRead := Tokens{Input: 1e6, Output: 1e6, CacheWrite5m: 1e6, CacheWrite1h: 1e6}
	a := c.Cost(Request{Model: "claude-fable-5", Tokens: noRead})
	b := c.Cost(Request{Model: "claude-fable-5-1", Tokens: noRead})
	closeTo(t, "identical outside cache read", a.Total, b.Total)
}

func TestCostFastMode(t *testing.T) {
	c := calc(t)
	std := c.Cost(Request{Model: "claude-opus-5", Tokens: oneMillionEach()})
	fast := c.Cost(Request{Model: "claude-opus-5", Speed: SpeedFast, Tokens: oneMillionEach()})
	if !fast.Fast {
		t.Fatal("fast row not applied")
	}
	closeTo(t, "fast total", fast.Total, 93.50)
	closeTo(t, "fast ratio", fast.Total/std.Total, 2)
	// Cache columns stack on the fast base rather than staying at standard.
	closeTo(t, "fast cache read", fast.CacheRead, 1.00)
	closeTo(t, "fast cache write 1h", fast.CacheWrite1h, 20)
}

func TestCostFastOnStandardSpeedModelIsNotDoubled(t *testing.T) {
	c := calc(t)
	std := c.Cost(Request{Model: "claude-opus-4-6", Tokens: oneMillionEach()})
	fast := c.Cost(Request{Model: "claude-opus-4-6", Speed: SpeedFast, Tokens: oneMillionEach()})
	closeTo(t, "opus-4-6 fast == standard", fast.Total, std.Total)
	if fast.Fast {
		t.Error("opus-4-6 must not report fast billing")
	}
}

func TestCostFastOnErrorModelWarns(t *testing.T) {
	c := calc(t)
	std := c.Cost(Request{Model: "claude-opus-4-7", Tokens: oneMillionEach()})
	fast := c.Cost(Request{Model: "claude-opus-4-7", Speed: SpeedFast, Tokens: oneMillionEach()})
	closeTo(t, "opus-4-7 fast billed at standard", fast.Total, std.Total)
	if !hasWarning(c, WarnFastAnomaly) {
		t.Error("a fast record on opus-4-7 should warn")
	}
}

func TestCostInferenceGeoUS(t *testing.T) {
	c := calc(t)
	base := c.Cost(Request{Model: "claude-opus-5", Tokens: oneMillionEach()})
	us := c.Cost(Request{Model: "claude-opus-5", InferenceGeo: GeoUS, Tokens: oneMillionEach()})
	closeTo(t, "us multiplier", us.Total/base.Total, 1.1)
	// Every token column is scaled, cache included.
	closeTo(t, "input", us.Input, 5*1.1)
	closeTo(t, "output", us.Output, 25*1.1)
	closeTo(t, "cache write 5m", us.CacheWrite5m, 6.25*1.1)
	closeTo(t, "cache write 1h", us.CacheWrite1h, 10*1.1)
	closeTo(t, "cache read", us.CacheRead, 0.50*1.1)
}

func TestCostNeutralGeoValues(t *testing.T) {
	c := calc(t)
	base := c.Cost(Request{Model: "claude-opus-5", Tokens: oneMillionEach()})
	for _, geo := range []string{"", "global", "not_available"} {
		got := c.Cost(Request{Model: "claude-opus-5", InferenceGeo: geo, Tokens: oneMillionEach()})
		closeTo(t, "geo "+geo, got.Total, base.Total)
	}
	if hasWarning(c, WarnUnknownGeo) {
		t.Error("documented neutral geo values should not warn")
	}
}

func TestCostUnknownGeoWarnsWithoutGuessing(t *testing.T) {
	c := calc(t)
	base := c.Cost(Request{Model: "claude-opus-5", Tokens: oneMillionEach()})
	got := c.Cost(Request{Model: "claude-opus-5", InferenceGeo: "mars", Tokens: oneMillionEach()})
	closeTo(t, "unknown geo unchanged", got.Total, base.Total)
	if !hasWarning(c, WarnUnknownGeo) {
		t.Error("an unrecognised geo should warn")
	}
}

func TestCostBatchTier(t *testing.T) {
	c := calc(t)
	base := c.Cost(Request{Model: "claude-opus-5", Tokens: oneMillionEach()})
	batch := c.Cost(Request{Model: "claude-opus-5", ServiceTier: TierBatch, Tokens: oneMillionEach()})
	closeTo(t, "batch halves", batch.Total, base.Total/2)
}

func TestCostUnknownTierIsNotDiscounted(t *testing.T) {
	c := calc(t)
	base := c.Cost(Request{Model: "claude-opus-5", Tokens: oneMillionEach()})
	got := c.Cost(Request{Model: "claude-opus-5", ServiceTier: "priority", Tokens: oneMillionEach()})
	closeTo(t, "priority billed at standard", got.Total, base.Total)
	if !hasWarning(c, WarnUnknownTier) {
		t.Error("an unrecognised tier should warn")
	}
}

func TestCostFastWithBatchWarns(t *testing.T) {
	c := calc(t)
	got := c.Cost(Request{Model: "claude-opus-5", Speed: SpeedFast,
		ServiceTier: TierBatch, Tokens: oneMillionEach()})
	// The batch discount is not applied on top of an explicit fast rate row.
	closeTo(t, "fast retained", got.Total, 93.50)
	if !hasWarning(c, WarnFastWithBatch) {
		t.Error("fast combined with batch should warn")
	}
}

func TestCostWebSearchAndFetch(t *testing.T) {
	c := calc(t)
	got := c.Cost(Request{Model: "claude-opus-5", Tokens: Tokens{WebSearches: 250}})
	closeTo(t, "250 searches", got.WebSearch, 2.50)
	closeTo(t, "total", got.Total, 2.50)

	// Web fetch carries no per-request charge and is not even in Tokens.
	none := c.Cost(Request{Model: "claude-opus-5", Tokens: Tokens{}})
	closeTo(t, "no searches", none.WebSearch, 0)
}

// Server tool charges are per request, so the regional multiplier must not
// scale them.
func TestCostWebSearchIgnoresMultipliers(t *testing.T) {
	c := calc(t)
	got := c.Cost(Request{Model: "claude-opus-5", InferenceGeo: GeoUS,
		Tokens: Tokens{WebSearches: 100}})
	closeTo(t, "web search unscaled", got.WebSearch, 1.00)
}

func TestCostUnknownModelCountsTokensNotDollars(t *testing.T) {
	c := calc(t)
	got := c.Cost(Request{Model: "claude-imaginary-9", Tokens: oneMillionEach()})
	if got.Priced {
		t.Error("an unknown model must not be reported as priced")
	}
	closeTo(t, "total", got.Total, 0)
	if ms := c.UnknownModels(); len(ms) != 1 || ms[0] != "claude-imaginary-9" {
		t.Errorf("UnknownModels() = %v", ms)
	}
	w := c.Warnings()
	if len(w) != 1 || w[0].Kind != WarnUnknownModel || w[0].Count != 1 {
		t.Fatalf("warnings = %+v", w)
	}
	if w[0].Detail == "" {
		t.Error("an unknown-model warning must explain how to fix it")
	}
}

func TestWarningsAggregateAndSort(t *testing.T) {
	c := calc(t)
	for i := 0; i < 3; i++ {
		c.Cost(Request{Model: "zzz-unknown"})
		c.Cost(Request{Model: "aaa-unknown"})
	}
	w := c.Warnings()
	if len(w) != 2 {
		t.Fatalf("want 2 aggregated warnings, got %d", len(w))
	}
	if w[0].Subject != "aaa-unknown" || w[1].Subject != "zzz-unknown" {
		t.Errorf("warnings not sorted: %+v", w)
	}
	for _, x := range w {
		if x.Count != 3 {
			t.Errorf("%s count = %d, want 3", x.Subject, x.Count)
		}
	}
}

func TestCostZeroTokens(t *testing.T) {
	c := calc(t)
	got := c.Cost(Request{Model: "claude-opus-5"})
	if !got.Priced {
		t.Error("a zero-token request against a known model is still priced")
	}
	closeTo(t, "total", got.Total, 0)
}

// Display hints must never reach the arithmetic.
func TestCostIgnoresDisplayHints(t *testing.T) {
	c := calc(t)
	got := c.Cost(Request{Model: "claude-opus-4-1", Tokens: Tokens{Input: 1e6}})
	closeTo(t, "retired model still priced", got.Input, 15)
	if !got.Priced {
		t.Error("retired models are still priced")
	}
}

func hasWarning(c *Calculator, kind string) bool {
	for _, w := range c.Warnings() {
		if w.Kind == kind {
			return true
		}
	}
	return false
}
