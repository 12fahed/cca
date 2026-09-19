package transcript

import (
	"fmt"
	"strings"
	"testing"
)

// assistant builds a minimal assistant record. Fields the tests do not care
// about get stable defaults so that each test states only what it exercises.
func assistant(msgID, reqID, uuid, model string, extra string) string {
	return fmt.Sprintf(`{"type":"assistant","uuid":%q,"sessionId":"s1","requestId":%q,`+
		`"timestamp":"2026-09-13T10:36:05.063Z","cwd":"/home/u/p","isSidechain":false,`+
		`"message":{"id":%q,"model":%q,"usage":{"input_tokens":10,"output_tokens":20,`+
		`"cache_creation_input_tokens":300,"cache_read_input_tokens":4000%s}}}`,
		uuid, reqID, msgID, model, extra)
}

func parse(t *testing.T, opts Options, lines ...string) ([]Record, Stats) {
	t.Helper()
	p := New(opts)
	recs, err := p.ParseReader(strings.NewReader(strings.Join(lines, "\n")+"\n"), "proj")
	if err != nil {
		t.Fatalf("ParseReader: %v", err)
	}
	return recs, p.Stats()
}

func TestParseExtractsUsageAndAttribution(t *testing.T) {
	line := assistant("msg_1", "req_1", "u1", "claude-opus-5",
		`,"cache_creation":{"ephemeral_5m_input_tokens":100,"ephemeral_1h_input_tokens":200},`+
			`"output_tokens_details":{"thinking_tokens":7},`+
			`"server_tool_use":{"web_search_requests":3,"web_fetch_requests":9},`+
			`"service_tier":"standard","inference_geo":"US","speed":"fast"`)
	recs, stats := parse(t, Options{}, line)

	if len(recs) != 1 {
		t.Fatalf("want 1 record, got %d", len(recs))
	}
	r := recs[0]
	want := Usage{Input: 10, Output: 20, CacheWrite5m: 100, CacheWrite1h: 200,
		CacheRead: 4000, Thinking: 7, WebSearches: 3}
	if r.Usage != want {
		t.Errorf("usage = %+v, want %+v", r.Usage, want)
	}
	if r.Usage.Total() != 4330 {
		t.Errorf("Total() = %d, want 4330 (thinking must not be added)", r.Usage.Total())
	}
	if r.Model != "claude-opus-5" || r.SessionID != "s1" || r.Project != "proj" || r.CWD != "/home/u/p" {
		t.Errorf("attribution wrong: %+v", r)
	}
	if r.Timestamp.IsZero() || r.Timestamp.UTC().Format("2006-01-02") != "2026-09-13" {
		t.Errorf("timestamp = %v", r.Timestamp)
	}
	if r.Speed != "fast" || r.ServiceTier != "standard" || r.InferenceGeo != "us" {
		t.Errorf("rate inputs: speed=%q tier=%q geo=%q", r.Speed, r.ServiceTier, r.InferenceGeo)
	}
	if stats.CacheSplitMismatch != 0 {
		t.Errorf("100+200 == 300 flat is consistent, got %d mismatches", stats.CacheSplitMismatch)
	}
}

func TestParseFlagsCacheSplitMismatch(t *testing.T) {
	// The nested pair summing to something other than the flat total would mean
	// the transcript schema changed under us; flag it rather than trust either.
	line := assistant("m", "r", "u", "claude-opus-5",
		`,"cache_creation":{"ephemeral_5m_input_tokens":1,"ephemeral_1h_input_tokens":1}`)
	_, stats := parse(t, Options{}, line)
	if stats.CacheSplitMismatch != 1 {
		t.Errorf("1+1 != 300 flat should be flagged, got %d", stats.CacheSplitMismatch)
	}
}

func TestParseNestedCacheSplitPreferredOverFlat(t *testing.T) {
	// The flat field is the sum; the split decides the rate, so it must win.
	line := assistant("m", "r", "u", "claude-opus-5",
		`,"cache_creation":{"ephemeral_5m_input_tokens":50,"ephemeral_1h_input_tokens":250}`)
	recs, stats := parse(t, Options{}, line)
	if recs[0].Usage.CacheWrite5m != 50 || recs[0].Usage.CacheWrite1h != 250 {
		t.Errorf("got 5m=%d 1h=%d, want 50/250", recs[0].Usage.CacheWrite5m, recs[0].Usage.CacheWrite1h)
	}
	if stats.FlatCacheFallback != 0 || stats.CacheSplitMismatch != 0 {
		t.Errorf("clean split should set no flags: %+v", stats)
	}
}

func TestParseFlatCacheFallback(t *testing.T) {
	// No nested object: everything becomes a 5-minute write, and the run is
	// flagged so the output can footnote the approximation.
	recs, stats := parse(t, Options{}, assistant("m", "r", "u", "claude-opus-5", ""))
	if recs[0].Usage.CacheWrite5m != 300 || recs[0].Usage.CacheWrite1h != 0 {
		t.Errorf("got 5m=%d 1h=%d, want 300/0", recs[0].Usage.CacheWrite5m, recs[0].Usage.CacheWrite1h)
	}
	if stats.FlatCacheFallback != 1 {
		t.Errorf("FlatCacheFallback = %d, want 1", stats.FlatCacheFallback)
	}
}

func TestParseNormalizesAbsentRateInputs(t *testing.T) {
	recs, _ := parse(t, Options{}, assistant("m", "r", "u", "claude-opus-5",
		`,"service_tier":null,"inference_geo":null,"speed":null`))
	r := recs[0]
	if r.Speed != "standard" || r.ServiceTier != "standard" || r.InferenceGeo != "" {
		t.Errorf("null rate inputs: speed=%q tier=%q geo=%q", r.Speed, r.ServiceTier, r.InferenceGeo)
	}
}

func TestParseTolerance(t *testing.T) {
	lines := []string{
		`{not json at all`,
		`{"type":"user","uuid":"u0"}`,
		`{"type":"assistant","uuid":"u1","message":{"id":"m","model":"claude-opus-5"}}`,
		assistant("m1", "r1", "u2", "claude-opus-5", ""),
		``,
		`{"type":"assistant","uuid":"u3","message":{"id":"m2","model":"cl`, // truncated
		assistant("m3", "r3", "u4", "claude-opus-5", ""),
	}
	recs, stats := parse(t, Options{}, lines...)
	if len(recs) != 2 {
		t.Fatalf("want 2 surviving records, got %d", len(recs))
	}
	if stats.Skipped[SkipMalformed] != 2 {
		t.Errorf("malformed = %d, want 2", stats.Skipped[SkipMalformed])
	}
	if stats.Skipped[SkipNotAssistant] != 1 {
		t.Errorf("not-assistant = %d, want 1", stats.Skipped[SkipNotAssistant])
	}
	if stats.Skipped[SkipNoUsage] != 1 {
		t.Errorf("no-usage = %d, want 1", stats.Skipped[SkipNoUsage])
	}
}

func TestParseHugeLineBeyondDefaultScannerBuffer(t *testing.T) {
	// Well past bufio.Scanner's 64 KiB default, which would otherwise abort the
	// file and silently lose every record after it.
	huge := assistant("big", "rbig", "ubig", "claude-opus-5",
		`,"padding":"`+strings.Repeat("x", 2<<20)+`"`)
	recs, stats := parse(t, Options{}, huge, assistant("after", "rafter", "uafter", "claude-opus-5", ""))
	if len(recs) != 2 {
		t.Fatalf("want 2 records, got %d (stats %+v)", len(recs), stats)
	}
	if recs[0].MessageID != "big" || recs[1].MessageID != "after" {
		t.Errorf("records after a huge line were lost: %v", recs)
	}
}

func TestParseOversizedLineSkippedWithoutLosingRest(t *testing.T) {
	over := `{"type":"assistant","pad":"` + strings.Repeat("y", maxLineBytes) + `"}`
	recs, stats := parse(t, Options{}, over, assistant("ok", "rok", "uok", "claude-opus-5", ""))
	if len(recs) != 1 || recs[0].MessageID != "ok" {
		t.Fatalf("want the following record to survive, got %v", recs)
	}
	if stats.Skipped[SkipLineTooLong] != 1 {
		t.Errorf("line-too-long = %d, want 1", stats.Skipped[SkipLineTooLong])
	}
}

func TestParseStripsCarriageReturns(t *testing.T) {
	p := New(Options{})
	body := assistant("m", "r", "u", "claude-opus-5", "") + "\r\n"
	recs, err := p.ParseReader(strings.NewReader(body), "proj")
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 {
		t.Fatalf("CRLF line not parsed: %+v", p.Stats())
	}
}

func TestParseDropsNonModels(t *testing.T) {
	recs, stats := parse(t, Options{NonModels: []string{"<synthetic>"}},
		assistant("m1", "r1", "u1", "<synthetic>", ""),
		assistant("m2", "r2", "u2", "claude-opus-5", ""))
	if len(recs) != 1 || recs[0].Model != "claude-opus-5" {
		t.Fatalf("synthetic not dropped: %v", recs)
	}
	if stats.Skipped[SkipNonModel] != 1 {
		t.Errorf("non-model = %d, want 1", stats.Skipped[SkipNonModel])
	}
}

func TestParseSidechains(t *testing.T) {
	side := strings.Replace(assistant("m2", "r2", "u2", "claude-opus-5", ""),
		`"isSidechain":false`, `"isSidechain":true`, 1)
	main := assistant("m1", "r1", "u1", "claude-opus-5", "")

	recs, _ := parse(t, Options{}, main, side)
	if len(recs) != 2 {
		t.Fatalf("sidechains are included by default, got %d", len(recs))
	}
	if !recs[1].IsSidechain {
		t.Error("sidechain flag not carried onto the record")
	}

	recs, stats := parse(t, Options{ExcludeSidechains: true}, main, side)
	if len(recs) != 1 || recs[0].IsSidechain {
		t.Fatalf("ExcludeSidechains left %d records", len(recs))
	}
	if stats.Skipped[SkipSidechain] != 1 {
		t.Errorf("sidechain-excluded = %d, want 1", stats.Skipped[SkipSidechain])
	}
}

func TestParseBadTimestampKeepsRecord(t *testing.T) {
	line := strings.Replace(assistant("m", "r", "u", "claude-opus-5", ""),
		`"timestamp":"2026-09-13T10:36:05.063Z"`, `"timestamp":"not-a-time"`, 1)
	recs, stats := parse(t, Options{}, line)
	// Spend is real even when the timestamp is not parseable; drop the date,
	// never the tokens.
	if len(recs) != 1 || !recs[0].Timestamp.IsZero() {
		t.Fatalf("want one record with a zero time, got %v", recs)
	}
	if stats.BadTimestamp != 1 {
		t.Errorf("BadTimestamp = %d, want 1", stats.BadTimestamp)
	}
}
