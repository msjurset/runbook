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
//
// Vars is the list of CLI variables baked into the schedule, in
// "key=value" form. They're appended to the scheduled `runbook run`
// invocation as `--var key=value` pairs so the same runbook can be
// scheduled multiple times with different inputs (daily vs monthly
// report, dev vs prod, etc.).
type Entry struct {
	Name     string
	Schedule string
	Command  string
	Backend  Backend
	Vars     []string
}

// Add installs a schedule for the runbook. Multiple schedules per runbook
// are allowed. Adding the same name+schedule combination replaces the
// existing entry. The launchd backend is macOS-only; callers should fall
// back to BackendCron on other platforms (Add returns an error for an
// unsupported backend rather than silently dispatching).
//
// vars is the list of CLI variables (in "key=value" form) to bake into
// the scheduled invocation. They become trailing `--var key=value`
// arguments to `runbook run`. Pass nil for a vanilla schedule.
func Add(name, schedule, logDir string, backend Backend, vars []string) error {
	if err := validateVars(vars); err != nil {
		return err
	}
	switch backend {
	case BackendLaunchd:
		return addLaunchd(name, schedule, logDir, vars)
	case BackendCron, "":
		return addCron(name, schedule, logDir, vars)
	default:
		return fmt.Errorf("unknown backend %q", backend)
	}
}

func addCron(name, schedule, logDir string, vars []string) error {
	binPath, err := resolveRunbookBin()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("creating log dir: %w", err)
	}

	logFile := filepath.Join(logDir, name+".log")
	cmd := fmt.Sprintf("%s run --no-tui --yes %s", binPath, name)
	if v := formatVarsForShell(vars); v != "" {
		cmd += " " + v
	}
	cmd += fmt.Sprintf(" >> %s 2>&1", logFile)
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

func addLaunchd(name, schedule, logDir string, vars []string) error {
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

	if err := launchd.Install(name, schedule, binPath, logFile, vars); err != nil {
		return err
	}

	// Drop any prior crontab entries for this name (active or launchd
	// marker) and add the marker pointing at the new schedule.
	existing, err := readCrontab()
	if err != nil {
		return err
	}
	filtered := filterOutByName(existing, name)
	marker := fmt.Sprintf("%s%s: %s", launchdPrefix, name, schedule)
	if v := formatVarsForShell(vars); v != "" {
		marker += " " + v
	}
	filtered = append(filtered, marker)
	return writeCrontab(filtered)
}

// validateVars rejects malformed entries before they hit the crontab so
// the user gets a clear error instead of a silently broken schedule.
func validateVars(vars []string) error {
	for _, v := range vars {
		k, _, ok := strings.Cut(v, "=")
		if !ok || k == "" {
			return fmt.Errorf("invalid --var %q (expected key=value)", v)
		}
		if strings.ContainsAny(k, " \t\n\r") {
			return fmt.Errorf("invalid --var key %q (no whitespace allowed)", k)
		}
	}
	return nil
}

// formatVarsForShell renders --var pairs as space-separated args suitable
// for inclusion in a crontab line. Values are wrapped in single quotes so
// internal whitespace and shell metacharacters survive cron's shell-parse;
// literal single quotes inside a value are escaped with the POSIX '\''
// idiom.
func formatVarsForShell(vars []string) string {
	if len(vars) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, v := range vars {
		if i > 0 {
			sb.WriteByte(' ')
		}
		k, val, _ := strings.Cut(v, "=")
		sb.WriteString("--var ")
		sb.WriteString(k)
		sb.WriteByte('=')
		sb.WriteByte('\'')
		sb.WriteString(strings.ReplaceAll(val, "'", `'\''`))
		sb.WriteByte('\'')
	}
	return sb.String()
}

// parseVarsFromTokens pulls "--var key=value" pairs out of a slice of
// already-tokenized command parts (the output of splitting a stored
// crontab command on whitespace AFTER single-quote handling). Returns the
// vars in original order. Used by List to recover the structured form
// from a stored crontab line.
func parseVarsFromTokens(tokens []string) []string {
	var out []string
	for i := 0; i < len(tokens); i++ {
		if tokens[i] != "--var" {
			continue
		}
		if i+1 >= len(tokens) {
			break
		}
		out = append(out, tokens[i+1])
		i++
	}
	return out
}

// shellSplit is a small shell-style tokenizer that understands single
// quotes (with POSIX '\'' escaping). Sufficient for our crontab lines,
// which we authored ourselves — we don't need full shell grammar.
func shellSplit(s string) []string {
	var out []string
	var cur strings.Builder
	inQuote := false
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\'' && !inQuote:
			inQuote = true
		case c == '\'' && inQuote:
			// POSIX '\'' escape: closing quote immediately followed by
			// \' then ' = a literal single quote inside the value.
			if i+3 < len(s) && s[i+1] == '\\' && s[i+2] == '\'' && s[i+3] == '\'' {
				cur.WriteByte('\'')
				i += 3
				continue
			}
			inQuote = false
		case !inQuote && (c == ' ' || c == '\t'):
			flush()
		default:
			cur.WriteByte(c)
		}
	}
	flush()
	return out
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
				_, sched, _ := parseLaunchdMarker(line)
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
			// Schedule is the first 5 whitespace-separated fields; the
			// command is whatever's left up to the marker. We shellSplit
			// the command so single-quoted --var values come back as
			// single tokens for parseVarsFromTokens.
			parts := strings.Fields(line[:idx])
			if len(parts) < 5 {
				continue
			}
			schedule := strings.Join(parts[:5], " ")
			command := strings.TrimSpace(strings.Join(parts[5:], " "))
			vars := parseVarsFromTokens(shellSplit(command))
			entries = append(entries, Entry{
				Name: name, Schedule: schedule, Command: command, Backend: BackendCron, Vars: vars,
			})
			continue
		}
		// Launchd marker: "# runbook(launchd) <name>: <schedule> [--var k=v ...]"
		if name, schedule, vars := parseLaunchdMarker(line); name != "" {
			entries = append(entries, Entry{
				Name: name, Schedule: schedule, Backend: BackendLaunchd, Vars: vars,
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
	got, _, _ := parseLaunchdMarker(line)
	return got == name
}

// parseLaunchdMarker pulls (name, schedule, vars) out of a
// "# runbook(launchd) name: <5-field schedule> [--var k=v ...]" line.
// Returns ("", "", nil) if line isn't a launchd marker. The first 5
// whitespace-separated fields after the colon are the cron schedule; the
// remainder, if any, is parsed for --var pairs via shellSplit so single-
// quoted values containing spaces survive the round-trip.
func parseLaunchdMarker(line string) (name, schedule string, vars []string) {
	t := strings.TrimSpace(line)
	if !strings.HasPrefix(t, launchdPrefix) {
		return "", "", nil
	}
	rest := strings.TrimPrefix(t, launchdPrefix)
	colon := strings.Index(rest, ":")
	if colon < 0 {
		return "", "", nil
	}
	name = strings.TrimSpace(rest[:colon])
	tail := strings.TrimSpace(rest[colon+1:])
	tokens := shellSplit(tail)
	if len(tokens) < 5 {
		return name, tail, nil
	}
	schedule = strings.Join(tokens[:5], " ")
	vars = parseVarsFromTokens(tokens[5:])
	return name, schedule, vars
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
		if got, _, _ := parseLaunchdMarker(line); got == name {
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
