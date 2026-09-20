// Package config discovers and resolves configuration from the XDG directory on
// Unix and %APPDATA% on Windows, applying flag-over-file-over-default precedence.
package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// appName is the directory cca keeps its own files under. It matches the binary
// name deliberately: the module path, help text, config directory, and README
// all use the same word.
const appName = "cca"

// File names inside the config directory.
const (
	ConfigFileName  = "config.json"
	PricingFileName = "pricing.json"
)

// DefaultClaudeDir returns the Claude Code data directory for the current user.
//
// os.UserHomeDir is used rather than $HOME, which is frequently unset on
// Windows where the location is C:\Users\<name>\.claude.
func DefaultClaudeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude"), nil
}

// Dir returns the directory cca reads its configuration from: %APPDATA%\cca on
// Windows and $XDG_CONFIG_HOME/cca, or ~/.config/cca, elsewhere.
func Dir() (string, error) {
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, appName), nil
		}
	} else if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, appName), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		// APPDATA is normally set; this is the fallback for a stripped
		// environment such as a service account.
		return filepath.Join(home, "AppData", "Roaming", appName), nil
	}
	return filepath.Join(home, ".config", appName), nil
}

// ConfigPath is where cca looks for its settings file.
func ConfigPath() (string, error) { return inDir(ConfigFileName) }

// PricingPath is where cca looks for a rate table overriding the embedded one.
func PricingPath() (string, error) { return inDir(PricingFileName) }

func inDir(name string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

// exists reports whether path is present and readable as a regular file.
func exists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
