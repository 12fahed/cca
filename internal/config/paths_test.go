package config

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestDirHonoursXDG(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("XDG_CONFIG_HOME does not apply on Windows")
	}
	custom := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", custom)

	got, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(custom, "cca"); got != want {
		t.Errorf("Dir() = %q, want %q", got, want)
	}
}

func TestDirFallsBackToHome(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the Unix fallback does not apply on Windows")
	}
	t.Setenv("XDG_CONFIG_HOME", "")
	home := t.TempDir()
	t.Setenv("HOME", home)

	got, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".config", "cca"); got != want {
		t.Errorf("Dir() = %q, want %q", got, want)
	}
}

func TestPathsLiveInTheConfigDir(t *testing.T) {
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	pricing, err := PricingPath()
	if err != nil {
		t.Fatal(err)
	}
	if cfg != filepath.Join(dir, "config.json") {
		t.Errorf("ConfigPath() = %q", cfg)
	}
	if pricing != filepath.Join(dir, "pricing.json") {
		t.Errorf("PricingPath() = %q", pricing)
	}
}

// Paths are built with filepath.Join throughout, so they carry the host
// separator rather than a hardcoded slash.
func TestPathsUseHostSeparator(t *testing.T) {
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.ToSlash(dir) == dir && runtime.GOOS == "windows" {
		t.Errorf("Dir() = %q, which is not a Windows path", dir)
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("Dir() = %q, want an absolute path", dir)
	}
}

func TestDefaultClaudeDir(t *testing.T) {
	home := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	} else {
		t.Setenv("HOME", home)
	}
	got, err := DefaultClaudeDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".claude"); got != want {
		t.Errorf("DefaultClaudeDir() = %q, want %q", got, want)
	}
}
