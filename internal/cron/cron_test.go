package cron

import (
	"strings"
	"testing"
)

func TestFilterOut(t *testing.T) {
	lines := []string{
		"0 3 * * 0 /usr/bin/runbook run --no-tui --yes deploy >> /tmp/deploy.log 2>&1 # runbook: deploy",
		"*/5 * * * * /usr/bin/check-disk # not a runbook entry",
		"0 * * * * /usr/bin/runbook run --no-tui --yes backup >> /tmp/backup.log 2>&1 # runbook: backup",
	}

	filtered := filterOut(lines, "deploy")
	if len(filtered) != 2 {
		t.Errorf("filterOut(deploy) = %d lines, want 2", len(filtered))
	}
	for _, line := range filtered {
		if strings.Contains(line, "# runbook: deploy") {
			t.Error("deploy entry should have been removed")
		}
	}
}

func TestFilterOutNoMatch(t *testing.T) {
	lines := []string{
		"0 3 * * 0 something # runbook: deploy",
	}
	filtered := filterOut(lines, "nonexistent")
	if len(filtered) != 1 {
		t.Errorf("filterOut(nonexistent) = %d lines, want 1", len(filtered))
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
