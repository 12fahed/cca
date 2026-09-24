package transcript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func customTitle(session, title string) string {
	return `{"type":"custom-title","sessionId":"` + session + `","customTitle":` + quote(title) + `}`
}

// The two record types carry their text under different keys and in a different
// key order, which is why decoding is by field name. The fixtures mirror that.
func aiTitle(session, title string) string {
	return `{"type":"ai-title","aiTitle":` + quote(title) + `,"sessionId":"` + session + `"}`
}

func userText(session, text string) string {
	return `{"type":"user","sessionId":"` + session + `","isSidechain":false,` +
		`"message":{"role":"user","content":` + quote(text) + `}}`
}

func userBlocks(session, blockType, text string) string {
	return `{"type":"user","sessionId":"` + session + `","isSidechain":false,` +
		`"message":{"role":"user","content":[{"type":"` + blockType + `","text":` + quote(text) + `}]}}`
}

func sidechainText(session, text string) string {
	return `{"type":"user","sessionId":"` + session + `","isSidechain":true,` +
		`"message":{"role":"user","content":` + quote(text) + `}}`
}

func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				b.WriteString(`\u00`)
				const hex = "0123456789abcdef"
				b.WriteByte(hex[(r>>4)&0xf])
				b.WriteByte(hex[r&0xf])
				continue
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// titlesFrom parses a single unnamed stream, for cases where file provenance
// does not matter.
func titlesFrom(t *testing.T, lines ...string) map[string]SessionTitle {
	t.Helper()
	p := New(Options{})
	if _, err := p.ParseReader(strings.NewReader(strings.Join(lines, "\n")+"\n"), "proj"); err != nil {
		t.Fatal(err)
	}
	return p.Titles()
}

func TestCustomTitleBeatsAITitle(t *testing.T) {
	const s = "aaaa1111-0000-4000-8000-000000000001"
	// Order reversed on purpose: precedence is by kind, not by position.
	got := titlesFrom(t,
		aiTitle(s, "generated summary"),
		customTitle(s, "chosen name"),
		aiTitle(s, "regenerated summary"),
	)
	if got[s].Text != "chosen name" || got[s].Source != TitleCustom {
		t.Errorf("got %+v, want the custom title", got[s])
	}
}

func TestAITitleUsedWhenNoCustom(t *testing.T) {
	const s = "aaaa1111-0000-4000-8000-000000000002"
	got := titlesFrom(t, aiTitle(s, "generated summary"))
	if got[s].Text != "generated summary" || got[s].Source != TitleAI {
		t.Errorf("got %+v", got[s])
	}
}

// Records are appended, not headers: a session renamed three times carries
// three of them and only the last is current.
func TestLastRecordOfAKindWins(t *testing.T) {
	const s = "aaaa1111-0000-4000-8000-000000000003"
	got := titlesFrom(t,
		customTitle(s, "first name"),
		customTitle(s, "second name"),
		customTitle(s, "third name"),
	)
	if got[s].Text != "third name" {
		t.Errorf("got %q, want the last record", got[s].Text)
	}
}

// A cleared title must fall through rather than render an empty cell.
func TestEmptyAndWhitespaceTitlesFallThrough(t *testing.T) {
	for name, title := range map[string]string{
		"empty":      "",
		"spaces":     "   ",
		"whitespace": " \t \n ",
	} {
		t.Run(name, func(t *testing.T) {
			const s = "aaaa1111-0000-4000-8000-000000000004"
			got := titlesFrom(t, aiTitle(s, "generated"), customTitle(s, title))
			if got[s].Source != TitleAI || got[s].Text != "generated" {
				t.Errorf("got %+v, want the ai title", got[s])
			}
		})
	}
}

func TestNoTitleAtAll(t *testing.T) {
	const s = "aaaa1111-0000-4000-8000-000000000005"
	got := titlesFrom(t, `{"type":"mode","sessionId":"`+s+`"}`)
	if _, ok := got[s]; ok {
		t.Errorf("a session with no title source should not appear: %+v", got[s])
	}
}

func TestFirstPromptFallback(t *testing.T) {
	const s = "aaaa1111-0000-4000-8000-000000000006"
	got := titlesFrom(t,
		userText(s, "help me refactor the billing module"),
		userText(s, "and then write tests"),
	)
	if got[s].Source != TitleFirstPrompt {
		t.Fatalf("got source %q", got[s].Source)
	}
	if got[s].Text != "help me refactor the billing module" {
		t.Errorf("got %q, want the first prompt", got[s].Text)
	}
}

// The fallback exists for sessions with no title records; a real title always
// displaces it.
func TestFirstPromptLosesToRealTitles(t *testing.T) {
	const s = "aaaa1111-0000-4000-8000-000000000007"
	got := titlesFrom(t, userText(s, "a prompt"), aiTitle(s, "generated"))
	if got[s].Source != TitleAI {
		t.Errorf("got %+v, want the ai title", got[s])
	}
}

// Command wrappers, hook payloads and compaction preambles are inserted on the
// user's behalf and describe nothing about the session.
func TestFirstPromptSkipsInjectedContent(t *testing.T) {
	const s = "aaaa1111-0000-4000-8000-000000000008"
	got := titlesFrom(t,
		sidechainText(s, "sub-agent instructions"),
		userBlocks(s, "tool_result", "command output here"),
		userText(s, "<command-name>/fk-development</command-name>"),
		userText(s, "<task-notification>done</task-notification>"),
		userText(s, "This session is being continued from a previous conversation"),
		userText(s, "the actual question"),
	)
	if got[s].Text != "the actual question" {
		t.Errorf("got %q, want the first genuine prompt", got[s].Text)
	}
}

// User content arrives either as a bare string or as a block list, and the
// overwhelming majority of block lists are tool output.
func TestFirstPromptReadsBothContentShapes(t *testing.T) {
	const s = "aaaa1111-0000-4000-8000-000000000009"
	got := titlesFrom(t, userBlocks(s, "text", "a block-shaped prompt"))
	if got[s].Text != "a block-shaped prompt" {
		t.Errorf("got %q", got[s].Text)
	}
}

// writeTitleTree lays out files with explicit modification times, since the
// cross-file ordering rules depend on them.
func writeTitleTree(t *testing.T, files map[string][]string, times map[string]time.Time) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "projects", "proj")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, lines := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if ts, ok := times[name]; ok {
			if err := os.Chtimes(path, ts, ts); err != nil {
				t.Fatal(err)
			}
		}
	}
	return root
}

func titlesFromTree(t *testing.T, root string) map[string]SessionTitle {
	t.Helper()
	files, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	p := New(Options{})
	for _, f := range files {
		if _, err := p.ParseFile(f); err != nil {
			t.Fatal(err)
		}
	}
	return p.Titles()
}

// A forked transcript carries the original session's title records. Attaching
// them by filename would relabel every branched session.
func TestTitlesAttachByRecordSessionIDNotFilename(t *testing.T) {
	const original = "11111111-0000-4000-8000-00000000000a"
	const forked = "22222222-0000-4000-8000-00000000000b"

	root := writeTitleTree(t, map[string][]string{
		original + ".jsonl": {customTitle(original, "the original")},
		// The fork copies the original's records, then gets its own.
		forked + ".jsonl": {customTitle(original, "the original"), customTitle(forked, "the fork")},
	}, nil)

	got := titlesFromTree(t, root)
	if got[original].Text != "the original" {
		t.Errorf("original = %q", got[original].Text)
	}
	if got[forked].Text != "the fork" {
		t.Errorf("fork = %q, want its own title", got[forked].Text)
	}
}

// A session's own file is the authority on its own name, whatever the
// modification times say.
func TestNamedFileWinsOverOtherFiles(t *testing.T) {
	const s = "33333333-0000-4000-8000-00000000000c"
	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	recent := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

	root := writeTitleTree(t, map[string][]string{
		s + ".jsonl":  {customTitle(s, "from its own file")},
		"other.jsonl": {customTitle(s, "from a newer copy")},
	}, map[string]time.Time{
		s + ".jsonl":  old,
		"other.jsonl": recent,
	})

	if got := titlesFromTree(t, root); got[s].Text != "from its own file" {
		t.Errorf("got %q, want the session's own file to win", got[s].Text)
	}
}

// With no named file to prefer, the later modification time wins.
func TestNewerFileWinsAmongUnnamedFiles(t *testing.T) {
	const s = "44444444-0000-4000-8000-00000000000d"
	root := writeTitleTree(t, map[string][]string{
		"older.jsonl": {customTitle(s, "older")},
		"newer.jsonl": {customTitle(s, "newer")},
	}, map[string]time.Time{
		"older.jsonl": time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		"newer.jsonl": time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
	})

	if got := titlesFromTree(t, root); got[s].Text != "newer" {
		t.Errorf("got %q, want the newer file", got[s].Text)
	}
}

// Equal timestamps must still produce one answer, not whichever the filesystem
// happened to hand back first.
func TestEqualModTimesResolveDeterministically(t *testing.T) {
	const s = "55555555-0000-4000-8000-00000000000e"
	same := time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC)

	build := func() string {
		return writeTitleTree(t, map[string][]string{
			"aaa.jsonl": {customTitle(s, "from aaa")},
			"zzz.jsonl": {customTitle(s, "from zzz")},
		}, map[string]time.Time{"aaa.jsonl": same, "zzz.jsonl": same})
	}

	first := titlesFromTree(t, build())[s].Text
	if first != "from zzz" {
		t.Errorf("got %q, want the lexicographically later path", first)
	}
	// Rebuilding must not change the answer.
	for i := 0; i < 5; i++ {
		if again := titlesFromTree(t, build())[s].Text; again != first {
			t.Fatalf("run %d disagreed: %q vs %q", i, again, first)
		}
	}
}

// The resolution rules must not depend on the order files happen to be read.
func TestCrossFileResultIsOrderIndependent(t *testing.T) {
	const s = "66666666-0000-4000-8000-00000000000f"
	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

	files := map[string][]string{
		"one.jsonl": {customTitle(s, "from one")},
		"two.jsonl": {customTitle(s, "from two")},
	}
	times := map[string]time.Time{"one.jsonl": older, "two.jsonl": newer}

	root := writeTitleTree(t, files, times)
	discovered, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}

	forward := New(Options{})
	for _, f := range discovered {
		if _, err := forward.ParseFile(f); err != nil {
			t.Fatal(err)
		}
	}
	reverse := New(Options{})
	for i := len(discovered) - 1; i >= 0; i-- {
		if _, err := reverse.ParseFile(discovered[i]); err != nil {
			t.Fatal(err)
		}
	}

	a, b := forward.Titles()[s], reverse.Titles()[s]
	if a != b {
		t.Errorf("order changed the result: %+v vs %+v", a, b)
	}
	if a.Text != "from two" {
		t.Errorf("got %q, want the newer file", a.Text)
	}
}

// A session can be renamed and produce no assistant turns at all.
func TestTitleWithoutUsageIsStillCollected(t *testing.T) {
	const s = "77777777-0000-4000-8000-000000000010"
	root := writeTitleTree(t, map[string][]string{
		s + ".jsonl": {customTitle(s, "renamed but never ran")},
	}, nil)

	files, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	p := New(Options{})
	var records int
	for _, f := range files {
		got, err := p.ParseFile(f)
		if err != nil {
			t.Fatal(err)
		}
		records += len(got)
	}
	if records != 0 {
		t.Errorf("expected no usage records, got %d", records)
	}
	if p.Titles()[s].Text != "renamed but never ran" {
		t.Errorf("title lost for a session with no usage: %+v", p.Titles()[s])
	}
}

// Two sessions may legitimately carry the same title; nothing may group by it.
func TestDuplicateTitlesAreKeptSeparate(t *testing.T) {
	const a = "88888888-0000-4000-8000-000000000011"
	const b = "99999999-0000-4000-8000-000000000012"
	got := titlesFrom(t, customTitle(a, "same name"), customTitle(b, "same name"))
	if got[a].Text != "same name" || got[b].Text != "same name" {
		t.Errorf("got %+v and %+v", got[a], got[b])
	}
	if len(got) != 2 {
		t.Errorf("want 2 sessions, got %d", len(got))
	}
}

// Title collection must not disturb usage accounting.
func TestTitleRecordsAreNotCountedAsUsage(t *testing.T) {
	const s = "aaaaaaaa-0000-4000-8000-000000000013"
	p := New(Options{})
	body := customTitle(s, "a name") + "\n" +
		aiTitle(s, "a summary") + "\n" +
		assistant("m", "r", "u", "claude-opus-5", "") + "\n"
	recs, err := p.ParseReader(strings.NewReader(body), "proj")
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 {
		t.Fatalf("want 1 usage record, got %d", len(recs))
	}
	if p.Stats().Records != 1 {
		t.Errorf("Records = %d, want 1", p.Stats().Records)
	}
}

// Malformed title records are ignored rather than fatal, like every other line.
func TestMalformedTitleRecordsAreIgnored(t *testing.T) {
	got := titlesFrom(t,
		`{"type":"custom-title","customTitle":"no session id"}`,
		`{"type":"ai-title","sessionId":"","aiTitle":"blank session"}`,
	)
	if len(got) != 0 {
		t.Errorf("want none, got %+v", got)
	}
}
