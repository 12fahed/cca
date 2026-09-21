package render

import "strings"

// Terminal escape sequences occupy no columns on screen but are still bytes in
// the string, so any layout measuring with len would over-pad a styled cell and
// leave the column ragged.
//
// text/tabwriter cannot solve this. Its Escape mechanism treats a bracketed
// segment as one character wide rather than zero, so a styled cell still
// measures wider than it prints. Columns are therefore measured and padded here
// against visible width, which also keeps a styled table byte-identical in
// layout to an unstyled one.

// visibleWidth returns how many columns s occupies once escape sequences are
// discounted.
func visibleWidth(s string) int {
	n := 0
	for range stripEscapes(s) {
		n++
	}
	return n
}

// stripEscapes removes the escape sequences cca emits: SGR colour runs
// (ESC [ ... m) and OSC 8 hyperlinks (ESC ] 8 ; ; ... ST).
func stripEscapes(s string) string {
	if !strings.ContainsRune(s, 0x1b) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] != 0x1b {
			b.WriteByte(s[i])
			i++
			continue
		}
		if j := endOfEscape(s, i); j > i {
			i = j
			continue
		}
		// A lone ESC with no recognisable sequence: keep it rather than swallow
		// the rest of the string.
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// endOfEscape returns the index just past the escape sequence starting at i, or
// i when there is no complete sequence there.
func endOfEscape(s string, i int) int {
	if i+1 >= len(s) {
		return i
	}
	switch s[i+1] {
	case '[': // CSI: runs to a final byte in the range @ to ~
		for j := i + 2; j < len(s); j++ {
			if s[j] >= 0x40 && s[j] <= 0x7e {
				return j + 1
			}
		}
	case ']': // OSC: runs to BEL, or to ST written as ESC backslash
		for j := i + 2; j < len(s); j++ {
			if s[j] == 0x07 {
				return j + 1
			}
			if s[j] == 0x1b && j+1 < len(s) && s[j+1] == '\\' {
				return j + 2
			}
		}
	}
	return i
}

// padRight left-aligns s in a field of the given width.
func padRight(s string, width int) string {
	if n := visibleWidth(s); n < width {
		return s + strings.Repeat(" ", width-n)
	}
	return s
}

// padLeft right-aligns s in a field of the given width.
func padLeft(s string, width int) string {
	if n := visibleWidth(s); n < width {
		return strings.Repeat(" ", width-n) + s
	}
	return s
}
