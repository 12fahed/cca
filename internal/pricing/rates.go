package pricing

// Speed and service tier values as they appear in a transcript's usage object.
const (
	SpeedStandard = "standard"
	SpeedFast     = "fast"

	TierStandard = "standard"
	TierBatch    = "batch"

	GeoUS = "us"
)

// RateChoice is the outcome of picking a rate row for one request.
type RateChoice struct {
	Rates Rates
	// Fast reports whether the premium fast-mode row was actually applied,
	// which is not the same as the request having asked for it.
	Fast bool
	// Anomaly names a condition worth warning about, or "" when there is none.
	Anomaly string
}

// Anomalies that rate selection can detect.
const (
	AnomalyFastOnErrorModel   = "fast-on-unsupported-model"
	AnomalyFastOnUnknownModel = "fast-on-model-with-no-fast-row"
)

// RatesFor picks the rate row for a model at a given speed.
//
// Fast mode is not a uniform doubling, and both directions of the mistake cost
// money. Applying the fast row where it does not belong overcharges; failing to
// apply it where it does undercharges. The table partitions models three ways:
//
//   - supported: the fast row applies, with cache columns already stacked on
//     the fast base rather than left at standard
//   - runs at standard speed: the request is accepted, served at normal speed,
//     and billed at normal rates, so the fast row must not be used
//   - errors: the API rejects the request outright, so such a record should not
//     exist; bill it at standard and say so
func (t *Table) RatesFor(m *Model, speed string) RateChoice {
	if m == nil {
		return RateChoice{}
	}
	if speed != SpeedFast {
		return RateChoice{Rates: m.Rates}
	}

	switch t.fastKind[m.ID] {
	case fastSupported:
		if r, ok := m.Speeds[SpeedFast]; ok {
			return RateChoice{Rates: r, Fast: true}
		}
		// Listed as supported but carrying no fast row: the table is
		// inconsistent, so fall back rather than invent a multiplier.
		return RateChoice{Rates: m.Rates, Anomaly: AnomalyFastOnUnknownModel}

	case fastRunsStandard:
		// Documented behaviour, not an anomaly: the request runs at standard
		// speed and bills at standard rates.
		return RateChoice{Rates: m.Rates}

	case fastErrors:
		return RateChoice{Rates: m.Rates, Anomaly: AnomalyFastOnErrorModel}

	default:
		if r, ok := m.Speeds[SpeedFast]; ok {
			return RateChoice{Rates: r, Fast: true}
		}
		return RateChoice{Rates: m.Rates, Anomaly: AnomalyFastOnUnknownModel}
	}
}
