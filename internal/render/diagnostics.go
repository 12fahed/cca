package render

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"cca/internal/config"
	"cca/internal/report"
)

// Diagnostics writes the --verbose block: what was scanned, what was dropped,
// and why. It is what makes the headline numbers auditable rather than asking
// the reader to trust them.
func Diagnostics(w io.Writer, rep *report.Report, opts Options) error {
	g := glyphsFor(opts.ASCII)
	var b strings.Builder
	st := rep.Stats

	p := opts.Color
	fmt.Fprintf(&b, "\n%s%s\n", indent, p.Heading(fmt.Sprintf("Scan %s %s, %s", g.sep,
		Plural(st.Files, "file", "files"), Plural(int(st.Lines), "line", "lines"))))

	t := newTable(1, "", "count").withPalette(p)
	t.add("usage records kept", p.Strong(Count(st.Records)))
	// Duplicates are the headline diagnostic: the same assistant message is
	// replayed into several files, and without deduplication every total would
	// be inflated by roughly the ratio shown here.
	seen := st.Records + st.Duplicates
	t.add("duplicates dropped", fmt.Sprintf("%s %s", Count(st.Duplicates),
		p.Muted("("+share(float64(st.Duplicates), float64(seen))+")")))

	reasons := make([]string, 0, len(st.Skipped))
	for r := range st.Skipped {
		reasons = append(reasons, r)
	}
	sort.Strings(reasons)
	for _, r := range reasons {
		t.add("skipped: "+r, p.Muted(Count(st.Skipped[r])))
	}

	for label, n := range map[string]int64{
		"records with no cache TTL split":  st.FlatCacheFallback,
		"cache split disagreed with total": st.CacheSplitMismatch,
		"records with no usable timestamp": st.BadTimestamp,
		"records with no dedup key":        st.NoDedupKey,
	} {
		if n > 0 {
			// These counts each mark something cca could not do cleanly, so
			// they are flagged rather than listed alongside routine figures.
			t.add(label, p.Warn(Count(n)))
		}
	}
	if rep.Filtered > 0 {
		t.add("outside the window", p.Muted(Count(rep.Filtered)))
	}
	t.render(&b)

	if len(rep.Warnings) > 0 {
		fmt.Fprintf(&b, "\n%s%s\n", indent, p.Heading("Warnings"))
		for _, warn := range rep.Warnings {
			subject := warn.Subject
			if subject == "" {
				subject = warn.Kind
			}
			fmt.Fprintf(&b, "%s%s %s %s %s\n", indent, p.Warn(g.bullet),
				p.Warn(subject), p.Muted("(x"+Count(warn.Count)+")"),
				p.Muted(warn.Detail))
		}
	}

	_, err := io.WriteString(w, b.String())
	return err
}

// Config writes the resolved settings alongside where each one came from, so a
// surprising result can be traced to the layer that caused it.
func Config(w io.Writer, cfg config.Resolved, tableOrigin string, opts Options) error {
	g := glyphsFor(opts.ASCII)
	var b strings.Builder

	p := opts.Color
	fmt.Fprintf(&b, "%s%s\n\n", indent,
		p.Heading("Claude Code analyzer "+g.sep+" resolved configuration"))

	// A value that came from a flag or a file is the reason cca is behaving as
	// it is, so the source column is the part worth picking out.
	source := func(key string) string {
		src := cfg.Sources[key]
		if src == config.FromDefault {
			return p.Muted(string(src))
		}
		return p.Warn(string(src))
	}

	t := newTable(3, "Setting", "value", "from").withPalette(p)
	t.add(config.KeyClaudeDir, cfg.ClaudeDir, source(config.KeyClaudeDir))
	t.add(config.KeyWater, p.Water(fmt.Sprintf("%.2f mL / 1k tokens", cfg.WaterMLPer1k)),
		source(config.KeyWater))
	t.add(config.KeyNoSidechains, fmt.Sprintf("%t", cfg.NoSidechains),
		source(config.KeyNoSidechains))
	t.add(config.KeyASCII, fmt.Sprintf("%t", cfg.ASCII), source(config.KeyASCII))
	t.add(config.KeyNoColor, fmt.Sprintf("%t", cfg.NoColor), source(config.KeyNoColor))
	t.render(&b)

	fmt.Fprintf(&b, "\n%s%s\n", indent, p.Heading("Files"))
	f := newTable(3, "File", "path", "status").withPalette(p)
	f.add("config", cfg.ConfigPath, presentStyled(cfg.ConfigFound, p))
	pricing := cfg.PricingPath
	if pricing == "" {
		pricing, _ = config.PricingPath()
	}
	f.add("pricing", pricing, p.Muted(tableOrigin))
	f.render(&b)

	b.WriteString("\n")
	writeNoteStyled(&b, g, opts.Color,
		"The water rate is a rough placeholder, not a measured value.\n"+
			"Set "+config.KeyWater+" in the config file to substitute your own; see "+
			docsRef(opts.Color, "README")+".")
	if !cfg.ConfigFound {
		writeNoteStyled(&b, g, opts.Color,
			"No config file yet. Create one at the path above to change these defaults;\n"+
				"every key is optional and any flag overrides it.")
	}

	_, err := io.WriteString(w, b.String())
	return err
}

func presentStyled(found bool, p Palette) string {
	if found {
		return p.Cost("found")
	}
	return p.Muted("not present")
}
