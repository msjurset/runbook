package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(cfg.RunbookDir, home) {
		t.Errorf("RunbookDir %q does not start with home dir %q", cfg.RunbookDir, home)
	}
	if filepath.Base(cfg.RunbookDir) != "books" {
		t.Errorf("RunbookDir %q should end with 'books'", cfg.RunbookDir)
	}
	if !strings.HasPrefix(cfg.HistoryDir, home) {
		t.Errorf("HistoryDir %q does not start with home dir %q", cfg.HistoryDir, home)
	}
	if filepath.Base(cfg.HistoryDir) != "history" {
		t.Errorf("HistoryDir %q should end with 'history'", cfg.HistoryDir)
	}
}

func TestEnsureDirs(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{
		RunbookDir: filepath.Join(dir, "books"),
		HistoryDir: filepath.Join(dir, "history"),
	}

	if err := cfg.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs() error: %v", err)
	}

	for _, d := range []string{cfg.RunbookDir, cfg.HistoryDir} {
		info, err := os.Stat(d)
		if err != nil {
			t.Errorf("directory %q not created: %v", d, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%q is not a directory", d)
		}
	}

	// Calling again should not error (idempotent)
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs() second call error: %v", err)
	}
}
