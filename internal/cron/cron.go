package cron

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const marker = "# runbook:"

// Entry represents a runbook cron job.
type Entry struct {
	Name     string
	Schedule string
	Command  string
}

// Add installs a crontab entry for the given runbook.
func Add(name, schedule, logDir string) error {
	binPath, err := resolveRunbookBin()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("creating log dir: %w", err)
	}

	logFile := filepath.Join(logDir, name+".log")
	cmd := fmt.Sprintf("%s run --no-tui --yes %s >> %s 2>&1", binPath, name, logFile)
	line := fmt.Sprintf("%s %s %s %s", schedule, cmd, marker, name)

	existing, err := readCrontab()
	if err != nil {
		return err
	}

	// Remove any existing entry for this runbook
	filtered := filterOut(existing, name)
	filtered = append(filtered, line)

	return writeCrontab(filtered)
}

// Remove deletes the crontab entry for the given runbook.
func Remove(name string) error {
	existing, err := readCrontab()
	if err != nil {
		return err
	}

	filtered := filterOut(existing, name)
	if len(filtered) == len(existing) {
		return fmt.Errorf("no cron entry found for %q", name)
	}

	return writeCrontab(filtered)
}

// List returns all runbook-managed cron entries.
func List() ([]Entry, error) {
	lines, err := readCrontab()
	if err != nil {
		return nil, err
	}

	var entries []Entry
	for _, line := range lines {
		idx := strings.Index(line, marker)
		if idx < 0 {
			continue
		}
		name := strings.TrimSpace(line[idx+len(marker):])
		// Parse schedule: first 5 fields
		parts := strings.Fields(line[:idx])
		if len(parts) < 5 {
			continue
		}
		schedule := strings.Join(parts[:5], " ")
		command := strings.TrimSpace(strings.Join(parts[5:], " "))

		entries = append(entries, Entry{
			Name:     name,
			Schedule: schedule,
			Command:  command,
		})
	}
	return entries, nil
}

func readCrontab() ([]string, error) {
	out, err := exec.Command("crontab", "-l").Output()
	if err != nil {
		// No crontab for user is normal
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if strings.Contains(stderr, "no crontab") {
				return nil, nil
			}
		}
		return nil, fmt.Errorf("reading crontab: %w", err)
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	// Filter empty trailing line
	if len(lines) == 1 && lines[0] == "" {
		return nil, nil
	}
	return lines, nil
}

func writeCrontab(lines []string) error {
	content := strings.Join(lines, "\n") + "\n"
	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(content)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("writing crontab: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func filterOut(lines []string, name string) []string {
	tag := marker + " " + name
	var result []string
	for _, line := range lines {
		if !strings.Contains(line, tag) {
			result = append(result, line)
		}
	}
	return result
}

func resolveRunbookBin() (string, error) {
	// Try to find the absolute path of the running binary
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolving runbook binary path: %w", err)
	}
	abs, err := filepath.Abs(exe)
	if err != nil {
		return exe, nil
	}
	// Resolve symlinks
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return abs, nil
	}
	return resolved, nil
}
