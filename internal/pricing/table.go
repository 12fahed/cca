// Package pricing resolves a model identifier to its rate row and computes cost
// from token counts. The default rate table is embedded; an override file
// replaces it wholesale.
package pricing

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
)

// supportedSchema is the only pricing.json layout this build understands. A
// bumped version in an override means the file expects fields we do not read,
// so it is refused rather than silently half-applied.
const supportedSchema = 1

//go:embed pricing.json
var embedded []byte

// Rates are absolute USD per million tokens for all five billed classes.
//
// Absolute figures rather than a base rate plus multipliers: the published price
// sheet lists them this way, per-model exceptions need no special case (Fable
// 5.1 reads at 0.025x base where every other model reads at 0.1x), and every
// number stays auditable against the sheet by eye.
type Rates struct {
	Input        float64 `json:"input"`
	Output       float64 `json:"output"`
	CacheWrite5m float64 `json:"cache_write_5m"`
	CacheWrite1h float64 `json:"cache_write_1h"`
	CacheRead    float64 `json:"cache_read"`
}

type Model struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Retired and InferredID are display hints. Neither changes the math.
	Retired    bool             `json:"retired"`
	InferredID bool             `json:"inferred_id"`
	Rates      Rates            `json:"rates"`
	Speeds     map[string]Rates `json:"speeds"`
	Note       string           `json:"note"`
}

type Modifier struct {
	Multiplier float64  `json:"multiplier"`
	AppliesTo  []string `json:"applies_to"`
	Note       string   `json:"note"`
}

type Modifiers struct {
	Batch          Modifier `json:"batch"`
	InferenceGeoUS Modifier `json:"inference_geo_us"`
}

// FastMode partitions models by how they answer a speed=="fast" request.
type FastMode struct {
	SupportedModels     []string `json:"supported_models"`
	StandardSpeedModels []string `json:"standard_speed_models"`
	ErrorModels         []string `json:"error_models"`
}

type ServerTool struct {
	USDPerRequest float64 `json:"usd_per_request"`
	UsageField    string  `json:"usage_field"`
	Note          string  `json:"note"`
}

type ServerTools struct {
	WebSearch ServerTool `json:"web_search"`
	WebFetch  ServerTool `json:"web_fetch"`
}

type Table struct {
	SchemaVersion int         `json:"schema_version"`
	Source        string      `json:"source"`
	VerifiedOn    string      `json:"verified_on"`
	Currency      string      `json:"currency"`
	Unit          string      `json:"unit"`
	Modifiers     Modifiers   `json:"modifiers"`
	FastMode      FastMode    `json:"fast_mode"`
	Models        []Model     `json:"models"`
	ServerTools   ServerTools `json:"server_tools"`
	// NonModels lists placeholder strings that appear where a model id belongs,
	// such as "<synthetic>". They are filtered before unknown-model detection so
	// they never produce a spurious warning.
	NonModels     []string `json:"non_models"`
	TokenizerNote string   `json:"tokenizer_note"`

	// Origin records where the table came from, for `cca config`.
	Origin string `json:"-"`

	byID     map[string]*Model
	fastKind map[string]fastKind
}

type fastKind int

const (
	fastUnknown fastKind = iota
	fastSupported
	fastRunsStandard
	fastErrors
)

// Embedded returns the compiled-in rate table.
func Embedded() (*Table, error) {
	return parse(embedded, "embedded")
}

// Load returns the table at path, or the embedded table when path is empty.
// An override replaces the embedded table entirely rather than merging into it,
// so a price change is a whole-file edit and never a partial one.
func Load(path string) (*Table, error) {
	if path == "" {
		return Embedded()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read pricing table: %w", err)
	}
	return parse(data, path)
}

// LoadWithFallback uses the override at path when it exists and the embedded
// table otherwise. A path given explicitly by the user must exist; a discovered
// default path is allowed to be absent.
func LoadWithFallback(path string) (*Table, error) {
	if path == "" {
		return Embedded()
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return Embedded()
		}
		return nil, err
	}
	return Load(path)
}

func parse(data []byte, origin string) (*Table, error) {
	// Unknown fields are ignored on purpose: the file carries prose keys such as
	// _comment and per-entry note that document the numbers for whoever edits it.
	var t Table
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("parse pricing table (%s): %w", origin, err)
	}
	t.Origin = origin
	if err := t.validate(); err != nil {
		return nil, fmt.Errorf("invalid pricing table (%s): %w", origin, err)
	}
	t.index()
	return &t, nil
}

func (t *Table) validate() error {
	if t.SchemaVersion != supportedSchema {
		return fmt.Errorf("schema_version %d is not supported (this build reads %d)",
			t.SchemaVersion, supportedSchema)
	}
	if len(t.Models) == 0 {
		return fmt.Errorf("table lists no models")
	}
	seen := make(map[string]bool, len(t.Models))
	for i, m := range t.Models {
		if m.ID == "" {
			return fmt.Errorf("models[%d] has no id", i)
		}
		if seen[m.ID] {
			return fmt.Errorf("duplicate model id %q", m.ID)
		}
		seen[m.ID] = true
		if err := checkRates(m.ID, "rates", m.Rates); err != nil {
			return err
		}
		for speed, r := range m.Speeds {
			if err := checkRates(m.ID, "speeds."+speed, r); err != nil {
				return err
			}
		}
	}
	if t.Modifiers.InferenceGeoUS.Multiplier < 0 || t.Modifiers.Batch.Multiplier < 0 {
		return fmt.Errorf("modifier multipliers must not be negative")
	}
	if t.ServerTools.WebSearch.USDPerRequest < 0 || t.ServerTools.WebFetch.USDPerRequest < 0 {
		return fmt.Errorf("server tool prices must not be negative")
	}
	return nil
}

func checkRates(id, field string, r Rates) error {
	for name, v := range map[string]float64{
		"input": r.Input, "output": r.Output, "cache_write_5m": r.CacheWrite5m,
		"cache_write_1h": r.CacheWrite1h, "cache_read": r.CacheRead,
	} {
		if v < 0 {
			return fmt.Errorf("model %q %s.%s is negative", id, field, name)
		}
	}
	return nil
}

func (t *Table) index() {
	t.byID = make(map[string]*Model, len(t.Models))
	for i := range t.Models {
		t.byID[t.Models[i].ID] = &t.Models[i]
	}
	t.fastKind = make(map[string]fastKind)
	for _, id := range t.FastMode.SupportedModels {
		t.fastKind[id] = fastSupported
	}
	for _, id := range t.FastMode.StandardSpeedModels {
		t.fastKind[id] = fastRunsStandard
	}
	for _, id := range t.FastMode.ErrorModels {
		t.fastKind[id] = fastErrors
	}
}

// NonModelSet returns the placeholder model strings as a set, ready to hand to
// the transcript parser.
func (t *Table) NonModelSet() []string { return t.NonModels }

// ByID returns an exact model entry, without prefix matching.
func (t *Table) ByID(id string) (*Model, bool) {
	m, ok := t.byID[id]
	return m, ok
}
