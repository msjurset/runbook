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

func TestFormatVarsForShell(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want string
	}{
		{"empty", nil, ""},
		{"single simple", []string{"k=v"}, "--var k='v'"},
		{"multiple", []string{"a=1", "b=2"}, "--var a='1' --var b='2'"},
		{"value with spaces", []string{"path=/tmp/my report.csv"}, "--var path='/tmp/my report.csv'"},
		{"value with apostrophe", []string{"msg=it's fine"}, `--var msg='it'\''s fine'`},
		{"value with equals", []string{"q=a=b"}, "--var q='a=b'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatVarsForShell(tt.in)
			if got != tt.want {
				t.Errorf("formatVarsForShell() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestShellSplitRoundTrip(t *testing.T) {
	// Roundtrip: format then shellSplit then parseVarsFromTokens should
	// recover the original key=value list.
	tests := [][]string{
		{"a=1"},
		{"a=1", "b=2"},
		{"path=/tmp/with space/file.csv"},
		{"msg=it's complicated", "x=y"},
		{"empty_val="},
	}
	for _, in := range tests {
		formatted := formatVarsForShell(in)
		tokens := shellSplit(formatted)
		got := parseVarsFromTokens(tokens)
		if len(got) != len(in) {
			t.Errorf("roundtrip(%v): got %v (%d), want %d entries", in, got, len(got), len(in))
			continue
		}
		for i := range in {
			if got[i] != in[i] {
				t.Errorf("roundtrip(%v)[%d] = %q, want %q", in, i, got[i], in[i])
			}
		}
	}
}

func TestValidateVars(t *testing.T) {
	tests := []struct {
		name    string
		in      []string
		wantErr bool
	}{
		{"empty", nil, false},
		{"valid", []string{"a=1", "b=2"}, false},
		{"empty value", []string{"k="}, false},
		{"no equals", []string{"foo"}, true},
		{"empty key", []string{"=value"}, true},
		{"whitespace key", []string{"a b=v"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVars(tt.in)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateVars(%v) err=%v, wantErr=%v", tt.in, err, tt.wantErr)
			}
		})
	}
}

func TestParseLaunchdMarkerWithVars(t *testing.T) {
	line := "# runbook(launchd) my-report: 0 6 * * * --var report_type='daily' --var path='/tmp/r.csv'"
	name, schedule, vars := parseLaunchdMarker(line)
	if name != "my-report" {
		t.Errorf("name = %q, want %q", name, "my-report")
	}
	if schedule != "0 6 * * *" {
		t.Errorf("schedule = %q, want %q", schedule, "0 6 * * *")
	}
	want := []string{"report_type=daily", "path=/tmp/r.csv"}
	if len(vars) != len(want) {
		t.Fatalf("vars len = %d, want %d (got %v)", len(vars), len(want), vars)
	}
	for i := range want {
		if vars[i] != want[i] {
			t.Errorf("vars[%d] = %q, want %q", i, vars[i], want[i])
		}
	}
}

func TestParseLaunchdMarkerNoVars(t *testing.T) {
	// Existing markers without --var must still parse — we ship to users
	// who already have schedules and must keep their crontab valid.
	line := "# runbook(launchd) my-report: 0 6 * * *"
	name, schedule, vars := parseLaunchdMarker(line)
	if name != "my-report" {
		t.Errorf("name = %q", name)
	}
	if schedule != "0 6 * * *" {
		t.Errorf("schedule = %q", schedule)
	}
	if len(vars) != 0 {
		t.Errorf("vars = %v, want empty", vars)
	}
}
