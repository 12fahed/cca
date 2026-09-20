package transcript

import "sort"

// CostState is Claude Code's own accounting for one session, as it recorded it.
//
// It is not a source of usage: the records are periodic snapshots of the same
// session, so summing them would multiply that session's spend. Its value is as
// an independent check on cca's own cost arithmetic.
type CostState struct {
	SessionID    string
	TotalCostUSD float64
	// HasUnknownModelCost is Claude Code's own admission that it could not
	// price part of the session, which makes that total unusable as an oracle.
	HasUnknownModelCost bool
	ModelUsage          map[string]CostStateUsage
}

// CostStateUsage is one model's contribution to a session's recorded cost.
//
// Cache creation arrives as a single flat number with no TTL split, so any
// comparison has to supply that split from somewhere else.
type CostStateUsage struct {
	InputTokens              int64
	OutputTokens             int64
	ThinkingTokens           int64
	CacheReadInputTokens     int64
	CacheCreationInputTokens int64
	WebSearchRequests        int64
	CostUSD                  float64
}

// CostStates returns the richest snapshot seen for each session, ordered by
// session id.
//
// Snapshots accumulate through a session, so the one reporting the highest
// total is the most complete and the only one worth comparing against.
func (p *Parser) CostStates() []CostState {
	out := make([]CostState, 0, len(p.costStates))
	for _, cs := range p.costStates {
		out = append(out, cs)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SessionID < out[j].SessionID })
	return out
}

func (p *Parser) recordCostState(line []byte) {
	var raw rawCostState
	if err := unmarshal(line, &raw); err != nil || raw.SessionID == "" {
		return
	}
	if prev, ok := p.costStates[raw.SessionID]; ok && prev.TotalCostUSD >= raw.TotalCostUSD {
		return
	}
	cs := CostState{
		SessionID:           raw.SessionID,
		TotalCostUSD:        raw.TotalCostUSD,
		HasUnknownModelCost: raw.HasUnknown,
		ModelUsage:          make(map[string]CostStateUsage, len(raw.ModelUsage)),
	}
	for model, u := range raw.ModelUsage {
		cs.ModelUsage[model] = CostStateUsage{
			InputTokens:              u.InputTokens,
			OutputTokens:             u.OutputTokens,
			ThinkingTokens:           u.ThinkingTokens,
			CacheReadInputTokens:     u.CacheReadInputTokens,
			CacheCreationInputTokens: u.CacheCreationInputTokens,
			WebSearchRequests:        u.WebSearchRequests,
			CostUSD:                  u.CostUSD,
		}
	}
	p.costStates[raw.SessionID] = cs
}
