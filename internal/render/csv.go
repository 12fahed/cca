package render

import (
	"encoding/csv"
	"io"
	"strconv"
	"time"

	"cca/internal/report"
	"cca/internal/transcript"
)

// CSV writes the rows behind the current view.
//
// Unlike the tables, every figure is exact: token counts are not abbreviated,
// costs are not rounded to cents, and session identifiers are full uuids. A
// spreadsheet re-deriving a total from these columns gets the same answer cca
// prints.
func CSV(w io.Writer, rep *report.Report, opts Options) error {
	out := csv.NewWriter(w)
	switch opts.View {
	case ViewProjects:
		writeCSV(out, []string{"project", "sessions", "requests"},
			rep.ByProject, func(g report.Group) []string {
				return []string{g.Key, strconv.Itoa(g.Sessions), strconv.FormatInt(g.Requests, 10)}
			})
	case ViewDaily:
		writeCSV(out, []string{"date", "requests"},
			rep.ByDay, func(g report.Group) []string {
				return []string{g.Key, strconv.FormatInt(g.Requests, 10)}
			})
	case ViewSessions:
		// Titles are opt-in here for the same reason as in JSON: a spreadsheet
		// of session titles describes what someone has been working on.
		headers := []string{"session", "project", "started", "ended", "requests"}
		if opts.ShowTitles {
			headers = append(headers, "title", "title_source")
		}
		writeCSV(out, headers, rep.BySession, func(g report.Group) []string {
			row := []string{g.Key, g.Project, stamp(g.First), stamp(g.Last),
				strconv.FormatInt(g.Requests, 10)}
			if opts.ShowTitles {
				row = append(row, g.Title.Text, titleSourceValue(g.Title.Source))
			}
			return row
		})
	default:
		// The summary and the model view share a table, so they share rows.
		writeCSV(out, []string{"model", "requests"},
			rep.ByModel, func(g report.Group) []string {
				return []string{g.Key, strconv.FormatInt(g.Requests, 10)}
			})
	}
	out.Flush()
	return out.Error()
}

// tokenAndCostHeaders are appended to every view's own columns, so that each
// row carries the full breakdown rather than only the figure its table showed.
var tokenAndCostHeaders = []string{
	"input_tokens", "output_tokens", "cache_write_5m_tokens", "cache_write_1h_tokens",
	"cache_read_tokens", "thinking_tokens", "web_searches", "total_tokens",
	"input_usd", "output_usd", "cache_write_5m_usd", "cache_write_1h_usd",
	"cache_read_usd", "web_search_usd", "total_usd", "unpriced_tokens",
}

func writeCSV(w *csv.Writer, headers []string, groups []report.Group, lead func(report.Group) []string) {
	_ = w.Write(append(append([]string{}, headers...), tokenAndCostHeaders...))
	for _, g := range groups {
		_ = w.Write(append(lead(g), tokenAndCost(g)...))
	}
}

func tokenAndCost(g report.Group) []string {
	t, c := g.Tokens, g.Cost
	return []string{
		i(t.Input), i(t.Output), i(t.CacheWrite5m), i(t.CacheWrite1h),
		i(t.CacheRead), i(t.Thinking), i(t.WebSearches), i(t.Total()),
		f(c.Input), f(c.Output), f(c.CacheWrite5m), f(c.CacheWrite1h),
		f(c.CacheRead), f(c.WebSearch), f(c.Total), i(g.UnpricedTokens),
	}
}

func i(v int64) string { return strconv.FormatInt(v, 10) }

// f formats a cost at full precision rather than to cents. Rounding here would
// stop a column of rows from summing to the total cca reports.
func f(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

// titleSourceValue renders the source as a bare value, with no dash
// substitution: a CSV consumer wants an empty field, not a glyph.
func titleSourceValue(source transcript.TitleSource) string {
	if source == transcript.TitleNone {
		return ""
	}
	return string(source)
}

func stamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
