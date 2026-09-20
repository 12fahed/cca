package water

import (
	"math"
	"strings"
	"testing"
)

func TestForComputesFlatRate(t *testing.T) {
	tests := []struct {
		tokens  int64
		mlPer1k float64
		wantML  float64
	}{
		{0, 0.30, 0},
		{1_000, 0.30, 0.30},
		{1_000_000, 0.30, 300},
		{1_000_000, 0.60, 600},
		{1_318_089_194, 0.30, 395426.7582},
		{10_000_000_000, 0.30, 3_000_000},
	}
	for _, tt := range tests {
		got := For(tt.tokens, tt.mlPer1k)
		if math.Abs(got.Millilitres-tt.wantML) > 1e-6 {
			t.Errorf("For(%d, %v) = %v mL, want %v", tt.tokens, tt.mlPer1k, got.Millilitres, tt.wantML)
		}
		if math.Abs(got.Litres-tt.wantML/1000) > 1e-9 {
			t.Errorf("litres out of step with millilitres: %+v", got)
		}
	}
}

// A flat rate across every model and token class is the honest choice given how
// soft the underlying figure is; anything finer would be false precision.
func TestForScalesLinearly(t *testing.T) {
	single := For(1_000_000, DefaultMLPer1kTokens)
	double := For(2_000_000, DefaultMLPer1kTokens)
	if math.Abs(double.Millilitres-2*single.Millilitres) > 1e-9 {
		t.Errorf("not linear: %v vs %v", double.Millilitres, single.Millilitres)
	}
}

func TestForCarriesItsAssumption(t *testing.T) {
	e := For(1_000_000, DefaultMLPer1kTokens)
	if e.MLPer1k != DefaultMLPer1kTokens {
		t.Errorf("MLPer1k = %v", e.MLPer1k)
	}
	a := e.Assumption()
	if !strings.Contains(a, "0.30") {
		t.Errorf("assumption must state the rate, got %q", a)
	}
	// The figure is never to be shown as if it were measured.
	if !strings.Contains(a, "rough estimate") {
		t.Errorf("assumption must flag itself as rough, got %q", a)
	}
}

func TestForRejectsNegativeRate(t *testing.T) {
	e := For(1_000_000, -5)
	if e.Millilitres != 0 {
		t.Errorf("a negative rate should yield no water, got %v", e.Millilitres)
	}
}

func TestEquivalenceLadder(t *testing.T) {
	tests := []struct {
		litres float64
		want   string
	}{
		{0, ""},
		{0.1, "less than a cup of coffee"},
		{0.25, "about one cup of coffee"},
		{0.4, "about two cups of coffee"}, // 0.4/0.25 = 1.6 -> 2
		{0.5, "about one bottle of water"},
		{4, "about eight bottles of water"},
		{9, "about one minute of shower"},
		{45, "about five minutes of shower"},
		{50, "about one load of laundry"},
		{128.7, "about three loads of laundry"},
		{150, "about one bathtub"},
		{900, "about six bathtubs"},
		{50000, "about one swimming pool"},
		{400000, "about eight swimming pools"},
	}
	for _, tt := range tests {
		got := equivalence(tt.litres)
		want := spellOut(tt.want)
		if got != want {
			t.Errorf("equivalence(%v) = %q, want %q", tt.litres, got, want)
		}
	}
}

// The reported unit should always be the largest that the volume covers at
// least once, so the count stays small and readable.
func TestEquivalencePicksLargestFittingUnit(t *testing.T) {
	for _, u := range ladder {
		got := equivalence(u.litres)
		if !strings.Contains(got, u.singular) {
			t.Errorf("at exactly %v L the unit should be %q, got %q", u.litres, u.singular, got)
		}
	}
}

func TestVolumeFormatting(t *testing.T) {
	tests := []struct {
		tokens  int64
		mlPer1k float64
		want    string
	}{
		{0, 0.30, "0 mL"},
		{1_000, 0.30, "under 1 mL"},
		{10_000, 0.30, "3 mL"},
		{10_000_000, 0.30, "3.0 L"},
		{1_318_089_194, 0.30, "395.4 L"},
		{10_000_000_000, 0.30, "3.0 kL"},
	}
	for _, tt := range tests {
		if got := For(tt.tokens, tt.mlPer1k).Volume(); got != tt.want {
			t.Errorf("For(%d).Volume() = %q, want %q", tt.tokens, got, tt.want)
		}
	}
}

// The tool reports; it does not lecture. Equivalences stay lighthearted and
// carry no judgement.
func TestEquivalencesDoNotMoralize(t *testing.T) {
	forbidden := []string{"waste", "wasted", "guilt", "should", "shame",
		"harm", "damage", "consider", "reduce", "too much", "excessive"}
	for _, u := range ladder {
		for _, phrase := range []string{u.singular, u.plural} {
			for _, bad := range forbidden {
				if strings.Contains(strings.ToLower(phrase), bad) {
					t.Errorf("equivalence %q contains judgemental wording %q", phrase, bad)
				}
			}
		}
	}
}

// spellOut lets the table above read naturally while the implementation emits
// digits.
func spellOut(s string) string {
	words := map[string]string{
		"one": "1", "two": "2", "three": "3", "five": "5",
		"six": "6", "eight": "8",
	}
	for word, digit := range words {
		if strings.HasPrefix(s, "about "+word+" ") && word != "one" {
			return strings.Replace(s, "about "+word+" ", "about "+digit+" ", 1)
		}
	}
	return s
}
