package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"cca/internal/config"
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
	// Dated now, so that the today, week, and month windows all include it.
	stamp := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	line := `{"type":"assistant","uuid":"u1","sessionId":"s1","requestId":"r1",` +
		`"timestamp":"` + stamp + `","cwd":"/demo","isSidechain":false,` +
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

// Version output carries the build stamp and the licence notice. The GPL asks
// that a program state its terms and the absence of warranty where it
// reasonably can, and for a non-interactive tool this is that place.
func TestVersionReportsBuildInfo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"version"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("version exited %d: %s", code, stderr.String())
	}
	for _, want := range []string{version, commit, date, "platform", "GPL-3.0"} {
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

// isolateConfig points config discovery at a temporary directory so the tests
// never read the real one, and returns the directory cca will actually look in.
//
// That directory is resolved through config.Dir rather than assembled here: it
// is ~/.config/cca on Unix but %APPDATA%\cca on Windows, so hardcoding either
// shape writes the fixture somewhere the tool will not look on the other
// platform.
func isolateConfig(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))

	dir, err := config.Dir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func writeUserConfig(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runSummaryOutput(t *testing.T, args ...string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if code := run(args, &stdout, &stderr); code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	return stdout.String()
}

func TestConfigFileSuppliesWaterRate(t *testing.T) {
	dir := isolateConfig(t)
	writeUserConfig(t, dir, `{"water_ml_per_1k_tokens": 0.75}`)

	out := runSummaryOutput(t, "--claude-dir", fixtureDir(t))
	if !strings.Contains(out, "0.75 mL / 1k tokens") {
		t.Errorf("config file water rate not applied:\n%s", out)
	}
}

// Flag over file over default is the whole contract.
func TestFlagBeatsConfigFile(t *testing.T) {
	dir := isolateConfig(t)
	writeUserConfig(t, dir, `{"water_ml_per_1k_tokens": 0.75}`)

	out := runSummaryOutput(t, "--claude-dir", fixtureDir(t), "--water-ml-per-1k", "2.25")
	if !strings.Contains(out, "2.25 mL / 1k tokens") {
		t.Errorf("flag should beat the config file:\n%s", out)
	}
	if strings.Contains(out, "0.75 mL") {
		t.Error("config value leaked through despite an explicit flag")
	}
}

func TestDefaultWaterRateWithoutConfigFile(t *testing.T) {
	isolateConfig(t)
	out := runSummaryOutput(t, "--claude-dir", fixtureDir(t))
	if !strings.Contains(out, "0.30 mL / 1k tokens") {
		t.Errorf("want the default rate with no config file:\n%s", out)
	}
}

func TestConfigFileSuppliesASCIIAndClaudeDir(t *testing.T) {
	dir := isolateConfig(t)
	claude := fixtureDir(t)
	writeUserConfig(t, dir, `{"ascii": true, "claude_dir": `+strconv.Quote(claude)+`}`)

	// No --claude-dir flag: the path has to come from the file.
	out := runSummaryOutput(t)
	if !strings.Contains(out, "claude-opus-5") {
		t.Errorf("claude_dir from the config file was not used:\n%s", out)
	}
	for _, glyph := range []string{"·", "≈", "─"} {
		if strings.Contains(out, glyph) {
			t.Errorf("ascii from the config file was not applied, found %q", glyph)
		}
	}
}

func TestBrokenConfigFileIsReported(t *testing.T) {
	dir := isolateConfig(t)
	writeUserConfig(t, dir, `{"water_ml_per_1k_tokens": -4}`)

	var stdout, stderr bytes.Buffer
	if code := run([]string{"--claude-dir", fixtureDir(t)}, &stdout, &stderr); code != exitError {
		t.Fatalf("exit = %d, want %d", code, exitError)
	}
	if !strings.Contains(stderr.String(), "water_ml_per_1k_tokens") {
		t.Errorf("error should name the offending setting, got: %s", stderr.String())
	}
}

func TestPricingOverrideDiscoveredFromConfigDir(t *testing.T) {
	dir := isolateConfig(t)
	// A table with only one model, so its effect is unmistakable.
	table := `{"schema_version":1,"currency":"USD","unit":"per_million_tokens",
	  "models":[{"id":"claude-opus-5","name":"Opus",
	    "rates":{"input":1000,"output":1000,"cache_write_5m":1000,
	             "cache_write_1h":1000,"cache_read":1000}}],
	  "non_models":["<synthetic>"]}`
	if err := os.WriteFile(filepath.Join(dir, "pricing.json"), []byte(table), 0o644); err != nil {
		t.Fatal(err)
	}

	out := runSummaryOutput(t, "--claude-dir", fixtureDir(t))
	// 8000 tokens at $1000/M is $8.00; the embedded table would give cents.
	if !strings.Contains(out, "$8.00") {
		t.Errorf("pricing override in the config dir was not picked up:\n%s", out)
	}
}

func TestEverySubcommandRuns(t *testing.T) {
	isolateConfig(t)
	dir := fixtureDir(t)
	tests := map[string][]string{
		"summary":  {"summary"},
		"today":    {"today"},
		"week":     {"week"},
		"month":    {"month"},
		"models":   {"models"},
		"projects": {"projects"},
		"daily":    {"daily"},
		"sessions": {"sessions"},
	}
	for name, args := range tests {
		t.Run(name, func(t *testing.T) {
			out := runSummaryOutput(t, append([]string{"--claude-dir", dir}, args...)...)
			if !strings.Contains(out, "Claude Code usage") {
				t.Errorf("%s produced no recognisable output:\n%s", name, out)
			}
			// Every view prints dollars, so every view owes the caveat.
			if !strings.Contains(out, "not what you were billed") {
				t.Errorf("%s is missing the cost-basis caveat", name)
			}
		})
	}
}

// Every command in the help listing must actually be runnable; a listed command
// that errors is worse than one that is absent.
func TestHelpListsOnlyWorkingCommands(t *testing.T) {
	isolateConfig(t)
	dir := fixtureDir(t)
	for _, c := range commands {
		if c.run == nil { // help is handled before dispatch
			continue
		}
		t.Run(c.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run([]string{"--claude-dir", dir, c.name}, &stdout, &stderr); code != exitOK {
				t.Errorf("%s exited %d: %s", c.name, code, stderr.String())
			}
			if stdout.Len() == 0 {
				t.Errorf("%s produced no output", c.name)
			}
		})
	}
}

func TestConfigCommandShowsResolvedValuesAndSources(t *testing.T) {
	dir := isolateConfig(t)
	writeUserConfig(t, dir, `{"water_ml_per_1k_tokens": 0.9}`)

	out := runSummaryOutput(t, "config")
	for _, want := range []string{
		"water_ml_per_1k_tokens", "0.90 mL / 1k tokens", "config file",
		"claude_dir", "default", filepath.Join(dir, "config.json"), "found",
		"placeholder",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("config view missing %q:\n%s", want, out)
		}
	}
}

// The config view must work even with no Claude directory, since explaining the
// configuration is exactly what a user needs when nothing else works.
func TestConfigCommandWorksWithoutTranscripts(t *testing.T) {
	isolateConfig(t)
	out := runSummaryOutput(t, "config", "--claude-dir", filepath.Join(t.TempDir(), "absent"))
	if !strings.Contains(out, "resolved configuration") {
		t.Errorf("config should still render:\n%s", out)
	}
}

func TestSessionsTopLimitsRows(t *testing.T) {
	isolateConfig(t)
	out := runSummaryOutput(t, "--claude-dir", fixtureDir(t), "sessions", "--top", "1")
	if !strings.Contains(out, "by session") {
		t.Errorf("sessions view did not render:\n%s", out)
	}
}

func TestVerboseAddsDiagnostics(t *testing.T) {
	isolateConfig(t)
	dir := fixtureDir(t)
	plain := runSummaryOutput(t, "--claude-dir", dir)
	verbose := runSummaryOutput(t, "--claude-dir", dir, "--verbose")

	if strings.Contains(plain, "duplicates dropped") {
		t.Error("diagnostics should not appear without --verbose")
	}
	for _, want := range []string{"Scan", "usage records kept", "duplicates dropped"} {
		if !strings.Contains(verbose, want) {
			t.Errorf("--verbose output missing %q:\n%s", want, verbose)
		}
	}
}

// A preset window and an explicit one cannot both be honoured, so the conflict
// is refused rather than silently resolved.
func TestPresetWindowRejectsExplicitRange(t *testing.T) {
	isolateConfig(t)
	dir := fixtureDir(t)
	for _, args := range [][]string{
		{"today", "--since", "7d"},
		{"week", "--until", "2026-09-14"},
	} {
		var stdout, stderr bytes.Buffer
		code := run(append([]string{"--claude-dir", dir}, args...), &stdout, &stderr)
		if code != exitError {
			t.Errorf("%v exited %d, want %d", args, code, exitError)
		}
		if !strings.Contains(stderr.String(), "time range") {
			t.Errorf("%v: unhelpful error %q", args, stderr.String())
		}
	}
}

// An existing Claude directory with no transcripts is a normal state for a new
// user and deserves a different message from one that is absent entirely.
func TestEmptyClaudeDirIsDistinguishedFromMissing(t *testing.T) {
	isolateConfig(t)
	empty := t.TempDir()

	var stdout, stderr bytes.Buffer
	if code := run([]string{"--claude-dir", empty}, &stdout, &stderr); code != exitError {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(stderr.String(), "holds no transcripts yet") {
		t.Errorf("an empty directory should say so, got: %s", stderr.String())
	}
	if strings.Contains(stderr.String(), "not found") {
		t.Error("an existing directory should not be reported as missing")
	}
}

// A projects directory that exists but is empty is not an error at all.
func TestEmptyProjectsDirRendersPlainly(t *testing.T) {
	isolateConfig(t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := runSummaryOutput(t, "--claude-dir", root)
	if !strings.Contains(out, "No usage found") {
		t.Errorf("want a plain message, got:\n%s", out)
	}
}

// The --json and --csv paths must work for every view, not just the default.
func TestMachineFormatsForEveryView(t *testing.T) {
	isolateConfig(t)
	dir := fixtureDir(t)
	for _, name := range []string{"summary", "models", "projects", "daily", "sessions"} {
		t.Run(name+"/json", func(t *testing.T) {
			out := runSummaryOutput(t, "--claude-dir", dir, name, "--json")
			var doc map[string]any
			if err := json.Unmarshal([]byte(out), &doc); err != nil {
				t.Fatalf("output is not valid JSON: %v\n%s", err, out)
			}
			if doc["view"] != name {
				t.Errorf("view = %v, want %q", doc["view"], name)
			}
		})
		t.Run(name+"/csv", func(t *testing.T) {
			out := runSummaryOutput(t, "--claude-dir", dir, name, "--csv")
			rows, err := csv.NewReader(strings.NewReader(out)).ReadAll()
			if err != nil {
				t.Fatalf("output is not valid CSV: %v\n%s", err, out)
			}
			if len(rows) < 2 {
				t.Errorf("want a header and at least one row, got %d", len(rows))
			}
		})
	}
}

// The definition-of-done item, checked through the CLI rather than the packages:
// the table and --json must report the same cost.
func TestCLIJSONAndTableAgreeOnCost(t *testing.T) {
	isolateConfig(t)
	dir := fixtureDir(t)

	var doc struct {
		Totals struct {
			Cost struct {
				Total float64 `json:"total"`
			} `json:"cost_usd"`
			Tokens struct {
				Total int64 `json:"total"`
			} `json:"tokens"`
		} `json:"totals"`
	}
	if err := json.Unmarshal([]byte(runSummaryOutput(t, "--claude-dir", dir, "--json")), &doc); err != nil {
		t.Fatal(err)
	}

	table := runSummaryOutput(t, "--claude-dir", dir)
	wantCost := fmt.Sprintf("$%.2f", doc.Totals.Cost.Total)
	if !strings.Contains(table, wantCost) {
		t.Errorf("table does not show the JSON cost %s:\n%s", wantCost, table)
	}
	if doc.Totals.Tokens.Total == 0 {
		t.Error("fixture produced no tokens, so the comparison proves nothing")
	}
}

func TestJSONAndCSVAreMutuallyExclusive(t *testing.T) {
	isolateConfig(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{"--claude-dir", fixtureDir(t), "--json", "--csv"}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("exit = %d, want %d", code, exitUsage)
	}
}

// Writing to a buffer rather than a terminal must never produce escapes,
// whatever the environment says.
func TestOutputToAPipeIsNeverStyled(t *testing.T) {
	isolateConfig(t)
	t.Setenv("TERM", "xterm-256color")
	out := runSummaryOutput(t, "--claude-dir", fixtureDir(t))
	if strings.Contains(out, "\x1b[") {
		t.Error("output captured from a pipe contains ANSI escapes")
	}
}

func TestNoColorEnvIsHonouredEndToEnd(t *testing.T) {
	isolateConfig(t)
	t.Setenv("NO_COLOR", "1")
	t.Setenv("CLICOLOR_FORCE", "1")
	out := runSummaryOutput(t, "--claude-dir", fixtureDir(t))
	if strings.Contains(out, "\x1b[") {
		t.Error("NO_COLOR must win over CLICOLOR_FORCE")
	}
}
