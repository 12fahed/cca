package render

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cca/internal/pricing"
	"cca/internal/report"
	"cca/internal/transcript"
	"cca/internal/water"
)

var update = flag.Bool("update", false, "rewrite golden files")

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run: go test ./internal/render -update)", err)
	}
	if got != string(want) {
		t.Errorf("rendered output differs from %s\n--- got ---\n%s\n--- want ---\n%s",
			path, got, want)
	}
}

func ts(day, hour int) time.Time {
	return time.Date(2026, 9, day, hour, 0, 0, 0, time.UTC)
}

func usage(in, out, cw5, cw1h, cr int64) transcript.Usage {
	return transcript.Usage{Input: in, Output: out, CacheWrite5m: cw5,
		CacheWrite1h: cw1h, CacheRead: cr}
}

func build(t *testing.T, recs []transcript.Record, opts report.Options) *report.Report {
	t.Helper()
	tbl, err := pricing.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	opts.Location = time.UTC
	return report.Build(recs, pricing.NewCalculator(tbl), opts)
}

func render(t *testing.T, rep *report.Report, ascii bool) string {
	t.Helper()
	var b strings.Builder
	est := water.For(rep.Overall.Tokens.Total(), water.DefaultMLPer1kTokens)
	if err := Summary(&b, rep, Options{ASCII: ascii, Water: est}); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func typical() []transcript.Record {
	return []transcript.Record{
		{Model: "claude-opus-5", Project: "api-server", SessionID: "s1",
			Timestamp: ts(14, 9), Usage: usage(4_000, 812_000, 180_000, 18_000_000, 140_000_000)},
		{Model: "claude-opus-5", Project: "api-server", SessionID: "s1",
			Timestamp: ts(15, 10), Usage: usage(12_000, 1_100_000, 90_000, 9_400_000, 158_000_000)},
		{Model: "claude-sonnet-5", Project: "web-ui", SessionID: "s2",
			Timestamp: ts(15, 14), Usage: usage(25_000, 930_000, 1_880_000, 20_100_000, 114_700_000)},
		{Model: "claude-haiku-4-5-20251001", Project: "web-ui", SessionID: "s3",
			Timestamp: ts(16, 11), IsSidechain: true, Usage: usage(200, 44_000, 2_000, 410_000, 2_800_000)},
	}
}

func TestSummaryGolden(t *testing.T) {
	rep := build(t, typical(), report.Options{})
	golden(t, "summary", render(t, rep, false))
}

// The ASCII variant must carry the same information, only with plain glyphs.
func TestSummaryASCIIGolden(t *testing.T) {
	rep := build(t, typical(), report.Options{})
	got := render(t, rep, true)
	golden(t, "summary_ascii", got)
	for _, g := range []string{"·", "≈", "─"} {
		if strings.Contains(got, g) {
			t.Errorf("--ascii output still contains %q", g)
		}
	}
}

func TestSummaryWindowedGolden(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	w, err := report.ParseWindow("2026-09-15", "2026-09-16", now)
	if err != nil {
		t.Fatal(err)
	}
	rep := build(t, typical(), report.Options{Window: w})
	golden(t, "summary_windowed", render(t, rep, false))
}

// Every conditional footnote at once, to pin their wording and ordering.
func TestSummaryAllFootnotesGolden(t *testing.T) {
	recs := []transcript.Record{
		{Model: "claude-opus-5", Project: "p", SessionID: "s1", Timestamp: ts(14, 9),
			Usage: usage(1_000, 50_000, 10_000, 900_000, 4_000_000)},
		{Model: "claude-opus-5", Project: "p", SessionID: "s1", Timestamp: ts(14, 10),
			Speed: pricing.SpeedFast, Usage: usage(500, 20_000, 0, 100_000, 800_000)},
		{Model: "claude-opus-5", Project: "p", SessionID: "s1", Timestamp: ts(14, 11),
			InferenceGeo: pricing.GeoUS, Usage: usage(500, 10_000, 0, 50_000, 400_000)},
		{Model: "claude-sonnet-5", Project: "p", SessionID: "s2", Timestamp: ts(14, 12),
			ServiceTier: pricing.TierBatch, Usage: usage(1_000, 30_000, 0, 60_000, 500_000)},
		{Model: "claude-sonnet-5", Project: "p", SessionID: "s2", Timestamp: ts(14, 13),
			Usage: transcript.Usage{Input: 400, Output: 9_000, WebSearches: 12}},
		{Model: "claude-imaginary-9", Project: "p", SessionID: "s2", Timestamp: ts(14, 14),
			Usage: usage(1_000, 5_000, 0, 20_000, 100_000)},
		// No usable timestamp: counted in the totals but not by day.
		{Model: "claude-opus-5", Project: "p", SessionID: "s2",
			Usage: usage(100, 2_000, 0, 5_000, 30_000)},
	}
	rep := build(t, recs, report.Options{
		Stats: transcript.Stats{FlatCacheFallback: 3, Skipped: map[string]int64{}},
	})
	got := render(t, rep, false)
	golden(t, "summary_all_footnotes", got)

	for _, want := range []string{
		"claude-imaginary-9", "fast mode", "pinned inference to the US",
		"batch tier", "web search", "upper bound", "no usable timestamp",
		"5-minute writes",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("footnote block is missing %q", want)
		}
	}
}

func TestSummaryEmptyGolden(t *testing.T) {
	rep := build(t, nil, report.Options{})
	golden(t, "summary_empty", render(t, rep, false))
}

func TestSummaryAllFilteredGolden(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	w, _ := report.ParseWindow("2027-01-01", "", now)
	rep := build(t, typical(), report.Options{Window: w})
	golden(t, "summary_filtered", render(t, rep, false))
}

// The two accuracy caveats are a definition-of-done item, not decoration. They
// appear on every non-empty view regardless of what else is true of the run.
func TestSummaryAlwaysCarriesBothCaveats(t *testing.T) {
	cases := map[string]*report.Report{
		"typical":   build(t, typical(), report.Options{}),
		"one model": build(t, typical()[:1], report.Options{}),
	}
	for name, rep := range cases {
		for _, ascii := range []bool{false, true} {
			got := render(t, rep, ascii)
			if !strings.Contains(got, "not what you were billed") {
				t.Errorf("%s (ascii=%v): missing the cost-basis caveat", name, ascii)
			}
			if !strings.Contains(got, "rough estimate") {
				t.Errorf("%s (ascii=%v): missing the water caveat", name, ascii)
			}
		}
	}
}

// No line may carry trailing whitespace; it shows up in diffs and in terminals
// that highlight it.
func TestSummaryHasNoTrailingWhitespace(t *testing.T) {
	rep := build(t, typical(), report.Options{})
	for i, line := range strings.Split(render(t, rep, false), "\n") {
		if line != strings.TrimRight(line, " \t") {
			t.Errorf("line %d has trailing whitespace: %q", i+1, line)
		}
	}
}

// The default view is meant to fit on one screen.
func TestSummaryFitsOneScreen(t *testing.T) {
	rep := build(t, typical(), report.Options{})
	out := render(t, rep, false)
	if n := strings.Count(out, "\n"); n > 24 {
		t.Errorf("default view is %d lines, which no longer fits a small terminal", n)
	}
	for i, line := range strings.Split(out, "\n") {
		if len([]rune(line)) > 80 {
			t.Errorf("line %d is %d columns wide: %q", i+1, len([]rune(line)), line)
		}
	}
}

func TestDescribeWindow(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	tests := []struct{ since, until, want string }{
		{"", "", "all time"},
		{"2026-09-01", "", "since 2026-09-01"},
		// The stored end bound is exclusive, so the name given back is the last
		// day actually included.
		{"", "2026-09-14", "until 2026-09-14"},
		{"2026-09-01", "2026-09-14", "2026-09-01 to 2026-09-14"},
	}
	for _, tt := range tests {
		w, err := report.ParseWindow(tt.since, tt.until, now)
		if err != nil {
			t.Fatal(err)
		}
		if got := DescribeWindow(w); got != tt.want {
			t.Errorf("DescribeWindow(%q,%q) = %q, want %q", tt.since, tt.until, got, tt.want)
		}
	}
}

// The same footnotes with more than one subject, so plural wording is pinned
// alongside the singular forms the golden files capture.
func TestSummaryFootnotePluralAgreement(t *testing.T) {
	recs := []transcript.Record{
		{Model: "claude-ghost-1", Project: "p", SessionID: "s", Timestamp: ts(14, 9),
			Usage: usage(1_000, 5_000, 0, 10_000, 50_000)},
		{Model: "claude-phantom-2", Project: "p", SessionID: "s", Timestamp: ts(14, 10),
			Usage: usage(1_000, 5_000, 0, 10_000, 50_000)},
		{Model: "claude-opus-5", Project: "p", SessionID: "s",
			Usage: usage(100, 1_000, 0, 1_000, 5_000)},
		{Model: "claude-opus-5", Project: "p", SessionID: "s",
			Usage: usage(100, 1_000, 0, 1_000, 5_000)},
		{Model: "claude-opus-5", Project: "p", SessionID: "s", Timestamp: ts(14, 11),
			Usage: transcript.Usage{Input: 100, WebSearches: 1}},
	}
	rep := build(t, recs, report.Options{})
	got := render(t, rep, false)

	for _, want := range []string{
		"2 models had no rate and were excluded",
		"Add them to pricing.json to price them; see README.",
		"2 records had no usable timestamp and are counted",
		"Includes 1 web search.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}
