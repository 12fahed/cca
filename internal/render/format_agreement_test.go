package render

import (
	"encoding/csv"
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"cca/internal/report"
)

// columnGap splits a rendered row on the run of spaces tabwriter inserts.
var columnGap = regexp.MustCompile(` {2,}`)

// tableRows parses a rendered table back into cells, dropping the header, the
// blank separator, the totals rule, and the footnotes.
func tableRows(out string) [][]string {
	var rows [][]string
	for _, line := range strings.Split(out, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "─") || strings.HasPrefix(t, "-") ||
			strings.HasPrefix(t, "Claude Code") {
			continue
		}
		rows = append(rows, columnGap.Split(t, -1))
	}
	return rows
}

func docFor(t *testing.T, rep *report.Report, view string) Document {
	t.Helper()
	var b strings.Builder
	if err := JSON(&b, rep, Options{View: view}); err != nil {
		t.Fatal(err)
	}
	var doc Document
	if err := json.Unmarshal([]byte(b.String()), &doc); err != nil {
		t.Fatalf("emitted JSON does not parse: %v", err)
	}
	return doc
}

// The definition-of-done item: the table and --json must report the same
// numbers. The table abbreviates and rounds, so agreement means every rendered
// cell is exactly the formatting of the corresponding exact figure, which is
// what a reader comparing the two would check.
func TestJSONAgreesWithTables(t *testing.T) {
	rep := viewReport(t, report.Options{})

	t.Run("models", func(t *testing.T) {
		doc := docFor(t, rep, ViewModels)
		rows := tableRows(renderTo(t, Models, rep))
		if len(rows) < len(doc.Models)+1 {
			t.Fatalf("parsed %d rows for %d models", len(rows), len(doc.Models))
		}
		for i, m := range doc.Models {
			row := rows[i+1] // row 0 is the header
			if row[0] != m.Key {
				t.Fatalf("row %d is %q, want %q", i, row[0], m.Key)
			}
			assertCell(t, m.Key+" requests", row[1], Count(m.Requests))
			assertCell(t, m.Key+" tokens", row[2], Tokens(m.Tokens.Total))
			assertCell(t, m.Key+" cost", row[3], USD(m.Cost.Total))
		}
		total := rows[len(rows)-1]
		assertCell(t, "total requests", total[1], Count(doc.Totals.Requests))
		assertCell(t, "total tokens", total[2], Tokens(doc.Totals.Tokens.Total))
		assertCell(t, "total cost", total[3], USD(doc.Totals.Cost.Total))
	})

	t.Run("daily", func(t *testing.T) {
		doc := docFor(t, rep, ViewDaily)
		rows := tableRows(renderTo(t, Daily, rep))
		for i, d := range doc.Daily {
			row := rows[i+1]
			assertCell(t, d.Key+" date", row[0], d.Key)
			assertCell(t, d.Key+" requests", row[1], Count(d.Requests))
			assertCell(t, d.Key+" tokens", row[2], Tokens(d.Tokens.Total))
			assertCell(t, d.Key+" cost", row[3], USD(d.Cost.Total))
		}
	})

	t.Run("summary", func(t *testing.T) {
		doc := docFor(t, rep, ViewSummary)
		out := render(t, rep, false)
		// The summary prints the overall cost and each token class.
		if !strings.Contains(out, USD(doc.Totals.Cost.Total)) {
			t.Errorf("summary does not show the JSON total %s", USD(doc.Totals.Cost.Total))
		}
		for name, v := range map[string]int64{
			"input": doc.Totals.Tokens.Input, "output": doc.Totals.Tokens.Output,
			"cache 5m":   doc.Totals.Tokens.CacheWrite5m,
			"cache 1h":   doc.Totals.Tokens.CacheWrite1h,
			"cache read": doc.Totals.Tokens.CacheRead,
		} {
			if !strings.Contains(out, Tokens(v)) {
				t.Errorf("summary is missing the %s figure %s", name, Tokens(v))
			}
		}
	})
}

func assertCell(t *testing.T, label, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: table shows %q, JSON formats to %q", label, got, want)
	}
}

// Groups must sum to the totals in the machine-readable output too, or a
// consumer aggregating rows would disagree with the total cca reports.
func TestJSONGroupsSumToTotals(t *testing.T) {
	rep := viewReport(t, report.Options{})
	doc := docFor(t, rep, ViewSummary)

	for name, groups := range map[string][]groupDoc{
		"models": doc.Models, "projects": doc.Projects, "daily": doc.Daily,
	} {
		var cost float64
		var tokens, requests int64
		for _, g := range groups {
			cost += g.Cost.Total
			tokens += g.Tokens.Total
			requests += g.Requests
		}
		if math.Abs(cost-doc.Totals.Cost.Total) > 1e-9 {
			t.Errorf("%s cost sums to %v, totals say %v", name, cost, doc.Totals.Cost.Total)
		}
		if tokens != doc.Totals.Tokens.Total {
			t.Errorf("%s tokens sum to %d, totals say %d", name, tokens, doc.Totals.Tokens.Total)
		}
		if requests != doc.Totals.Requests {
			t.Errorf("%s requests sum to %d, totals say %d", name, requests, doc.Totals.Requests)
		}
	}
}

// Token counts must be exact in JSON, never the abbreviated form the tables use.
func TestJSONCarriesExactCounts(t *testing.T) {
	rep := viewReport(t, report.Options{})
	doc := docFor(t, rep, ViewSummary)
	if doc.Totals.Tokens.Total != rep.Overall.Tokens.Total() {
		t.Errorf("JSON total = %d, report = %d", doc.Totals.Tokens.Total, rep.Overall.Tokens.Total())
	}
	// The table would have shown this abbreviated; the document must not.
	if Tokens(doc.Totals.Tokens.Total) == strconv.FormatInt(doc.Totals.Tokens.Total, 10) {
		t.Skip("this corpus is too small for abbreviation to differ")
	}
}

// Session identifiers are shortened for the tables but must stay whole in
// machine-readable output, which is where they are actually used to look
// something up.
func TestJSONKeepsFullSessionIDs(t *testing.T) {
	rep := viewReport(t, report.Options{})
	doc := docFor(t, rep, ViewSessions)
	for _, s := range doc.Sessions {
		if len(s.Key) < 36 {
			t.Errorf("session key %q was truncated", s.Key)
		}
	}
}

// The caveats travel with the numbers. A consumer reporting the cost figure
// onward needs to know it is not a bill.
func TestJSONCarriesTheCaveats(t *testing.T) {
	rep := viewReport(t, report.Options{})
	doc := docFor(t, rep, ViewSummary)
	joined := strings.Join(doc.Notes, " ")
	for _, want := range []string{"not what you were billed", "rough estimate"} {
		if !strings.Contains(joined, want) {
			t.Errorf("JSON notes are missing %q: %v", want, doc.Notes)
		}
	}
}

func TestJSONEmptyCollectionsAreNotNull(t *testing.T) {
	rep := build(t, nil, report.Options{})
	var b strings.Builder
	if err := JSON(&b, rep, Options{View: ViewSummary}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), ": null") && strings.Contains(b.String(), `"models": null`) {
		t.Errorf("empty collections should encode as [], not null:\n%s", b.String())
	}
}

func TestCSVParsesBackAndSums(t *testing.T) {
	rep := viewReport(t, report.Options{})
	doc := docFor(t, rep, ViewModels)

	var b strings.Builder
	if err := CSV(&b, rep, Options{View: ViewModels}); err != nil {
		t.Fatal(err)
	}
	records, err := csv.NewReader(strings.NewReader(b.String())).ReadAll()
	if err != nil {
		t.Fatalf("emitted CSV does not parse: %v", err)
	}
	if len(records) != len(doc.Models)+1 {
		t.Fatalf("got %d rows for %d models plus a header", len(records), len(doc.Models))
	}

	header := records[0]
	costCol := indexOf(header, "total_usd")
	tokenCol := indexOf(header, "total_tokens")
	if costCol < 0 || tokenCol < 0 {
		t.Fatalf("header is missing the total columns: %v", header)
	}

	var cost float64
	var tokens int64
	for _, row := range records[1:] {
		c, err := strconv.ParseFloat(row[costCol], 64)
		if err != nil {
			t.Fatalf("cost %q does not parse: %v", row[costCol], err)
		}
		n, err := strconv.ParseInt(row[tokenCol], 10, 64)
		if err != nil {
			t.Fatalf("tokens %q do not parse: %v", row[tokenCol], err)
		}
		cost += c
		tokens += n
	}
	// Costs are written at full precision precisely so that this holds.
	if math.Abs(cost-doc.Totals.Cost.Total) > 1e-9 {
		t.Errorf("CSV costs sum to %v, JSON total is %v", cost, doc.Totals.Cost.Total)
	}
	if tokens != doc.Totals.Tokens.Total {
		t.Errorf("CSV tokens sum to %d, JSON total is %d", tokens, doc.Totals.Tokens.Total)
	}
}

func TestCSVPerView(t *testing.T) {
	rep := viewReport(t, report.Options{})
	tests := map[string]string{
		ViewSummary:  "model",
		ViewModels:   "model",
		ViewProjects: "project",
		ViewDaily:    "date",
		ViewSessions: "session",
	}
	for view, wantFirst := range tests {
		t.Run(view, func(t *testing.T) {
			var b strings.Builder
			if err := CSV(&b, rep, Options{View: view}); err != nil {
				t.Fatal(err)
			}
			rows, err := csv.NewReader(strings.NewReader(b.String())).ReadAll()
			if err != nil {
				t.Fatalf("does not parse: %v", err)
			}
			if rows[0][0] != wantFirst {
				t.Errorf("first column = %q, want %q", rows[0][0], wantFirst)
			}
			// Every row must have as many fields as the header; encoding/csv
			// would already have rejected otherwise, which is the point.
			for i, r := range rows {
				if len(r) != len(rows[0]) {
					t.Errorf("row %d has %d fields, header has %d", i, len(r), len(rows[0]))
				}
			}
		})
	}
}

func TestCSVKeepsFullSessionIDs(t *testing.T) {
	rep := viewReport(t, report.Options{})
	var b strings.Builder
	if err := CSV(&b, rep, Options{View: ViewSessions}); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(strings.NewReader(b.String())).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows[1:] {
		if len(r[0]) < 36 {
			t.Errorf("session id %q was truncated", r[0])
		}
	}
}

// Machine-readable output must never carry styling.
func TestMachineFormatsAreNeverStyled(t *testing.T) {
	rep := viewReport(t, report.Options{})
	opts := Options{View: ViewModels, Color: Palette{enabled: true}}
	for name, f := range map[string]func(*strings.Builder) error{
		"json": func(b *strings.Builder) error { return JSON(b, rep, opts) },
		"csv":  func(b *strings.Builder) error { return CSV(b, rep, opts) },
	} {
		var b strings.Builder
		if err := f(&b); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(b.String(), "\x1b[") {
			t.Errorf("%s output contains ANSI escapes", name)
		}
	}
}

func indexOf(ss []string, want string) int {
	for i, s := range ss {
		if s == want {
			return i
		}
	}
	return -1
}
