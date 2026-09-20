// Package unit holds tests that exercise cca's packages through their exported
// APIs only, from outside the packages themselves.
//
// It deliberately does not hold every test. Go requires a test that touches
// unexported identifiers to live in the same directory as the code it tests,
// and cca's package-local suites do exactly that on purpose: the dedup key, the
// scanner buffer limit, the equivalence ladder, and the colour palette are
// internals whose edge cases matter and which should not be exported merely to
// relocate a file. What lives here instead is the seam between packages, which
// no package-local test can see: transcripts parsed, priced, aggregated, and
// rendered as one pipeline.
package unit

import (
	"encoding/csv"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"cca/internal/pricing"
	"cca/internal/render"
	"cca/internal/report"
	"cca/internal/transcript"
	"cca/internal/water"
)

// record renders one assistant line in the transcript format.
func record(session, model string, ts time.Time, in, out, cw5, cw1h, cr int64, sidechain bool) string {
	return `{"type":"assistant","uuid":"u-` + session + strconv.FormatInt(out, 10) +
		`","sessionId":"` + session + `","requestId":"r-` + session + strconv.FormatInt(out, 10) +
		`","timestamp":"` + ts.UTC().Format("2006-01-02T15:04:05.000Z") +
		`","cwd":"/work","isSidechain":` + strconv.FormatBool(sidechain) +
		`,"message":{"id":"m-` + session + strconv.FormatInt(out, 10) +
		`","model":"` + model + `","usage":{"input_tokens":` + strconv.FormatInt(in, 10) +
		`,"output_tokens":` + strconv.FormatInt(out, 10) +
		`,"cache_creation_input_tokens":` + strconv.FormatInt(cw5+cw1h, 10) +
		`,"cache_read_input_tokens":` + strconv.FormatInt(cr, 10) +
		`,"cache_creation":{"ephemeral_5m_input_tokens":` + strconv.FormatInt(cw5, 10) +
		`,"ephemeral_1h_input_tokens":` + strconv.FormatInt(cw1h, 10) + `}}}}`
}

// corpus writes a small but representative Claude directory: two projects, a
// duplicated message across files, a sub-agent transcript in the nested layout,
// and a malformed line.
func corpus(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	day := func(d int) time.Time { return time.Date(2026, 9, d, 12, 0, 0, 0, time.UTC) }

	shared := record("s1", "claude-opus-5", day(14), 1_000, 20_000, 5_000, 100_000, 400_000, false)
	files := map[string]string{
		filepath.Join("api", "s1.jsonl"): shared + "\n" +
			record("s1", "claude-opus-5", day(15), 500, 10_000, 0, 50_000, 200_000, false) + "\n" +
			`{"malformed` + "\n",
		// The same message replayed into a resumed session's file.
		filepath.Join("api", "s1-resumed.jsonl"): shared + "\n",
		filepath.Join("web", "s2.jsonl"):         record("s2", "claude-sonnet-5", day(16), 2_000, 30_000, 1_000, 20_000, 90_000, false) + "\n",
		// Sub-agent transcripts nest two levels deeper than top-level sessions.
		filepath.Join("web", "s2", "subagents", "agent-a.jsonl"): record("s3", "claude-haiku-4-5-20251001", day(16), 100, 4_000, 0, 2_000, 8_000, true) + "\n",
	}
	for rel, body := range files {
		path := filepath.Join(root, "projects", rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// pipeline runs the whole chain the way the command does.
func pipeline(t *testing.T, root string, opts report.Options) (*report.Report, *pricing.Table) {
	t.Helper()
	table, err := pricing.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	recs, stats, err := transcript.Load(transcript.Options{
		ClaudeDir: root, NonModels: table.NonModelSet(),
	})
	if err != nil {
		t.Fatal(err)
	}
	opts.Stats = stats
	if opts.Location == nil {
		opts.Location = time.UTC
	}
	return report.Build(recs, pricing.NewCalculator(table), opts), table
}

func TestPipelineDeduplicatesAcrossFiles(t *testing.T) {
	rep, _ := pipeline(t, corpus(t), report.Options{})

	// Four distinct messages were written; one of them twice.
	if rep.Overall.Requests != 4 {
		t.Errorf("requests = %d, want 4", rep.Overall.Requests)
	}
	if rep.Stats.Duplicates != 1 {
		t.Errorf("duplicates = %d, want 1", rep.Stats.Duplicates)
	}
	if rep.Stats.Skipped["malformed-json"] != 1 {
		t.Errorf("malformed = %d, want 1", rep.Stats.Skipped["malformed-json"])
	}
}

// The sub-agent transcript lives two directories below its project, which a
// non-recursive walk would miss along with every sidechain record.
func TestPipelineFindsNestedSubagentUsage(t *testing.T) {
	rep, _ := pipeline(t, corpus(t), report.Options{})
	if rep.Sidechain.Requests != 1 {
		t.Fatalf("sidechain requests = %d, want 1", rep.Sidechain.Requests)
	}
	var haiku bool
	for _, g := range rep.ByModel {
		if strings.HasPrefix(g.Key, "claude-haiku") {
			haiku = true
		}
	}
	if !haiku {
		t.Error("the sub-agent's model is missing from the breakdown")
	}
	// A sub-agent's records belong to the project that contains them.
	for _, g := range rep.ByProject {
		if g.Key == "web" && g.Requests != 2 {
			t.Errorf("project web has %d requests, want 2", g.Requests)
		}
	}
}

// Every grouping partitions the same records, so each must reconcile with the
// totals. This is the property most likely to break when packages change
// independently of one another.
func TestPipelineGroupingsReconcile(t *testing.T) {
	rep, _ := pipeline(t, corpus(t), report.Options{})
	for name, groups := range map[string][]report.Group{
		"model": rep.ByModel, "project": rep.ByProject,
		"day": rep.ByDay, "session": rep.BySession,
	} {
		var cost float64
		var tokens, requests int64
		for _, g := range groups {
			cost += g.Cost.Total
			tokens += g.Tokens.Total()
			requests += g.Requests
		}
		if math.Abs(cost-rep.Overall.Cost.Total) > 1e-9 {
			t.Errorf("%s cost = %v, totals = %v", name, cost, rep.Overall.Cost.Total)
		}
		if tokens != rep.Overall.Tokens.Total() || requests != rep.Overall.Requests {
			t.Errorf("%s tokens/requests = %d/%d, totals = %d/%d",
				name, tokens, requests, rep.Overall.Tokens.Total(), rep.Overall.Requests)
		}
	}
}

// The table, JSON, and CSV renderings of one report must agree, since they are
// three views of the same figures and a reader may compare them.
func TestPipelineFormatsAgree(t *testing.T) {
	rep, _ := pipeline(t, corpus(t), report.Options{})
	opts := render.Options{
		View:  render.ViewModels,
		Water: water.For(rep.Overall.Tokens.Total(), water.DefaultMLPer1kTokens),
	}

	var jsonOut, csvOut, tableOut strings.Builder
	for name, err := range map[string]error{
		"json":  render.JSON(&jsonOut, rep, opts),
		"csv":   render.CSV(&csvOut, rep, opts),
		"table": render.Models(&tableOut, rep, opts),
	} {
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}

	var doc struct {
		Totals struct {
			Cost   struct{ Total float64 } `json:"cost_usd"`
			Tokens struct{ Total int64 }   `json:"tokens"`
		} `json:"totals"`
		Models []struct {
			Key    string                  `json:"key"`
			Cost   struct{ Total float64 } `json:"cost_usd"`
			Tokens struct{ Total int64 }   `json:"tokens"`
		} `json:"models"`
	}
	if err := json.Unmarshal([]byte(jsonOut.String()), &doc); err != nil {
		t.Fatalf("JSON does not parse: %v", err)
	}

	// The table shows the same figures, formatted.
	for _, m := range doc.Models {
		if !strings.Contains(tableOut.String(), m.Key) {
			t.Errorf("table omits model %s", m.Key)
		}
	}
	if !strings.Contains(tableOut.String(), render.USD(doc.Totals.Cost.Total)) {
		t.Errorf("table does not show the total %s", render.USD(doc.Totals.Cost.Total))
	}

	// The CSV rows sum to the same total, which is why costs are written there
	// at full precision rather than rounded to cents.
	rows, err := csv.NewReader(strings.NewReader(csvOut.String())).ReadAll()
	if err != nil {
		t.Fatalf("CSV does not parse: %v", err)
	}
	var sum float64
	var tokens int64
	costCol, tokenCol := indexOf(rows[0], "total_usd"), indexOf(rows[0], "total_tokens")
	for _, r := range rows[1:] {
		v, err := strconv.ParseFloat(r[costCol], 64)
		if err != nil {
			t.Fatal(err)
		}
		n, err := strconv.ParseInt(r[tokenCol], 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		sum += v
		tokens += n
	}
	if math.Abs(sum-doc.Totals.Cost.Total) > 1e-9 {
		t.Errorf("CSV sums to %v, JSON total is %v", sum, doc.Totals.Cost.Total)
	}
	if tokens != doc.Totals.Tokens.Total {
		t.Errorf("CSV tokens sum to %d, JSON total is %d", tokens, doc.Totals.Tokens.Total)
	}
}

// The two accuracy caveats have to survive the whole pipeline, in the tables
// and in machine-readable output alike.
func TestPipelineKeepsTheCaveats(t *testing.T) {
	rep, _ := pipeline(t, corpus(t), report.Options{})
	opts := render.Options{
		View:  render.ViewSummary,
		Water: water.For(rep.Overall.Tokens.Total(), water.DefaultMLPer1kTokens),
	}

	var table, doc strings.Builder
	if err := render.Summary(&table, rep, opts); err != nil {
		t.Fatal(err)
	}
	if err := render.JSON(&doc, rep, opts); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"not what you were billed", "rough estimate"} {
		if !strings.Contains(table.String(), want) {
			t.Errorf("table output is missing %q", want)
		}
		if !strings.Contains(doc.String(), want) {
			t.Errorf("JSON output is missing %q", want)
		}
	}
}

// A window applied at the report layer must narrow every downstream view
// consistently.
func TestPipelineWindowNarrowsEveryView(t *testing.T) {
	root := corpus(t)
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	w, err := report.ParseWindow("2026-09-16", "", now)
	if err != nil {
		t.Fatal(err)
	}

	full, _ := pipeline(t, root, report.Options{})
	windowed, _ := pipeline(t, root, report.Options{Window: w})

	if windowed.Overall.Requests >= full.Overall.Requests {
		t.Fatalf("window did not narrow the report: %d vs %d",
			windowed.Overall.Requests, full.Overall.Requests)
	}
	if windowed.Filtered == 0 {
		t.Error("filtered count should record what the window excluded")
	}
	for _, g := range windowed.ByDay {
		if g.Key < "2026-09-16" {
			t.Errorf("day %s is outside the window", g.Key)
		}
	}
}

// An unknown model must reach the rendered output as a warning rather than
// silently vanishing from the cost column.
func TestPipelineSurfacesUnknownModels(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "projects", "p")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := record("s1", "claude-opus-5", time.Now(), 100, 1_000, 0, 0, 0, false) + "\n" +
		record("s1", "claude-nonexistent-9", time.Now(), 100, 2_000, 0, 0, 0, false) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "s1.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	rep, _ := pipeline(t, root, report.Options{})
	if len(rep.UnknownModels) != 1 || rep.UnknownModels[0] != "claude-nonexistent-9" {
		t.Fatalf("unknown models = %v", rep.UnknownModels)
	}
	// The tokens still count even though the dollars cannot.
	if rep.Overall.UnpricedTokens == 0 {
		t.Error("unpriced tokens should still be counted")
	}

	var out strings.Builder
	if err := render.Summary(&out, rep, render.Options{View: render.ViewSummary}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "claude-nonexistent-9") {
		t.Errorf("the unknown model should be named in the output:\n%s", out.String())
	}
}

// The placeholder model comes from the rate table rather than a literal, so the
// two have to stay wired together.
func TestPipelineDropsPlaceholderModels(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "projects", "p")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := record("s1", "claude-opus-5", time.Now(), 100, 1_000, 0, 0, 0, false) + "\n" +
		record("s1", "<synthetic>", time.Now(), 0, 0, 0, 0, 0, false) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "s1.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	rep, _ := pipeline(t, root, report.Options{})
	if len(rep.UnknownModels) != 0 {
		t.Errorf("a placeholder must not be reported as an unknown model: %v", rep.UnknownModels)
	}
	if rep.Stats.Skipped["non-model"] != 1 {
		t.Errorf("non-model skips = %d, want 1", rep.Stats.Skipped["non-model"])
	}
}

func TestPipelineHandlesMissingAndEmptyDirectories(t *testing.T) {
	table, err := pricing.Embedded()
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = transcript.Load(transcript.Options{ClaudeDir: filepath.Join(t.TempDir(), "absent")})
	if err == nil {
		t.Error("a missing directory should be reported")
	}

	empty := t.TempDir()
	if err := os.MkdirAll(filepath.Join(empty, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	recs, _, err := transcript.Load(transcript.Options{ClaudeDir: empty, NonModels: table.NonModelSet()})
	if err != nil {
		t.Fatalf("an empty projects directory is not an error: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("got %d records from an empty directory", len(recs))
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
