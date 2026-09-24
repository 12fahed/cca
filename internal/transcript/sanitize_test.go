package transcript

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// esc builds escape sequences without putting literal control bytes in this
// source file.
func esc() string { return string(rune(27)) }

func TestSanitizeStripsANSI(t *testing.T) {
	tests := map[string]string{
		esc() + "[31mred title" + esc() + "[0m":                                              "red title",
		"before" + esc() + "[1;32mafter":                                                     "beforeafter",
		esc() + "]8;;https://evil.example" + esc() + `\click` + esc() + "]8;;" + esc() + `\`: "click",
		esc() + "c" + "reset attempt":                                                        "reset attempt",
		"plain":                                                                              "plain",
	}
	for in, want := range tests {
		if got := sanitizeTitle(in); got != want {
			t.Errorf("sanitizeTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

// A title is user-controlled text going into a terminal table. Escapes are an
// injection vector; control characters wreck alignment even when harmless.
func TestSanitizeStripsControlCharacters(t *testing.T) {
	in := "a" + string(rune(0x07)) + "b" + string(rune(0x00)) + "c" + string(rune(0x7f)) + "d"
	if got := sanitizeTitle(in); got != "abcd" {
		t.Errorf("got %q, want %q", got, "abcd")
	}
	// C1 controls too.
	if got := sanitizeTitle("x" + string(rune(0x9b)) + "y"); got != "xy" {
		t.Errorf("got %q, want %q", got, "xy")
	}
}

// A title has to occupy exactly one line and one run of spaces between words.
func TestSanitizeCollapsesWhitespace(t *testing.T) {
	tests := map[string]string{
		"two  spaces":             "two spaces",
		"line\nbreak":             "line break",
		"tab\tseparated":          "tab separated",
		"  leading and trailing ": "leading and trailing",
		"mixed \t\n  runs":        "mixed runs",
		"\n\n\n":                  "",
		"   ":                     "",
	}
	for in, want := range tests {
		got := sanitizeTitle(in)
		if got != want {
			t.Errorf("sanitizeTitle(%q) = %q, want %q", in, got, want)
		}
		if strings.Contains(got, "\n") {
			t.Errorf("sanitizeTitle(%q) left a newline", in)
		}
	}
}

func TestSanitizeKeepsLegitimateText(t *testing.T) {
	for _, s := range []string{
		"review: netflow ERG E_26 audit toggle",
		"fix: display dash for empty role",
		"日本語のタイトル",
		"emoji \U0001f680 title",
		"punctuation: a/b, c-d (e) [f] {g} 100%",
	} {
		if got := sanitizeTitle(s); got != s {
			t.Errorf("sanitizeTitle(%q) altered it to %q", s, got)
		}
	}
}

func TestSanitizeProducesValidUTF8(t *testing.T) {
	// A lone continuation byte is not valid UTF-8 and must not survive as a
	// replacement character, which would occupy a column it does not deserve.
	in := "good" + string([]byte{0xff, 0xfe}) + "text"
	got := sanitizeTitle(in)
	if !utf8.ValidString(got) {
		t.Errorf("sanitizeTitle produced invalid UTF-8: %q", got)
	}
	if strings.ContainsRune(got, utf8.RuneError) {
		t.Errorf("sanitizeTitle leaked a replacement character: %q", got)
	}
	if got != "goodtext" {
		t.Errorf("got %q, want %q", got, "goodtext")
	}
}

// Sanitization happens on the way in, so a stored title is already safe and
// machine-readable output cannot carry an escape sequence either.
func TestTitlesAreSanitizedWhenCollected(t *testing.T) {
	const s = "bbbb2222-0000-4000-8000-000000000001"
	dirty := esc() + "[31mdanger" + esc() + "[0m\nsecond line"
	got := titlesFrom(t, customTitle(s, dirty))

	if strings.ContainsRune(got[s].Text, 27) {
		t.Errorf("stored title still contains an escape: %q", got[s].Text)
	}
	if strings.Contains(got[s].Text, "\n") {
		t.Errorf("stored title still contains a newline: %q", got[s].Text)
	}
	if got[s].Text != "danger second line" {
		t.Errorf("got %q", got[s].Text)
	}
}

// A title consisting only of escapes and control characters sanitizes to
// nothing, which must be treated as absent rather than rendered blank.
func TestTitleThatSanitizesToNothingFallsThrough(t *testing.T) {
	const s = "bbbb2222-0000-4000-8000-000000000002"
	got := titlesFrom(t,
		aiTitle(s, "a real summary"),
		customTitle(s, esc()+"[1m"+esc()+"[0m"),
	)
	if got[s].Source != TitleAI || got[s].Text != "a real summary" {
		t.Errorf("got %+v, want the ai title", got[s])
	}
}
