package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type LogIndex struct {
	Entries map[string]string `json:"entries"` // history record ID → log file path
}

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Manage run output logs",
}

var logReindexCmd = &cobra.Command{
	Use:   "reindex",
	Short: "Rebuild log index from files in the logs directory",
	Args:  cobra.NoArgs,
	RunE:  runLogReindex,
}

var logResetCmd = &cobra.Command{
	Use:   "reset-index",
	Short: "Clear the log index",
	Args:  cobra.NoArgs,
	RunE:  runLogReset,
}

func init() {
	logCmd.AddCommand(logReindexCmd)
	logCmd.AddCommand(logResetCmd)
	rootCmd.AddCommand(logCmd)
}

func logIndexPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".runbook", "logs", "index.json")
}

func loadLogIndex() *LogIndex {
	data, err := os.ReadFile(logIndexPath())
	if err != nil {
		return &LogIndex{Entries: map[string]string{}}
	}
	var idx LogIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return &LogIndex{Entries: map[string]string{}}
	}
	if idx.Entries == nil {
		idx.Entries = map[string]string{}
	}
	return &idx
}

func saveLogIndex(idx *LogIndex) error {
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(logIndexPath())
	os.MkdirAll(dir, 0o755)
	return os.WriteFile(logIndexPath(), data, 0o644)
}

func runLogReindex(cmd *cobra.Command, args []string) error {
	home, _ := os.UserHomeDir()
	logsDir := filepath.Join(home, ".runbook", "logs")

	entries, err := os.ReadDir(logsDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No logs directory found.")
			return nil
		}
		return err
	}

	idx := &LogIndex{Entries: map[string]string{}}
	count := 0

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}

		// Parse filename: {name}-{timestamp}.log
		name := strings.TrimSuffix(e.Name(), ".log")
		// Find the timestamp portion (last ISO-like segment)
		// Convention: runbook-name-2026-04-06T110349.log
		// The name portion can contain dashes, so find from the date pattern
		parts := splitLogFilename(name)
		if parts.runbook == "" {
			continue
		}

		logPath := filepath.Join(logsDir, e.Name())
		key := parts.runbook + "_" + parts.timestamp
		idx.Entries[key] = logPath
		count++
	}

	if err := saveLogIndex(idx); err != nil {
		return fmt.Errorf("saving index: %w", err)
	}

	fmt.Printf("Indexed %d log files.\n", count)
	return nil
}

func runLogReset(cmd *cobra.Command, args []string) error {
	idx := &LogIndex{Entries: map[string]string{}}
	if err := saveLogIndex(idx); err != nil {
		return fmt.Errorf("saving index: %w", err)
	}
	fmt.Println("Log index cleared.")
	return nil
}

type logFileParts struct {
	runbook   string
	timestamp string
}

// splitLogFilename parses "name-2026-04-06T110349" into runbook name and timestamp.
func splitLogFilename(s string) logFileParts {
	// Look for a date pattern: YYYY-MM-DD
	for i := 0; i < len(s)-10; i++ {
		if s[i] >= '2' && s[i] <= '2' && i > 0 && s[i-1] == '-' {
			// Check if this looks like a date: NNNN-NN-NN
			candidate := s[i:]
			if len(candidate) >= 10 && candidate[4] == '-' && candidate[7] == '-' {
				_, err := time.Parse("2006-01-02", candidate[:10])
				if err == nil {
					name := s[:i-1] // strip trailing dash
					return logFileParts{runbook: name, timestamp: candidate}
				}
			}
		}
	}
	return logFileParts{}
}
