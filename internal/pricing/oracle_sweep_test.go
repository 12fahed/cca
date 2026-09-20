package pricing_test

import (
	"fmt"
	"math"
	"os"
	"sort"
	"testing"

	"cca/internal/pricing"
	"cca/internal/report"
	"cca/internal/transcript"
)

// oracleDirEnv points the sweep at a real Claude directory. It is unset in CI
// on purpose: a test that reads the machine's own transcripts would depend on
// data that changes underneath it.
const oracleDirEnv = "CCA_ORACLE_CLAUDE_DIR"

// delta is one session's comparison against Claude Code's own accounting.
//
// Token counts are compared before costs on purpose. A transcript is not a
// guaranteed-complete record of billed turns, so cca can legitimately see fewer
// tokens than Claude Code charged for; treating that as a pricing error would
// blame the arithmetic for a gap in the data. Only when the token counts agree
// does a cost disagreement mean the rate table or the cost math is wrong.
type delta struct {
	session     string
	ours        float64
	theirs      float64
	absolute    float64
	relative    float64
	ourTokens   oracleTokens
	theirTokens oracleTokens
}

func (d delta) tokensAgree() bool { return d.ourTokens == d.theirTokens }

// oracleTokens is the subset of counts a cost-state snapshot reports. Cache
// creation is flat there, with no TTL split, so cca's two classes are summed to
// match.
type oracleTokens struct {
	input, output, cacheRead, cacheCreation int64
}

// sweep prices every session that carries a cost-state snapshot and compares.
//
// Sessions Claude Code could not fully price are skipped: its own total is not
// a reference when it admits to a gap.
func sweep(recs []transcript.Record, states []transcript.CostState, calc *pricing.Calculator) []delta {
	rep := report.Build(recs, calc, report.Options{})
	ours := make(map[string]report.Group, len(rep.BySession))
	for _, g := range rep.BySession {
		ours[g.Key] = g
	}

	var out []delta
	for _, cs := range states {
		if cs.HasUnknownModelCost {
			continue
		}
		g, ok := ours[cs.SessionID]
		if !ok {
			continue
		}
		d := delta{
			session: cs.SessionID, ours: g.Cost.Total, theirs: cs.TotalCostUSD,
			ourTokens: oracleTokens{
				input: g.Tokens.Input, output: g.Tokens.Output,
				cacheRead:     g.Tokens.CacheRead,
				cacheCreation: g.Tokens.CacheWrite5m + g.Tokens.CacheWrite1h,
			},
			theirTokens: sumOracleTokens(cs),
		}
		d.absolute = math.Abs(d.ours - d.theirs)
		if cs.TotalCostUSD != 0 {
			d.relative = d.absolute / cs.TotalCostUSD
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].relative > out[j].relative })
	return out
}

func sumOracleTokens(cs transcript.CostState) oracleTokens {
	var t oracleTokens
	for _, u := range cs.ModelUsage {
		t.input += u.InputTokens
		t.output += u.OutputTokens
		t.cacheRead += u.CacheReadInputTokens
		t.cacheCreation += u.CacheCreationInputTokens
	}
	return t
}

// A fixture sweep, so the comparison logic itself is covered without depending
// on any particular machine's data.
func TestSweepComparesPerSession(t *testing.T) {
	tbl, err := pricing.Embedded()
	if err != nil {
		t.Fatal(err)
	}

	// One million output tokens on Opus 5 is exactly $25.
	recs := []transcript.Record{
		{Model: "claude-opus-5", SessionID: "matching", Project: "p",
			Usage: transcript.Usage{Output: 1_000_000}},
		{Model: "claude-opus-5", SessionID: "diverging", Project: "p",
			Usage: transcript.Usage{Output: 1_000_000}},
		{Model: "claude-opus-5", SessionID: "unpriceable", Project: "p",
			Usage: transcript.Usage{Output: 1_000_000}},
	}
	states := []transcript.CostState{
		{SessionID: "matching", TotalCostUSD: 25},
		{SessionID: "diverging", TotalCostUSD: 30},
		// Claude Code says it could not price this one, so it is no oracle.
		{SessionID: "unpriceable", TotalCostUSD: 1, HasUnknownModelCost: true},
	}

	got := sweep(recs, states, pricing.NewCalculator(tbl))
	if len(got) != 2 {
		t.Fatalf("want 2 comparable sessions, got %d", len(got))
	}
	if got[0].session != "diverging" {
		t.Errorf("worst delta should sort first, got %q", got[0].session)
	}
	if math.Abs(got[0].absolute-5) > 1e-9 {
		t.Errorf("absolute delta = %v, want 5", got[0].absolute)
	}
	for _, d := range got {
		if d.session == "matching" && d.absolute > 1e-9 {
			t.Errorf("matching session should agree exactly, got %v", d.absolute)
		}
		if d.session == "unpriceable" {
			t.Error("a session with unknown model cost must be excluded")
		}
	}
}

// A session whose tokens do not match is a gap in the transcript, not a pricing
// error, and the sweep has to tell the two apart.
func TestSweepSeparatesTokenGapsFromPricingErrors(t *testing.T) {
	tbl, err := pricing.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	recs := []transcript.Record{
		{Model: "claude-opus-5", SessionID: "short", Project: "p",
			Usage: transcript.Usage{Output: 1_000_000}},
	}
	// Claude Code charged for twice the output cca can see.
	states := []transcript.CostState{{
		SessionID: "short", TotalCostUSD: 50,
		ModelUsage: map[string]transcript.CostStateUsage{
			"claude-opus-5": {OutputTokens: 2_000_000},
		},
	}}

	got := sweep(recs, states, pricing.NewCalculator(tbl))
	if len(got) != 1 {
		t.Fatalf("want 1 comparison, got %d", len(got))
	}
	if got[0].tokensAgree() {
		t.Error("token counts differ and the sweep should say so")
	}
	if got[0].ourTokens.output != 1_000_000 || got[0].theirTokens.output != 2_000_000 {
		t.Errorf("token comparison = %+v vs %+v", got[0].ourTokens, got[0].theirTokens)
	}
}

// The real sweep. Run it with CCA_ORACLE_CLAUDE_DIR=~/.claude go test ./internal/pricing/
//
// The assertion is deliberately narrow: where cca and Claude Code counted the
// same tokens, they must agree on the cost. A session where the token counts
// differ says only that the transcript is missing turns Claude Code billed for,
// which cca cannot do anything about and must not be failed for.
func TestSweepAgainstRealCostStates(t *testing.T) {
	dir := os.Getenv(oracleDirEnv)
	if dir == "" {
		t.Skipf("set %s to sweep a real Claude directory", oracleDirEnv)
	}

	tbl, err := pricing.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	files, err := transcript.Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := transcript.New(transcript.Options{ClaudeDir: dir, NonModels: tbl.NonModelSet()})
	var recs []transcript.Record
	for _, f := range files {
		got, err := p.ParseFile(f)
		if err != nil {
			t.Fatal(err)
		}
		recs = append(recs, got...)
	}

	states := p.CostStates()
	t.Logf("%d files, %d records, %d cost-state snapshots", len(files), len(recs), len(states))

	deltas := sweep(recs, states, pricing.NewCalculator(tbl))
	if len(deltas) == 0 {
		t.Skip("no comparable sessions in this corpus")
	}

	var compared int
	for _, d := range deltas {
		t.Log(describe(d))
		if !d.tokensAgree() {
			t.Logf("  token counts differ, so this session cannot test the cost math: "+
				"cca %+v vs claude code %+v", d.ourTokens, d.theirTokens)
			continue
		}
		compared++
		// Same tokens, same rates: any disagreement here is cca's fault.
		if d.absolute > 1e-6 {
			t.Errorf("same tokens but different cost for %s: %s", d.session[:8], describe(d))
		}
	}
	t.Logf("%d of %d sessions had comparable token counts", compared, len(deltas))
}

func describe(d delta) string {
	return fmt.Sprintf("session %s: cca $%.6f vs claude code $%.6f (delta $%.6f, %.3f%%)",
		d.session[:8], d.ours, d.theirs, d.ours-d.theirs, 100*d.relative)
}
