package render

import (
	"encoding/csv"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"testing"

	"cca/internal/water"
)

// waterJSON is the slice of the document these tests care about.
type waterJSON struct {
	Water struct {
		MLPer1k float64 `json:"ml_per_1k_tokens"`
	} `json:"water"`
	Totals   waterGroupJSON   `json:"totals"`
	Sessions []waterGroupJSON `json:"sessions"`
	Models   []waterGroupJSON `json:"models"`
	Daily    []waterGroupJSON `json:"daily"`
}

func parseWaterDoc(t *testing.T, opts Options) waterJSON {
	t.Helper()
	var b strings.Builder
	if err := JSON(&b, titledReport(t), opts); err != nil {
		t.Fatal(err)
	}
	var doc waterJSON
	if err := json.Unmarshal([]byte(b.String()), &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

type waterGroupJSON struct {
	Key    string `json:"key"`
	Tokens struct {
		Total int64 `json:"total"`
	} `json:"tokens"`
	Water struct {
		Millilitres float64 `json:"millilitres"`
		Litres      float64 `json:"litres"`
	} `json:"water"`
}

// Water is derived from token counts rather than anything the user wrote, so
// unlike a title it needs no opt-in.
func TestJSONCarriesWaterWithoutAFlag(t *testing.T) {
	doc := parseWaterDoc(t, Options{View: ViewSessions, Water: water.For(0, water.DefaultMLPer1kTokens)})

	if doc.Totals.Water.Millilitres <= 0 {
		t.Fatalf("totals carry no water: %+v", doc.Totals.Water)
	}
	for _, g := range doc.Sessions {
		if g.Water.Millilitres <= 0 {
			t.Errorf("session %s carries no water", g.Key)
		}
	}
}

// Every grouping gets the field, so a consumer does not have to know which
// views happen to carry it.
func TestJSONWaterOnEveryGrouping(t *testing.T) {
	doc := parseWaterDoc(t, Options{View: ViewSummary, Water: water.For(0, water.DefaultMLPer1kTokens)})
	for name, groups := range map[string][]waterGroupJSON{
		"sessions": doc.Sessions, "models": doc.Models, "daily": doc.Daily,
	} {
		if len(groups) == 0 {
			t.Fatalf("%s is empty", name)
		}
		for _, g := range groups {
			if g.Water.Millilitres <= 0 {
				t.Errorf("%s group %q carries no water", name, g.Key)
			}
		}
	}
}

// Water is a flat rate per token, so each group's figure must be exactly its
// token total rescaled — not independently accumulated.
func TestJSONWaterMatchesTokensExactly(t *testing.T) {
	rate := water.DefaultMLPer1kTokens
	doc := parseWaterDoc(t, Options{View: ViewSessions, Water: water.For(0, rate)})

	check := func(label string, groups []waterGroupJSON) {
		for _, g := range groups {
			want := water.For(g.Tokens.Total, rate).Millilitres
			if math.Abs(g.Water.Millilitres-want) > 1e-9 {
				t.Errorf("%s %q: water %v, tokens imply %v", label, g.Key, g.Water.Millilitres, want)
			}
		}
	}
	check("session", doc.Sessions)
	check("model", doc.Models)

	// And the groups must sum to the run total, as the tokens do.
	var sum float64
	for _, g := range doc.Sessions {
		sum += g.Water.Millilitres
	}
	if math.Abs(sum-doc.Totals.Water.Millilitres) > 1e-6 {
		t.Errorf("session water sums to %v, totals say %v", sum, doc.Totals.Water.Millilitres)
	}
}

// The document has to report the rate it actually used, so a consumer can
// recompute or requote the figure.
func TestJSONWaterFollowsTheConfiguredRate(t *testing.T) {
	const rate = 1.25
	doc := parseWaterDoc(t, Options{View: ViewSessions, Water: water.For(0, rate)})

	if doc.Water.MLPer1k != rate {
		t.Errorf("document reports rate %v, want %v", doc.Water.MLPer1k, rate)
	}
	for _, g := range doc.Sessions {
		want := water.For(g.Tokens.Total, rate).Millilitres
		if math.Abs(g.Water.Millilitres-want) > 1e-9 {
			t.Errorf("session %q used the wrong rate: %v vs %v", g.Key, g.Water.Millilitres, want)
		}
	}
}

func TestCSVCarriesWater(t *testing.T) {
	rate := water.DefaultMLPer1kTokens
	var b strings.Builder
	opts := Options{View: ViewSessions, Water: water.For(0, rate)}
	if err := CSV(&b, titledReport(t), opts); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(strings.NewReader(b.String())).ReadAll()
	if err != nil {
		t.Fatalf("invalid CSV: %v", err)
	}

	waterCol := indexOf(rows[0], "water_millilitres")
	tokenCol := indexOf(rows[0], "total_tokens")
	if waterCol < 0 || tokenCol < 0 {
		t.Fatalf("missing columns: %v", rows[0])
	}

	var sum float64
	for _, row := range rows[1:] {
		ml, err := strconv.ParseFloat(row[waterCol], 64)
		if err != nil {
			t.Fatalf("water %q does not parse: %v", row[waterCol], err)
		}
		tokens, err := strconv.ParseInt(row[tokenCol], 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		if want := water.For(tokens, rate).Millilitres; math.Abs(ml-want) > 1e-9 {
			t.Errorf("row water %v does not match its tokens (%v)", ml, want)
		}
		sum += ml
	}
	if sum <= 0 {
		t.Error("no water in the CSV")
	}
	// Every row keeps the same field count.
	for i, row := range rows {
		if len(row) != len(rows[0]) {
			t.Errorf("row %d has %d fields, header has %d", i, len(row), len(rows[0]))
		}
	}
}

// --no-water is a display choice. Machine output is not a display, and a
// consumer asking for CSV should not silently lose a derived column.
func TestMachineOutputKeepsWaterRegardlessOfTheDisplayFlag(t *testing.T) {
	rate := water.DefaultMLPer1kTokens
	for _, show := range []bool{true, false} {
		opts := Options{View: ViewSessions, ShowWater: show, Water: water.For(0, rate)}

		var b strings.Builder
		if err := CSV(&b, titledReport(t), opts); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(b.String(), "water_millilitres") {
			t.Errorf("ShowWater=%v dropped the CSV water column", show)
		}

		doc := parseWaterDoc(t, opts)
		if doc.Totals.Water.Millilitres <= 0 {
			t.Errorf("ShowWater=%v dropped the JSON water figure", show)
		}
	}
}
