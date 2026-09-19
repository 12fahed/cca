package transcript

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDedupWithinOneReader(t *testing.T) {
	line := assistant("msg_1", "req_1", "u1", "claude-opus-5", "")
	recs, stats := parse(t, Options{}, line, line, line)
	if len(recs) != 1 {
		t.Fatalf("want 1 record, got %d", len(recs))
	}
	if stats.Duplicates != 2 {
		t.Errorf("Duplicates = %d, want 2", stats.Duplicates)
	}
	if stats.Records != 1 {
		t.Errorf("Records = %d, want 1", stats.Records)
	}
}

func TestDedupAcrossFiles(t *testing.T) {
	// Resumed sessions and compaction replay earlier turns into new files, so
	// deduplication has to be global rather than per-file.
	shared := assistant("msg_shared", "req_shared", "u1", "claude-opus-5", "")
	root := writeTree(t, map[string]map[string]string{
		"proj-a": {"one.jsonl": shared + "\n" + assistant("msg_a", "req_a", "u2", "claude-opus-5", "") + "\n"},
		"proj-b": {"two.jsonl": shared + "\n" + assistant("msg_b", "req_b", "u3", "claude-opus-5", "") + "\n"},
	})

	recs, stats, err := Load(Options{ClaudeDir: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 3 {
		t.Fatalf("want 3 unique records across 2 files, got %d", len(recs))
	}
	if stats.Duplicates != 1 {
		t.Errorf("Duplicates = %d, want 1", stats.Duplicates)
	}
	if stats.Files != 2 {
		t.Errorf("Files = %d, want 2", stats.Files)
	}
	// First occurrence wins, and discovery is sorted, so the shared message is
	// attributed to proj-a.
	for _, r := range recs {
		if r.MessageID == "msg_shared" && r.Project != "proj-a" {
			t.Errorf("shared message attributed to %q, want proj-a", r.Project)
		}
	}
}

func TestDedupDistinguishesRequestID(t *testing.T) {
	// Same message id, different request: two real billed calls, not a dupe.
	recs, stats := parse(t, Options{},
		assistant("msg_1", "req_1", "u1", "claude-opus-5", ""),
		assistant("msg_1", "req_2", "u2", "claude-opus-5", ""))
	if len(recs) != 2 {
		t.Fatalf("want 2 records, got %d", len(recs))
	}
	if stats.Duplicates != 0 {
		t.Errorf("Duplicates = %d, want 0", stats.Duplicates)
	}
}

func TestDedupFallsBackToUUID(t *testing.T) {
	// requestId is absent on a small number of real records.
	noReq := `{"type":"assistant","uuid":%q,"sessionId":"s1",` +
		`"timestamp":"2026-09-13T10:36:05.063Z","cwd":"/c","isSidechain":false,` +
		`"message":{"id":"msg_x","model":"claude-opus-5","usage":{"input_tokens":1,` +
		`"output_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}`
	a := strings.Replace(noReq, "%q", `"uuid-a"`, 1)
	b := strings.Replace(noReq, "%q", `"uuid-b"`, 1)

	recs, stats := parse(t, Options{}, a, a, b)
	if len(recs) != 2 {
		t.Fatalf("want 2 records keyed by uuid, got %d", len(recs))
	}
	if stats.Duplicates != 1 {
		t.Errorf("Duplicates = %d, want 1", stats.Duplicates)
	}
}

func TestDedupKeylessRecordsAreKept(t *testing.T) {
	// Neither an id/request pair nor a uuid. Dropping these would lose real
	// spend, so they are kept and counted instead.
	line := `{"type":"assistant","sessionId":"s","timestamp":"2026-09-13T10:36:05.063Z",` +
		`"message":{"id":"","model":"claude-opus-5","usage":{"input_tokens":5,` +
		`"output_tokens":5,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}`
	recs, stats := parse(t, Options{}, line, line)
	if len(recs) != 2 {
		t.Fatalf("want both keyless records kept, got %d", len(recs))
	}
	if stats.NoDedupKey != 2 {
		t.Errorf("NoDedupKey = %d, want 2", stats.NoDedupKey)
	}
}

func TestDedupCountsAreIndependentOfSidechainFilter(t *testing.T) {
	// Dedup runs before the sidechain filter so that --verbose reports the same
	// dedup figures whichever way --no-sidechains is set.
	side := strings.Replace(assistant("m_s", "r_s", "u_s", "claude-opus-5", ""),
		`"isSidechain":false`, `"isSidechain":true`, 1)
	lines := []string{assistant("m", "r", "u", "claude-opus-5", ""), side, side}

	_, withSide := parse(t, Options{}, lines...)
	_, without := parse(t, Options{ExcludeSidechains: true}, lines...)
	if withSide.Duplicates != without.Duplicates {
		t.Errorf("dupes differ by filter: %d vs %d", withSide.Duplicates, without.Duplicates)
	}
	if withSide.Duplicates != 1 {
		t.Errorf("Duplicates = %d, want 1", withSide.Duplicates)
	}
}

func TestLoadMissingDirIsTyped(t *testing.T) {
	_, _, err := Load(Options{ClaudeDir: filepath.Join(t.TempDir(), "absent")})
	if err == nil {
		t.Fatal("want an error for a missing claude dir")
	}
}

func TestLoadIsReproducible(t *testing.T) {
	root := writeTree(t, map[string]map[string]string{
		"p1": {
			"a.jsonl": assistant("m1", "r1", "u1", "claude-opus-5", "") + "\n" +
				assistant("m2", "r2", "u2", "claude-haiku-4-5-20251001", "") + "\n",
			"b.jsonl": assistant("m1", "r1", "u1", "claude-opus-5", "") + "\n",
		},
		"p2": {"c.jsonl": assistant("m3", "r3", "u3", "claude-sonnet-5", "") + "\n"},
	})
	first, firstStats, err := Load(Options{ClaudeDir: root})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		again, againStats, err := Load(Options{ClaudeDir: root})
		if err != nil {
			t.Fatal(err)
		}
		if len(again) != len(first) || againStats.Duplicates != firstStats.Duplicates {
			t.Fatalf("run %d diverged: %d recs/%d dupes vs %d/%d", i,
				len(again), againStats.Duplicates, len(first), firstStats.Duplicates)
		}
		for j := range first {
			if first[j] != again[j] {
				t.Fatalf("run %d record %d diverged", i, j)
			}
		}
	}
}
