package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/msjurset/runbook/internal/logwriter"
	"github.com/spf13/cobra"
)

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

var logUpdateCmd = &cobra.Command{
	Use:   "update <old-path> <new-path>",
	Short: "Update a log index entry to point to a new file path",
	Args:  cobra.ExactArgs(2),
	RunE:  runLogUpdate,
}

func init() {
	logCmd.AddCommand(logReindexCmd)
	logCmd.AddCommand(logResetCmd)
	logCmd.AddCommand(logUpdateCmd)
	rootCmd.AddCommand(logCmd)
}

func runLogUpdate(cmd *cobra.Command, args []string) error {
	oldPath, _ := filepath.Abs(args[0])
	newPath, _ := filepath.Abs(args[1])

	updated := logwriter.UpdatePath(oldPath, newPath)
	if updated == 0 {
		fmt.Fprintf(os.Stderr, "no index entry found for %s\n", oldPath)
		return nil
	}
	fmt.Printf("Updated %d index entry(s): %s → %s\n", updated, oldPath, newPath)
	return nil
}

func runLogReindex(cmd *cobra.Command, args []string) error {
	logsDir := logwriter.DefaultDir()

	entries, err := os.ReadDir(logsDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No logs directory found.")
			return nil
		}
		return err
	}

	// Clear and rebuild
	logwriter.ClearIndex()
	count := 0

	// Index live log files
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}

		name := strings.TrimSuffix(e.Name(), ".log")
		parts := splitLogFilename(name)
		if parts.runbook == "" {
			continue
		}

		logPath := filepath.Join(logsDir, e.Name())
		info, _ := e.Info()
		ts := time.Now()
		if info != nil {
			ts = info.ModTime()
		}
		logwriter.RecordIndex(parts.runbook, ts, logPath)
		count++
	}

	// Index archived log files
	archiveDir := filepath.Join(logsDir, "archive")
	archiveEntries, _ := os.ReadDir(archiveDir)
	for _, e := range archiveEntries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".gz") {
			continue
		}

		// Strip .gz then .log to get the base name
		name := strings.TrimSuffix(e.Name(), ".gz")
		name = strings.TrimSuffix(name, ".log")
		parts := splitLogFilename(name)
		if parts.runbook == "" {
			continue
		}

		logPath := filepath.Join(archiveDir, e.Name())
		info, _ := e.Info()
		ts := time.Now()
		if info != nil {
			ts = info.ModTime()
		}
		logwriter.RecordIndex(parts.runbook, ts, logPath)
		count++
	}

	fmt.Printf("Indexed %d log files.\n", count)
	return nil
}

func runLogReset(cmd *cobra.Command, args []string) error {
	logwriter.ClearIndex()
	fmt.Println("Log index cleared.")
	return nil
}

type logFileParts struct {
	runbook   string
	timestamp string
}

func splitLogFilename(s string) logFileParts {
	for i := 0; i < len(s)-10; i++ {
		if s[i] >= '2' && s[i] <= '2' && i > 0 && s[i-1] == '-' {
			candidate := s[i:]
			if len(candidate) >= 10 && candidate[4] == '-' && candidate[7] == '-' {
				_, err := time.Parse("2006-01-02", candidate[:10])
				if err == nil {
					name := s[:i-1]
					return logFileParts{runbook: name, timestamp: candidate}
				}
			}
		}
	}
	// For append-mode files like "check-my-pi" (no timestamp)
	return logFileParts{runbook: s}
}
