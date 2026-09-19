package transcript

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// writeTree builds a claude-dir fixture: project name -> file name -> contents.
func writeTree(t *testing.T, tree map[string]map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for project, files := range tree {
		dir := filepath.Join(root, "projects", project)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, body := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return root
}

func TestDiscoverMissingClaudeDir(t *testing.T) {
	_, err := Discover(filepath.Join(t.TempDir(), "nope"))
	if !errors.Is(err, ErrNoClaudeDir) {
		t.Fatalf("want ErrNoClaudeDir, got %v", err)
	}
}

func TestDiscoverEmptyClaudeDir(t *testing.T) {
	root := writeTree(t, map[string]map[string]string{})
	if err := os.MkdirAll(filepath.Join(root, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	files, err := Discover(root)
	if err != nil {
		t.Fatalf("empty projects dir should not error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("want no files, got %d", len(files))
	}
}

func TestDiscoverIsSortedAndFiltered(t *testing.T) {
	root := writeTree(t, map[string]map[string]string{
		"proj-b": {"z.jsonl": "", "a.jsonl": "", "notes.md": "", "b.json": ""},
		"proj-a": {"only.jsonl": ""},
	})
	if err := os.WriteFile(filepath.Join(root, "projects", "stray.jsonl"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range files {
		got = append(got, f.Project+"/"+filepath.Base(f.Path))
	}
	want := []string{"proj-a/only.jsonl", "proj-b/a.jsonl", "proj-b/z.jsonl"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("position %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDiscoverIsDeterministic(t *testing.T) {
	root := writeTree(t, map[string]map[string]string{
		"p1": {"c.jsonl": "", "a.jsonl": "", "b.jsonl": ""},
		"p2": {"e.jsonl": "", "d.jsonl": ""},
	})
	first, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		again, err := Discover(root)
		if err != nil {
			t.Fatal(err)
		}
		for j := range first {
			if first[j] != again[j] {
				t.Fatalf("run %d diverged at %d: %v vs %v", i, j, first[j], again[j])
			}
		}
	}
}

func TestDiscoverFindsNestedSubagentTranscripts(t *testing.T) {
	// Sub-agent sessions nest two levels deeper than the documented layout:
	// projects/<slug>/<session>/subagents/agent-<id>.jsonl. A non-recursive
	// walk drops them silently, and with them every sidechain record.
	root := t.TempDir()
	nested := filepath.Join(root, "projects", "proj-a", "9f8c-session", "subagents")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "agent-abc.jsonl"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	top := filepath.Join(root, "projects", "proj-a", "session.jsonl")
	if err := os.WriteFile(top, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("want both the top-level and nested transcript, got %d: %v", len(files), files)
	}
	for _, f := range files {
		if f.Project != "proj-a" {
			t.Errorf("nested file attributed to %q, want the top-level project proj-a", f.Project)
		}
	}
}
