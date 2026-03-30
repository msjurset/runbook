package cron

import (
	"strings"
	"testing"
)

func TestFilterOutByName(t *testing.T) {
	lines := []string{
		"0 3 * * 0 /usr/bin/runbook run --no-tui --yes deploy >> /tmp/deploy.log 2>&1 # runbook: deploy",
		"*/5 * * * * /usr/bin/check-disk # not a runbook entry",
		"0 * * * * /usr/bin/runbook run --no-tui --yes backup >> /tmp/backup.log 2>&1 # runbook: backup",
		"0 9 * * 1 /usr/bin/runbook run --no-tui --yes deploy >> /tmp/deploy.log 2>&1 # runbook: deploy",
	}

	// Remove all entries for "deploy" — should remove 2
	filtered := filterOutByName(lines, "deploy")
	if len(filtered) != 2 {
		t.Errorf("filterOutByName(deploy) = %d lines, want 2", len(filtered))
	}
	for _, line := range filtered {
		if strings.Contains(line, "# runbook: deploy") {
			t.Error("deploy entry should have been removed")
		}
	}
}

func TestFilterOutByNameNoMatch(t *testing.T) {
	lines := []string{
		"0 3 * * 0 something # runbook: deploy",
	}
	filtered := filterOutByName(lines, "nonexistent")
	if len(filtered) != 1 {
		t.Errorf("filterOutByName(nonexistent) = %d lines, want 1", len(filtered))
	}
}

func TestFilterOutExact(t *testing.T) {
	lines := []string{
		"0 3 * * 0 /usr/bin/runbook run --no-tui --yes deploy >> /tmp/deploy.log 2>&1 # runbook: deploy",
		"0 9 * * 1 /usr/bin/runbook run --no-tui --yes deploy >> /tmp/deploy.log 2>&1 # runbook: deploy",
		"0 * * * * /usr/bin/runbook run --no-tui --yes backup >> /tmp/backup.log 2>&1 # runbook: backup",
	}

	// Remove only the Sunday 3 AM schedule for deploy
	filtered := filterOutExact(lines, "deploy", "0 3 * * 0")
	if len(filtered) != 2 {
		t.Errorf("filterOutExact(deploy, 0 3 * * 0) = %d lines, want 2", len(filtered))
	}

	// The Monday 9 AM deploy should remain
	found := false
	for _, line := range filtered {
		if strings.Contains(line, "# runbook: deploy") && strings.HasPrefix(line, "0 9 * * 1") {
			found = true
		}
	}
	if !found {
		t.Error("Monday deploy entry should have been kept")
	}
}

func TestFilterOutExactNoMatch(t *testing.T) {
	lines := []string{
		"0 3 * * 0 something # runbook: deploy",
	}
	filtered := filterOutExact(lines, "deploy", "0 9 * * 1")
	if len(filtered) != 1 {
		t.Errorf("filterOutExact with non-matching schedule = %d lines, want 1", len(filtered))
	}
}

func TestFilterOutExactDoesNotRemoveOtherRunbooks(t *testing.T) {
	lines := []string{
		"0 3 * * 0 /usr/bin/runbook run --no-tui --yes deploy # runbook: deploy",
		"0 3 * * 0 /usr/bin/runbook run --no-tui --yes backup # runbook: backup",
	}

	// Remove deploy at 0 3 * * 0 — should NOT remove backup at the same schedule
	filtered := filterOutExact(lines, "deploy", "0 3 * * 0")
	if len(filtered) != 1 {
		t.Errorf("filterOutExact = %d lines, want 1", len(filtered))
	}
	if !strings.Contains(filtered[0], "# runbook: backup") {
		t.Error("backup entry should have been kept")
	}
}

func TestMultipleSchedulesPerRunbook(t *testing.T) {
	// Simulate adding two schedules for the same runbook
	lines := []string{}

	// Add first schedule
	line1 := "0 3 * * 0 /usr/bin/runbook run --no-tui --yes deploy >> /tmp/deploy.log 2>&1 # runbook: deploy"
	lines = filterOutExact(lines, "deploy", "0 3 * * 0")
	lines = append(lines, line1)

	// Add second schedule — should NOT remove the first
	line2 := "0 9 * * 1 /usr/bin/runbook run --no-tui --yes deploy >> /tmp/deploy.log 2>&1 # runbook: deploy"
	lines = filterOutExact(lines, "deploy", "0 9 * * 1")
	lines = append(lines, line2)

	if len(lines) != 2 {
		t.Errorf("after adding two schedules: %d lines, want 2", len(lines))
	}

	// Adding same schedule again should replace it (not duplicate)
	line3 := "0 3 * * 0 /usr/bin/runbook run --no-tui --yes deploy >> /tmp/deploy.log 2>&1 # runbook: deploy"
	lines = filterOutExact(lines, "deploy", "0 3 * * 0")
	lines = append(lines, line3)

	if len(lines) != 2 {
		t.Errorf("after re-adding same schedule: %d lines, want 2", len(lines))
	}
}

func TestRemoveAllSchedulesForRunbook(t *testing.T) {
	lines := []string{
		"0 3 * * 0 cmd # runbook: deploy",
		"0 9 * * 1 cmd # runbook: deploy",
		"0 0 * * * cmd # runbook: backup",
	}

	filtered := filterOutByName(lines, "deploy")
	if len(filtered) != 1 {
		t.Errorf("filterOutByName(deploy) = %d lines, want 1", len(filtered))
	}
	if !strings.Contains(filtered[0], "# runbook: backup") {
		t.Error("backup should remain")
	}
}

func TestResolveRunbookBin(t *testing.T) {
	path, err := resolveRunbookBin()
	if err != nil {
		t.Fatalf("resolveRunbookBin() error: %v", err)
	}
	if path == "" {
		t.Error("resolveRunbookBin() returned empty path")
	}
}
