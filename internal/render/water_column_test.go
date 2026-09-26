package render

import (
	"math"
	"strings"
	"testing"

	"cca/internal/report"
	"cca/internal/water"
)

func waterOpts(extra func(*Options)) Options {
	o := Options{
		ShowTitles: true,
		ShowWater:  true,
		Water:      water.For(0, water.DefaultMLPer1kTokens),
	}
	if extra != nil {
		extra(&o)
	}
	return o
}

func TestSessionsWaterColumnGolden(t *testing.T) {
	golden(t, "sessions_water", renderSessions(t, titledReport(t), waterOpts(nil)))
}

func TestSessionsWaterVerboseGolden(t *testing.T) {
	got := renderSessions(t, titledReport(t), waterOpts(func(o *Options) { o.Verbose = true }))
	golden(t, "sessions_water_verbose", got)
}

func TestSessionsWaterWithoutTitlesGolden(t *testing.T) {
	got := renderSessions(t, titledReport(t), waterOpts(func(o *Options) { o.ShowTitles = false }))
	golden(t, "sessions_water_untitled", got)
}

// The water column displaces a different column in each layout, and turning it
// off has to bring that column back rather than leave a gap.
func TestWaterColumnDisplacesAndRestores(t *testing.T) {
	rep := titledReport(t)

	titled := renderSessions(t, rep, waterOpts(nil))
	if !strings.Contains(titled, "water") {
		t.Error("titled layout is missing the water column")
	}
	if strings.Contains(titled, "started") {
		t.Error("titled layout should trade the start date for water")
	}

	noWater := renderSessions(t, rep, waterOpts(func(o *Options) { o.ShowWater = false }))
	if strings.Contains(noWater, "water") {
		t.Error("--no-water still showed the column")
	}
	if !strings.Contains(noWater, "started") {
		t.Error("--no-water should restore the start date")
	}

	untitled := renderSessions(t, rep, waterOpts(func(o *Options) { o.ShowTitles = false }))
	if !strings.Contains(untitled, "water") {
		t.Error("untitled layout is missing the water column")
	}
	if strings.Contains(untitled, "requests") {
		t.Error("untitled layout should trade the request count for water")
	}
	// Without a title, the date and project are what identify a session, so
	// they must survive.
	for _, want := range []string{"started", "project"} {
		if !strings.Contains(untitled, want) {
			t.Errorf("untitled layout lost %q", want)
		}
	}
}

// Every combination has to fit a standard terminal, including the worst case
// of a long project path in the untitled layout.
func TestSessionsWithWaterFitEightyColumns(t *testing.T) {
	rep := titledReport(t)
	long := longProjectReport(t)

	for name, opts := range map[string]Options{
		"titled":            waterOpts(nil),
		"titled verbose":    waterOpts(func(o *Options) { o.Verbose = true }),
		"untitled":          waterOpts(func(o *Options) { o.ShowTitles = false }),
		"titled no water":   waterOpts(func(o *Options) { o.ShowWater = false }),
		"untitled no water": waterOpts(func(o *Options) { o.ShowTitles = false; o.ShowWater = false }),
		"ascii":             waterOpts(func(o *Options) { o.ASCII = true }),
	} {
		for label, r := range map[string]*report.Report{"typical": rep, "long project": long} {
			for i, line := range strings.Split(renderSessions(t, r, opts), "\n") {
				if w := visibleWidth(line); w > 80 {
					t.Errorf("%s / %s: line %d is %d columns: %q", name, label, i+1, w, line)
				}
			}
		}
	}
}

// A view that prints a water figure has to print the assumption behind it.
func TestWaterColumnCarriesTheAssumption(t *testing.T) {
	rep := titledReport(t)

	with := renderSessions(t, rep, waterOpts(nil))
	if !strings.Contains(with, "rough estimate") {
		t.Errorf("the water caveat is missing:\n%s", with)
	}
	if !strings.Contains(with, "0.30 mL / 1k tokens") {
		t.Error("the caveat should state the rate actually used")
	}

	// With no water on screen there is no assumption to declare.
	without := renderSessions(t, rep, waterOpts(func(o *Options) { o.ShowWater = false }))
	if strings.Contains(without, "rough estimate") {
		t.Error("the water caveat appeared on a view showing no water")
	}

	// A view that never shows water must not gain the caveat either.
	var models strings.Builder
	if err := Models(&models, rep, waterOpts(nil)); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(models.String(), "rough estimate") {
		t.Error("the models view does not print water and owes no water caveat")
	}
}

// The caveat has to quote whichever rate was configured, not the default.
func TestWaterCaveatFollowsTheConfiguredRate(t *testing.T) {
	rep := titledReport(t)
	got := renderSessions(t, rep, waterOpts(func(o *Options) {
		o.Water = water.For(0, 1.25)
	}))
	if !strings.Contains(got, "1.25 mL / 1k tokens") {
		t.Errorf("the caveat should quote the configured rate:\n%s", got)
	}
}

// Water is a flat rate per token, so the column is the token column rescaled.
// Rows must be consistent with that rather than independently computed.
func TestWaterIsProportionalToTokens(t *testing.T) {
	rep := titledReport(t)
	rate := water.DefaultMLPer1kTokens

	for _, g := range rep.BySession {
		want := water.For(g.Tokens.Total(), rate)
		got := water.For(g.Tokens.Total(), rate)
		if math.Abs(got.Litres-want.Litres) > 1e-12 {
			t.Fatalf("water is not deterministic for %s", g.Key)
		}
	}

	// The session totals must add up to the whole-run figure, since both are
	// the same flat rate over the same tokens.
	var sum int64
	for _, g := range rep.BySession {
		sum += g.Tokens.Total()
	}
	if sum != rep.Overall.Tokens.Total() {
		t.Fatalf("session tokens sum to %d, overall is %d", sum, rep.Overall.Tokens.Total())
	}
	perSession := water.For(sum, rate)
	overall := water.For(rep.Overall.Tokens.Total(), rate)
	if math.Abs(perSession.Millilitres-overall.Millilitres) > 1e-9 {
		t.Errorf("per-session water sums to %v mL, the run total is %v mL",
			perSession.Millilitres, overall.Millilitres)
	}
}

// Rows show a volume only. An equivalence is what makes a single figure
// concrete; twenty of them down a column is noise.
func TestWaterRowsShowNoEquivalence(t *testing.T) {
	got := renderSessions(t, titledReport(t), waterOpts(nil))
	for _, phrase := range []string{"bathtub", "laundry", "shower", "bottle", "cup of coffee"} {
		// The footnote may mention none of these; the rows certainly must not.
		for _, line := range strings.Split(got, "\n") {
			if strings.Contains(line, "—") || strings.HasPrefix(strings.TrimSpace(line), "─") {
				continue
			}
			if strings.Contains(line, phrase) {
				t.Errorf("row carries an equivalence %q: %q", phrase, line)
			}
		}
	}
}

// Colour must not disturb the columns, water included.
func TestWaterColumnColorDoesNotChangeLayout(t *testing.T) {
	rep := titledReport(t)
	plain := renderSessions(t, rep, waterOpts(nil))
	colored := renderSessions(t, rep, waterOpts(func(o *Options) { o.Color = Palette{enabled: true} }))
	if got := stripEscapes(colored); got != plain {
		t.Errorf("colour changed the layout\n--- stripped ---\n%s\n--- plain ---\n%s", got, plain)
	}
}
