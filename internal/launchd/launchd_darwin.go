//go:build darwin

package launchd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Available reports whether this build can install LaunchAgent plists. It's
// always true on Darwin and false elsewhere — callers use this to decide
// between the launchd and crontab backends without a build-tag layer.
func Available() bool { return true }

// PlistPathFor returns the canonical install location for a runbook's
// LaunchAgent. ~/Library/LaunchAgents is the per-user agent directory
// loaded by launchd into the user's GUI session.
func PlistPathFor(name string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", LabelFor(name)+".plist")
}

// Install writes the plist for a runbook and bootstraps it into the user's
// gui domain. Idempotent: if a plist already exists at the target path it's
// booted out first so the new one takes effect.
func Install(name, schedule, binPath, logFile string) error {
	entries, err := ParseCron(schedule)
	if err != nil {
		return fmt.Errorf("parsing schedule: %w", err)
	}

	plistPath := PlistPathFor(name)
	label := LabelFor(name)
	contents := PlistFor(label, binPath, name, logFile, entries)

	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		return fmt.Errorf("creating LaunchAgents dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(logFile), 0o755); err != nil {
		return fmt.Errorf("creating log dir: %w", err)
	}

	// Best-effort bootout of any prior version. Errors are ignored — the
	// plist may not be loaded, and bootstrap below will surface a real
	// failure if there's still a conflict.
	_ = exec.Command("launchctl", "bootout", guiTarget(), plistPath).Run()

	if err := os.WriteFile(plistPath, []byte(contents), 0o644); err != nil {
		return fmt.Errorf("writing plist: %w", err)
	}

	out, err := exec.Command("launchctl", "bootstrap", guiTarget(), plistPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl bootstrap: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// Uninstall removes a runbook's LaunchAgent: bootout, then delete the
// plist. Returns nil if there was nothing to remove (idempotent on the
// "already gone" case so callers don't need to stat first).
func Uninstall(name string) error {
	plistPath := PlistPathFor(name)
	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		return nil
	}
	// Bootout's error is ignored: if it wasn't loaded we still want to
	// delete the file. The os.Remove below surfaces real I/O issues.
	_ = exec.Command("launchctl", "bootout", guiTarget(), plistPath).Run()
	if err := os.Remove(plistPath); err != nil {
		return fmt.Errorf("removing plist: %w", err)
	}
	return nil
}

// IsInstalled reports whether a LaunchAgent plist exists for this runbook.
// Doesn't check `launchctl list`; presence of the plist is the source of
// truth (the cron package's marker line in crontab is a parallel record).
func IsInstalled(name string) bool {
	_, err := os.Stat(PlistPathFor(name))
	return err == nil
}

// guiTarget builds the launchctl service target string for the current
// user, e.g. "gui/501". `bootstrap`/`bootout` need this to know which
// per-user domain to load the agent into.
func guiTarget() string {
	return "gui/" + strconv.Itoa(os.Getuid())
}
