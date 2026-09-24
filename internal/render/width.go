package render

import (
	"strings"
	"unicode"
)

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
//
// Columns, not runes. A CJK ideograph or an emoji occupies two cells, and a
// combining mark occupies none, so counting runes misaligns any table carrying
// text the user controls — project directory names and session titles both
// qualify.
func visibleWidth(s string) int {
	n := 0
	for _, r := range stripEscapes(s) {
		n += runeWidth(r)
	}
	return n
}

// runeWidth returns the number of terminal cells r occupies.
//
// This is the narrow slice of wcwidth cca actually needs, implemented here
// rather than taken from a dependency because the project ships with none. The
// combining and format categories come from the standard library's Unicode
// tables, which are complete and maintained; only the East Asian width ranges,
// which the standard library does not expose, are carried below.
func runeWidth(r rune) int {
	switch {
	case r == 0:
		return 0
	case r < 0x20 || (r >= 0x7f && r < 0xa0):
		// Control characters are stripped before display; if one survives, it
		// prints as nothing.
		return 0
	case unicode.Is(unicode.Mn, r), unicode.Is(unicode.Me, r):
		// Combining marks attach to the preceding character.
		return 0
	case unicode.Is(unicode.Cf, r):
		// Format characters: zero-width joiner, bidi marks, and friends.
		return 0
	case isWide(r):
		return 2
	}
	return 1
}

// wideRanges are the East Asian Wide and Fullwidth blocks, plus the emoji
// blocks that render double-width. Sorted, so lookup is a binary search.
var wideRanges = [...][2]rune{
	{0x1100, 0x115f},   // Hangul Jamo initial consonants
	{0x231a, 0x231b},   // watch, hourglass
	{0x2329, 0x232a},   // angle brackets
	{0x23e9, 0x23ec},   // media controls
	{0x23f0, 0x23f0},   // alarm clock
	{0x23f3, 0x23f3},   // hourglass flowing
	{0x25fd, 0x25fe},   // medium small squares
	{0x2614, 0x2615},   // umbrella, hot beverage
	{0x2648, 0x2653},   // zodiac
	{0x267f, 0x267f},   // wheelchair
	{0x2693, 0x2693},   // anchor
	{0x26a1, 0x26a1},   // high voltage
	{0x26aa, 0x26ab},   // circles
	{0x26bd, 0x26be},   // soccer, baseball
	{0x26c4, 0x26c5},   // snowman, sun behind cloud
	{0x26ce, 0x26ce},   // ophiuchus
	{0x26d4, 0x26d4},   // no entry
	{0x26ea, 0x26ea},   // church
	{0x26f2, 0x26f3},   // fountain, golf
	{0x26f5, 0x26f5},   // sailboat
	{0x26fa, 0x26fa},   // tent
	{0x26fd, 0x26fd},   // fuel pump
	{0x2705, 0x2705},   // check mark button
	{0x270a, 0x270b},   // raised fist, raised hand
	{0x2728, 0x2728},   // sparkles
	{0x274c, 0x274c},   // cross mark
	{0x274e, 0x274e},   // cross mark button
	{0x2753, 0x2755},   // question marks
	{0x2757, 0x2757},   // exclamation
	{0x2795, 0x2797},   // heavy math signs
	{0x27b0, 0x27b0},   // curly loop
	{0x27bf, 0x27bf},   // double curly loop
	{0x2b1b, 0x2b1c},   // large squares
	{0x2b50, 0x2b50},   // star
	{0x2b55, 0x2b55},   // hollow red circle
	{0x2e80, 0x303e},   // CJK radicals through CJK symbols
	{0x3041, 0x33ff},   // kana, Bopomofo, Hangul compat, enclosed CJK
	{0x3400, 0x4dbf},   // CJK extension A
	{0x4e00, 0x9fff},   // CJK unified ideographs
	{0xa000, 0xa4cf},   // Yi
	{0xa960, 0xa97f},   // Hangul Jamo extended-A
	{0xac00, 0xd7a3},   // Hangul syllables
	{0xf900, 0xfaff},   // CJK compatibility ideographs
	{0xfe10, 0xfe19},   // vertical forms
	{0xfe30, 0xfe6f},   // CJK compatibility forms, small form variants
	{0xff00, 0xff60},   // fullwidth forms
	{0xffe0, 0xffe6},   // fullwidth signs
	{0x1f300, 0x1f64f}, // emoji: symbols, pictographs, emoticons
	{0x1f680, 0x1f6ff}, // emoji: transport and map
	{0x1f7e0, 0x1f7eb}, // emoji: coloured circles and squares
	{0x1f900, 0x1f9ff}, // emoji: supplemental symbols and pictographs
	{0x1fa70, 0x1faff}, // emoji: extended-A
	{0x20000, 0x2fffd}, // CJK extensions B onwards
	{0x30000, 0x3fffd}, // CJK extension G onwards
}

func isWide(r rune) bool {
	lo, hi := 0, len(wideRanges)-1
	for lo <= hi {
		mid := (lo + hi) / 2
		switch {
		case r < wideRanges[mid][0]:
			hi = mid - 1
		case r > wideRanges[mid][1]:
			lo = mid + 1
		default:
			return true
		}
	}
	return false
}

// truncateWidth shortens s to at most max columns, appending an ellipsis when
// it had to cut.
//
// Truncation is by display width rather than by byte or rune count, so a title
// of CJK or emoji lands in the same column as an ASCII one. A UTF-8 sequence is
// never split, and a double-width character is never half-printed: if it would
// straddle the boundary it is dropped and the cell is left one column short.
func truncateWidth(s string, max int, ascii bool) string {
	if max <= 0 {
		return ""
	}
	if visibleWidth(s) <= max {
		return s
	}
	ellipsis := "…"
	if ascii {
		ellipsis = "..."
	}
	budget := max - visibleWidth(ellipsis)
	if budget <= 0 {
		return ellipsis
	}

	var b strings.Builder
	used := 0
	for _, r := range s {
		w := runeWidth(r)
		if used+w > budget {
			break
		}
		b.WriteRune(r)
		used += w
	}
	return b.String() + ellipsis
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
