package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunExitCodes(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"version", []string{"version"}, exitOK},
		{"help", []string{"help"}, exitOK},
		{"flag before command", []string{"--json", "version"}, exitOK},
		{"flag after command", []string{"version", "--json"}, exitOK},
		{"not implemented", []string{"models"}, exitError},
		{"unknown command", []string{"bogus"}, exitUsage},
		{"unknown flag", []string{"--nope"}, exitUsage},
		{"extra argument", []string{"version", "extra"}, exitUsage},
		{"json and csv", []string{"--json", "--csv", "models"}, exitUsage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := run(tt.args, &stdout, &stderr); got != tt.want {
				t.Errorf("run(%q) = %d, want %d (stderr: %s)",
					tt.args, got, tt.want, stderr.String())
			}
		})
	}
}

// fixtureDir writes a minimal claude directory so that command tests never
// read the real one: the user's own transcripts would make results depend on
// the machine, and the suite must not touch them at all.
func fixtureDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "projects", "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	line := `{"type":"assistant","uuid":"u1","sessionId":"s1","requestId":"r1",` +
		`"timestamp":"2026-09-14T10:00:00.000Z","cwd":"/demo","isSidechain":false,` +
		`"message":{"id":"m1","model":"claude-opus-5","usage":{"input_tokens":1000,` +
		`"output_tokens":2000,"cache_creation_input_tokens":0,"cache_read_input_tokens":5000,` +
		`"cache_creation":{"ephemeral_5m_input_tokens":0,"ephemeral_1h_input_tokens":0}}}}`
	if err := os.WriteFile(filepath.Join(dir, "s1.jsonl"), []byte(line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestDefaultCommandRendersSummary(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--claude-dir", fixtureDir(t)}, &stdout, &stderr); code != exitOK {
		t.Fatalf("bare cca exited %d: %s", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"Claude Code usage", "Tokens", "claude-opus-5", "Cost", "Water",
		// Both accuracy caveats are mandatory on the default view.
		"not what you were billed", "rough estimate",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("summary is missing %q:\n%s", want, out)
		}
	}
}

func TestSummaryFlagsAreHonoured(t *testing.T) {
	dir := fixtureDir(t)

	var ascii bytes.Buffer
	if code := run([]string{"--claude-dir", dir, "--ascii"}, &ascii, &bytes.Buffer{}); code != exitOK {
		t.Fatalf("--ascii exited %d", code)
	}
	for _, glyph := range []string{"·", "≈", "─"} {
		if strings.Contains(ascii.String(), glyph) {
			t.Errorf("--ascii output still contains %q", glyph)
		}
	}

	var water bytes.Buffer
	if code := run([]string{"--claude-dir", dir, "--water-ml-per-1k", "1.5"}, &water, &bytes.Buffer{}); code != exitOK {
		t.Fatalf("--water-ml-per-1k exited %d", code)
	}
	if !strings.Contains(water.String(), "1.50 mL / 1k tokens") {
		t.Errorf("the water footnote should state the overridden rate:\n%s", water.String())
	}
}

func TestMissingClaudeDirIsExplained(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--claude-dir", filepath.Join(t.TempDir(), "absent")}, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("exit = %d, want %d", code, exitError)
	}
	// A missing directory is a normal situation, so it earns an explanation
	// rather than a bare error.
	if !strings.Contains(stderr.String(), "--claude-dir") {
		t.Errorf("error should suggest how to fix it, got: %s", stderr.String())
	}
}

func TestBadWindowIsRejected(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--claude-dir", fixtureDir(t), "--since", "yesterday"}, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("exit = %d, want %d", code, exitError)
	}
	if !strings.Contains(stderr.String(), "--since") {
		t.Errorf("error should name the offending flag, got: %s", stderr.String())
	}
}

func TestVersionReportsBuildInfo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"version"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("version exited %d: %s", code, stderr.String())
	}
	for _, want := range []string{version, commit, date, "platform"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("version output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestHelpListsEveryCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	run([]string{"help"}, &stdout, &stderr)
	for _, c := range commands {
		if !strings.Contains(stdout.String(), c.name) {
			t.Errorf("help output missing command %q", c.name)
		}
	}
}

func TestOptionalFloat(t *testing.T) {
	var f optionalFloat
	if f.set || f.String() != "" {
		t.Errorf("zero value should be unset and render empty, got %q", f.String())
	}
	if err := f.Set("0.45"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !f.set || f.value != 0.45 || f.String() != "0.45" {
		t.Errorf("after Set: set=%v value=%v string=%q", f.set, f.value, f.String())
	}
	if err := f.Set("not-a-number"); err == nil {
		t.Error("Set should reject a non-numeric value")
	}
}

func TestWaterFlagUnsetUntilPassed(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"version"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("unexpected exit %d", code)
	}
	// Absent flag must stay unset so config resolution can supply the default.
	opts := &options{}
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	opts.register(fs)
	if err := fs.Parse([]string{"version"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if opts.waterMLPer1k.set {
		t.Error("water flag should be unset when not passed")
	}
	if err := fs.Parse([]string{"--water-ml-per-1k", "0.5"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !opts.waterMLPer1k.set || opts.waterMLPer1k.value != 0.5 {
		t.Errorf("water flag not captured: %+v", opts.waterMLPer1k)
	}
}
