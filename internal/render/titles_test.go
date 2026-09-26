package render

import (
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"

	"cca/internal/report"
	"cca/internal/transcript"
)

// titledReport builds a report whose sessions carry a deliberate mix: a chosen
// title, a generated one, a long one that must truncate, and one with none.
func titledReport(t *testing.T) *report.Report {
	t.Helper()
	const (
		s1 = "aaaaaaaa-1111-4000-8000-000000000001"
		s2 = "bbbbbbbb-2222-4000-8000-000000000002"
		s3 = "cccccccc-3333-4000-8000-000000000003"
		s4 = "dddddddd-4444-4000-8000-000000000004"
	)
	recs := []transcript.Record{
		{Model: "claude-opus-5", Project: "api", SessionID: s1, Timestamp: ts(14, 9),
			Usage: usage(1_000, 40_000, 0, 200_000, 900_000)},
		{Model: "claude-opus-5", Project: "api", SessionID: s2, Timestamp: ts(15, 9),
			Usage: usage(1_000, 30_000, 0, 150_000, 700_000)},
		{Model: "claude-sonnet-5", Project: "web", SessionID: s3, Timestamp: ts(16, 9),
			Usage: usage(1_000, 20_000, 0, 100_000, 500_000)},
		{Model: "claude-sonnet-5", Project: "web", SessionID: s4, Timestamp: ts(16, 10),
			Usage: usage(1_000, 10_000, 0, 50_000, 200_000)},
	}
	titles := map[string]transcript.SessionTitle{
		s1: {Text: "refactor the billing module", Source: transcript.TitleCustom},
		s2: {Text: "Dashboard layout and chart work", Source: transcript.TitleAI},
		s3: {
			Text:   "a deliberately long session title that will certainly need truncating somewhere",
			Source: transcript.TitleCustom,
		},
		// s4 has none.
	}
	return build(t, recs, report.Options{
		Stats: transcript.Stats{Titles: titles, Skipped: map[string]int64{}},
	})
}

func renderSessions(t *testing.T, rep *report.Report, opts Options) string {
	t.Helper()
	var b strings.Builder
	if err := Sessions(&b, rep, opts); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestSessionsWithTitlesGolden(t *testing.T) {
	got := renderSessions(t, titledReport(t), Options{ShowTitles: true})
	golden(t, "sessions_titled", got)
}

func TestSessionsVerboseTitlesGolden(t *testing.T) {
	got := renderSessions(t, titledReport(t), Options{ShowTitles: true, Verbose: true})
	golden(t, "sessions_titled_verbose", got)
}

func TestSessionsTitlesASCIIGolden(t *testing.T) {
	got := renderSessions(t, titledReport(t), Options{ShowTitles: true, ASCII: true})
	golden(t, "sessions_titled_ascii", got)
	for _, glyph := range []string{"\u2026", "\u2014", "\u00b7"} {
		if strings.Contains(got, glyph) {
			t.Errorf("--ascii output still contains %q", glyph)
		}
	}
}

// A session with no title gets a dash, never a blank cell and never an invented
// placeholder.
func TestSessionWithoutTitleRendersADash(t *testing.T) {
	got := renderSessions(t, titledReport(t), Options{ShowTitles: true})
	var found bool
	for _, line := range strings.Split(got, "\n") {
		if strings.Contains(line, "dddddddd") {
			found = true
			if !strings.Contains(line, "\u2014") {
				t.Errorf("untitled session should show a dash: %q", line)
			}
		}
	}
	if !found {
		t.Error("the untitled session is missing from the table")
	}
}

// The identifier stays present and copy-pasteable even when a title is shown.
func TestSessionsShowBothTitleAndShortID(t *testing.T) {
	got := renderSessions(t, titledReport(t), Options{ShowTitles: true})
	if !strings.Contains(got, "refactor the billing module") {
		t.Error("title missing")
	}
	if !strings.Contains(got, "aaaaaaaa") {
		t.Error("short session id missing")
	}
	if strings.Contains(got, "aaaaaaaa-1111-4000-8000-000000000001") {
		t.Error("the table should shorten the identifier")
	}
}

func TestSessionsTitleTruncates(t *testing.T) {
	got := renderSessions(t, titledReport(t), Options{ShowTitles: true})
	if strings.Contains(got, "truncating somewhere") {
		t.Error("an over-long title should have been cut")
	}
	if !strings.Contains(got, "\u2026") {
		t.Error("a truncated title should be marked with an ellipsis")
	}
}

// --no-titles restores the layout that existed before titles, project column
// and all.
func TestNoTitlesRestoresTheProjectColumn(t *testing.T) {
	got := renderSessions(t, titledReport(t), Options{ShowTitles: false})
	if strings.Contains(got, "refactor the billing module") {
		t.Error("titles should be absent")
	}
	if !strings.Contains(got, "project") {
		t.Error("the project column should return when titles are off")
	}
	if !strings.Contains(got, "requests") {
		t.Error("the requests column should return when titles are off")
	}
}

// --verbose distinguishes a name the user chose from one a model generated.
func TestVerboseReportsTheTitleSource(t *testing.T) {
	got := renderSessions(t, titledReport(t), Options{ShowTitles: true, Verbose: true})
	for _, want := range []string{"from", "custom", "ai"} {
		if !strings.Contains(got, want) {
			t.Errorf("verbose output missing %q:\n%s", want, got)
		}
	}
}

// The sessions view has to stay inside a standard terminal, which is why the
// project and request columns give way to the title.
func TestSessionsViewFitsEightyColumns(t *testing.T) {
	for _, opts := range []Options{
		{ShowTitles: true},
		{ShowTitles: true, Verbose: true},
		{ShowTitles: false},
	} {
		for i, line := range strings.Split(renderSessions(t, titledReport(t), opts), "\n") {
			if w := visibleWidth(line); w > 80 {
				t.Errorf("titles=%v verbose=%v line %d is %d columns: %q",
					opts.ShowTitles, opts.Verbose, i+1, w, line)
			}
		}
	}
}

// A title of double-width characters must not push the columns out.
func TestWideTitlesDoNotBreakAlignment(t *testing.T) {
	const s = "eeeeeeee-5555-4000-8000-000000000005"
	recs := []transcript.Record{
		{Model: "claude-opus-5", Project: "p", SessionID: s, Timestamp: ts(14, 9),
			Usage: usage(1_000, 10_000, 0, 50_000, 200_000)},
	}
	for name, title := range map[string]string{
		"cjk":   strings.Repeat("\u65e5\u672c\u8a9e", 12),
		"emoji": strings.Repeat("\U0001f680", 24),
		"mixed": "\u4e2d\u6587 mixed with ascii and \U0001f680 emoji, at some length",
	} {
		t.Run(name, func(t *testing.T) {
			rep := build(t, recs, report.Options{Stats: transcript.Stats{
				Titles:  map[string]transcript.SessionTitle{s: {Text: title, Source: transcript.TitleCustom}},
				Skipped: map[string]int64{},
			}})
			out := renderSessions(t, rep, Options{ShowTitles: true})

			var widths []int
			for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
				if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "\u2500") ||
					strings.HasPrefix(strings.TrimSpace(line), "Claude Code") {
					continue
				}
				widths = append(widths, visibleWidth(line))
			}
			if len(widths) < 2 {
				t.Fatalf("expected a header and a row, got %d lines", len(widths))
			}
			// Header and row must occupy the same columns.
			for _, w := range widths[1:] {
				if w != widths[0] {
					t.Errorf("row width %d differs from header width %d:\n%s", w, widths[0], out)
				}
			}
		})
	}
}

// A title describes what someone was working on, so machine output has to be
// asked for it rather than volunteering it.
func TestJSONOmitsTitlesUnlessAsked(t *testing.T) {
	rep := titledReport(t)

	var without, with strings.Builder
	if err := JSON(&without, rep, Options{View: ViewSessions}); err != nil {
		t.Fatal(err)
	}
	if err := JSON(&with, rep, Options{View: ViewSessions, ShowTitles: true}); err != nil {
		t.Fatal(err)
	}

	if strings.Contains(without.String(), "refactor the billing module") {
		t.Error("JSON leaked a title without --titles")
	}
	if strings.Contains(without.String(), "title_source") {
		t.Error("JSON leaked a title source without --titles")
	}
	if !strings.Contains(with.String(), "refactor the billing module") {
		t.Error("--titles did not include the title")
	}

	// The full title survives into JSON even though the table truncates it.
	var doc struct {
		Sessions []struct {
			Key         string `json:"key"`
			Title       string `json:"title"`
			TitleSource string `json:"title_source"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal([]byte(with.String()), &doc); err != nil {
		t.Fatal(err)
	}
	var sawLong bool
	for _, s := range doc.Sessions {
		if strings.HasSuffix(s.Title, "truncating somewhere") {
			sawLong = true
		}
		if len(s.Key) < 36 {
			t.Errorf("session id was shortened in JSON: %q", s.Key)
		}
		if s.Title != "" && s.TitleSource == "" {
			t.Errorf("a title without its source: %+v", s)
		}
	}
	if !sawLong {
		t.Error("JSON should carry the untruncated title")
	}
}

func TestCSVOmitsTitlesUnlessAsked(t *testing.T) {
	rep := titledReport(t)

	read := func(opts Options) [][]string {
		t.Helper()
		var b strings.Builder
		if err := CSV(&b, rep, opts); err != nil {
			t.Fatal(err)
		}
		rows, err := csv.NewReader(strings.NewReader(b.String())).ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}

	without := read(Options{View: ViewSessions})
	if indexOf(without[0], "title") >= 0 {
		t.Error("CSV leaked a title column without --titles")
	}

	with := read(Options{View: ViewSessions, ShowTitles: true})
	titleCol := indexOf(with[0], "title")
	sourceCol := indexOf(with[0], "title_source")
	if titleCol < 0 || sourceCol < 0 {
		t.Fatalf("--titles did not add the columns: %v", with[0])
	}
	var sawTitle bool
	for _, row := range with[1:] {
		if row[titleCol] != "" {
			sawTitle = true
			if row[sourceCol] == "" {
				t.Errorf("a title with no source: %v", row)
			}
		}
	}
	if !sawTitle {
		t.Error("no titles appeared in the CSV")
	}
	// Every row keeps the same field count, titles or not.
	for i, row := range with {
		if len(row) != len(with[0]) {
			t.Errorf("row %d has %d fields, header has %d", i, len(row), len(with[0]))
		}
	}
}

// Machine output is never styled, and a title must not change that.
func TestTitlesAreNeverStyledInMachineOutput(t *testing.T) {
	rep := titledReport(t)
	opts := Options{View: ViewSessions, ShowTitles: true, Color: Palette{enabled: true}}
	for name, render := range map[string]func(*strings.Builder) error{
		"json": func(b *strings.Builder) error { return JSON(b, rep, opts) },
		"csv":  func(b *strings.Builder) error { return CSV(b, rep, opts) },
	} {
		var b strings.Builder
		if err := render(&b); err != nil {
			t.Fatal(err)
		}
		if strings.ContainsRune(b.String(), 27) {
			t.Errorf("%s output contains an escape sequence", name)
		}
	}
}

// longProjectReport is the worst case for the untitled sessions layout: a
// slugified path long enough to push the row past eighty columns if uncapped.
func longProjectReport(t *testing.T) *report.Report {
	t.Helper()
	const s = "ffffffff-6666-4000-8000-000000000006"
	recs := []transcript.Record{{
		Model:     "claude-opus-5",
		Project:   "-home-someone-very-long-workspace-path-with-many-segments-project",
		SessionID: s,
		Timestamp: ts(14, 9),
		Usage:     usage(1_000, 40_000, 0, 200_000, 900_000),
	}}
	return build(t, recs, report.Options{Stats: transcript.Stats{
		Titles: map[string]transcript.SessionTitle{
			s: {Text: "a reasonably long session title here", Source: transcript.TitleCustom},
		},
		Skipped: map[string]int64{},
	}})
}
