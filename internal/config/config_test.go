package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cca/internal/water"
)

func writeConfig(t *testing.T, body string) (dir, path string) {
	t.Helper()
	dir = t.TempDir()
	path = filepath.Join(dir, ConfigFileName)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, path
}

// isolate points the config and home lookups at temporary directories so tests
// never read or depend on the real ones.
func isolate(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	return home
}

func ptrF(v float64) *float64 { return &v }
func ptrS(v string) *string   { return &v }
func ptrB(v bool) *bool       { return &v }

func TestLoadFileMissingIsNotAnError(t *testing.T) {
	f, found, err := LoadFile(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil {
		t.Fatalf("running without a config file is normal: %v", err)
	}
	if found {
		t.Error("found should be false")
	}
	if f.WaterMLPer1k != nil {
		t.Error("an absent file must leave every setting unset")
	}
}

func TestLoadFileReadsSettings(t *testing.T) {
	_, path := writeConfig(t, `{
	  "_comment": ["ignored"],
	  "water_ml_per_1k_tokens": 0.45,
	  "claude_dir": "/data/claude",
	  "no_sidechains": true
	}`)
	f, found, err := LoadFile(path)
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if f.WaterMLPer1k == nil || *f.WaterMLPer1k != 0.45 {
		t.Errorf("water = %v", f.WaterMLPer1k)
	}
	if f.ClaudeDir == nil || *f.ClaudeDir != "/data/claude" {
		t.Errorf("claude_dir = %v", f.ClaudeDir)
	}
	if f.NoSidechains == nil || !*f.NoSidechains {
		t.Errorf("no_sidechains = %v", f.NoSidechains)
	}
	// Unmentioned keys stay unset so they fall through to the default.
	if f.ASCII != nil {
		t.Errorf("ascii should be unset, got %v", *f.ASCII)
	}
}

func TestLoadFileRejectsBadInput(t *testing.T) {
	tests := map[string]string{
		"malformed":     `{not json`,
		"negative rate": `{"water_ml_per_1k_tokens": -1}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			_, path := writeConfig(t, body)
			if _, _, err := LoadFile(path); err == nil {
				t.Fatal("want an error")
			}
		})
	}
}

func TestResolveDefaults(t *testing.T) {
	isolate(t)
	r, err := Resolve(Overrides{}, File{}, false, "/some/config.json")
	if err != nil {
		t.Fatal(err)
	}
	if r.WaterMLPer1k != water.DefaultMLPer1kTokens {
		t.Errorf("water = %v, want %v", r.WaterMLPer1k, water.DefaultMLPer1kTokens)
	}
	if !strings.HasSuffix(r.ClaudeDir, ".claude") {
		t.Errorf("claude dir = %q", r.ClaudeDir)
	}
	// With no override file present, the embedded table is used and no path is
	// reported.
	if r.PricingPath != "" {
		t.Errorf("pricing path = %q, want empty", r.PricingPath)
	}
	for key, src := range r.Sources {
		if src != FromDefault {
			t.Errorf("%s came from %q, want default", key, src)
		}
	}
}

// The whole point of the pointer fields: flag beats file beats default.
func TestResolvePrecedence(t *testing.T) {
	isolate(t)
	file := File{
		WaterMLPer1k: ptrF(0.45),
		ClaudeDir:    ptrS("/from/file"),
		NoSidechains: ptrB(true),
	}
	flags := Overrides{
		WaterMLPer1k: ptrF(1.5),
		ClaudeDir:    ptrS("/from/flag"),
	}
	r, err := Resolve(flags, file, true, "/cfg.json")
	if err != nil {
		t.Fatal(err)
	}
	if r.WaterMLPer1k != 1.5 {
		t.Errorf("water = %v, want the flag value 1.5", r.WaterMLPer1k)
	}
	if r.Sources[KeyWater] != FromFlag {
		t.Errorf("water source = %q, want flag", r.Sources[KeyWater])
	}
	if r.ClaudeDir != "/from/flag" {
		t.Errorf("claude dir = %q", r.ClaudeDir)
	}
	// Set in the file and not on the command line: the file wins over the default.
	if !r.NoSidechains {
		t.Error("no_sidechains should have come from the file")
	}
	if r.Sources[KeyNoSidechains] != FromFile {
		t.Errorf("no_sidechains source = %q, want config file", r.Sources[KeyNoSidechains])
	}
	// Mentioned nowhere: still the default.
	if r.ASCII || r.Sources[KeyASCII] != FromDefault {
		t.Errorf("ascii = %v from %q", r.ASCII, r.Sources[KeyASCII])
	}
}

// A boolean deliberately set to false in the file must override a true default
// path; that is why the fields are pointers rather than plain values.
func TestResolveFalseInFileIsStillASetting(t *testing.T) {
	isolate(t)
	r, err := Resolve(Overrides{}, File{ASCII: ptrB(false)}, true, "/cfg.json")
	if err != nil {
		t.Fatal(err)
	}
	if r.Sources[KeyASCII] != FromFile {
		t.Errorf("an explicit false should register as a file setting, got %q", r.Sources[KeyASCII])
	}
}

// A flag set to false explicitly must beat a true in the config file.
func TestResolveFlagFalseBeatsFileTrue(t *testing.T) {
	isolate(t)
	r, err := Resolve(Overrides{NoSidechains: ptrB(false)},
		File{NoSidechains: ptrB(true)}, true, "/cfg.json")
	if err != nil {
		t.Fatal(err)
	}
	if r.NoSidechains {
		t.Error("an explicit --no-sidechains=false should win over the config file")
	}
	if r.Sources[KeyNoSidechains] != FromFlag {
		t.Errorf("source = %q, want flag", r.Sources[KeyNoSidechains])
	}
}

func TestResolveFindsPricingOverride(t *testing.T) {
	home := isolate(t)
	dir := filepath.Join(home, ".config", "cca")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, PricingFileName)
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := Resolve(Overrides{}, File{}, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if r.PricingPath != path {
		t.Errorf("pricing path = %q, want %q", r.PricingPath, path)
	}
}

func TestResolveRejectsNegativeWater(t *testing.T) {
	isolate(t)
	if _, err := Resolve(Overrides{WaterMLPer1k: ptrF(-1)}, File{}, false, ""); err == nil {
		t.Fatal("want an error for a negative water rate")
	}
}

func TestSampleIsValidAndDocumentsTheCaveat(t *testing.T) {
	var f File
	if err := json.Unmarshal([]byte(Sample), &f); err != nil {
		t.Fatalf("the sample config must parse: %v", err)
	}
	if f.WaterMLPer1k == nil || *f.WaterMLPer1k != water.DefaultMLPer1kTokens {
		t.Errorf("sample water rate = %v, want the default", f.WaterMLPer1k)
	}
	// JSON has no comments, so the caveat lives in a key the loader ignores.
	// It must still be there: the constant is not a measurement.
	lower := strings.ToLower(Sample)
	for _, want := range []string{"placeholder", "not a measured value", "order of magnitude"} {
		if !strings.Contains(lower, want) {
			t.Errorf("sample config does not mention %q", want)
		}
	}
}

// The sample must round-trip through the loader, or copying it into place would
// fail on first use.
func TestSampleLoads(t *testing.T) {
	_, path := writeConfig(t, Sample)
	f, found, err := LoadFile(path)
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if f.WaterMLPer1k == nil {
		t.Error("sample should set the water rate")
	}
}
