package pricing

import "testing"

func TestRatesForStandardSpeed(t *testing.T) {
	tbl, _ := Embedded()
	m, _ := tbl.Resolve("claude-opus-5")
	for _, speed := range []string{SpeedStandard, "", "STANDARD-ish"} {
		got := tbl.RatesFor(m, speed)
		if got.Fast {
			t.Errorf("speed %q should not select the fast row", speed)
		}
		if got.Rates != m.Rates {
			t.Errorf("speed %q rates = %+v, want %+v", speed, got.Rates, m.Rates)
		}
	}
}

func TestRatesForFastSupported(t *testing.T) {
	tbl, _ := Embedded()
	for _, id := range []string{"claude-opus-5", "claude-opus-4-8"} {
		m, _ := tbl.Resolve(id)
		got := tbl.RatesFor(m, SpeedFast)
		if !got.Fast || got.Anomaly != "" {
			t.Errorf("%s: fast=%v anomaly=%q", id, got.Fast, got.Anomaly)
		}
		// Double the base pair, with cache columns stacked on the fast base.
		if (got.Rates != Rates{10, 50, 12.50, 20, 1.00}) {
			t.Errorf("%s fast rates = %+v", id, got.Rates)
		}
		if got.Rates.Input != m.Rates.Input*2 || got.Rates.CacheRead != m.Rates.CacheRead*2 {
			t.Errorf("%s: fast should double both base and cache columns", id)
		}
	}
}

// Opus 4.6 accepts a fast request, serves it at standard speed, and bills at
// standard rates. Keying on (model, speed) alone would overcharge it 2x.
func TestRatesForFastOnStandardSpeedModel(t *testing.T) {
	tbl, _ := Embedded()
	m, _ := tbl.Resolve("claude-opus-4-6")
	got := tbl.RatesFor(m, SpeedFast)
	if got.Fast {
		t.Error("opus-4-6 must not bill at fast rates")
	}
	if got.Rates != m.Rates {
		t.Errorf("rates = %+v, want standard %+v", got.Rates, m.Rates)
	}
	if got.Anomaly != "" {
		t.Errorf("documented behaviour should not raise an anomaly, got %q", got.Anomaly)
	}
}

// Opus 4.7 rejects fast requests, so a fast record for it should not exist.
func TestRatesForFastOnErrorModel(t *testing.T) {
	tbl, _ := Embedded()
	m, _ := tbl.Resolve("claude-opus-4-7")
	got := tbl.RatesFor(m, SpeedFast)
	if got.Fast {
		t.Error("opus-4-7 has no fast tier")
	}
	if got.Anomaly != AnomalyFastOnErrorModel {
		t.Errorf("anomaly = %q, want %q", got.Anomaly, AnomalyFastOnErrorModel)
	}
	if got.Rates != m.Rates {
		t.Errorf("rates = %+v, want standard", got.Rates)
	}
}

func TestRatesForFastOnModelWithoutFastRow(t *testing.T) {
	tbl, _ := Embedded()
	m, _ := tbl.Resolve("claude-haiku-4-5")
	got := tbl.RatesFor(m, SpeedFast)
	if got.Fast || got.Anomaly != AnomalyFastOnUnknownModel {
		t.Errorf("fast=%v anomaly=%q", got.Fast, got.Anomaly)
	}
	if got.Rates != m.Rates {
		t.Errorf("rates = %+v, want standard", got.Rates)
	}
}

func TestRatesForNilModel(t *testing.T) {
	tbl, _ := Embedded()
	if got := tbl.RatesFor(nil, SpeedFast); got.Rates != (Rates{}) || got.Fast {
		t.Errorf("nil model = %+v, want a zero choice", got)
	}
}
