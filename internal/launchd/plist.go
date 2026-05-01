package launchd

import (
	"fmt"
	"strings"
)

// LabelPrefix is the launchd Label namespace for runbook-managed agents.
// `launchctl list` and `launchctl bootout` use the label, so a stable
// prefix lets us discover and unload runbook agents reliably.
const LabelPrefix = "dev.runbook."

// LabelFor returns the stable launchd Label for a runbook by name.
func LabelFor(name string) string { return LabelPrefix + name }

// PlistFor renders the LaunchAgent property list for one runbook.
// `binPath` is the runbook executable (resolved by the caller),
// `runbookName` is the YAML runbook to invoke, `logFile` is where
// stdout/stderr are appended, and `entries` are the trigger times
// produced by ParseCron.
func PlistFor(label, binPath, runbookName, logFile string, entries []CalEntry) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	b.WriteString(`<plist version="1.0">` + "\n")
	b.WriteString("<dict>\n")

	keyString(&b, "Label", label, "  ")

	b.WriteString("  <key>ProgramArguments</key>\n")
	b.WriteString("  <array>\n")
	for _, arg := range []string{binPath, "run", "--no-tui", "--yes", runbookName} {
		fmt.Fprintf(&b, "    <string>%s</string>\n", xmlEscape(arg))
	}
	b.WriteString("  </array>\n")

	if len(entries) == 1 {
		b.WriteString("  <key>StartCalendarInterval</key>\n")
		writeEntryDict(&b, entries[0], "  ")
	} else {
		b.WriteString("  <key>StartCalendarInterval</key>\n")
		b.WriteString("  <array>\n")
		for _, e := range entries {
			writeEntryDict(&b, e, "    ")
		}
		b.WriteString("  </array>\n")
	}

	keyString(&b, "StandardOutPath", logFile, "  ")
	keyString(&b, "StandardErrorPath", logFile, "  ")

	b.WriteString("</dict>\n")
	b.WriteString("</plist>\n")
	return b.String()
}

func writeEntryDict(b *strings.Builder, e CalEntry, indent string) {
	fmt.Fprintf(b, "%s<dict>\n", indent)
	if e.Minute != nil {
		keyInt(b, "Minute", *e.Minute, indent+"  ")
	}
	if e.Hour != nil {
		keyInt(b, "Hour", *e.Hour, indent+"  ")
	}
	if e.Day != nil {
		keyInt(b, "Day", *e.Day, indent+"  ")
	}
	if e.Month != nil {
		keyInt(b, "Month", *e.Month, indent+"  ")
	}
	if e.Weekday != nil {
		keyInt(b, "Weekday", *e.Weekday, indent+"  ")
	}
	fmt.Fprintf(b, "%s</dict>\n", indent)
}

func keyString(b *strings.Builder, key, value, indent string) {
	fmt.Fprintf(b, "%s<key>%s</key>\n", indent, key)
	fmt.Fprintf(b, "%s<string>%s</string>\n", indent, xmlEscape(value))
}

func keyInt(b *strings.Builder, key string, value int, indent string) {
	fmt.Fprintf(b, "%s<key>%s</key>\n", indent, key)
	fmt.Fprintf(b, "%s<integer>%d</integer>\n", indent, value)
}

// xmlEscape replaces the five XML-special characters. Plist values come from
// trusted local config (binary paths, runbook names, log file paths) but we
// escape defensively in case a runbook name contains an apostrophe etc.
func xmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return r.Replace(s)
}
