package pricing

import "strings"

// minSnapshotDigits is the shortest all-digit suffix treated as a dated release
// snapshot rather than a version number. Real snapshots are YYYYMMDD (eight);
// version bumps are one or two digits, so the gap is wide.
const minSnapshotDigits = 6

// Normalize prepares a transcript's model string for matching. It lowercases,
// trims, and drops a bracketed context-window suffix.
//
// The suffix matters: cost-state records carry ids like "claude-opus-5[1m]",
// while assistant records carry the bare id. The 1M window is the model's
// standard context at its standard price, so the tag is a label and never a
// separate rate row.
func Normalize(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	if i := strings.IndexByte(id, '['); i >= 0 {
		id = strings.TrimSpace(id[:i])
	}
	return id
}

// Resolve maps a transcript model string to its rate row, preferring the most
// specific match.
//
// Longest-prefix-wins is load-bearing. Several table entries are prefixes of
// others and priced differently: claude-opus-4 is $15/$75 while claude-opus-4-8
// is $5/$25, and claude-fable-5 reads at $1.00/MTok while claude-fable-5-1 reads
// at $0.25. First-match or shortest-match would misprice all of them.
//
// A prefix only counts when what follows is a dated snapshot, so an unreleased
// claude-opus-4-9 does not quietly inherit retired claude-opus-4's triple rate.
// Refusing to match makes it an unknown model, which warns loudly; guessing
// would not.
func (t *Table) Resolve(id string) (*Model, bool) {
	norm := Normalize(id)
	if norm == "" {
		return nil, false
	}
	if m, ok := t.byID[norm]; ok {
		return m, true
	}
	var best *Model
	for i := range t.Models {
		candidate := &t.Models[i]
		if !strings.HasPrefix(norm, candidate.ID) {
			continue
		}
		if !isSnapshotSuffix(norm[len(candidate.ID):]) {
			continue
		}
		if best == nil || len(candidate.ID) > len(best.ID) {
			best = candidate
		}
	}
	return best, best != nil
}

// IsNonModel reports whether id is a placeholder rather than a real model, such
// as "<synthetic>" on a cancelled turn. These are dropped before unknown-model
// detection so they never trigger a warning.
func (t *Table) IsNonModel(id string) bool {
	norm := Normalize(id)
	for _, s := range t.NonModels {
		if Normalize(s) == norm {
			return true
		}
	}
	return false
}

func isSnapshotSuffix(rest string) bool {
	if rest == "" {
		return true
	}
	if rest[0] != '-' {
		return false
	}
	digits := rest[1:]
	if len(digits) < minSnapshotDigits {
		return false
	}
	for i := 0; i < len(digits); i++ {
		if digits[i] < '0' || digits[i] > '9' {
			return false
		}
	}
	return true
}
