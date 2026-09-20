package render

import (
	"io"
	"strings"
	"testing"
	"time"

	"cca/internal/report"
	"cca/internal/transcript"
)

func viewRecords() []transcript.Record {
	return []transcript.Record{
		{Model: "claude-opus-5", Project: "api-server", SessionID: "aaaaaaaa-1111-2222-3333-444444444444",
			Timestamp: ts(14, 9), Usage: usage(4_000, 812_000, 180_000, 18_000_000, 140_000_000)},
		{Model: "claude-opus-5", Project: "api-server", SessionID: "aaaaaaaa-1111-2222-3333-444444444444",
			Timestamp: ts(15, 10), Usage: usage(12_000, 1_100_000, 90_000, 9_400_000, 158_000_000)},
		{Model: "claude-sonnet-5", Project: "a-very-long-slugified-project-directory-name-here",
			SessionID: "bbbbbbbb-5555-6666-7777-888888888888",
			Timestamp: ts(15, 14), Usage: usage(25_000, 930_000, 1_880_000, 20_100_000, 114_700_000)},
		{Model: "claude-haiku-4-5-20251001", Project: "api-server", SessionID: "cccccccc-9999-0000-1111-222222222222",
			Timestamp: ts(16, 11), IsSidechain: true, Usage: usage(200, 44_000, 2_000, 410_000, 2_800_000)},
	}
}

func viewReport(t *testing.T, opts report.Options) *report.Report {
	t.Helper()
	return build(t, viewRecords(), opts)
}

func TestModelsGolden(t *testing.T) {
	rep := viewReport(t, report.Options{})
	var b strings.Builder
	if err := Models(&b, rep, Options{}); err != nil {
		t.Fatal(err)
	}
	golden(t, "models", b.String())
}

func TestProjectsGolden(t *testing.T) {
	rep := viewReport(t, report.Options{})
	var b strings.Builder
	if err := Projects(&b, rep, Options{}); err != nil {
		t.Fatal(err)
	}
	golden(t, "projects", b.String())
}

func TestDailyGolden(t *testing.T) {
	rep := viewReport(t, report.Options{})
	var b strings.Builder
	if err := Daily(&b, rep, Options{}); err != nil {
		t.Fatal(err)
	}
	golden(t, "daily", b.String())
}

func TestSessionsGolden(t *testing.T) {
	rep := viewReport(t, report.Options{TopSessions: 2})
	var b strings.Builder
	if err := Sessions(&b, rep, Options{}); err != nil {
		t.Fatal(err)
	}
	golden(t, "sessions", b.String())
}

// Every view that prints dollars has to say what those dollars mean.
func TestViewsCarryTheCostCaveat(t *testing.T) {
	rep := viewReport(t, report.Options{})
	for name, f := range allViews() {
		var b strings.Builder
		if err := f(&b, rep, Options{}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(b.String(), "not what you were billed") {
			t.Errorf("%s view is missing the cost-basis caveat", name)
		}
	}
}

// Column totals must equal the report totals, or a reader adding up the rows
// would get a different answer from the one printed.
func TestViewTotalsMatchTheReport(t *testing.T) {
	rep := viewReport(t, report.Options{})
	want := USD(rep.Overall.Cost.Total)
	for name, out := range map[string]string{
		"models":   renderTo(t, Models, rep),
		"projects": renderTo(t, Projects, rep),
		"daily":    renderTo(t, Daily, rep),
	} {
		if !strings.Contains(out, want) {
			t.Errorf("%s view does not show the overall total %s:\n%s", name, want, out)
		}
	}
}

func TestViewsHandleEmptyReport(t *testing.T) {
	rep := build(t, nil, report.Options{})
	for name, f := range allViews() {
		var b strings.Builder
		if err := f(&b, rep, Options{}); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !strings.Contains(b.String(), "No usage found") {
			t.Errorf("%s view should say so plainly when there is nothing: %q", name, b.String())
		}
	}
}

func TestViewsHaveNoTrailingWhitespace(t *testing.T) {
	rep := viewReport(t, report.Options{})
	for name, out := range map[string]string{
		"models":   renderTo(t, Models, rep),
		"projects": renderTo(t, Projects, rep),
		"daily":    renderTo(t, Daily, rep),
		"sessions": renderTo(t, Sessions, rep),
	} {
		for i, line := range strings.Split(out, "\n") {
			if line != strings.TrimRight(line, " \t") {
				t.Errorf("%s line %d has trailing whitespace: %q", name, i+1, line)
			}
		}
	}
}

// Slugified project directories share a long prefix, so the tail is what tells
// them apart and must be what survives truncation.
func TestProjectsTruncateFromTheLeft(t *testing.T) {
	rep := viewReport(t, report.Options{})
	out := renderTo(t, Projects, rep)
	if !strings.Contains(out, "directory-name-here") {
		t.Errorf("the distinguishing tail of a long project name was lost:\n%s", out)
	}
	if strings.Contains(out, "a-very-long-slugified-project-directory-name-here") {
		t.Error("an over-long project name should have been shortened")
	}
	for _, line := range strings.Split(out, "\n") {
		if len([]rune(line)) > 80 {
			t.Errorf("line exceeds 80 columns: %q", line)
		}
	}
}

func TestSessionsShowShortIDs(t *testing.T) {
	rep := viewReport(t, report.Options{})
	out := renderTo(t, Sessions, rep)
	if !strings.Contains(out, "aaaaaaaa") {
		t.Errorf("session id prefix missing:\n%s", out)
	}
	if strings.Contains(out, "aaaaaaaa-1111-2222-3333-444444444444") {
		t.Error("the full uuid should be shortened for the table view")
	}
}

func TestSessionsRespectTopN(t *testing.T) {
	full := renderTo(t, Sessions, viewReport(t, report.Options{}))
	capped := renderTo(t, Sessions, viewReport(t, report.Options{TopSessions: 1}))
	if strings.Count(capped, "\n") >= strings.Count(full, "\n") {
		t.Error("--top should shorten the session table")
	}
}

func TestViewsASCII(t *testing.T) {
	rep := viewReport(t, report.Options{})
	var b strings.Builder
	if err := Projects(&b, rep, Options{ASCII: true}); err != nil {
		t.Fatal(err)
	}
	for _, glyph := range []string{"·", "…", "─", "≈"} {
		if strings.Contains(b.String(), glyph) {
			t.Errorf("ascii output still contains %q:\n%s", glyph, b.String())
		}
	}
}

func TestViewsShowWindowInHeader(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	w, err := report.ParseWindow("2026-09-15", "", now)
	if err != nil {
		t.Fatal(err)
	}
	out := renderTo(t, Daily, viewReport(t, report.Options{Window: w}))
	if !strings.Contains(out, "since 2026-09-15") {
		t.Errorf("view header should name the window:\n%s", out)
	}
}

// viewFunc matches every exported view in this package.
type viewFunc func(io.Writer, *report.Report, Options) error

func renderTo(t *testing.T, f viewFunc, rep *report.Report) string {
	t.Helper()
	var b strings.Builder
	if err := f(&b, rep, Options{}); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// allViews is every view that renders a cost column.
func allViews() map[string]viewFunc {
	return map[string]viewFunc{
		"models": Models, "projects": Projects, "daily": Daily, "sessions": Sessions,
	}
}
