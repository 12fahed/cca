package transcript

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"time"
)

// Record types cca recognises. Everything else is skipped.
const (
	typeAssistant   = "assistant"
	typeCostState   = "cost-state"
	typeCustomTitle = "custom-title"
	typeAITitle     = "ai-title"
	typeUser        = "user"
)

// unmarshal is json.Unmarshal, named so the cost-state decoder reads the same
// way as the record decoder.
func unmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }

// maxLineBytes caps a single record. Transcript lines carry whole tool results
// and can reach megabytes, so the limit is generous; anything past it is
// skipped rather than allowed to abort the file.
const maxLineBytes = 16 << 20

// Skip reasons, counted per run and surfaced by --verbose.
const (
	SkipMalformed    = "malformed-json"
	SkipNotAssistant = "not-assistant"
	SkipNoUsage      = "no-usage"
	SkipNonModel     = "non-model"
	SkipSidechain    = "sidechain-excluded"
	SkipLineTooLong  = "line-too-long"
)

// Usage holds one request's billed token counts, already split by rate class.
type Usage struct {
	Input        int64
	Output       int64
	CacheWrite5m int64
	CacheWrite1h int64
	CacheRead    int64
	// Thinking is a subset of Output, not an addend. Reasoning tokens are
	// billed as output tokens; this field reports how many of them there were.
	// Never add it to Output.
	Thinking int64
	// WebSearches is billable per request. Errored searches are not billed but
	// are indistinguishable here, so treat the count as an upper bound.
	WebSearches int64
}

// Total returns every billed token in the record. Thinking is excluded because
// it is already counted inside Output.
func (u Usage) Total() int64 {
	return u.Input + u.Output + u.CacheWrite5m + u.CacheWrite1h + u.CacheRead
}

// Record is one deduplicated assistant response with its usage and attribution.
type Record struct {
	MessageID   string
	RequestID   string
	UUID        string
	SessionID   string
	Model       string
	Project     string
	CWD         string
	Timestamp   time.Time
	IsSidechain bool

	// Rate-selection inputs. These pick the price row, so they are parsed here
	// rather than treated as metadata.
	Speed        string // "standard" or "fast"; absent normalizes to "standard"
	ServiceTier  string // "standard" or a batch tier; absent normalizes to "standard"
	InferenceGeo string // lowercased as found; "" when absent

	Usage Usage
}

// Stats records what a run saw, including everything it chose to drop.
type Stats struct {
	Files      int
	Lines      int64
	Records    int64 // emitted, after dedup and filtering
	Duplicates int64
	Skipped    map[string]int64

	// FlatCacheFallback counts records with no nested cache_creation object,
	// whose cache writes were therefore all attributed to the 5-minute TTL.
	FlatCacheFallback int64
	// CacheSplitMismatch counts records where the nested 5m+1h values disagree
	// with the flat total, which would indicate a schema change.
	CacheSplitMismatch int64
	// BadTimestamp counts records whose timestamp did not parse; they are kept
	// with a zero time and must be excluded from per-day views.
	BadTimestamp int64
	// NoDedupKey counts records with neither a message id and request id pair
	// nor a uuid. They are kept, since dropping them would lose real spend.
	NoDedupKey int64

	// Titles maps sessionId to the resolved display title, for the sessions
	// that have one. It rides here because it is gathered by the same pass;
	// nothing in the cost or token path may read it.
	Titles map[string]SessionTitle
}

// Options configures a parse run. The zero value includes sidechain traffic,
// which matches the default: sub-agent usage is real spend.
type Options struct {
	ClaudeDir string
	// NonModels lists model strings that are placeholders rather than models,
	// such as "<synthetic>". They are dropped before unknown-model detection so
	// they never trigger a spurious warning.
	NonModels         []string
	ExcludeSidechains bool
}

type Parser struct {
	opts       Options
	nonModels  map[string]bool
	seen       map[string]struct{}
	stats      Stats
	costStates map[string]CostState

	// Title collection rides along in the same streaming pass as usage, so the
	// transcripts are read exactly once.
	titles      map[string]*titleSet
	firstPrompt map[string]struct{}
	current     fileContext
}

func New(opts Options) *Parser {
	nm := make(map[string]bool, len(opts.NonModels))
	for _, m := range opts.NonModels {
		nm[m] = true
	}
	return &Parser{
		opts:        opts,
		nonModels:   nm,
		seen:        make(map[string]struct{}),
		stats:       Stats{Skipped: make(map[string]int64)},
		costStates:  make(map[string]CostState),
		titles:      make(map[string]*titleSet),
		firstPrompt: make(map[string]struct{}),
	}
}

func (p *Parser) Stats() Stats { return p.stats }

// Load discovers and parses every transcript under opts.ClaudeDir. A nil error
// with zero records means the directory exists but holds no usage.
func Load(opts Options) ([]Record, Stats, error) {
	files, err := Discover(opts.ClaudeDir)
	if err != nil {
		return nil, Stats{Skipped: map[string]int64{}}, err
	}
	p := New(opts)
	var all []Record
	for _, f := range files {
		recs, err := p.ParseFile(f)
		if err != nil {
			return nil, p.Stats(), err
		}
		all = append(all, recs...)
	}
	stats := p.Stats()
	stats.Titles = p.Titles()
	return all, stats, nil
}

// ParseFile streams one transcript. The file is opened read-only.
func (p *Parser) ParseFile(f File) ([]Record, error) {
	fh, err := os.Open(f.Path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	p.stats.Files++
	// Title ordering needs to know which file a record came from; see
	// titleCandidate.beats.
	p.current = newFileContext(f)
	defer func() { p.current = fileContext{} }()
	return p.ParseReader(fh, f.Project)
}

// ParseReader streams newline-delimited records from r, attributing each to
// project. Deduplication state is held on the Parser, so successive calls
// deduplicate across files as well as within one.
func (p *Parser) ParseReader(r io.Reader, project string) ([]Record, error) {
	// A bufio.Reader rather than a Scanner: a Scanner stops the entire file
	// when one line exceeds its buffer, and a single oversized line must never
	// cost us the rest of the transcript.
	br := bufio.NewReaderSize(r, 256<<10)
	var out []Record
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			p.stats.Lines++
			if rec, ok := p.parseLine(line, project); ok {
				out = append(out, rec)
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return out, nil
			}
			return out, err
		}
	}
}

func (p *Parser) skip(reason string) {
	p.stats.Skipped[reason]++
}

func (p *Parser) parseLine(line []byte, project string) (Record, bool) {
	line = bytes.TrimRight(line, "\r\n")
	if len(line) == 0 {
		return Record{}, false
	}
	if len(line) > maxLineBytes {
		p.skip(SkipLineTooLong)
		return Record{}, false
	}

	var raw rawRecord
	if err := json.Unmarshal(line, &raw); err != nil {
		p.skip(SkipMalformed)
		return Record{}, false
	}
	if raw.Type != typeAssistant {
		// Records that are not usage still carry information cca wants, and
		// the format grows new types between Claude Code releases. Recognised
		// ones are handled; everything else is counted as skipped rather than
		// malformed, so a new type does not look like corruption.
		switch raw.Type {
		case typeCostState:
			p.recordCostState(line)
		case typeCustomTitle:
			p.recordTitle(line, TitleCustom)
		case typeAITitle:
			p.recordTitle(line, TitleAI)
		case typeUser:
			p.recordFirstPrompt(line)
		}
		p.skip(SkipNotAssistant)
		return Record{}, false
	}
	if raw.Message == nil || raw.Message.Usage == nil {
		p.skip(SkipNoUsage)
		return Record{}, false
	}
	if p.nonModels[raw.Message.Model] {
		p.skip(SkipNonModel)
		return Record{}, false
	}

	// Deduplicate before the sidechain filter so that dedup counts stay
	// identical whether or not sidechains are excluded.
	if key := dedupKey(&raw); key == "" {
		p.stats.NoDedupKey++
	} else if _, dup := p.seen[key]; dup {
		p.stats.Duplicates++
		return Record{}, false
	} else {
		p.seen[key] = struct{}{}
	}

	if raw.IsSidechain && p.opts.ExcludeSidechains {
		p.skip(SkipSidechain)
		return Record{}, false
	}

	rec := Record{
		MessageID:    raw.Message.ID,
		RequestID:    raw.RequestID,
		UUID:         raw.UUID,
		SessionID:    raw.SessionID,
		Model:        raw.Message.Model,
		Project:      project,
		CWD:          raw.CWD,
		IsSidechain:  raw.IsSidechain,
		Speed:        normalize(raw.Message.Usage.Speed, "standard"),
		ServiceTier:  normalize(raw.Message.Usage.ServiceTier, "standard"),
		InferenceGeo: normalize(raw.Message.Usage.InferenceGeo, ""),
		Usage:        p.usage(raw.Message.Usage),
	}
	if raw.Timestamp != "" {
		if ts, err := time.Parse(time.RFC3339, raw.Timestamp); err == nil {
			rec.Timestamp = ts
		} else {
			p.stats.BadTimestamp++
		}
	} else {
		p.stats.BadTimestamp++
	}

	p.stats.Records++
	return rec, true
}

func (p *Parser) usage(u *rawUsage) Usage {
	out := Usage{
		Input:     u.InputTokens,
		Output:    u.OutputTokens,
		CacheRead: u.CacheReadInputTokens,
	}
	if u.OutputTokensDetails != nil {
		out.Thinking = u.OutputTokensDetails.ThinkingTokens
	}
	if u.ServerToolUse != nil {
		out.WebSearches = u.ServerToolUse.WebSearchRequests
	}

	// The nested breakdown is authoritative when present: 5-minute and 1-hour
	// cache writes bill at different rates, and the 1-hour class dominates in
	// practice, so collapsing them would understate cost badly.
	if u.CacheCreation != nil {
		out.CacheWrite5m = u.CacheCreation.Ephemeral5m
		out.CacheWrite1h = u.CacheCreation.Ephemeral1h
		if out.CacheWrite5m+out.CacheWrite1h != u.CacheCreationInputTokens {
			p.stats.CacheSplitMismatch++
		}
	} else {
		out.CacheWrite5m = u.CacheCreationInputTokens
		p.stats.FlatCacheFallback++
	}
	return out
}

// dedupKey identifies one assistant response. The same message is replayed into
// several files by resumed sessions, compaction, and sidechains, so this is what
// keeps totals from inflating.
func dedupKey(r *rawRecord) string {
	if r.Message != nil && r.Message.ID != "" && r.RequestID != "" {
		return r.Message.ID + "\x00" + r.RequestID
	}
	return r.UUID
}

func normalize(s *string, fallback string) string {
	if s == nil {
		return fallback
	}
	v := strings.ToLower(strings.TrimSpace(*s))
	if v == "" {
		return fallback
	}
	return v
}
