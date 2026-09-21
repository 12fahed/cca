package render

import (
	"io"
	"os"
	"runtime"
	"strings"
)

// Palette styles terminal output. Its zero value writes plain text, so every
// path that forgets to configure colour degrades to something correct.
type Palette struct {
	enabled bool
}

// ANSI sequences.
//
// The palette is deliberately small and semantic: one hue per kind of quantity,
// so a reader learns it once and can then find a figure by colour rather than
// by reading the header. Everything that is not a quantity stays dim, which
// keeps the numbers the brightest thing on screen.
//
// 256-colour codes are used rather than the 8 basic ones because the basic
// palette is remapped by most themes, and a terminal that understands SGR at
// all has understood 256 colours for many years.
const (
	ansiReset = "\x1b[0m"
	ansiDim   = "\x1b[2m"
	ansiBold  = "\x1b[1m"

	// Warm orange for token counts, echoing Claude's own palette.
	ansiTokens = "\x1b[38;5;209m"
	// Green for money, the long-standing convention for currency.
	ansiCost = "\x1b[38;5;114m"
	// Light blue for the water figure: it reads as water, and it separates the
	// playful estimate from the two figures meant to be taken seriously.
	ansiWater = "\x1b[38;5;117m"
	// Amber for anything the reader is being warned about.
	ansiWarn = "\x1b[38;5;214m"
	// Underlined blue for a hyperlink, for terminals that render OSC 8 without
	// styling it themselves.
	ansiLink = "\x1b[4;38;5;111m"
)

func (p Palette) Enabled() bool { return p.enabled }

// Dim de-emphasises supporting text such as headers and footnotes.
func (p Palette) Dim(s string) string { return p.wrap(ansiDim, s) }

// Bold marks the figures a reader is most likely looking for.
func (p Palette) Bold(s string) string { return p.wrap(ansiBold, s) }

// Semantic styles. Call sites name what a value means rather than what colour
// it should be, so the scheme can be retuned in one place.

// Tokens styles a token count.
func (p Palette) Tokens(s string) string { return p.wrap(ansiTokens, s) }

// Cost styles a money figure.
func (p Palette) Cost(s string) string { return p.wrap(ansiCost, s) }

// Water styles the water estimate and its equivalence.
func (p Palette) Water(s string) string { return p.wrap(ansiWater, s) }

// Warn styles something the reader is being cautioned about.
func (p Palette) Warn(s string) string { return p.wrap(ansiWarn, s) }

// Heading styles a column header row.
func (p Palette) Heading(s string) string { return p.wrap(ansiDim, s) }

// Muted styles supporting prose beside a figure.
func (p Palette) Muted(s string) string { return p.wrap(ansiDim, s) }

// Strong styles a totals row.
func (p Palette) Strong(s string) string { return p.wrap(ansiBold, s) }

func (p Palette) wrap(code, s string) string {
	if !p.enabled || s == "" {
		return s
	}
	return code + s + ansiReset
}

// ColorOptions describes everything that decides whether to emit escapes.
type ColorOptions struct {
	// Out is the destination being written to.
	Out io.Writer
	// Disabled reflects --no-color.
	Disabled bool
	// Getenv reads the environment; nil means os.Getenv.
	Getenv func(string) string
	// GOOS overrides runtime.GOOS, for tests.
	GOOS string
}

// NewPalette decides whether colour is safe to emit.
//
// The order matters. An explicit refusal wins over everything; NO_COLOR is
// honoured whatever its value, per the convention that its mere presence is the
// signal; a forced setting then allows colour through a pipe, which is how a
// user pages output while keeping it; and otherwise colour requires a terminal.
func NewPalette(opts ColorOptions) Palette {
	getenv := opts.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	goos := opts.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}

	if opts.Disabled {
		return Palette{}
	}
	if _, present := lookupEnv(getenv, "NO_COLOR"); present {
		return Palette{}
	}
	forced := getenv("CLICOLOR_FORCE") != "" && getenv("CLICOLOR_FORCE") != "0"
	if !forced && !isTerminal(opts.Out) {
		return Palette{}
	}
	if !ansiCapable(goos, getenv) {
		return Palette{}
	}
	return Palette{enabled: true}
}

// isTerminal reports whether w is a character device.
//
// Checking the file mode keeps this dependency-free and works on both Unix and
// Windows, where a console handle also reports as a character device.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// ansiCapable reports whether the terminal understands escape sequences.
//
// Everywhere but Windows, a character device that is not explicitly dumb is
// assumed capable. Windows is the exception: ANSI works in Windows Terminal and
// PowerShell 7 but legacy conhost renders the escapes literally, and enabling
// virtual terminal processing to find out would mean a syscall dependency. So
// Windows must positively identify itself through one of the markers a modern
// terminal sets. Losing colour is a far smaller harm than printing garbage.
func ansiCapable(goos string, getenv func(string) string) bool {
	if strings.EqualFold(getenv("TERM"), "dumb") {
		return false
	}
	if goos != "windows" {
		return true
	}
	for _, key := range []string{"WT_SESSION", "ANSICON", "ConEmuANSI", "TERM_PROGRAM"} {
		if getenv(key) != "" {
			return true
		}
	}
	// A TERM value at all on Windows implies an emulator that sets one, such as
	// Git Bash or MSYS.
	return getenv("TERM") != ""
}

// lookupEnv reports presence as well as value, since NO_COLOR counts even when
// it is set to the empty string.
func lookupEnv(getenv func(string) string, key string) (string, bool) {
	if v := getenv(key); v != "" {
		return v, true
	}
	// os.LookupEnv distinguishes unset from empty; an injected getenv cannot,
	// so fall back to it when reading the real environment.
	return os.LookupEnv(key)
}
