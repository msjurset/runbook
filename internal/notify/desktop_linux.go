package notify

import (
	"fmt"
	"os/exec"
)

func sendDesktop(runbookName, subject string) error {
	cmd := exec.Command("notify-send",
		"--app-name=runbook",
		runbookName,
		subject,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("notify-send: %s: %w", string(out), err)
	}
	return nil
}
