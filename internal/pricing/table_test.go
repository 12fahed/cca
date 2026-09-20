package pricing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddedTableLoads(t *testing.T) {
	tbl, err := Embedded()
	if err != nil {
		t.Fatalf("embedded table must always load: %v", err)
	}
	if tbl.Origin != "embedded" {
		t.Errorf("Origin = %q", tbl.Origin)
	}
	if tbl.Unit != "per_million_tokens" || tbl.Currency != "USD" {
		t.Errorf("unit/currency = %q/%q", tbl.Unit, tbl.Currency)
	}
	if tbl.VerifiedOn == "" || tbl.Source == "" {
		t.Error("table must record where its numbers came from and when")
	}
}

// Rates the cost math depends on. A silent edit to any of these changes every
// figure cca reports, so they are pinned here rather than trusted.
func TestEmbeddedRatesArePinned(t *testing.T) {
	tbl, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]Rates{
		"claude-fable-5-1":  {10, 50, 12.50, 20, 0.25},
		"claude-fable-5":    {10, 50, 12.50, 20, 1.00},
		"claude-opus-5":     {5, 25, 6.25, 10, 0.50},
		"claude-opus-4-8":   {5, 25, 6.25, 10, 0.50},
		"claude-opus-4-1":   {15, 75, 18.75, 30, 1.50},
		"claude-sonnet-5":   {2, 10, 2.50, 4, 0.20},
		"claude-sonnet-4-6": {3, 15, 3.75, 6, 0.30},
		"claude-haiku-4-5":  {1, 5, 1.25, 2, 0.10},
	}
	for id, w := range want {
		m, ok := tbl.ByID(id)
		if !ok {
			t.Errorf("model %q missing from the table", id)
			continue
		}
		if m.Rates != w {
			t.Errorf("%s rates = %+v, want %+v", id, m.Rates, w)
		}
	}
}

// The 5 and 5.1 generations differ only in cache reads. Nothing else separates
// them, so a collapsed match stays invisible in every other column.
func TestFableGenerationsDifferOnlyInCacheRead(t *testing.T) {
	tbl, _ := Embedded()
	five, ok1 := tbl.ByID("claude-fable-5")
	fiveOne, ok2 := tbl.ByID("claude-fable-5-1")
	if !ok1 || !ok2 {
		t.Fatal("both Fable generations must be present")
	}
	if five.Rates.CacheRead != 1.00 || fiveOne.Rates.CacheRead != 0.25 {
		t.Errorf("cache reads = %v / %v, want 1.00 / 0.25",
			five.Rates.CacheRead, fiveOne.Rates.CacheRead)
	}
	a, b := five.Rates, fiveOne.Rates
	a.CacheRead, b.CacheRead = 0, 0
	if a != b {
		t.Errorf("generations should match outside cache read: %+v vs %+v", a, b)
	}
}

func TestEmbeddedFastModeAndModifiers(t *testing.T) {
	tbl, _ := Embedded()
	opus5, _ := tbl.ByID("claude-opus-5")
	fast, ok := opus5.Speeds["fast"]
	if !ok {
		t.Fatal("opus-5 must carry a fast rate row")
	}
	// Cache multipliers stack on the fast base rather than staying at standard.
	if (fast != Rates{10, 50, 12.50, 20, 1.00}) {
		t.Errorf("opus-5 fast rates = %+v", fast)
	}
	if tbl.Modifiers.InferenceGeoUS.Multiplier != 1.1 {
		t.Errorf("us geo multiplier = %v, want 1.1", tbl.Modifiers.InferenceGeoUS.Multiplier)
	}
	if tbl.Modifiers.Batch.Multiplier != 0.5 {
		t.Errorf("batch multiplier = %v, want 0.5", tbl.Modifiers.Batch.Multiplier)
	}
	if tbl.ServerTools.WebSearch.USDPerRequest != 0.01 {
		t.Errorf("web search = %v, want 0.01", tbl.ServerTools.WebSearch.USDPerRequest)
	}
	if tbl.ServerTools.WebFetch.USDPerRequest != 0 {
		t.Errorf("web fetch = %v, want 0", tbl.ServerTools.WebFetch.USDPerRequest)
	}
}

func TestEmbeddedNonModels(t *testing.T) {
	tbl, _ := Embedded()
	found := false
	for _, s := range tbl.NonModelSet() {
		if s == "<synthetic>" {
			found = true
		}
	}
	if !found {
		t.Error("non_models must list <synthetic> so the parser can drop it")
	}
}

func writeTable(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pricing.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const minimalTable = `{
  "schema_version": 1,
  "currency": "USD",
  "unit": "per_million_tokens",
  "models": [
    {"id":"only-model","rates":{"input":1,"output":2,"cache_write_5m":3,"cache_write_1h":4,"cache_read":5}}
  ],
  "non_models": ["<fake>"]
}`

func TestOverrideReplacesTableWholesale(t *testing.T) {
	tbl, err := Load(writeTable(t, minimalTable))
	if err != nil {
		t.Fatal(err)
	}
	if len(tbl.Models) != 1 {
		t.Fatalf("override should replace, not merge: got %d models", len(tbl.Models))
	}
	if _, ok := tbl.ByID("claude-opus-5"); ok {
		t.Error("embedded models leaked into an override table")
	}
	if got := tbl.NonModelSet(); len(got) != 1 || got[0] != "<fake>" {
		t.Errorf("non_models not replaced: %v", got)
	}
}

func TestLoadWithFallback(t *testing.T) {
	tbl, err := LoadWithFallback(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil {
		t.Fatalf("a missing default override should fall back, not fail: %v", err)
	}
	if tbl.Origin != "embedded" {
		t.Errorf("Origin = %q, want embedded", tbl.Origin)
	}

	path := writeTable(t, minimalTable)
	if tbl, err = LoadWithFallback(path); err != nil || tbl.Origin != path {
		t.Errorf("present override not used: origin=%q err=%v", tbl.Origin, err)
	}
}

func TestLoadRejectsBadTables(t *testing.T) {
	tests := []struct {
		name, body, wantErr string
	}{
		{"not json", `{nope`, "parse"},
		{"future schema", `{"schema_version":99,"models":[{"id":"a","rates":{}}]}`, "schema_version"},
		{"no models", `{"schema_version":1,"models":[]}`, "no models"},
		{"missing id", `{"schema_version":1,"models":[{"rates":{}}]}`, "no id"},
		{"duplicate id", `{"schema_version":1,"models":[{"id":"a","rates":{}},{"id":"a","rates":{}}]}`, "duplicate"},
		{"negative rate", `{"schema_version":1,"models":[{"id":"a","rates":{"input":-1}}]}`, "negative"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(writeTable(t, tt.body))
			if err == nil {
				t.Fatal("want an error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not mention %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadMissingExplicitPathFails(t *testing.T) {
	// An override named explicitly by the user must not silently fall back.
	if _, err := Load(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("want an error for a missing explicit pricing path")
	}
}
