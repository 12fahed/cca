// Package config discovers and resolves configuration from the XDG directory on
// Unix and %APPDATA% on Windows, applying flag-over-file-over-default precedence.
package config

import (
	"os"
	"path/filepath"
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
