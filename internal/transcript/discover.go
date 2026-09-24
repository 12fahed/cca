// Package transcript discovers Claude Code session files under the projects
// directory, streams their JSONL records, and deduplicates assistant usage
// events across files.
//
// Everything here opens files read-only. Nothing under the Claude directory is
// ever created, modified, or removed.
package transcript

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ErrNoClaudeDir reports that the Claude directory does not exist, which is
// distinct from it existing but holding no transcripts.
var ErrNoClaudeDir = errors.New("claude directory not found")

// File is one transcript, tagged with the project directory that contained it.
type File struct {
	Path string
	// ModTime orders title records across files, which carry no timestamp of
	// their own. See titleCandidate.beats.
	ModTime time.Time
	// Project is the slugified working directory. It encodes drive letters and
	// separators on Windows, so treat it as an opaque label and prefer a
	// record's cwd field when a real path is needed.
	Project string
}

// Discover lists every transcript under claudeDir/projects in a deterministic
// order. Ordering matters: deduplication keeps the first occurrence of a
// message, so a stable walk is what makes attribution reproducible.
//
// The walk is recursive because the layout is not flat. Top-level sessions are
// projects/<slug>/<session>.jsonl, but sub-agent transcripts nest a further two
// levels down as projects/<slug>/<session>/subagents/agent-<id>.jsonl. Walking
// only the first level silently drops every sub-agent, which is real spend.
func Discover(claudeDir string) ([]File, error) {
	root := filepath.Join(claudeDir, "projects")
	projects, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrNoClaudeDir, root)
		}
		return nil, err
	}

	var files []File
	for _, p := range projects {
		if !p.IsDir() {
			continue
		}
		dir := filepath.Join(root, p.Name())
		// Errors are swallowed per entry: one unreadable subtree must not cost
		// us the rest of the corpus.
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if d != nil && d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if d.IsDir() || !strings.HasSuffix(d.Name(), ".jsonl") {
				return nil
			}
			f := File{Path: path, Project: p.Name()}
			if info, err := d.Info(); err == nil {
				f.ModTime = info.ModTime()
			}
			files = append(files, f)
			return nil
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}
