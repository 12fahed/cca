// Package water converts a token total into an estimated water volume and picks
// a real-world equivalence that suits the magnitude.
package water

import "fmt"

// DefaultMLPer1kTokens is the assumed water cost of a thousand tokens.
//
// It is a placeholder, not a measurement. There is no reliable public
// per-token water figure for any specific model: published estimates vary by
// more than an order of magnitude depending on datacenter location, cooling
// design, power mix, and whether the figure counts only on-site evaporation or
// also the water consumed generating the electricity. Anyone quoting a precise
// number is extrapolating, so cca prints its assumption alongside every result
// and lets the user substitute their own.
const DefaultMLPer1kTokens = 0.30

const (
	mlPerLitre    = 1000.0
	tokensPerUnit = 1000.0
)

// equivalent is a familiar volume used to make a litre figure legible.
//
// The figures are deliberately round household approximations, not precise
// measurements; the whole estimate is softer than any of them.
type equivalent struct {
	litres   float64
	singular string
	plural   string
}

// Ascending by volume. The largest unit that the estimate covers at least once
// is the one reported, which keeps the count small and readable.
var ladder = []equivalent{
	{0.25, "cup of coffee", "cups of coffee"},
	{0.5, "bottle of water", "bottles of water"},
	{9, "minute of shower", "minutes of shower"},
	{50, "load of laundry", "loads of laundry"},
	{150, "bathtub", "bathtubs"},
	{50000, "swimming pool", "swimming pools"},
}

// Estimate is a water figure with the assumption that produced it.
type Estimate struct {
	Tokens      int64
	MLPer1k     float64
	Millilitres float64
	Litres      float64
	// Equivalence is a lighthearted real-world comparison, or "" when the
	// volume is too small for even the smallest unit on the ladder.
	Equivalence string
}

// Estimate converts a token count to a water volume.
//
// The rate is flat across every model and token class. Given how soft the
// underlying figure is, scaling it per model or per token class would be false
// precision dressed up as rigour.
func For(tokens int64, mlPer1k float64) Estimate {
	if mlPer1k < 0 {
		mlPer1k = 0
	}
	ml := float64(tokens) / tokensPerUnit * mlPer1k
	e := Estimate{
		Tokens:      tokens,
		MLPer1k:     mlPer1k,
		Millilitres: ml,
		Litres:      ml / mlPerLitre,
	}
	e.Equivalence = equivalence(e.Litres)
	return e
}

// Assumption states the rate inline, so the figure is never shown without the
// caveat that produced it.
func (e Estimate) Assumption() string {
	return fmt.Sprintf("assumes %.2f mL / 1k tokens, a rough estimate", e.MLPer1k)
}

// Volume renders the figure in whichever unit reads naturally.
func (e Estimate) Volume() string {
	switch {
	case e.Litres >= 1000:
		return fmt.Sprintf("%.1f kL", e.Litres/1000)
	case e.Litres >= 1:
		return fmt.Sprintf("%.1f L", e.Litres)
	case e.Millilitres >= 1:
		return fmt.Sprintf("%.0f mL", e.Millilitres)
	case e.Millilitres > 0:
		return "under 1 mL"
	}
	return "0 mL"
}

func equivalence(litres float64) string {
	if litres <= 0 {
		return ""
	}
	for i := len(ladder) - 1; i >= 0; i-- {
		unit := ladder[i]
		if litres < unit.litres {
			continue
		}
		n := litres / unit.litres
		// Round to a whole count; the comparison is illustrative, so a decimal
		// would imply precision the estimate does not have.
		switch rounded := int(n + 0.5); {
		case rounded <= 1:
			return "about one " + unit.singular
		default:
			return fmt.Sprintf("about %d %s", rounded, unit.plural)
		}
	}
	return "less than a " + ladder[0].singular
}
