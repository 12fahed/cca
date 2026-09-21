package render

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"cca/internal/config"
	"cca/internal/report"
	"cca/internal/water"
)

// env builds a getenv for the injected environment a test wants.
func env(pairs ...string) func(string) string {
	m := map[string]string{}
	for i := 0; i+1 < len(pairs); i += 2 {
		m[pairs[i]] = pairs[i+1]
	}
	return func(k string) string { return m[k] }
}

// tty stands in for a terminal; a bytes.Buffer stands in for a pipe.
func tty(t *testing.T) *os.File {
	t.Helper()
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Skipf("no character device available: %v", err)
	}
	t.Cleanup(func() { f.Close() })
	info, err := f.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		t.Skip("os.DevNull is not a character device here")
	}
	return f
}

func TestPaletteZeroValueIsPlain(t *testing.T) {
	var p Palette
	if p.Enabled() {
		t.Error("the zero palette must be disabled")
	}
	if got := p.Dim("x"); got != "x" {
		t.Errorf("Dim on a disabled palette = %q, want %q", got, "x")
	}
	if got := p.Bold("x"); got != "x" {
		t.Errorf("Bold on a disabled palette = %q, want %q", got, "x")
	}
}

// Output that is not going to a terminal must be plain, so a redirect to a file
// or a pipe into another tool does not collect escape sequences.
func TestColorDisabledWhenNotATerminal(t *testing.T) {
	p := NewPalette(ColorOptions{Out: &bytes.Buffer{}, Getenv: env("TERM", "xterm-256color")})
	if p.Enabled() {
		t.Error("colour should be off when stdout is not a terminal")
	}
}

func TestColorEnabledOnATerminal(t *testing.T) {
	p := NewPalette(ColorOptions{Out: tty(t), Getenv: env("TERM", "xterm-256color"), GOOS: "linux"})
	if !p.Enabled() {
		t.Error("colour should be on for a terminal")
	}
	if !strings.Contains(p.Dim("x"), "\x1b[") {
		t.Error("an enabled palette should emit escapes")
	}
}

func TestNoColorFlagWins(t *testing.T) {
	p := NewPalette(ColorOptions{Out: tty(t), Disabled: true,
		Getenv: env("TERM", "xterm"), GOOS: "linux"})
	if p.Enabled() {
		t.Error("--no-color must win over everything else")
	}
}

// The NO_COLOR convention is that the variable's presence is the signal,
// whatever its value.
func TestNoColorEnvHonoured(t *testing.T) {
	for _, value := range []string{"1", "true", "0", "no"} {
		p := NewPalette(ColorOptions{Out: tty(t), GOOS: "linux",
			Getenv: env("NO_COLOR", value, "TERM", "xterm")})
		if p.Enabled() {
			t.Errorf("NO_COLOR=%q should disable colour", value)
		}
	}
}

// CLICOLOR_FORCE is how a user keeps colour while paging through a pipe.
func TestForcedColorSurvivesAPipe(t *testing.T) {
	p := NewPalette(ColorOptions{Out: &bytes.Buffer{}, GOOS: "linux",
		Getenv: env("CLICOLOR_FORCE", "1", "TERM", "xterm")})
	if !p.Enabled() {
		t.Error("CLICOLOR_FORCE should allow colour through a pipe")
	}
	// But an explicit refusal still wins.
	p = NewPalette(ColorOptions{Out: &bytes.Buffer{}, Disabled: true, GOOS: "linux",
		Getenv: env("CLICOLOR_FORCE", "1")})
	if p.Enabled() {
		t.Error("--no-color must beat CLICOLOR_FORCE")
	}
}

func TestDumbTerminalGetsNoColor(t *testing.T) {
	p := NewPalette(ColorOptions{Out: tty(t), GOOS: "linux", Getenv: env("TERM", "dumb")})
	if p.Enabled() {
		t.Error("TERM=dumb should disable colour")
	}
}

// Windows is the case worth being careful about: modern terminals handle ANSI,
// but legacy conhost prints the escapes literally. Colour is therefore opt-in
// there, keyed on a marker only a capable terminal sets.
func TestWindowsRequiresAKnownTerminal(t *testing.T) {
	tests := []struct {
		name string
		envs []string
		want bool
	}{
		{"bare conhost", nil, false},
		{"windows terminal", []string{"WT_SESSION", "abc"}, true},
		{"ansicon", []string{"ANSICON", "1"}, true},
		{"conemu", []string{"ConEmuANSI", "ON"}, true},
		{"vs code", []string{"TERM_PROGRAM", "vscode"}, true},
		{"git bash", []string{"TERM", "xterm"}, true},
		{"dumb wins", []string{"WT_SESSION", "abc", "TERM", "dumb"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPalette(ColorOptions{Out: tty(t), GOOS: "windows", Getenv: env(tt.envs...)})
			if p.Enabled() != tt.want {
				t.Errorf("enabled = %v, want %v", p.Enabled(), tt.want)
			}
		})
	}
}

// Styling must not disturb column alignment. Columns are measured against
// visible width, so a styled table has to print identically to a plain one once
// the escapes are removed. This holds for every view, not just one.
func TestColorDoesNotChangeLayout(t *testing.T) {
	rep := viewReport(t, report.Options{})
	for name, view := range allViews() {
		t.Run(name, func(t *testing.T) {
			plain := renderTo(t, view, rep)
			var colored strings.Builder
			if err := view(&colored, rep, Options{Color: Palette{enabled: true}}); err != nil {
				t.Fatal(err)
			}
			if got := stripANSI(colored.String()); got != plain {
				t.Errorf("colour changed the layout\n--- stripped ---\n%s\n--- plain ---\n%s",
					got, plain)
			}
		})
	}

	t.Run("summary", func(t *testing.T) {
		plain := render(t, rep, false)
		var colored strings.Builder
		opts := Options{Color: Palette{enabled: true},
			Water: water.For(rep.Overall.Tokens.Total(), water.DefaultMLPer1kTokens)}
		if err := Summary(&colored, rep, opts); err != nil {
			t.Fatal(err)
		}
		if got := stripANSI(colored.String()); got != plain {
			t.Errorf("colour changed the summary layout\n--- stripped ---\n%s\n--- plain ---\n%s",
				got, plain)
		}
	})
}

// Each kind of quantity gets its own hue, so a reader can find a figure by
// colour. Distinctness is the property worth pinning; the exact codes are free
// to be retuned.
func TestSemanticStylesAreDistinct(t *testing.T) {
	p := Palette{enabled: true}
	seen := map[string]string{}
	for name, styled := range map[string]string{
		"tokens": p.Tokens("x"), "cost": p.Cost("x"), "water": p.Water("x"),
		"warn": p.Warn("x"), "heading": p.Heading("x"),
	} {
		if styled == "x" {
			t.Errorf("%s produced no styling", name)
		}
		if prev, dup := seen[styled]; dup && prev != "heading" && name != "muted" {
			t.Errorf("%s and %s render identically", name, prev)
		}
		seen[styled] = name
	}
}

func TestSemanticStylesArePlainWhenDisabled(t *testing.T) {
	var p Palette
	for name, styled := range map[string]string{
		"tokens": p.Tokens("x"), "cost": p.Cost("x"), "water": p.Water("x"),
		"warn": p.Warn("x"), "heading": p.Heading("x"), "muted": p.Muted("x"),
		"strong": p.Strong("x"),
	} {
		if styled != "x" {
			t.Errorf("%s styled a disabled palette: %q", name, styled)
		}
	}
}

// stripANSI defers to the production helper: it already handles both SGR runs
// and OSC 8 hyperlinks, and reusing it means the test cannot drift from what
// the layout code actually measures.
func stripANSI(s string) string { return stripEscapes(s) }

// A documentation reference becomes a terminal hyperlink where the terminal can
// render one, and stays plain words where it cannot. Splicing a raw URL into
// piped output would change what every script sees for no gain.
func TestDocsReferenceLinksOnlyWhenStyled(t *testing.T) {
	plain := docsRef(Palette{}, "README")
	if plain != "README" {
		t.Errorf("unstyled reference = %q, want plain text", plain)
	}

	linked := docsRef(Palette{enabled: true}, "README")
	if !strings.Contains(linked, RepoURL) {
		t.Errorf("styled reference does not carry the URL: %q", linked)
	}
	if !strings.Contains(linked, "\x1b]8;;") {
		t.Errorf("styled reference is not an OSC 8 hyperlink: %q", linked)
	}
	// The link text still reads as the word, and occupies its width.
	if got := visibleWidth(linked); got != len("README") {
		t.Errorf("hyperlink measures %d columns, want %d", got, len("README"))
	}
	if !strings.Contains(stripEscapes(linked), "README") {
		t.Errorf("link text lost: %q", stripEscapes(linked))
	}
}

func TestLinkIsInertWithoutAURL(t *testing.T) {
	p := Palette{enabled: true}
	if got := p.Link("text", ""); got != "text" {
		t.Errorf("Link with no URL = %q, want the text unchanged", got)
	}
}

// Every documentation reference in terminal output points at the project, so a
// reader can reach the explanation without first locating the source.
func TestFootnotesLinkToTheProject(t *testing.T) {
	rep := viewReport(t, report.Options{})
	opts := Options{Color: Palette{enabled: true},
		Water: water.For(rep.Overall.Tokens.Total(), water.DefaultMLPer1kTokens)}

	var b strings.Builder
	if err := Summary(&b, rep, opts); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.Contains(out, "README") {
		t.Fatal("the water caveat should still reference the README")
	}
	if strings.Count(out, RepoURL) < 1 {
		t.Errorf("the README reference is not linked to %s", RepoURL)
	}
}

// The diagnostics and configuration views are built from the same table helper
// and must hold their columns under colour too. They are not in allViews
// because neither takes the same arguments as a report view.
func TestColorDoesNotChangeDiagnosticsOrConfigLayout(t *testing.T) {
	rep := viewReport(t, report.Options{})
	styled := Options{Color: Palette{enabled: true}}

	t.Run("diagnostics", func(t *testing.T) {
		var plain, colored strings.Builder
		if err := Diagnostics(&plain, rep, Options{}); err != nil {
			t.Fatal(err)
		}
		if err := Diagnostics(&colored, rep, styled); err != nil {
			t.Fatal(err)
		}
		if got := stripEscapes(colored.String()); got != plain.String() {
			t.Errorf("colour changed the diagnostics layout\n--- stripped ---\n%s\n--- plain ---\n%s",
				got, plain.String())
		}
	})

	t.Run("config", func(t *testing.T) {
		cfg := config.Resolved{
			WaterMLPer1k: 0.3,
			ClaudeDir:    "/home/someone/.claude",
			ConfigPath:   "/home/someone/.config/cca/config.json",
			ConfigFound:  true,
			Sources: map[string]config.Source{
				config.KeyWater:     config.FromFlag,
				config.KeyClaudeDir: config.FromDefault,
			},
		}
		var plain, colored strings.Builder
		if err := Config(&plain, cfg, "embedded default", Options{}); err != nil {
			t.Fatal(err)
		}
		if err := Config(&colored, cfg, "embedded default", styled); err != nil {
			t.Fatal(err)
		}
		if got := stripEscapes(colored.String()); got != plain.String() {
			t.Errorf("colour changed the config layout\n--- stripped ---\n%s\n--- plain ---\n%s",
				got, plain.String())
		}
	})
}

// A setting that came from a flag or a config file is why cca is behaving as it
// is, so it should not look the same as an untouched default.
func TestConfigHighlightsNonDefaultSources(t *testing.T) {
	cfg := config.Resolved{
		WaterMLPer1k: 0.9,
		ClaudeDir:    "/home/someone/.claude",
		ConfigPath:   "/cfg.json",
		Sources: map[string]config.Source{
			config.KeyWater:     config.FromFlag,
			config.KeyClaudeDir: config.FromDefault,
		},
	}
	var b strings.Builder
	if err := Config(&b, cfg, "embedded default", Options{Color: Palette{enabled: true}}); err != nil {
		t.Fatal(err)
	}
	// Only the settings table carries a source column; the footnotes mention
	// the same key without being rows.
	var checked int
	for _, line := range strings.Split(b.String(), "\n") {
		plain := stripEscapes(line)
		if !strings.HasSuffix(plain, string(config.FromFlag)) &&
			!strings.HasSuffix(plain, string(config.FromDefault)) {
			continue
		}
		checked++
		switch {
		case strings.Contains(plain, config.KeyWater):
			if !strings.Contains(line, ansiWarn) {
				t.Errorf("a flag-supplied setting should stand out: %q", line)
			}
		case strings.Contains(plain, config.KeyClaudeDir):
			if strings.Contains(line, ansiWarn) {
				t.Errorf("an untouched default should not stand out: %q", line)
			}
		}
	}
	if checked < 2 {
		t.Fatalf("expected to inspect both settings rows, saw %d", checked)
	}
}
