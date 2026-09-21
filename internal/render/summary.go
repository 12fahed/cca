package render

import (
	"fmt"
	"io"
	"strings"
	"time"

	"cca/internal/report"
	"cca/internal/water"
)

// glyphs holds the few non-ASCII characters the table uses, with plain
// substitutes for terminals that mangle them.
type glyphs struct {
	sep    string // between header segments
	approx string // before the water figure
	bullet string // in front of a footnote
}

var (
	unicodeGlyphs = glyphs{sep: "·", approx: "≈", bullet: "─"}
	asciiGlyphs   = glyphs{sep: "-", approx: "~", bullet: "-"}
)

// Options controls how a report is rendered.
type Options struct {
	// ASCII swaps box-drawing and maths characters for plain substitutes.
	ASCII bool
	// Water is the estimate to display. It is computed by the caller, which
	// owns the configured rate.
	Water water.Estimate
	// View names the command being rendered, for machine-readable output.
	View string
	// Color styles output. The zero value writes plain text.
	Color Palette
}

const indent = "  "

// Summary writes the default one-screen view.
func Summary(w io.Writer, rep *report.Report, opts Options) error {
	g := glyphsFor(opts.ASCII)
	var b strings.Builder

	writeHeader(&b, rep, g, opts.Color)
	if rep.Overall.Requests == 0 {
		writeEmpty(&b, rep)
		_, err := io.WriteString(w, b.String())
		return err
	}

	b.WriteString("\n")
	writeTokens(&b, rep, opts)
	b.WriteString("\n")
	writeModels(&b, rep, opts)
	b.WriteString("\n")
	writeTotals(&b, rep, opts, g)
	b.WriteString("\n")
	writeFootnotes(&b, rep, opts, g)

	_, err := io.WriteString(w, b.String())
	return err
}

func writeHeader(b *strings.Builder, rep *report.Report, g glyphs, p Palette) {
	parts := []string{"Claude Code usage", DescribeWindow(rep.Window)}
	if rep.Overall.Requests > 0 {
		parts = append(parts, fmt.Sprintf("%s across %s",
			Plural(rep.Overall.Sessions, "session", "sessions"),
			Plural(rep.Overall.Projects, "project", "projects")))
	}
	fmt.Fprintf(b, "%s%s\n", indent, p.Dim(strings.Join(parts, " "+g.sep+" ")))
}

func writeEmpty(b *strings.Builder, rep *report.Report) {
	b.WriteString("\n")
	switch {
	case rep.Filtered > 0:
		fmt.Fprintf(b, "%sNo usage in this window. %s fell outside it.\n",
			indent, Plural(int(rep.Filtered), "record", "records"))
	default:
		fmt.Fprintf(b, "%sNo usage found.\n", indent)
	}
}

// writeTokens renders the five billed classes. The cache-write classes stay
// apart because they bill at different rates, and the 1-hour class dominates.
func writeTokens(b *strings.Builder, rep *report.Report, opts Options) {
	t := rep.Overall.Tokens
	p := opts.Color
	tbl := newTable(1, "Tokens", "input", "output", "cache 5m", "cache 1h", "cache read").
		withPalette(p)
	tbl.add("", p.Tokens(Tokens(t.Input)), p.Tokens(Tokens(t.Output)),
		p.Tokens(Tokens(t.CacheWrite5m)), p.Tokens(Tokens(t.CacheWrite1h)),
		p.Tokens(Tokens(t.CacheRead)))
	tbl.render(b)
}

func writeModels(b *strings.Builder, rep *report.Report, opts Options) {
	p := opts.Color
	tbl := newTable(1, "Model", "tokens", "cost").withPalette(p)
	for _, g := range rep.ByModel {
		tbl.add(g.Key, p.Tokens(Tokens(g.Tokens.Total())), costCell(g, p))
	}
	tbl.render(b)
}

func writeTotals(b *strings.Builder, rep *report.Report, opts Options, g glyphs) {
	// Left aligned throughout: the trailing column is prose, which would read
	// oddly ragged if right aligned.
	p := opts.Color
	tbl := newTable(3, "", "", "").withGap(3)
	tbl.add("Cost", p.Cost(USD(rep.Overall.Cost.Total)), p.Muted("at API list prices"))
	tbl.add("Water", p.Water(g.approx+" "+opts.Water.Volume()),
		p.Water(opts.Water.Equivalence))
	tbl.render(b)
}

// writeFootnotes emits the two mandatory caveats, then any that apply to this
// particular run. Both mandatory notes are part of the tool's honesty about its
// own accuracy and are never suppressed.
func writeFootnotes(b *strings.Builder, rep *report.Report, opts Options, g glyphs) {
	notes := []string{
		costCaveat,
		"Water " + opts.Water.Assumption() + ". See README.",
	}

	if n := len(rep.UnknownModels); n > 0 {
		notes = append(notes, fmt.Sprintf(
			"%s had no rate and %s excluded from the cost total: %s.\n"+
				"Add %s to pricing.json to price %s.",
			Plural(n, "model", "models"), pick(n, "was", "were"),
			strings.Join(rep.UnknownModels, ", "),
			pick(n, "it", "them"), pick(n, "it", "them")))
	}
	if rep.Stats.FlatCacheFallback > 0 {
		notes = append(notes, fmt.Sprintf(
			"%s carried no cache-write TTL split; those writes were billed as\n"+
				"5-minute writes, which understates any that were really 1-hour writes.",
			Plural(int(rep.Stats.FlatCacheFallback), "record", "records")))
	}
	if rep.FastRequests > 0 {
		notes = append(notes, fmt.Sprintf(
			"%s used fast mode, which bills at roughly double the standard rate.",
			Plural(int(rep.FastRequests), "request", "requests")))
	}
	if rep.USGeoRequests > 0 {
		notes = append(notes, fmt.Sprintf(
			"%s pinned inference to the US, which adds 10%% to every token class.",
			Plural(int(rep.USGeoRequests), "request", "requests")))
	}
	if rep.BatchRequests > 0 {
		notes = append(notes, fmt.Sprintf(
			"%s ran on the batch tier at half the standard rate.",
			Plural(int(rep.BatchRequests), "request", "requests")))
	}
	if n := rep.Overall.Tokens.WebSearches; n > 0 {
		notes = append(notes, fmt.Sprintf(
			"Includes %s. Failed searches are not billed but are\n"+
				"indistinguishable here, so that charge is an upper bound.",
			Plural(int(n), "web search", "web searches")))
	}
	if rep.Undated > 0 {
		notes = append(notes, fmt.Sprintf(
			"%s had no usable timestamp and %s counted in the totals but not by day.",
			Plural(int(rep.Undated), "record", "records"),
			pick(int(rep.Undated), "is", "are")))
	}

	for _, n := range notes {
		writeNoteStyled(b, g, opts.Color, n)
	}
}

// pick chooses between singular and plural wording so that a footnote reads
// naturally whether it describes one item or many.
func pick(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

// DescribeWindow names a window the way a reader would say it aloud.
func DescribeWindow(w report.Window) string {
	switch {
	case w.Unbounded():
		return "all time"
	case w.Start.IsZero():
		return "until " + day(lastIncludedDay(w.End))
	case w.End.IsZero():
		return "since " + day(w.Start)
	}
	return day(w.Start) + " to " + day(lastIncludedDay(w.End))
}

// The window's end is exclusive, so the last day a reader would name is the one
// before it.
func lastIncludedDay(end time.Time) time.Time { return end.AddDate(0, 0, -1) }

func day(t time.Time) string { return t.Format("2006-01-02") }
