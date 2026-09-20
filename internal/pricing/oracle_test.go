package pricing

import (
	"math"
	"testing"
)

// Claude Code writes its own cost accounting into cost-state records, which
// makes them an independent oracle for this package's arithmetic. The figures
// below are transcribed from one such record; they are plain numbers, so no
// transcript content is committed with them.
//
//	{"type":"cost-state","totalCostUSD":0.39104449999999996,
//	 "modelUsage":{"claude-opus-5[1m]":{"inputTokens":514,"outputTokens":2730,
//	   "thinkingTokens":512,"cacheReadInputTokens":240569,
//	   "cacheCreationInputTokens":19994,"webSearchRequests":0}}}
const (
	oracleModel        = "claude-opus-5[1m]"
	oracleInput        = 514
	oracleOutput       = 2730
	oracleThinking     = 512
	oracleCacheRead    = 240569
	oracleCacheCreate  = 19994
	oracleWebSearches  = 0
	oracleTotalCostUSD = 0.39104449999999996
)

// The oracle record reports cache creation as a single flat number with no TTL
// split, so the split has to be supplied. Attributing it to the 1-hour class
// reproduces Claude Code's own total exactly, which is itself evidence: the
// corpus is ~96% 1-hour cache writes.
func TestOracleMatchesClaudeCodeOwnCost(t *testing.T) {
	c := calc(t)
	got := c.Cost(Request{
		Model: oracleModel,
		Tokens: Tokens{
			Input:        oracleInput,
			Output:       oracleOutput,
			CacheWrite1h: oracleCacheCreate,
			CacheRead:    oracleCacheRead,
			WebSearches:  oracleWebSearches,
		},
	})
	if !got.Priced {
		t.Fatal("the oracle model must resolve")
	}
	if math.Abs(got.Total-oracleTotalCostUSD) > 1e-9 {
		t.Errorf("total = %.12f, want %.12f (delta %.2e)",
			got.Total, oracleTotalCostUSD, got.Total-oracleTotalCostUSD)
	}
	if w := c.Warnings(); len(w) != 0 {
		t.Errorf("oracle request should produce no warnings, got %+v", w)
	}
}

// Guards against the match above being a coincidence: the 5-minute attribution
// must not also land on the same total.
func TestOracleDistinguishesCacheWriteTTL(t *testing.T) {
	c := calc(t)
	fiveMin := c.Cost(Request{
		Model: oracleModel,
		Tokens: Tokens{
			Input:        oracleInput,
			Output:       oracleOutput,
			CacheWrite5m: oracleCacheCreate,
			CacheRead:    oracleCacheRead,
		},
	})
	if math.Abs(fiveMin.Total-oracleTotalCostUSD) < 1e-6 {
		t.Fatal("5m and 1h attribution cannot both match; the test proves nothing")
	}
}

// Thinking tokens are already inside output tokens. Adding them would overstate
// this record by the cost of 512 output tokens.
func TestOracleConfirmsThinkingIsInsideOutput(t *testing.T) {
	c := calc(t)
	doubled := c.Cost(Request{
		Model: oracleModel,
		Tokens: Tokens{
			Input:        oracleInput,
			Output:       oracleOutput + oracleThinking,
			CacheWrite1h: oracleCacheCreate,
			CacheRead:    oracleCacheRead,
		},
	})
	if math.Abs(doubled.Total-oracleTotalCostUSD) < 1e-9 {
		t.Fatal("adding thinking tokens should have broken the match")
	}
	opus, _ := c.Table().Resolve("claude-opus-5")
	want := oracleTotalCostUSD + rate(oracleThinking, opus.Rates.Output)
	if math.Abs(doubled.Total-want) > 1e-9 {
		t.Errorf("overstatement = %.12f, want %.12f", doubled.Total, want)
	}
}

// The oracle model id carries a [1m] tag. Matching Claude Code's total at the
// standard rate confirms the 1M context window carries no premium.
func TestOracleConfirmsNoLongContextPremium(t *testing.T) {
	c := calc(t)
	tagged, okTagged := c.Table().Resolve(oracleModel)
	bare, okBare := c.Table().Resolve("claude-opus-5")
	if !okTagged || !okBare {
		t.Fatal("both forms must resolve")
	}
	if tagged.ID != bare.ID || tagged.Rates != bare.Rates {
		t.Errorf("[1m] resolved to %q at %+v, want %q at %+v",
			tagged.ID, tagged.Rates, bare.ID, bare.Rates)
	}
}
