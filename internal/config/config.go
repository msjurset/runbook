package config

import (
	"os"
	"path/filepath"
)

// Config holds application-level settings.
type Config struct {
	RunbookDir string
	HistoryDir string
}

// Default returns the default configuration.
func Default() *Config {
	home, _ := os.UserHomeDir()
	base := filepath.Join(home, ".runbook")
	return &Config{
		RunbookDir: filepath.Join(base, "books"),
		HistoryDir: filepath.Join(base, "history"),
	}
}

// EnsureDirs creates the configuration directories if they don't exist.
func (c *Config) EnsureDirs() error {
	for _, dir := range []string{c.RunbookDir, c.HistoryDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}
