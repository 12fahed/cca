package main

import (
	"bytes"
	"flag"
	"io"
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
		{"default command", nil, exitError},
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
