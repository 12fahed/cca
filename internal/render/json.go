package render

import (
	"encoding/json"
	"io"
	"time"

	"cca/internal/pricing"
	"cca/internal/report"
)

// jsonSchemaVersion is bumped when the document shape changes incompatibly, so
// a consumer can refuse a document it does not understand.
const jsonSchemaVersion = 1

// Document is the machine-readable form of a report.
//
// Token counts are exact here, never abbreviated, and session identifiers are
// full uuids rather than the prefixes the tables show. Every grouping is present
// regardless of which view was asked for: a consumer gains nothing from cca
// withholding data it has already computed, and emitting one shape keeps the
// numbers verifiably identical to the tables.
type Document struct {
	SchemaVersion int            `json:"schema_version"`
	View          string         `json:"view"`
	Window        windowDoc      `json:"window"`
	Location      string         `json:"location"`
	Totals        groupDoc       `json:"totals"`
	Main          groupDoc       `json:"main"`
	Sidechain     groupDoc       `json:"sidechain"`
	Water         waterDoc       `json:"water"`
	Models        []groupDoc     `json:"models"`
	Projects      []groupDoc     `json:"projects"`
	Daily         []groupDoc     `json:"daily"`
	Sessions      []groupDoc     `json:"sessions"`
	UnknownModels []string       `json:"unknown_models"`
	Warnings      []warningDoc   `json:"warnings"`
	Diagnostics   diagnosticsDoc `json:"diagnostics"`
	// Notes carries the same caveats the tables print. Machine output does not
	// excuse dropping them: a consumer that reports the cost figure onward
	// needs to know it is not a bill.
	Notes []string `json:"notes"`
}

type windowDoc struct {
	Label string  `json:"label"`
	Since *string `json:"since"`
	Until *string `json:"until"`
}

type tokensDoc struct {
	Input        int64 `json:"input"`
	Output       int64 `json:"output"`
	CacheWrite5m int64 `json:"cache_write_5m"`
	CacheWrite1h int64 `json:"cache_write_1h"`
	CacheRead    int64 `json:"cache_read"`
	// Thinking is a subset of Output, not an addend, and is excluded from Total.
	Thinking    int64 `json:"thinking"`
	WebSearches int64 `json:"web_searches"`
	Total       int64 `json:"total"`
}

type costDoc struct {
	Input        float64 `json:"input"`
	Output       float64 `json:"output"`
	CacheWrite5m float64 `json:"cache_write_5m"`
	CacheWrite1h float64 `json:"cache_write_1h"`
	CacheRead    float64 `json:"cache_read"`
	WebSearch    float64 `json:"web_search"`
	Total        float64 `json:"total"`
}

type groupDoc struct {
	Key     string `json:"key,omitempty"`
	Project string `json:"project,omitempty"`
	// Title and TitleSource appear only when --titles is passed. A title
	// describes what someone was working on, and this output gets committed to
	// repositories and pasted into issues, so it is opt-in rather than opt-out.
	Title          string    `json:"title,omitempty"`
	TitleSource    string    `json:"title_source,omitempty"`
	Requests       int64     `json:"requests"`
	Sessions       int       `json:"sessions,omitempty"`
	Projects       int       `json:"projects,omitempty"`
	First          *string   `json:"first,omitempty"`
	Last           *string   `json:"last,omitempty"`
	Tokens         tokensDoc `json:"tokens"`
	Cost           costDoc   `json:"cost_usd"`
	UnpricedTokens int64     `json:"unpriced_tokens,omitempty"`
}

type waterDoc struct {
	MLPer1kTokens float64 `json:"ml_per_1k_tokens"`
	Millilitres   float64 `json:"millilitres"`
	Litres        float64 `json:"litres"`
	Equivalence   string  `json:"equivalence"`
}

type warningDoc struct {
	Kind    string `json:"kind"`
	Subject string `json:"subject"`
	Count   int64  `json:"count"`
	Detail  string `json:"detail"`
}

type diagnosticsDoc struct {
	Files              int              `json:"files"`
	Lines              int64            `json:"lines"`
	Records            int64            `json:"records"`
	Duplicates         int64            `json:"duplicates"`
	Skipped            map[string]int64 `json:"skipped"`
	FlatCacheFallback  int64            `json:"flat_cache_fallback"`
	CacheSplitMismatch int64            `json:"cache_split_mismatch"`
	BadTimestamp       int64            `json:"bad_timestamp"`
	NoDedupKey         int64            `json:"no_dedup_key"`
	Filtered           int64            `json:"filtered_by_window"`
	Undated            int64            `json:"undated"`
}

// JSON writes the report as a single indented document.
func JSON(w io.Writer, rep *report.Report, opts Options) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	// Escaping is off so that model ids and project paths read as written.
	enc.SetEscapeHTML(false)
	return enc.Encode(NewDocument(rep, opts))
}

// NewDocument converts a report into its machine-readable form.
func NewDocument(rep *report.Report, opts Options) Document {
	return Document{
		SchemaVersion: jsonSchemaVersion,
		View:          opts.View,
		Window:        newWindowDoc(rep.Window),
		Location:      rep.Location,
		Totals:        newGroupDoc(rep.Overall, false),
		Main:          newGroupDoc(rep.Main, false),
		Sidechain:     newGroupDoc(rep.Sidechain, false),
		Water: waterDoc{
			MLPer1kTokens: opts.Water.MLPer1k,
			Millilitres:   opts.Water.Millilitres,
			Litres:        opts.Water.Litres,
			Equivalence:   opts.Water.Equivalence,
		},
		Models:   newGroupDocs(rep.ByModel, false),
		Projects: newGroupDocs(rep.ByProject, false),
		Daily:    newGroupDocs(rep.ByDay, false),
		// Only session groups carry a title; the other groupings are not keyed
		// by session.
		Sessions:      newGroupDocs(rep.BySession, opts.ShowTitles),
		UnknownModels: nonNil(rep.UnknownModels),
		Warnings:      newWarningDocs(rep.Warnings),
		Diagnostics: diagnosticsDoc{
			Files: rep.Stats.Files, Lines: rep.Stats.Lines,
			Records: rep.Stats.Records, Duplicates: rep.Stats.Duplicates,
			Skipped:            nonNilMap(rep.Stats.Skipped),
			FlatCacheFallback:  rep.Stats.FlatCacheFallback,
			CacheSplitMismatch: rep.Stats.CacheSplitMismatch,
			BadTimestamp:       rep.Stats.BadTimestamp,
			NoDedupKey:         rep.Stats.NoDedupKey,
			Filtered:           rep.Filtered,
			Undated:            rep.Undated,
		},
		Notes: []string{
			flatten(costCaveat),
			"Water " + opts.Water.Assumption() + ".",
		},
	}
}

func newWindowDoc(w report.Window) windowDoc {
	d := windowDoc{Label: DescribeWindow(w)}
	if !w.Start.IsZero() {
		s := w.Start.Format(time.RFC3339)
		d.Since = &s
	}
	if !w.End.IsZero() {
		// The stored bound is exclusive; report the last instant included so
		// that a consumer filtering on it selects the same records cca did.
		s := w.End.Add(-time.Nanosecond).Format(time.RFC3339)
		d.Until = &s
	}
	return d
}

func newGroupDoc(g report.Group, titles bool) groupDoc {
	d := groupDoc{
		Key: g.Key, Project: g.Project, Requests: g.Requests,
		Sessions: g.Sessions, Projects: g.Projects,
		Tokens: tokensDoc{
			Input: g.Tokens.Input, Output: g.Tokens.Output,
			CacheWrite5m: g.Tokens.CacheWrite5m, CacheWrite1h: g.Tokens.CacheWrite1h,
			CacheRead: g.Tokens.CacheRead, Thinking: g.Tokens.Thinking,
			WebSearches: g.Tokens.WebSearches, Total: g.Tokens.Total(),
		},
		Cost: costDoc{
			Input: g.Cost.Input, Output: g.Cost.Output,
			CacheWrite5m: g.Cost.CacheWrite5m, CacheWrite1h: g.Cost.CacheWrite1h,
			CacheRead: g.Cost.CacheRead, WebSearch: g.Cost.WebSearch, Total: g.Cost.Total,
		},
		UnpricedTokens: g.UnpricedTokens,
	}
	if !g.First.IsZero() {
		s := g.First.Format(time.RFC3339)
		d.First = &s
	}
	if !g.Last.IsZero() {
		s := g.Last.Format(time.RFC3339)
		d.Last = &s
	}
	if titles && g.Title.Text != "" {
		d.Title = g.Title.Text
		d.TitleSource = string(g.Title.Source)
	}
	return d
}

func newGroupDocs(gs []report.Group, titles bool) []groupDoc {
	out := make([]groupDoc, 0, len(gs))
	for _, g := range gs {
		out = append(out, newGroupDoc(g, titles))
	}
	return out
}

func newWarningDocs(ws []pricing.Warning) []warningDoc {
	out := make([]warningDoc, 0, len(ws))
	for _, w := range ws {
		out = append(out, warningDoc{w.Kind, w.Subject, w.Count, w.Detail})
	}
	return out
}

// nonNil keeps empty collections as [] rather than null, so a consumer can
// iterate without a nil check.
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func nonNilMap(m map[string]int64) map[string]int64 {
	if m == nil {
		return map[string]int64{}
	}
	return m
}
