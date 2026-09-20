package pricing

import (
	"fmt"
	"sort"
)

// perMillion is the unit every rate in the table is quoted in.
const perMillion = 1_000_000.0

// Tokens is one request's billed counts, already split by rate class.
//
// Thinking tokens are deliberately absent: they are billed as output tokens and
// are already inside Output. Adding them would double-count.
type Tokens struct {
	Input        int64
	Output       int64
	CacheWrite5m int64
	CacheWrite1h int64
	CacheRead    int64
	WebSearches  int64
}

// Request is everything cost depends on, as plain data.
type Request struct {
	Model        string
	Speed        string
	ServiceTier  string
	InferenceGeo string
	Tokens       Tokens
}

// Cost is a priced request, broken down so a caller can show where money went.
type Cost struct {
	Input        float64
	Output       float64
	CacheWrite5m float64
	CacheWrite1h float64
	CacheRead    float64
	WebSearch    float64
	Total        float64

	// Priced is false when the model had no rate row. The tokens are still
	// real and still counted; only the dollars are unknown.
	Priced bool
	// Fast reports whether the premium rate row was applied.
	Fast bool
	// Multiplier is the combined geo and tier adjustment applied to the token
	// columns. Web search is a per-request charge and is not scaled by it.
	Multiplier float64
}

// Warning is an aggregated advisory, counted rather than repeated per record.
type Warning struct {
	Kind    string
	Subject string
	Count   int64
	Detail  string
}

// Warning kinds.
const (
	WarnUnknownModel  = "unknown-model"
	WarnUnknownGeo    = "unrecognized-inference-geo"
	WarnUnknownTier   = "unrecognized-service-tier"
	WarnFastAnomaly   = "fast-mode-anomaly"
	WarnFastWithBatch = "fast-mode-with-batch-tier"
)

// Calculator prices requests against a table, accumulating warnings so they can
// be reported once with a count instead of per record.
type Calculator struct {
	table *Table
	warn  map[string]map[string]int64
}

func NewCalculator(t *Table) *Calculator {
	return &Calculator{table: t, warn: make(map[string]map[string]int64)}
}

func (c *Calculator) Table() *Table { return c.table }

func (c *Calculator) note(kind, subject string) {
	if c.warn[kind] == nil {
		c.warn[kind] = make(map[string]int64)
	}
	c.warn[kind][subject]++
}

// Cost prices one request, following the documented pipeline: resolve the model,
// pick the rate row, multiply the five token columns, apply the geo and tier
// modifiers, then add per-request server tool charges.
func (c *Calculator) Cost(r Request) Cost {
	model, ok := c.table.Resolve(r.Model)
	if !ok {
		// Tokens still count; dollars do not. Guessing a rate would be worse
		// than reporting a gap, so the model is named in a warning instead.
		c.note(WarnUnknownModel, Normalize(r.Model))
		return Cost{Multiplier: 1}
	}

	choice := c.table.RatesFor(model, r.Speed)
	if choice.Anomaly != "" {
		c.note(WarnFastAnomaly, model.ID+": "+choice.Anomaly)
	}

	rates := choice.Rates
	cost := Cost{
		Input:        rate(r.Tokens.Input, rates.Input),
		Output:       rate(r.Tokens.Output, rates.Output),
		CacheWrite5m: rate(r.Tokens.CacheWrite5m, rates.CacheWrite5m),
		CacheWrite1h: rate(r.Tokens.CacheWrite1h, rates.CacheWrite1h),
		CacheRead:    rate(r.Tokens.CacheRead, rates.CacheRead),
		Priced:       true,
		Fast:         choice.Fast,
	}

	mult := c.multiplier(r, choice.Fast)
	cost.Multiplier = mult
	if mult != 1 {
		cost.Input *= mult
		cost.Output *= mult
		cost.CacheWrite5m *= mult
		cost.CacheWrite1h *= mult
		cost.CacheRead *= mult
	}

	// Server tool charges are per request, not per token, so they sit outside
	// the multiplier.
	cost.WebSearch = float64(r.Tokens.WebSearches) * c.table.ServerTools.WebSearch.USDPerRequest

	cost.Total = cost.Input + cost.Output + cost.CacheWrite5m +
		cost.CacheWrite1h + cost.CacheRead + cost.WebSearch
	return cost
}

// multiplier combines the inference-geo and service-tier adjustments.
func (c *Calculator) multiplier(r Request, fast bool) float64 {
	mult := 1.0

	switch geo := r.InferenceGeo; geo {
	case "", "global", "not_available":
		// No adjustment; these mean "not pinned" rather than a region.
	case GeoUS:
		mult *= c.table.Modifiers.InferenceGeoUS.Multiplier
	default:
		// An unrecognised region might carry its own multiplier, so warn rather
		// than assume parity.
		c.note(WarnUnknownGeo, geo)
	}

	switch tier := r.ServiceTier; tier {
	case "", TierStandard:
	case TierBatch:
		if fast {
			// The API refuses the combination, so a record carrying both is
			// anomalous. The explicit fast rate row is the more specific
			// signal, so the batch discount is not applied on top of it.
			c.note(WarnFastWithBatch, "")
		} else {
			mult *= c.table.Modifiers.Batch.Multiplier
		}
	default:
		// Priority tier costs more, not less; do not discount a tier we cannot
		// price.
		c.note(WarnUnknownTier, tier)
	}
	return mult
}

// Warnings returns the accumulated advisories, ordered for stable output.
func (c *Calculator) Warnings() []Warning {
	var out []Warning
	for kind, subjects := range c.warn {
		for subject, count := range subjects {
			out = append(out, Warning{
				Kind:    kind,
				Subject: subject,
				Count:   count,
				Detail:  detailFor(kind, subject),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Subject < out[j].Subject
	})
	return out
}

// UnknownModels lists model ids seen with no rate row, for the §4.3 warning.
func (c *Calculator) UnknownModels() []string {
	var out []string
	for id := range c.warn[WarnUnknownModel] {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func detailFor(kind, subject string) string {
	switch kind {
	case WarnUnknownModel:
		return fmt.Sprintf("no rate for %q; its tokens are counted but excluded "+
			"from the cost total. Add it to pricing.json to price it.", subject)
	case WarnUnknownGeo:
		return fmt.Sprintf("inference_geo %q is not in the rate table; "+
			"no regional multiplier applied.", subject)
	case WarnUnknownTier:
		return fmt.Sprintf("service_tier %q is not in the rate table; "+
			"billed at standard rates.", subject)
	case WarnFastAnomaly:
		return fmt.Sprintf("%s; billed at standard rates.", subject)
	case WarnFastWithBatch:
		return "records carry both fast mode and batch tier, which the API " +
			"does not allow; the batch discount was not applied."
	}
	return ""
}

func rate(tokens int64, usdPerMillion float64) float64 {
	return float64(tokens) / perMillion * usdPerMillion
}
