package engine

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/msjurset/runbook/internal/runbook"
)

// CLIObserver implements Observer with plain text output to stdout.
type CLIObserver struct {
	AutoConfirm bool
}

func (o *CLIObserver) OnStepStart(index int, step runbook.Step) {
	fmt.Printf("\n▸ Step %d: %s\n", index+1, step.Name)
	if step.Type != "" {
		fmt.Printf("  type: %s\n", step.Type)
	}
}

func (o *CLIObserver) OnStepOutput(_ int, line string) {
	fmt.Printf("  │ %s\n", line)
}

func (o *CLIObserver) OnStepComplete(index int, result StepResult) {
	switch result.Status {
	case StatusSuccess:
		fmt.Printf("  ✓ done (%s)\n", result.Duration.Round(100*1e6))
	case StatusFailed:
		fmt.Printf("  ✗ failed (%s): %v\n", result.Duration.Round(100*1e6), result.Error)
	case StatusSkipped:
		fmt.Printf("  - skipped\n")
	case StatusRetrying:
		fmt.Printf("  ~ retrying...\n")
	}
}

func (o *CLIObserver) OnRunComplete(result RunResult) {
	fmt.Println()
	if result.Success {
		fmt.Printf("✓ Runbook %q completed successfully (%s)\n", result.RunbookName, result.Duration.Round(100*1e6))
	} else {
		failed := 0
		for _, s := range result.Steps {
			if s.Status == StatusFailed {
				failed++
			}
		}
		fmt.Printf("✗ Runbook %q failed (%d step(s) failed, %s)\n", result.RunbookName, failed, result.Duration.Round(100*1e6))
	}
}

func (o *CLIObserver) OnPrompt(_ int, message string) (bool, error) {
	if o.AutoConfirm {
		fmt.Printf("  ? %s [auto-confirmed]\n", message)
		return true, nil
	}

	fmt.Printf("  ? %s [y/N] ", message)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes", nil
}
