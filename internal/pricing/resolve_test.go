package pricing

import "testing"

func TestNormalize(t *testing.T) {
	tests := map[string]string{
		"claude-opus-5":     "claude-opus-5",
		"claude-opus-5[1m]": "claude-opus-5",
		"  Claude-Opus-5 ":  "claude-opus-5",
		"CLAUDE-FABLE-5-1":  "claude-fable-5-1",
		"":                  "",
	}
	for in, want := range tests {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolve(t *testing.T) {
	tbl, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, in, want string
	}{
		{"exact", "claude-opus-5", "claude-opus-5"},
		{"context suffix stripped", "claude-opus-5[1m]", "claude-opus-5"},
		{"dated snapshot", "claude-haiku-4-5-20251001", "claude-haiku-4-5"},
		{"uppercase", "Claude-Sonnet-5", "claude-sonnet-5"},

		// Both base rates are identical; only cache read separates them, so a
		// wrong match here produces a plausible-looking total.
		{"fable 5.1 stays 5.1", "claude-fable-5-1", "claude-fable-5-1"},
		{"fable 5 stays 5", "claude-fable-5", "claude-fable-5"},
		{"fable 5.1 dated", "claude-fable-5-1-20260101", "claude-fable-5-1"},

		// Sonnet 4.6 is 50% dearer than Sonnet 5.
		{"sonnet 4.6 not sonnet 5", "claude-sonnet-4-6", "claude-sonnet-4-6"},
		{"sonnet 4.5 not sonnet 4", "claude-sonnet-4-5", "claude-sonnet-4-5"},

		// Retired claude-opus-4 is $15/$75, triple the current Opus tier.
		{"opus 4.8 not opus 4", "claude-opus-4-8", "claude-opus-4-8"},
		{"opus 4.1 not opus 4", "claude-opus-4-1", "claude-opus-4-1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, ok := tbl.Resolve(tt.in)
			if !ok {
				t.Fatalf("Resolve(%q) found nothing, want %q", tt.in, tt.want)
			}
			if m.ID != tt.want {
				t.Errorf("Resolve(%q) = %q, want %q", tt.in, m.ID, tt.want)
			}
		})
	}
}

// The single most consequential lookup in the tool: cache reads are ~96% of all
// tokens, and these two rows differ 4x on exactly that column.
func TestResolveFableGenerationsDoNotCollapse(t *testing.T) {
	tbl, _ := Embedded()
	five, _ := tbl.Resolve("claude-fable-5")
	fiveOne, _ := tbl.Resolve("claude-fable-5-1")
	if five.ID == fiveOne.ID {
		t.Fatal("fable 5 and 5.1 collapsed to the same row")
	}
	if five.Rates.CacheRead != 1.00 {
		t.Errorf("fable-5 cache read = %v, want 1.00", five.Rates.CacheRead)
	}
	if fiveOne.Rates.CacheRead != 0.25 {
		t.Errorf("fable-5-1 cache read = %v, want 0.25", fiveOne.Rates.CacheRead)
	}
}

func TestResolveUnknown(t *testing.T) {
	tbl, _ := Embedded()
	for _, id := range []string{
		"",
		"gpt-4",
		"claude-imaginary-9",
		// A version bump is not a snapshot: this must not inherit retired
		// claude-opus-4's $15/$75 rates.
		"claude-opus-4-9",
		// Too few digits to be a YYYYMMDD snapshot.
		"claude-opus-5-12",
	} {
		if m, ok := tbl.Resolve(id); ok {
			t.Errorf("Resolve(%q) matched %q, want no match", id, m.ID)
		}
	}
}

func TestIsNonModel(t *testing.T) {
	tbl, _ := Embedded()
	if !tbl.IsNonModel("<synthetic>") {
		t.Error("<synthetic> must be recognised as a placeholder")
	}
	if tbl.IsNonModel("claude-opus-5") {
		t.Error("a real model must not be treated as a placeholder")
	}
}

func TestResolveIgnoresDisplayHints(t *testing.T) {
	// retired and inferred_id are labels for humans; they must not affect
	// whether a model resolves or what it costs.
	tbl, _ := Embedded()
	retired, ok := tbl.Resolve("claude-opus-4-1")
	if !ok || !retired.Retired {
		t.Fatal("expected claude-opus-4-1 to resolve and be marked retired")
	}
	if retired.Rates.Input != 15 {
		t.Errorf("retired model rates = %+v, want input 15", retired.Rates)
	}
	inferred, ok := tbl.Resolve("claude-mythos-5-1")
	if !ok || !inferred.InferredID {
		t.Fatal("expected claude-mythos-5-1 to resolve and be marked inferred")
	}
}
