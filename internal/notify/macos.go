package notify

import (
	"fmt"
	"os/exec"
)

func sendMacOS(runbookName, subject string) error {
	script := fmt.Sprintf(
		`display notification %q with title "runbook" subtitle %q`,
		subject, runbookName,
	)
	cmd := exec.Command("osascript", "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("osascript: %s: %w", string(out), err)
	}
	return nil
}
