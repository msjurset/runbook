package cron

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/msjurset/runbook/internal/launchd"
)

// Backend selects how a scheduled runbook is actually fired.
//
// - BackendCron: a normal crontab line. Cheap, universal, but on macOS
//   cron-launched processes can't read the user's login keychain.
// - BackendLaunchd: a per-user LaunchAgent plist that runs in the user's
//   GUI session. Required on macOS for runbooks that resolve op:// secrets
//   at runtime; not available on Linux/Windows (where Available() == false).
type Backend string

const (
	BackendCron    Backend = "cron"
	BackendLaunchd Backend = "launchd"
)

// markers in crontab lines:
//
//	"<schedule> <command> # runbook: <name>"               → BackendCron
//	"# runbook(launchd) <name>: <schedule>"                 → BackendLaunchd
//
// The launchd marker is a pure comment line so cron skips it; its purpose
// is to keep `crontab -l` showing every scheduled runbook the user has,
// regardless of which backend actually fires it.
const (
	cronMarker    = "# runbook:"
	launchdPrefix = "# runbook(launchd) "
)

// Entry represents a runbook schedule, regardless of backend.
type Entry struct {
	Name     string
	Schedule string
	Command  string
	Backend  Backend
}

// Add installs a schedule for the runbook. Multiple schedules per runbook
// are allowed. Adding the same name+schedule combination replaces the
// existing entry. The launchd backend is macOS-only; callers should fall
// back to BackendCron on other platforms (Add returns an error for an
// unsupported backend rather than silently dispatching).
func Add(name, schedule, logDir string, backend Backend) error {
	switch backend {
	case BackendLaunchd:
		return addLaunchd(name, schedule, logDir)
	case BackendCron, "":
		return addCron(name, schedule, logDir)
	default:
		return fmt.Errorf("unknown backend %q", backend)
	}
}

func addCron(name, schedule, logDir string) error {
	binPath, err := resolveRunbookBin()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("creating log dir: %w", err)
	}

	logFile := filepath.Join(logDir, name+".log")
	cmd := fmt.Sprintf("%s run --no-tui --yes %s >> %s 2>&1", binPath, name, logFile)
	line := fmt.Sprintf("%s %s %s %s", schedule, cmd, cronMarker, name)

	existing, err := readCrontab()
	if err != nil {
		return err
	}

	// Replace any prior schedule for this name+sched (including a launchd
	// marker that may have been left over from a previous backend choice).
	filtered := filterOutExact(existing, name, schedule)
	// Also drop any active launchd plist so we don't end up double-firing.
	if launchd.IsInstalled(name) {
		_ = launchd.Uninstall(name)
	}
	filtered = append(filtered, line)
	return writeCrontab(filtered)
}

func addLaunchd(name, schedule, logDir string) error {
	if !launchd.Available() {
		return fmt.Errorf("LaunchAgent backend not available on this platform")
	}
	binPath, err := resolveRunbookBin()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("creating log dir: %w", err)
	}
	logFile := filepath.Join(logDir, name+".log")

	if err := launchd.Install(name, schedule, binPath, logFile); err != nil {
		return err
	}

	// Drop any prior crontab entries for this name (active or launchd
	// marker) and add the marker pointing at the new schedule.
	existing, err := readCrontab()
	if err != nil {
		return err
	}
	filtered := filterOutByName(existing, name)
	filtered = append(filtered, fmt.Sprintf("%s%s: %s", launchdPrefix, name, schedule))
	return writeCrontab(filtered)
}

// Remove deletes ALL schedules for a runbook (both backends).
func Remove(name string) error {
	return RemoveSchedule(name, "")
}

// RemoveSchedule deletes a specific schedule for a runbook. Empty schedule
// removes all of them. Tears down the LaunchAgent (if any) too — for the
// "all" case unconditionally, for the "specific schedule" case only when
// the matching marker is launchd-backed.
func RemoveSchedule(name, schedule string) error {
	existing, err := readCrontab()
	if err != nil {
		return err
	}

	matched := false
	hadLaunchd := false
	var filtered []string

	if schedule == "" {
		// Remove every line tagged with this runbook (cron entries, launchd
		// markers, both).
		for _, line := range existing {
			if isCronLineFor(line, name) {
				matched = true
				continue
			}
			if isLaunchdMarkerFor(line, name) {
				matched = true
				hadLaunchd = true
				continue
			}
			filtered = append(filtered, line)
		}
	} else {
		// Remove only the matching schedule. Try cron first; if no match,
		// fall through to launchd marker.
		for _, line := range existing {
			if isCronLineFor(line, name) && strings.HasPrefix(strings.TrimSpace(line), schedule+" ") {
				matched = true
				continue
			}
			if isLaunchdMarkerFor(line, name) {
				_, sched := parseLaunchdMarker(line)
				if sched == schedule {
					matched = true
					hadLaunchd = true
					continue
				}
			}
			filtered = append(filtered, line)
		}
	}

	if !matched {
		if schedule != "" {
			return fmt.Errorf("no schedule found for %q at %q", name, schedule)
		}
		return fmt.Errorf("no schedules found for %q", name)
	}

	// If there are no remaining launchd markers for this name, tear the
	// plist down. Multiple launchd schedules per runbook aren't a thing
	// today (the plist is overwritten on each Install), so this is
	// straightforward: any matched launchd marker → uninstall.
	if hadLaunchd {
		if err := launchd.Uninstall(name); err != nil {
			return fmt.Errorf("uninstalling LaunchAgent: %w", err)
		}
	}
	return writeCrontab(filtered)
}

// List returns all runbook-managed schedules across both backends, sourced
// from the crontab (active lines + launchd marker comments). The launchd
// plist on disk is the executable schedule; the marker line is what makes
// it visible in `crontab -l`.
func List() ([]Entry, error) {
	lines, err := readCrontab()
	if err != nil {
		return nil, err
	}

	var entries []Entry
	for _, line := range lines {
		// Active cron entry: "<schedule> <command> # runbook: <name>"
		if idx := strings.Index(line, cronMarker); idx >= 0 && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			name := strings.TrimSpace(line[idx+len(cronMarker):])
			parts := strings.Fields(line[:idx])
			if len(parts) < 5 {
				continue
			}
			schedule := strings.Join(parts[:5], " ")
			command := strings.TrimSpace(strings.Join(parts[5:], " "))
			entries = append(entries, Entry{
				Name: name, Schedule: schedule, Command: command, Backend: BackendCron,
			})
			continue
		}
		// Launchd marker: "# runbook(launchd) <name>: <schedule>"
		if name, schedule := parseLaunchdMarker(line); name != "" {
			entries = append(entries, Entry{
				Name: name, Schedule: schedule, Backend: BackendLaunchd,
			})
		}
	}
	return entries, nil
}

// isCronLineFor returns true if the line is an ACTIVE cron entry for name
// (i.e. the marker appears and the line itself isn't a comment).
func isCronLineFor(line, name string) bool {
	tag := cronMarker + " " + name
	if !strings.Contains(line, tag) {
		return false
	}
	return !strings.HasPrefix(strings.TrimSpace(line), "#")
}

func isLaunchdMarkerFor(line, name string) bool {
	got, _ := parseLaunchdMarker(line)
	return got == name
}

// parseLaunchdMarker pulls (name, schedule) out of a "# runbook(launchd) name: schedule"
// line. Returns ("", "") if line isn't a launchd marker.
func parseLaunchdMarker(line string) (name, schedule string) {
	t := strings.TrimSpace(line)
	if !strings.HasPrefix(t, launchdPrefix) {
		return "", ""
	}
	rest := strings.TrimPrefix(t, launchdPrefix)
	colon := strings.Index(rest, ":")
	if colon < 0 {
		return "", ""
	}
	return strings.TrimSpace(rest[:colon]), strings.TrimSpace(rest[colon+1:])
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

// filterOutByName removes all cron lines AND launchd markers for the
// given runbook name.
func filterOutByName(lines []string, name string) []string {
	tag := cronMarker + " " + name
	var result []string
	for _, line := range lines {
		if strings.Contains(line, tag) {
			continue
		}
		if got, _ := parseLaunchdMarker(line); got == name {
			continue
		}
		result = append(result, line)
	}
	return result
}

// filterOutExact removes the cron entry matching both name and schedule.
// Doesn't touch launchd markers — RemoveSchedule handles those because it
// also needs to call launchd.Uninstall.
func filterOutExact(lines []string, name, schedule string) []string {
	tag := cronMarker + " " + name
	var result []string
	for _, line := range lines {
		if strings.Contains(line, tag) && strings.HasPrefix(strings.TrimSpace(line), schedule+" ") {
			continue
		}
		result = append(result, line)
	}
	return result
}

func resolveRunbookBin() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolving runbook binary path: %w", err)
	}
	abs, err := filepath.Abs(exe)
	if err != nil {
		return exe, nil
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return abs, nil
	}
	return resolved, nil
}
