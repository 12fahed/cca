package render

import (
	"strings"
)

// table lays out rows with a run of left-aligned label columns followed by
// right-aligned numeric columns.
//
// Columns are measured against visible width rather than byte length, so a
// styled cell lines up exactly as its unstyled equivalent would. See width.go
// for why text/tabwriter cannot do this.
type table struct {
	palette glyphPalette
	headers []string
	// labels is how many leading columns are left-aligned.
	labels int
	rows   [][]string
	// rule marks rows preceded by a blank line, used to set a total apart.
	rule map[int]bool
	// gap is the number of spaces between columns.
	gap int
}

// glyphPalette is the subset of Palette a table needs.
type glyphPalette interface{ Heading(string) string }

func newTable(labels int, headers ...string) *table {
	return &table{headers: headers, labels: labels, rule: map[int]bool{}, gap: columnGap}
}

// withGap widens the space between columns, for a block whose trailing column
// is prose rather than a figure.
func (t *table) withGap(n int) *table {
	t.gap = n
	return t
}

func (t *table) withPalette(p glyphPalette) *table {
	t.palette = p
	return t
}

func (t *table) add(cells ...string) { t.rows = append(t.rows, cells) }

func (t *table) addTotal(cells ...string) {
	t.rule[len(t.rows)] = true
	t.rows = append(t.rows, cells)
}

// columnGap separates adjacent columns.
const columnGap = 2

func (t *table) render(b *strings.Builder) {
	widths := t.widths()

	// A table whose headers are all blank is a labelled block rather than a
	// column listing, and printing an empty header line would just add a gap.
	if hasHeader(t.headers) {
		header := t.line(t.headers, widths)
		if t.palette != nil {
			header = t.palette.Heading(header)
		}
		b.WriteString(indent)
		b.WriteString(header)
		b.WriteByte('\n')
	}

	for i, row := range t.rows {
		if t.rule[i] {
			b.WriteByte('\n')
		}
		b.WriteString(indent)
		b.WriteString(t.line(row, widths))
		b.WriteByte('\n')
	}
}

func (t *table) widths() []int {
	widths := make([]int, len(t.headers))
	for i, h := range t.headers {
		widths[i] = visibleWidth(h)
	}
	for _, row := range t.rows {
		for i, cell := range row {
			if i >= len(widths) {
				continue
			}
			if n := visibleWidth(cell); n > widths[i] {
				widths[i] = n
			}
		}
	}
	return widths
}

func (t *table) line(cells []string, widths []int) string {
	var sb strings.Builder
	for i := range widths {
		var cell string
		if i < len(cells) {
			cell = cells[i]
		}
		if i > 0 {
			sb.WriteString(strings.Repeat(" ", t.gap))
		}
		if i < t.labels {
			sb.WriteString(padRight(cell, widths[i]))
		} else {
			sb.WriteString(padLeft(cell, widths[i]))
		}
	}
	// Trailing padding on the final column would show up as trailing
	// whitespace, which diffs and some terminals highlight.
	return strings.TrimRight(sb.String(), " ")
}

func hasHeader(headers []string) bool {
	for _, h := range headers {
		if h != "" {
			return true
		}
	}
	return false
}

// truncLeft shortens a label by dropping its head, which is where the shared
// prefix of a slugified path sits; the tail is what tells two projects apart.
func truncLeft(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return "…" + string(r[len(r)-max+1:])
}

func truncLeftASCII(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return "..." + string(r[len(r)-max+3:])
}
