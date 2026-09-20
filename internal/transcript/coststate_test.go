package transcript

import (
	"strconv"
	"strings"
	"testing"
)

// costState builds a snapshot line. The shape mirrors what Claude Code writes,
// with cache creation as one flat number and no TTL split.
func costState(session string, total float64, unknown bool) string {
	return `{"type":"cost-state","sessionId":"` + session + `","totalCostUSD":` +
		ftoa(total) + `,"hasUnknownModelCost":` + btoa(unknown) +
		`,"modelUsage":{"claude-opus-5[1m]":{"inputTokens":514,"outputTokens":2730,` +
		`"thinkingTokens":512,"cacheReadInputTokens":240569,` +
		`"cacheCreationInputTokens":19994,"webSearchRequests":0,"costUSD":` + ftoa(total) + `}}}`
}

func TestCostStatesAreCollected(t *testing.T) {
	p := New(Options{})
	if _, err := p.ParseReader(strings.NewReader(
		costState("s1", 0.391, false)+"\n"+
			assistant("m", "r", "u", "claude-opus-5", "")+"\n"), "proj"); err != nil {
		t.Fatal(err)
	}
	got := p.CostStates()
	if len(got) != 1 {
		t.Fatalf("want 1 cost state, got %d", len(got))
	}
	cs := got[0]
	if cs.SessionID != "s1" || cs.TotalCostUSD != 0.391 {
		t.Errorf("got %+v", cs)
	}
	u, ok := cs.ModelUsage["claude-opus-5[1m]"]
	if !ok {
		t.Fatalf("model usage missing: %+v", cs.ModelUsage)
	}
	if u.OutputTokens != 2730 || u.ThinkingTokens != 512 || u.CacheCreationInputTokens != 19994 {
		t.Errorf("usage = %+v", u)
	}
}

// Snapshots accumulate through a session, so only the richest is worth keeping;
// summing them would multiply that session's spend by however many were written.
func TestCostStatesKeepTheRichestSnapshot(t *testing.T) {
	p := New(Options{})
	body := costState("s1", 0.10, false) + "\n" +
		costState("s1", 0.39, false) + "\n" +
		costState("s1", 0.25, false) + "\n"
	if _, err := p.ParseReader(strings.NewReader(body), "proj"); err != nil {
		t.Fatal(err)
	}
	got := p.CostStates()
	if len(got) != 1 {
		t.Fatalf("want one entry per session, got %d", len(got))
	}
	if got[0].TotalCostUSD != 0.39 {
		t.Errorf("total = %v, want the highest snapshot 0.39", got[0].TotalCostUSD)
	}
}

func TestCostStatesAreOrderedAndPerSession(t *testing.T) {
	p := New(Options{})
	body := costState("zebra", 1, false) + "\n" + costState("alpha", 2, false) + "\n"
	if _, err := p.ParseReader(strings.NewReader(body), "proj"); err != nil {
		t.Fatal(err)
	}
	got := p.CostStates()
	if len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
	if got[0].SessionID != "alpha" || got[1].SessionID != "zebra" {
		t.Errorf("not ordered by session: %v", []string{got[0].SessionID, got[1].SessionID})
	}
}

// Claude Code's own admission that it could not price part of a session makes
// that total useless as an oracle, so the flag has to survive parsing.
func TestCostStateCarriesUnknownModelFlag(t *testing.T) {
	p := New(Options{})
	if _, err := p.ParseReader(strings.NewReader(costState("s1", 1, true)+"\n"), "proj"); err != nil {
		t.Fatal(err)
	}
	if !p.CostStates()[0].HasUnknownModelCost {
		t.Error("hasUnknownModelCost was lost")
	}
}

// Collecting cost states must not disturb the usage totals.
func TestCostStatesAreNotCountedAsUsage(t *testing.T) {
	p := New(Options{})
	body := costState("s1", 0.39, false) + "\n" + assistant("m", "r", "u", "claude-opus-5", "") + "\n"
	recs, err := p.ParseReader(strings.NewReader(body), "proj")
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 {
		t.Fatalf("want 1 usage record, got %d", len(recs))
	}
	if p.Stats().Records != 1 {
		t.Errorf("Records = %d, want 1", p.Stats().Records)
	}
}

func TestMalformedCostStateIsIgnored(t *testing.T) {
	p := New(Options{})
	body := `{"type":"cost-state","sessionId":"","totalCostUSD":1}` + "\n" +
		`{"type":"cost-state"}` + "\n"
	if _, err := p.ParseReader(strings.NewReader(body), "proj"); err != nil {
		t.Fatal(err)
	}
	if got := p.CostStates(); len(got) != 0 {
		t.Errorf("want none, got %+v", got)
	}
}

func ftoa(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

func btoa(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
