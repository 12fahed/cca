package transcript

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// snapshot records every file under root with its size, mode, and content hash.
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			out[rel+"/"] = "dir"
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		out[rel] = fmt.Sprintf("%s size=%d mode=%s", hex.EncodeToString(sum[:8]), info.Size(), info.Mode())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// Reading a Claude directory must leave it byte-for-byte identical. This is the
// tool's one hard safety rule: it analyses someone's transcripts, which hold
// their source code and conversations, and must never alter or add to them.
func TestLoadNeverModifiesTheClaudeDirectory(t *testing.T) {
	root := writeTree(t, map[string]map[string]string{
		"proj-a": {
			"one.jsonl": assistant("m1", "r1", "u1", "claude-opus-5", "") + "\n" +
				assistant("m2", "r2", "u2", "claude-opus-5", "") + "\n",
			"two.jsonl": assistant("m1", "r1", "u1", "claude-opus-5", "") + "\n",
			"notes.md":  "not a transcript\n",
		},
		"proj-b": {"three.jsonl": `{"malformed` + "\n"},
	})
	// A nested sub-agent transcript, so the recursive walk is covered too.
	nested := filepath.Join(root, "projects", "proj-a", "sess", "subagents")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "agent-x.jsonl"),
		[]byte(assistant("m9", "r9", "u9", "claude-opus-5", "")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	before := snapshot(t, root)
	if _, _, err := Load(Options{ClaudeDir: root, NonModels: []string{"<synthetic>"}}); err != nil {
		t.Fatal(err)
	}
	after := snapshot(t, root)

	for path, want := range before {
		got, ok := after[path]
		if !ok {
			t.Errorf("%s was removed", path)
			continue
		}
		if got != want {
			t.Errorf("%s changed:\n  before %s\n  after  %s", path, want, got)
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			t.Errorf("%s was created", path)
		}
	}
}

// Even a directory the process cannot write to must read cleanly, which catches
// any attempt to create a lock or scratch file alongside the transcripts.
func TestLoadWorksAgainstAReadOnlyDirectory(t *testing.T) {
	root := writeTree(t, map[string]map[string]string{
		"proj": {"a.jsonl": assistant("m", "r", "u", "claude-opus-5", "") + "\n"},
	})
	projects := filepath.Join(root, "projects")
	dir := filepath.Join(projects, "proj")
	for _, p := range []string{filepath.Join(dir, "a.jsonl")} {
		if err := os.Chmod(p, 0o444); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	recs, _, err := Load(Options{ClaudeDir: root})
	if err != nil {
		t.Fatalf("a read-only transcript directory must still be readable: %v", err)
	}
	if len(recs) != 1 {
		t.Errorf("got %d records, want 1", len(recs))
	}
}

// writeCalls are the standard library functions that can create, modify, or
// remove a file. None belongs in the package that walks the Claude directory.
var writeCalls = map[string][]string{
	"os": {
		"Create", "CreateTemp", "WriteFile", "Remove", "RemoveAll", "Rename",
		"Mkdir", "MkdirAll", "MkdirTemp", "Truncate", "Chmod", "Chown", "Link",
		"Symlink", "Append",
	},
	"io": {"Copy"},
}

// A structural check alongside the behavioural one: the behaviour test proves
// today's code is safe, this refuses the next edit that would not be. os.Open
// is read-only by definition, so the rule is simply that nothing else appears.
func TestPackageContainsNoFileWritingCalls(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for name, pkg := range pkgs {
		for path, file := range pkg.Files {
			if strings.HasSuffix(path, "_test.go") {
				continue // fixtures legitimately write into temporary directories
			}
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				ident, ok := sel.X.(*ast.Ident)
				if !ok {
					return true
				}
				for _, banned := range writeCalls[ident.Name] {
					if sel.Sel.Name == banned {
						t.Errorf("%s: %s.%s at %s writes to the filesystem; "+
							"the transcript package must only read",
							name, ident.Name, banned, fset.Position(call.Pos()))
					}
				}
				return true
			})
		}
	}
}

// os.Open is the only way this package may obtain a file handle. OpenFile can
// be given write flags, so its use would need justifying rather than assuming.
func TestOnlyReadOnlyOpensAreUsed(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var opens []string
	for _, pkg := range pkgs {
		for path, file := range pkg.Files {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			ast.Inspect(file, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				ident, ok := sel.X.(*ast.Ident)
				if !ok || ident.Name != "os" {
					return true
				}
				if strings.HasPrefix(sel.Sel.Name, "Open") {
					opens = append(opens, sel.Sel.Name)
				}
				return true
			})
		}
	}
	sort.Strings(opens)
	for _, name := range opens {
		if name != "Open" {
			t.Errorf("%s may open a file for writing; use os.Open", name)
		}
	}
	if len(opens) == 0 {
		t.Error("expected the package to open transcripts with os.Open")
	}
}
