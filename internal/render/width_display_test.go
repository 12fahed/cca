package render

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// Columns, not runes. Counting runes misaligns any table carrying text the user
// controls, which project names and session titles both are.
func TestRuneWidth(t *testing.T) {
	tests := map[rune]int{
		'a':     1,
		' ':     1,
		'…':     1, // horizontal ellipsis
		'中':     2, // CJK ideograph
		'한':     2, // Hangul syllable
		'あ':     2, // Hiragana
		'Ａ':     2, // fullwidth Latin A
		'́':     0, // combining acute accent
		'‍':     0, // zero-width joiner
		'️':     0, // variation selector 16
		0x0000:  0, // NUL
		0x1f600: 2, // grinning face
		0x1f680: 2, // rocket
		0x2705:  2, // check mark button
		'é':     1, // precomposed e-acute
	}
	for r, want := range tests {
		if got := runeWidth(r); got != want {
			t.Errorf("runeWidth(U+%04X) = %d, want %d", r, got, want)
		}
	}
}

func TestVisibleWidthCountsColumns(t *testing.T) {
	tests := map[string]int{
		"hello":                 5,
		"中文":                    4, // two ideographs
		"a中b":                   4,
		"\U0001f680 launch":     9, // rocket is two columns
		"é":                    1, // e plus combining acute is one column
		"\U0001f468‍\U0001f4bb": 4, // two pictographs joined; the joiner is zero
	}
	for in, want := range tests {
		if got := visibleWidth(in); got != want {
			t.Errorf("visibleWidth(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestTruncateWidth(t *testing.T) {
	tests := []struct {
		in   string
		max  int
		want string
	}{
		{"short", 10, "short"},
		{"exactly-10", 10, "exactly-10"},
		{"truncate me please", 10, "truncate …"},
		{"", 5, ""},
		{"anything", 0, ""},
	}
	for _, tt := range tests {
		if got := truncateWidth(tt.in, tt.max, false); got != tt.want {
			t.Errorf("truncateWidth(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
		}
	}
}

// Under --ascii the marker is three characters rather than one, so the budget
// differs; what must hold either way is the width bound.
func TestTruncateWidthASCIIMarker(t *testing.T) {
	got := truncateWidth("truncate me please", 12, true)
	if !strings.HasSuffix(got, "...") {
		t.Errorf("got %q, want an ASCII ellipsis", got)
	}
	if w := visibleWidth(got); w > 12 {
		t.Errorf("got %q, width %d exceeds the budget", got, w)
	}
	if strings.ContainsRune(got, '…') {
		t.Errorf("ASCII output should not contain the Unicode ellipsis: %q", got)
	}
}

// Truncation is by display width, so a CJK or emoji title lands in the same
// column as an ASCII one rather than overflowing it.
func TestTruncateWidthRespectsWideCharacters(t *testing.T) {
	for _, in := range []string{
		"日本語のセッション",
		strings.Repeat("\U0001f680", 12),
		"mixed 中文 and ascii text here",
		"plain ascii title that is quite long",
	} {
		for _, max := range []int{4, 8, 12, 20} {
			got := truncateWidth(in, max, false)
			if w := visibleWidth(got); w > max {
				t.Errorf("truncateWidth(%q, %d) = %q, width %d exceeds budget", in, max, got, w)
			}
		}
	}
}

// A double-width character must never be half-printed at the boundary: it is
// dropped and the cell left one column short instead.
func TestTruncateWidthNeverSplitsAWideRune(t *testing.T) {
	const s = "中文字符測試" // six ideographs, twelve columns
	tests := map[int]string{
		5: "中文…",
		6: "中文…", // the third would straddle, so it is dropped
		7: "中文字…",
	}
	for max, want := range tests {
		if got := truncateWidth(s, max, false); got != want {
			t.Errorf("truncateWidth(max=%d) = %q, want %q", max, got, want)
		}
	}
}

func TestTruncateWidthProducesValidUTF8(t *testing.T) {
	for _, in := range []string{
		"日本語テスト",
		"\U0001f680 launch pad",
		"héllo wörld",
	} {
		for max := 1; max <= 12; max++ {
			got := truncateWidth(in, max, false)
			if !utf8.ValidString(got) {
				t.Errorf("truncateWidth(%q, %d) produced invalid UTF-8: %q", in, max, got)
			}
		}
	}
}

// Padding is what actually holds a table together, so it has to agree with the
// width measurement for wide text too.
func TestPaddingWithWideCharacters(t *testing.T) {
	for _, s := range []string{"ascii", "中文", "\U0001f680x", "é"} {
		for _, w := range []int{8, 12} {
			if got := visibleWidth(padRight(s, w)); got != w {
				t.Errorf("padRight(%q, %d) measured %d columns", s, w, got)
			}
			if got := visibleWidth(padLeft(s, w)); got != w {
				t.Errorf("padLeft(%q, %d) measured %d columns", s, w, got)
			}
		}
	}
}
