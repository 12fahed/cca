package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// titledFixture writes a claude directory holding one session with a chosen
// title, one with only a generated title, and one with neither.
func titledFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "projects", "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	stamp := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")

	usage := func(session, id string) string {
		return `{"type":"assistant","uuid":"u` + id + `","sessionId":"` + session +
			`","requestId":"r` + id + `","timestamp":"` + stamp +
			`","cwd":"/demo","isSidechain":false,"message":{"id":"m` + id +
			`","model":"claude-opus-5","usage":{"input_tokens":1000,"output_tokens":2000,` +
			`"cache_creation_input_tokens":0,"cache_read_input_tokens":5000,` +
			`"cache_creation":{"ephemeral_5m_input_tokens":0,"ephemeral_1h_input_tokens":0}}}}`
	}

	sessions := map[string][]string{
		"11111111-0000-4000-8000-000000000001": {
			`{"type":"custom-title","sessionId":"11111111-0000-4000-8000-000000000001","customTitle":"chosen session name"}`,
			usage("11111111-0000-4000-8000-000000000001", "1"),
		},
		"22222222-0000-4000-8000-000000000002": {
			`{"type":"ai-title","aiTitle":"generated session summary","sessionId":"22222222-0000-4000-8000-000000000002"}`,
			usage("22222222-0000-4000-8000-000000000002", "2"),
		},
		"33333333-0000-4000-8000-000000000003": {
			usage("33333333-0000-4000-8000-000000000003", "3"),
		},
	}
	for id, lines := range sessions {
		path := filepath.Join(dir, id+".jsonl")
		if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func sessionsOutput(t *testing.T, dir string, extra ...string) string {
	t.Helper()
	args := append([]string{"--claude-dir", dir, "sessions"}, extra...)
	var stdout, stderr bytes.Buffer
	if code := run(args, &stdout, &stderr); code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	return stdout.String()
}

// Tables show titles without being asked.
func TestSessionsShowTitlesByDefault(t *testing.T) {
	isolateConfig(t)
	out := sessionsOutput(t, titledFixture(t))

	for _, want := range []string{"Title", "chosen session name", "generated session summary"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	// The identifier stays, shortened.
	if !strings.Contains(out, "11111111") {
		t.Error("short session id missing")
	}
	if strings.Contains(out, "11111111-0000-4000-8000-000000000001") {
		t.Error("the table should shorten the identifier")
	}
}

func TestNoTitlesSuppressesThemEverywhere(t *testing.T) {
	isolateConfig(t)
	dir := titledFixture(t)

	out := sessionsOutput(t, dir, "--no-titles")
	if strings.Contains(out, "chosen session name") {
		t.Errorf("--no-titles still showed a title:\n%s", out)
	}
	if !strings.Contains(out, "project") {
		t.Error("--no-titles should restore the project column")
	}

	var stdout, stderr bytes.Buffer
	if code := run([]string{"--claude-dir", dir, "sessions", "--json", "--titles", "--no-titles"},
		&stdout, &stderr); code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "chosen session name") {
		t.Error("--no-titles must win over --titles")
	}
}

// Machine output carries titles only when asked, because it gets committed to
// repositories and pasted into issues.
func TestMachineOutputOmitsTitlesByDefault(t *testing.T) {
	isolateConfig(t)
	dir := titledFixture(t)

	for _, format := range []string{"--json", "--csv"} {
		t.Run(format, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run([]string{"--claude-dir", dir, "sessions", format}, &stdout, &stderr); code != exitOK {
				t.Fatalf("exit %d: %s", code, stderr.String())
			}
			if strings.Contains(stdout.String(), "chosen session name") {
				t.Errorf("%s leaked a title without --titles:\n%s", format, stdout.String())
			}
		})
	}
}

func TestTitlesFlagIncludesThemInJSON(t *testing.T) {
	isolateConfig(t)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--claude-dir", titledFixture(t), "sessions", "--json", "--titles"},
		&stdout, &stderr); code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}

	var doc struct {
		Sessions []struct {
			Key         string `json:"key"`
			Title       string `json:"title"`
			TitleSource string `json:"title_source"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal([]byte(stdout.String()), &doc); err != nil {
		t.Fatal(err)
	}

	bySource := map[string]string{}
	for _, s := range doc.Sessions {
		if len(s.Key) < 36 {
			t.Errorf("session id shortened in JSON: %q", s.Key)
		}
		if s.TitleSource != "" {
			bySource[s.TitleSource] = s.Title
		}
	}
	if bySource["custom"] != "chosen session name" {
		t.Errorf("custom title = %q", bySource["custom"])
	}
	if bySource["ai"] != "generated session summary" {
		t.Errorf("ai title = %q", bySource["ai"])
	}
}

func TestTitlesFlagIncludesThemInCSV(t *testing.T) {
	isolateConfig(t)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--claude-dir", titledFixture(t), "sessions", "--csv", "--titles"},
		&stdout, &stderr); code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	rows, err := csv.NewReader(strings.NewReader(stdout.String())).ReadAll()
	if err != nil {
		t.Fatalf("invalid CSV: %v", err)
	}
	if !strings.Contains(strings.Join(rows[0], ","), "title") {
		t.Fatalf("no title column: %v", rows[0])
	}
	if !strings.Contains(stdout.String(), "chosen session name") {
		t.Error("--titles did not include the title")
	}
}

// --verbose distinguishes a chosen name from a generated one.
func TestVerboseShowsTitleSource(t *testing.T) {
	isolateConfig(t)
	out := sessionsOutput(t, titledFixture(t), "--verbose")
	for _, want := range []string{"from", "custom", "ai"} {
		if !strings.Contains(out, want) {
			t.Errorf("verbose output missing %q:\n%s", want, out)
		}
	}
}

// A title must never be able to move a number, which is the containment that
// keeps the upstream title-inheritance bug from mattering.
func TestTitlesDoNotAffectAnyFigure(t *testing.T) {
	isolateConfig(t)
	dir := titledFixture(t)

	totals := func(args ...string) string {
		t.Helper()
		var stdout, stderr bytes.Buffer
		full := append([]string{"--claude-dir", dir, "--json"}, args...)
		if code := run(full, &stdout, &stderr); code != exitOK {
			t.Fatalf("exit %d: %s", code, stderr.String())
		}
		var doc struct {
			Totals json.RawMessage `json:"totals"`
		}
		if err := json.Unmarshal([]byte(stdout.String()), &doc); err != nil {
			t.Fatal(err)
		}
		return string(doc.Totals)
	}

	base := totals()
	for _, args := range [][]string{{"--titles"}, {"--no-titles"}} {
		if got := totals(args...); got != base {
			t.Errorf("%v changed the totals:\n got %s\nwant %s", args, got, base)
		}
	}
}

// Water is derived from token counts, so unlike a title it needs no opt-in in
// machine output. --no-water is a display choice only.
func TestWaterColumnEndToEnd(t *testing.T) {
	isolateConfig(t)
	dir := titledFixture(t)

	withWater := sessionsOutput(t, dir)
	if !strings.Contains(withWater, "water") {
		t.Errorf("sessions view is missing the water column:\n%s", withWater)
	}
	if !strings.Contains(withWater, "rough estimate") {
		t.Error("a view printing water owes the assumption behind it")
	}

	noWater := sessionsOutput(t, dir, "--no-water")
	if strings.Contains(noWater, "water") {
		t.Errorf("--no-water still showed the column:\n%s", noWater)
	}
	if strings.Contains(noWater, "rough estimate") {
		t.Error("with no water on screen there is no assumption to declare")
	}
	if !strings.Contains(noWater, "started") {
		t.Error("--no-water should restore the column it displaced")
	}
}

// The configured rate has to reach the per-session figures, not just the
// summary's single number.
func TestWaterRateReachesSessionRows(t *testing.T) {
	isolateConfig(t)
	dir := titledFixture(t)

	var stdout, stderr bytes.Buffer
	if code := run([]string{"--claude-dir", dir, "sessions", "--json",
		"--water-ml-per-1k", "2.5"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}

	var doc struct {
		Water struct {
			MLPer1k float64 `json:"ml_per_1k_tokens"`
		} `json:"water"`
		Sessions []struct {
			Tokens struct {
				Total int64 `json:"total"`
			} `json:"tokens"`
			Water struct {
				Millilitres float64 `json:"millilitres"`
			} `json:"water"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal([]byte(stdout.String()), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Water.MLPer1k != 2.5 {
		t.Errorf("rate = %v, want 2.5", doc.Water.MLPer1k)
	}
	for _, s := range doc.Sessions {
		want := float64(s.Tokens.Total) / 1000 * 2.5
		if diff := s.Water.Millilitres - want; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("session water %v, tokens imply %v", s.Water.Millilitres, want)
		}
	}
}

// Water is derived at render time and must not be able to move a figure.
func TestWaterFlagsDoNotAffectCostOrTokens(t *testing.T) {
	isolateConfig(t)
	dir := titledFixture(t)

	figures := func(args ...string) string {
		t.Helper()
		var stdout, stderr bytes.Buffer
		full := append([]string{"--claude-dir", dir, "--json"}, args...)
		if code := run(full, &stdout, &stderr); code != exitOK {
			t.Fatalf("exit %d: %s", code, stderr.String())
		}
		var doc struct {
			Totals struct {
				Cost   json.RawMessage `json:"cost_usd"`
				Tokens json.RawMessage `json:"tokens"`
			} `json:"totals"`
		}
		if err := json.Unmarshal([]byte(stdout.String()), &doc); err != nil {
			t.Fatal(err)
		}
		return string(doc.Totals.Cost) + string(doc.Totals.Tokens)
	}

	base := figures()
	for _, args := range [][]string{{"--no-water"}, {"--water-ml-per-1k", "9.5"}} {
		if got := figures(args...); got != base {
			t.Errorf("%v changed cost or tokens:\n got %s\nwant %s", args, got, base)
		}
	}
}
