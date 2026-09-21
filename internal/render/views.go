package render

import (
	"fmt"
	"io"
	"strings"

	"cca/internal/report"
)

// costCaveat is the §4.4 note. Any view that prints a dollar figure carries it:
// the number is what the usage would have cost at API rates, which is not a bill.
const costCaveat = "Cost is what this usage would cost at API rates, not what you were billed;\n" +
	"Claude Code on a subscription draws from your plan allowance instead."

const maxProjectWidth = 34

// Models writes the per-model breakdown.
func Models(w io.Writer, rep *report.Report, opts Options) error {
	p := opts.Color
	return writeView(w, rep, opts, "by model", func(b *strings.Builder) {
		t := newTable(1, "Model", "requests", "tokens", "cost", "share")
		for _, g := range rep.ByModel {
			t.add(g.Key, Count(g.Requests), p.Tokens(Tokens(g.Tokens.Total())),
				costCell(g, p), p.Muted(share(g.Cost.Total, rep.Overall.Cost.Total)))
		}
		addTotals(t, rep, p)
		t.withPalette(p).render(b)
	})
}

// Projects writes the per-project breakdown. Project keys are slugified working
// directories, so they are shown tail-first to keep the distinguishing part.
func Projects(w io.Writer, rep *report.Report, opts Options) error {
	p := opts.Color
	return writeView(w, rep, opts, "by project", func(b *strings.Builder) {
		t := newTable(1, "Project", "sessions", "requests", "tokens", "cost", "share")
		for _, g := range rep.ByProject {
			t.add(shorten(g.Key, maxProjectWidth, opts.ASCII), Count(int64(g.Sessions)),
				Count(g.Requests), p.Tokens(Tokens(g.Tokens.Total())),
				costCell(g, p), p.Muted(share(g.Cost.Total, rep.Overall.Cost.Total)))
		}
		t.addTotal(p.Strong("Total"), p.Strong(Count(int64(rep.Overall.Sessions))),
			p.Strong(Count(rep.Overall.Requests)),
			p.Strong(p.Tokens(Tokens(rep.Overall.Tokens.Total()))),
			p.Strong(p.Cost(USD(rep.Overall.Cost.Total))), "")
		t.withPalette(p).render(b)
	})
}

// Daily writes the per-day table, oldest first so the newest row sits nearest
// the prompt.
func Daily(w io.Writer, rep *report.Report, opts Options) error {
	p := opts.Color
	return writeView(w, rep, opts, "by day", func(b *strings.Builder) {
		t := newTable(1, "Date", "requests", "tokens", "cost")
		for _, g := range rep.ByDay {
			t.add(g.Key, Count(g.Requests), p.Tokens(Tokens(g.Tokens.Total())), costCell(g, p))
		}
		t.addTotal(p.Strong("Total"), p.Strong(Count(rep.Overall.Requests)),
			p.Strong(p.Tokens(Tokens(rep.Overall.Tokens.Total()))),
			p.Strong(p.Cost(USD(rep.Overall.Cost.Total))))
		t.withPalette(p).render(b)
	})
}

// Sessions writes the costliest sessions.
//
// Session ids are 36-character uuids, which would crowd out the project on an
// 80-column terminal. A short prefix is enough to identify one, and the full id
// is available in machine-readable output.
func Sessions(w io.Writer, rep *report.Report, opts Options) error {
	p := opts.Color
	return writeView(w, rep, opts, "by session", func(b *strings.Builder) {
		t := newTable(3, "Session", "started", "project", "requests", "tokens", "cost")
		for _, g := range rep.BySession {
			started := "—"
			if opts.ASCII {
				started = "-"
			}
			if !g.First.IsZero() {
				started = g.First.Format("2006-01-02")
			}
			t.add(shortID(g.Key), p.Muted(started),
				shorten(g.Project, maxProjectWidth, opts.ASCII),
				Count(g.Requests), p.Tokens(Tokens(g.Tokens.Total())), costCell(g, p))
		}
		t.withPalette(p).render(b)
	})
}

// writeView frames a table with the shared header, the empty-result message,
// and the cost caveat.
func writeView(w io.Writer, rep *report.Report, opts Options, title string, body func(*strings.Builder)) error {
	g := glyphsFor(opts.ASCII)
	var b strings.Builder

	fmt.Fprintf(&b, "%s%s\n", indent, opts.Color.Dim(fmt.Sprintf(
		"Claude Code usage %s %s %s %s", g.sep, title, g.sep, DescribeWindow(rep.Window))))
	if rep.Overall.Requests == 0 {
		writeEmpty(&b, rep)
		_, err := io.WriteString(w, b.String())
		return err
	}

	b.WriteString("\n")
	body(&b)
	b.WriteString("\n")
	writeNoteStyled(&b, g, opts.Color, costCaveat)
	if n := len(rep.UnknownModels); n > 0 {
		writeNoteStyled(&b, g, opts.Color, fmt.Sprintf(
			"%s had no rate and %s left out of the cost column: %s.",
			Plural(n, "model", "models"), pick(n, "was", "were"),
			strings.Join(rep.UnknownModels, ", ")))
	}

	_, err := io.WriteString(w, b.String())
	return err
}

func addTotals(t *table, rep *report.Report, p Palette) {
	cells := []string{
		p.Strong("Total"),
		p.Strong(Count(rep.Overall.Requests)),
		p.Strong(p.Tokens(Tokens(rep.Overall.Tokens.Total()))),
		p.Strong(p.Cost(USD(rep.Overall.Cost.Total))),
	}
	if len(t.headers) == 5 {
		cells = append(cells, "")
	}
	t.addTotal(cells...)
}

// costCell renders a group's spend, marking the groups whose model had no rate
// rather than printing a figure that silently omits them. An unpriced row is
// warned about rather than coloured as money, since there is no money in it.
func costCell(g report.Group, p Palette) string {
	if g.UnpricedTokens > 0 && g.Cost.Total == 0 {
		return p.Warn("unpriced")
	}
	return p.Cost(USD(g.Cost.Total))
}

func share(part, whole float64) string {
	if whole == 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f%%", 100*part/whole)
}

func shortID(id string) string {
	if i := strings.IndexByte(id, '-'); i > 0 {
		return id[:i]
	}
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func shorten(s string, max int, ascii bool) string {
	if ascii {
		return truncLeftASCII(s, max)
	}
	return truncLeft(s, max)
}

func glyphsFor(ascii bool) glyphs {
	if ascii {
		return asciiGlyphs
	}
	return unicodeGlyphs
}

func writeNote(b *strings.Builder, g glyphs, note string) {
	writeNoteStyled(b, g, Palette{}, note)
}

// writeNoteStyled dims a footnote. Notes sit outside any aligned column, so
// escape bytes here cannot disturb a table's widths.
func writeNoteStyled(b *strings.Builder, g glyphs, p Palette, note string) {
	lines := strings.Split(note, "\n")
	fmt.Fprintf(b, "%s%s\n", indent, p.Dim(g.bullet+" "+lines[0]))
	for _, l := range lines[1:] {
		fmt.Fprintf(b, "%s  %s\n", indent, p.Dim(l))
	}
}

// flatten turns a wrapped note into a single line, for formats with no column
// width to respect.
func flatten(s string) string { return strings.ReplaceAll(s, "\n", " ") }

// View names, used to pick the rows a machine-readable format emits.
const (
	ViewSummary  = "summary"
	ViewModels   = "models"
	ViewProjects = "projects"
	ViewDaily    = "daily"
	ViewSessions = "sessions"
	ViewConfig   = "config"
)
