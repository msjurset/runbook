// Package launchd installs LaunchAgent plists as an alternative scheduling
// backend on macOS. It exists because cron-launched runbooks run in a
// reduced launchd session that cannot read the user's login keychain;
// LaunchAgents in ~/Library/LaunchAgents bind to the user's GUI session
// instead, where keychain access works as expected.
package launchd

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// CalEntry is one StartCalendarInterval dictionary worth of trigger fields.
// Nil means "wildcard" (the corresponding key is omitted from the plist
// dict, which makes launchd treat it as 'match any').
type CalEntry struct {
	Minute  *int
	Hour    *int
	Day     *int // day of month
	Month   *int
	Weekday *int // 0 = Sunday, 6 = Saturday (matches both cron and launchd)
}

// ParseCron converts a 5-field cron expression into one or more CalEntry
// values. Each field may be:
//
//   - "*"            wildcard
//   - "n"            specific value
//   - "a,b,c"        list
//   - "a-b"          range
//   - "*/n" or "a-b/n"  step
//
// Cartesian product across fields with explicit values produces multiple
// entries. The returned slice is capped at maxCalEntries because launchd
// reads the entire array on every fire and "* * * * *" would blow up to
// roughly 16M tuples.
func ParseCron(expr string) ([]CalEntry, error) {
	parts := strings.Fields(expr)
	if len(parts) != 5 {
		return nil, fmt.Errorf("expected 5 cron fields, got %d in %q", len(parts), expr)
	}

	minute, err := parseField(parts[0], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("minute: %w", err)
	}
	hour, err := parseField(parts[1], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("hour: %w", err)
	}
	day, err := parseField(parts[2], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("day-of-month: %w", err)
	}
	month, err := parseField(parts[3], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("month: %w", err)
	}
	weekday, err := parseField(parts[4], 0, 6)
	if err != nil {
		return nil, fmt.Errorf("day-of-week: %w", err)
	}

	const maxCalEntries = 256
	var entries []CalEntry
	for _, mn := range expandField(minute) {
		for _, hr := range expandField(hour) {
			for _, dy := range expandField(day) {
				for _, mo := range expandField(month) {
					for _, wd := range expandField(weekday) {
						if len(entries) >= maxCalEntries {
							return nil, fmt.Errorf("schedule %q expands to more than %d trigger times — too granular for launchd", expr, maxCalEntries)
						}
						entries = append(entries, CalEntry{
							Minute: mn, Hour: hr, Day: dy, Month: mo, Weekday: wd,
						})
					}
				}
			}
		}
	}
	return entries, nil
}

// fieldSpec is a parsed cron field. wildcard==true means "match any value
// in this field"; otherwise values is a sorted list of explicit ints.
type fieldSpec struct {
	wildcard bool
	values   []int
}

func parseField(s string, lo, hi int) (fieldSpec, error) {
	if s == "*" {
		return fieldSpec{wildcard: true}, nil
	}
	seen := make(map[int]struct{})
	for _, part := range strings.Split(s, ",") {
		vals, err := parsePart(part, lo, hi)
		if err != nil {
			return fieldSpec{}, err
		}
		for _, v := range vals {
			seen[v] = struct{}{}
		}
	}
	out := make([]int, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Ints(out)
	return fieldSpec{values: out}, nil
}

// parsePart handles a single comma-delimited piece: a literal int, a range
// "a-b", a step "*/n", or a stepped range "a-b/n".
func parsePart(part string, lo, hi int) ([]int, error) {
	step := 1
	if i := strings.Index(part, "/"); i >= 0 {
		stepStr := part[i+1:]
		n, err := strconv.Atoi(stepStr)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("bad step %q", part)
		}
		step = n
		part = part[:i]
	}
	// "*" by itself with a step → expand from lo..hi
	if part == "*" {
		var out []int
		for v := lo; v <= hi; v += step {
			out = append(out, v)
		}
		return out, nil
	}
	// Range "a-b"
	if i := strings.Index(part, "-"); i > 0 {
		a, errA := strconv.Atoi(part[:i])
		b, errB := strconv.Atoi(part[i+1:])
		if errA != nil || errB != nil {
			return nil, fmt.Errorf("bad range %q", part)
		}
		if a < lo || b > hi || a > b {
			return nil, fmt.Errorf("range %q out of bounds [%d,%d]", part, lo, hi)
		}
		var out []int
		for v := a; v <= b; v += step {
			out = append(out, v)
		}
		return out, nil
	}
	// Literal integer
	n, err := strconv.Atoi(part)
	if err != nil {
		return nil, fmt.Errorf("bad value %q", part)
	}
	if n < lo || n > hi {
		return nil, fmt.Errorf("value %d out of bounds [%d,%d]", n, lo, hi)
	}
	return []int{n}, nil
}

// expandField converts a parsed field into the per-entry pointer values
// used by CalEntry. A wildcard yields one nil (omit the key in the plist
// dict); explicit values yield one *int per value.
func expandField(f fieldSpec) []*int {
	if f.wildcard {
		return []*int{nil}
	}
	out := make([]*int, len(f.values))
	for i, v := range f.values {
		v := v
		out[i] = &v
	}
	return out
}
