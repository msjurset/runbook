package logwriter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/msjurset/runbook/internal/runbook"
)

type IndexEntry struct {
	LogPath     string    `json:"logPath"`
	RunbookName string    `json:"runbookName"`
	Timestamp   time.Time `json:"timestamp"`
}

type IndexData struct {
	Entries []IndexEntry `json:"entries"`
}

// DefaultDir returns the default log directory.
func DefaultDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".runbook", "logs")
}

func indexPath() string {
	return filepath.Join(DefaultDir(), "index.json")
}

// LogPath returns the log file path for a runbook based on its log config.
func LogPath(book *runbook.Runbook) string {
	cfg := book.Log
	if cfg == nil {
		return ""
	}

	dir := cfg.Dir
	if dir == "" {
		dir = DefaultDir()
	}
	dir = expandHome(dir)
	os.MkdirAll(dir, 0o755)

	if cfg.Mode == "append" {
		return filepath.Join(dir, book.Name+".log")
	}

	if cfg.Filename != "" {
		ts := time.Now().Format("2006-01-02T150405")
		name := strings.ReplaceAll(cfg.Filename, "{name}", book.Name)
		name = strings.ReplaceAll(name, "{timestamp}", ts)
		return filepath.Join(dir, name+".log")
	}

	ts := time.Now().Format("2006-01-02T150405")
	return filepath.Join(dir, fmt.Sprintf("%s-%s.log", book.Name, ts))
}

// Write saves output to the log file, respecting append mode.
func Write(book *runbook.Runbook, output string, startedAt time.Time) (string, error) {
	logPath := LogPath(book)
	if logPath == "" {
		return "", nil
	}

	if book.Log.Mode == "append" {
		separator := fmt.Sprintf("\n--- run: %s ---\n", startedAt.Format("2006-01-02T15:04:05"))
		content := separator + output + "\n"

		f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return "", fmt.Errorf("opening log file: %w", err)
		}
		defer f.Close()
		if _, err := f.WriteString(content); err != nil {
			return "", fmt.Errorf("writing log: %w", err)
		}
	} else {
		if err := os.WriteFile(logPath, []byte(output), 0o644); err != nil {
			return "", fmt.Errorf("writing log: %w", err)
		}
	}

	// Record in index
	RecordIndex(book.Name, startedAt, logPath)

	return logPath, nil
}

// RecordIndex adds an entry to the log index.
func RecordIndex(runbookName string, timestamp time.Time, logPath string) {
	idx := loadIndex()

	idx.Entries = append(idx.Entries, IndexEntry{
		LogPath:     logPath,
		RunbookName: runbookName,
		Timestamp:   timestamp,
	})
	saveIndex(idx)
}

// ClearIndex removes all entries from the index.
func ClearIndex() {
	saveIndex(&IndexData{})
}

// UpdatePath changes a log path in the index (used by rotation).
func UpdatePath(oldPath, newPath string) int {
	idx := loadIndex()
	updated := 0
	for i := range idx.Entries {
		abs, _ := filepath.Abs(idx.Entries[i].LogPath)
		absOld, _ := filepath.Abs(oldPath)
		if abs == absOld {
			idx.Entries[i].LogPath = newPath
			updated++
		}
	}
	if updated > 0 {
		saveIndex(idx)
	}
	return updated
}

func loadIndex() *IndexData {
	data, err := os.ReadFile(indexPath())
	if err != nil {
		return &IndexData{}
	}
	var idx IndexData
	if err := json.Unmarshal(data, &idx); err != nil {
		return &IndexData{}
	}
	return &idx
}

func saveIndex(idx *IndexData) {
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return
	}
	os.MkdirAll(filepath.Dir(indexPath()), 0o755)
	os.WriteFile(indexPath(), data, 0o644)
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
