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

	fmt.Fprintf(&b, "\n%sScan %s %s, %s\n", indent, g.sep,
		Plural(st.Files, "file", "files"), Plural(int(st.Lines), "line", "lines"))

	t := newTable(1, "", "count")
	t.add("usage records kept", Count(st.Records))
	// Duplicates are the headline diagnostic: the same assistant message is
	// replayed into several files, and without deduplication every total would
	// be inflated by roughly the ratio shown here.
	seen := st.Records + st.Duplicates
	t.add("duplicates dropped", fmt.Sprintf("%s (%s)", Count(st.Duplicates), share(float64(st.Duplicates), float64(seen))))

	reasons := make([]string, 0, len(st.Skipped))
	for r := range st.Skipped {
		reasons = append(reasons, r)
	}
	sort.Strings(reasons)
	for _, r := range reasons {
		t.add("skipped: "+r, Count(st.Skipped[r]))
	}

	for label, n := range map[string]int64{
		"records with no cache TTL split":  st.FlatCacheFallback,
		"cache split disagreed with total": st.CacheSplitMismatch,
		"records with no usable timestamp": st.BadTimestamp,
		"records with no dedup key":        st.NoDedupKey,
	} {
		if n > 0 {
			t.add(label, Count(n))
		}
	}
	if rep.Filtered > 0 {
		t.add("outside the window", Count(rep.Filtered))
	}
	t.render(&b)

	if len(rep.Warnings) > 0 {
		fmt.Fprintf(&b, "\n%sWarnings\n", indent)
		for _, warn := range rep.Warnings {
			subject := warn.Subject
			if subject == "" {
				subject = warn.Kind
			}
			fmt.Fprintf(&b, "%s%s %s (x%s) %s\n", indent, g.bullet,
				subject, Count(warn.Count), warn.Detail)
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

	fmt.Fprintf(&b, "%sClaude Code analyzer %s resolved configuration\n\n", indent, g.sep)

	t := newTable(3, "Setting", "value", "from")
	t.add(config.KeyClaudeDir, cfg.ClaudeDir, string(cfg.Sources[config.KeyClaudeDir]))
	t.add(config.KeyWater, fmt.Sprintf("%.2f mL / 1k tokens", cfg.WaterMLPer1k),
		string(cfg.Sources[config.KeyWater]))
	t.add(config.KeyNoSidechains, fmt.Sprintf("%t", cfg.NoSidechains),
		string(cfg.Sources[config.KeyNoSidechains]))
	t.add(config.KeyASCII, fmt.Sprintf("%t", cfg.ASCII), string(cfg.Sources[config.KeyASCII]))
	t.add(config.KeyNoColor, fmt.Sprintf("%t", cfg.NoColor), string(cfg.Sources[config.KeyNoColor]))
	t.render(&b)

	fmt.Fprintf(&b, "\n%sFiles\n", indent)
	f := newTable(3, "File", "path", "status")
	f.add("config", cfg.ConfigPath, present(cfg.ConfigFound))
	pricing := cfg.PricingPath
	if pricing == "" {
		pricing, _ = config.PricingPath()
	}
	f.add("pricing", pricing, tableOrigin)
	f.render(&b)

	b.WriteString("\n")
	writeNote(&b, g, "The water rate is a rough placeholder, not a measured value.\n"+
		"Set "+config.KeyWater+" in the config file to substitute your own.")
	if !cfg.ConfigFound {
		writeNote(&b, g, "No config file yet. Create one at the path above to change "+
			"these defaults;\nevery key is optional and any flag overrides it.")
	}

	_, err := io.WriteString(w, b.String())
	return err
}

func present(found bool) string {
	if found {
		return "found"
	}
	return "not present"
}
