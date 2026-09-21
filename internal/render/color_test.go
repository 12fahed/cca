package render

import (
	"bytes"
	"os"
	"strings"
	"testing"

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

func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\x1b' {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			i++
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}
