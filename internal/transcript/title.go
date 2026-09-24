package transcript

import (
	"path/filepath"
	"strings"
	"time"
)

// TitleSource records which of the several possible origins supplied a session's
// title, so that --verbose can distinguish a name the user chose from one a
// model guessed, or from their own first prompt.
type TitleSource string

const (
	TitleNone        TitleSource = "none"
	TitleCustom      TitleSource = "custom"
	TitleAI          TitleSource = "ai"
	TitleFirstPrompt TitleSource = "first-prompt"
)

// SessionTitle is a resolved, untruncated title for one session.
//
// It is display-only metadata. Nothing in the cost, token, deduplication,
// sorting, or filtering logic may read it — sessionId remains the join key
// everywhere. That containment matters beyond tidiness: there is an upstream
// bug where a reused session UUID inherits an unrelated session's title, and
// cca cannot detect it. A wrong title must never be able to move a number.
type SessionTitle struct {
	Text   string
	Source TitleSource
}

// titleCandidate is one observed title record, carrying enough provenance to be
// ordered against every other candidate for the same session.
type titleCandidate struct {
	source TitleSource
	text   string
	// Provenance, used only by the ordering rules below.
	fromNamedFile bool // the file's basename equals this record's sessionId
	modTime       time.Time
	path          string
	line          int64
}

// beats reports whether c should displace other as the current best candidate
// of the same source kind for one session.
//
// Title records carry no timestamp, so "last" has to be defined rather than
// observed. The order, highest priority first:
//
//  1. A file named <sessionId>.jsonl wins over one that merely mentions the
//     session. /branch and --fork-session copy a transcript into a new file, so
//     a file can contain title records belonging to the session it was forked
//     from; the session's own file is the authority on its own name.
//  2. Otherwise the later modification time wins, since that is the closest
//     thing to a clock these records have.
//  3. Equal modification times fall back to the lexicographically later path,
//     so the result does not depend on filesystem iteration order.
//  4. Within one file, the later line wins — these records are appended, and a
//     session renamed three times has three of them.
func (c titleCandidate) beats(other titleCandidate) bool {
	if c.fromNamedFile != other.fromNamedFile {
		return c.fromNamedFile
	}
	if c.path != other.path {
		if !c.modTime.Equal(other.modTime) {
			return c.modTime.After(other.modTime)
		}
		return c.path > other.path
	}
	return c.line > other.line
}

// titleSet accumulates the best candidate seen for each source kind of one
// session. Precedence between kinds is applied at resolution time, not here,
// because a later ai-title must not displace an earlier custom-title.
type titleSet struct {
	best map[TitleSource]titleCandidate
}

func (t *titleSet) offer(c titleCandidate) {
	if strings.TrimSpace(c.text) == "" {
		// An empty or whitespace-only title is treated as absent, so that a
		// cleared custom title falls through to the generated one rather than
		// rendering a blank cell.
		return
	}
	if t.best == nil {
		t.best = make(map[TitleSource]titleCandidate, 2)
	}
	if prev, ok := t.best[c.source]; ok && !c.beats(prev) {
		return
	}
	t.best[c.source] = c
}

// resolve applies precedence between kinds: a name the user chose beats one a
// model generated, which beats their first prompt.
//
// It is pure — no file access, no clock — so the ordering rules above can be
// tested directly against a constructed candidate stream.
func (t *titleSet) resolve() SessionTitle {
	for _, source := range []TitleSource{TitleCustom, TitleAI, TitleFirstPrompt} {
		if c, ok := t.best[source]; ok {
			if text := strings.TrimSpace(c.text); text != "" {
				return SessionTitle{Text: text, Source: source}
			}
		}
	}
	return SessionTitle{Source: TitleNone}
}

// Titles returns the resolved title for every session that has one.
//
// Sessions are keyed by the sessionId carried *in the record*, never by the
// file it was found in, so a forked transcript cannot relabel the session it
// was forked from.
func (p *Parser) Titles() map[string]SessionTitle {
	out := make(map[string]SessionTitle, len(p.titles))
	for id, set := range p.titles {
		if resolved := set.resolve(); resolved.Source != TitleNone {
			out[id] = resolved
		}
	}
	return out
}

// recordTitle handles a custom-title or ai-title record.
func (p *Parser) recordTitle(line []byte, source TitleSource) {
	var raw rawTitle
	if err := unmarshal(line, &raw); err != nil || raw.SessionID == "" {
		return
	}
	text := raw.CustomTitle
	if source == TitleAI {
		text = raw.AITitle
	}
	p.offerTitle(raw.SessionID, source, text)
}

// recordFirstPrompt captures a user turn as a last-resort title.
//
// Only sessions with neither title record reach this, which in practice means
// `claude -p` runs started from a shell: those never get a generated title. The
// earliest usable turn wins, so the ordering rules deliberately run in reverse
// here — the *first* candidate is kept rather than the last.
func (p *Parser) recordFirstPrompt(line []byte) {
	var raw rawUserRecord
	if err := unmarshal(line, &raw); err != nil || raw.SessionID == "" {
		return
	}
	if raw.IsSidechain {
		// A sub-agent's prompt describes the sub-agent's task, not the
		// session's purpose.
		return
	}
	text := humanText(raw.Message)
	if text == "" {
		return
	}
	if _, seen := p.firstPrompt[raw.SessionID]; seen {
		return
	}
	if p.firstPrompt == nil {
		p.firstPrompt = make(map[string]struct{})
	}
	p.firstPrompt[raw.SessionID] = struct{}{}
	p.offerTitle(raw.SessionID, TitleFirstPrompt, text)
}

func (p *Parser) offerTitle(sessionID string, source TitleSource, text string) {
	if p.titles == nil {
		p.titles = make(map[string]*titleSet)
	}
	set, ok := p.titles[sessionID]
	if !ok {
		set = &titleSet{}
		p.titles[sessionID] = set
	}
	set.offer(titleCandidate{
		source:        source,
		text:          text,
		fromNamedFile: p.current.basename == sessionID,
		modTime:       p.current.modTime,
		path:          p.current.path,
		line:          p.stats.Lines,
	})
}

// humanText extracts something a person actually typed from a user record,
// which may carry its content either as a bare string or as a block list.
func humanText(m *rawUserMessage) string {
	if m == nil {
		return ""
	}
	if s := strings.TrimSpace(m.Content.Text); s != "" {
		if isInjected(s) {
			return ""
		}
		return s
	}
	for _, block := range m.Content.Blocks {
		// tool_result blocks are command output, not a prompt; they are the
		// overwhelming majority of user records.
		if block.Type != "text" {
			continue
		}
		if s := strings.TrimSpace(block.Text); s != "" && !isInjected(s) {
			return s
		}
	}
	return ""
}

// injectedPrefixes mark content Claude Code inserts on the user's behalf. A
// session whose first user record is one of these is not described by it.
var injectedPrefixes = []string{
	"<command-message>",
	"<command-name>",
	"<local-command-stdout>",
	"<local-command-stderr>",
	"<task-notification>",
	"<system-reminder>",
	"<user-prompt-submit-hook>",
	"<ide_opened_file>",
	"caveat: the messages below were generated by the user while running",
	"this session is being continued from a previous conversation",
}

func isInjected(s string) bool {
	lower := strings.ToLower(s)
	for _, prefix := range injectedPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// fileContext is what the title ordering rules need to know about the file
// currently being read.
type fileContext struct {
	path     string
	basename string
	modTime  time.Time
}

func newFileContext(f File) fileContext {
	base := filepath.Base(f.Path)
	return fileContext{
		path:     f.Path,
		basename: strings.TrimSuffix(base, filepath.Ext(base)),
		modTime:  f.ModTime,
	}
}
