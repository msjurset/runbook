package notify

import (
	"fmt"
	"strings"

	"github.com/msjurset/runbook/internal/credentials"
	"github.com/msjurset/runbook/internal/engine"
	"github.com/msjurset/runbook/internal/runbook"
)

// Send dispatches notifications based on the runbook's notify config and the run result.
func Send(book *runbook.Runbook, result engine.RunResult) []error {
	cfg := book.Notify
	if cfg == nil {
		return nil
	}

	if !shouldNotify(cfg.On, result.Success) {
		return nil
	}

	subject, body := formatMessage(book.Name, result)

	var errs []error

	if cfg.Slack != nil {
		if err := sendSlack(cfg.Slack, subject, body); err != nil {
			errs = append(errs, fmt.Errorf("slack: %w", err))
		}
	}

	if cfg.MacOS {
		if err := sendMacOS(book.Name, subject); err != nil {
			errs = append(errs, fmt.Errorf("macos: %w", err))
		}
	}

	if cfg.Email != nil {
		if err := sendEmail(cfg.Email, subject, body); err != nil {
			errs = append(errs, fmt.Errorf("email: %w", err))
		}
	}

	return errs
}

func shouldNotify(on string, success bool) bool {
	switch strings.ToLower(on) {
	case "failure":
		return !success
	case "success":
		return success
	case "always", "":
		return true
	default:
		return true
	}
}

func formatMessage(name string, result engine.RunResult) (subject, body string) {
	status := "succeeded"
	emoji := "✓"
	if !result.Success {
		status = "failed"
		emoji = "✗"
	}

	subject = fmt.Sprintf("%s Runbook %q %s", emoji, name, status)

	var b strings.Builder
	fmt.Fprintf(&b, "Runbook: %s\n", name)
	fmt.Fprintf(&b, "Status: %s\n", status)
	fmt.Fprintf(&b, "Duration: %s\n", result.Duration.Round(100*1e6))
	fmt.Fprintf(&b, "Steps: %d\n\n", len(result.Steps))

	for _, s := range result.Steps {
		indicator := "✓"
		switch s.Status {
		case engine.StatusFailed:
			indicator = "✗"
		case engine.StatusSkipped:
			indicator = "-"
		case engine.StatusPending:
			indicator = " "
		}
		line := fmt.Sprintf("  %s %s (%s)", indicator, s.StepName, s.Duration.Round(100*1e6))
		if s.Error != nil {
			line += fmt.Sprintf(" — %v", s.Error)
		}
		fmt.Fprintln(&b, line)
	}

	return subject, b.String()
}

// resolveOpRef resolves a value that may be an op:// reference.
func resolveOpRef(val, keychainKey string) (string, error) {
	if credentials.IsOpRef(val) {
		return credentials.LoadOrResolve(keychainKey, val)
	}
	return val, nil
}
