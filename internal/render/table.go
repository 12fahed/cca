package render

import (
	"bytes"
	"fmt"
	"strings"
	"text/tabwriter"
)

// table lays out rows with a run of left-aligned label columns followed by
// right-aligned numeric columns.
//
// tabwriter aligns every column the same way, so the label cells are padded to
// a common width first; right-aligning an already-full-width cell is a no-op,
// which leaves the alignment flag acting on the numbers alone.
type table struct {
	headers []string
	// labels is how many leading columns are left-aligned.
	labels int
	rows   [][]string
	// rule marks rows that should be preceded by a blank line, used to set a
	// total apart from the body.
	rule map[int]bool
}

func newTable(labels int, headers ...string) *table {
	return &table{headers: headers, labels: labels, rule: map[int]bool{}}
}

func (t *table) add(cells ...string) { t.rows = append(t.rows, cells) }

func (t *table) addTotal(cells ...string) {
	t.rule[len(t.rows)] = true
	t.rows = append(t.rows, cells)
}

func (t *table) render(b *strings.Builder) {
	widths := make([]int, t.labels)
	for i := 0; i < t.labels && i < len(t.headers); i++ {
		widths[i] = len(t.headers[i])
	}
	for _, row := range t.rows {
		for i := 0; i < t.labels && i < len(row); i++ {
			if n := len([]rune(row[i])); n > widths[i] {
				widths[i] = n
			}
		}
	}

	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 0, tabPadding, ' ', tabwriter.AlignRight)
	fmt.Fprintln(tw, t.line(t.headers, widths))
	for i, row := range t.rows {
		if t.rule[i] {
			fmt.Fprintln(tw, t.line(blanks(len(t.headers)), widths))
		}
		fmt.Fprintln(tw, t.line(row, widths))
	}
	tw.Flush()
	flushTable(b, &buf, "")
}

func (t *table) line(cells []string, widths []int) string {
	var sb strings.Builder
	for i, c := range cells {
		if i < t.labels {
			sb.WriteString(fmt.Sprintf("%-*s", widths[i], c))
		} else {
			sb.WriteString(c)
		}
		sb.WriteByte('\t')
	}
	return sb.String()
}

func blanks(n int) []string { return make([]string, n) }

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
