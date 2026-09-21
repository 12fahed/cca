package render

import (
	"strings"
	"testing"
)

const (
	orange = "\x1b[38;5;209m"
	reset  = "\x1b[0m"
	link   = "\x1b]8;;https://example.com\x1b\\"
	unlink = "\x1b]8;;\x1b\\"
)

func TestVisibleWidthIgnoresEscapes(t *testing.T) {
	tests := map[string]int{
		"":                         0,
		"603.8M":                   6,
		orange + "603.8M" + reset:  6,
		orange + "" + reset:        0,
		link + "README" + unlink:   6,
		"a" + orange + "b" + reset: 2,
		"≈ 18.0 L":                 8,
		// A bare escape with no sequence must not swallow the text after it.
		"\x1bplain": 6,
	}
	for in, want := range tests {
		if got := visibleWidth(in); got != want {
			t.Errorf("visibleWidth(%q) = %d, want %d", in, got, want)
		}
	}
}

// OSC 8 terminates with BEL as well as with ST, and both forms are in the wild.
func TestVisibleWidthHandlesBothOSCTerminators(t *testing.T) {
	bel := "\x1b]8;;https://example.com\x07README\x1b]8;;\x07"
	if got := visibleWidth(bel); got != 6 {
		t.Errorf("BEL-terminated OSC 8 measured %d, want 6", got)
	}
}

func TestPadding(t *testing.T) {
	styled := orange + "42" + reset
	if got := padLeft(styled, 6); visibleWidth(got) != 6 {
		t.Errorf("padLeft visible width = %d, want 6", visibleWidth(got))
	}
	if got := padRight(styled, 6); visibleWidth(got) != 6 {
		t.Errorf("padRight visible width = %d, want 6", visibleWidth(got))
	}
	// The styling has to survive padding, or the colour is lost.
	if !strings.Contains(padLeft(styled, 6), orange) {
		t.Error("padLeft dropped the escape sequence")
	}
	// Padding never truncates.
	if got := padLeft("toolong", 3); visibleWidth(got) != 7 {
		t.Errorf("padLeft truncated: %q", got)
	}
}

// A styled cell must occupy the same columns as its plain equivalent, which is
// the whole reason this package measures width itself.
func TestStyledAndPlainCellsPadIdentically(t *testing.T) {
	for _, plain := range []string{"1", "42", "603.8M", "$1,234.56"} {
		styled := orange + plain + reset
		if got, want := stripEscapes(padLeft(styled, 12)), padLeft(plain, 12); got != want {
			t.Errorf("styled %q pads to %q, plain pads to %q", plain, got, want)
		}
		if got, want := stripEscapes(padRight(styled, 12)), padRight(plain, 12); got != want {
			t.Errorf("styled %q pads to %q, plain pads to %q", plain, got, want)
		}
	}
}

func TestStripEscapes(t *testing.T) {
	if got := stripEscapes("plain"); got != "plain" {
		t.Errorf("got %q", got)
	}
	if got := stripEscapes(orange + "x" + reset); got != "x" {
		t.Errorf("got %q", got)
	}
	if got := stripEscapes(link + "x" + unlink); got != "x" {
		t.Errorf("got %q", got)
	}
}
