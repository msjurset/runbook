package engine

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/msjurset/runbook/internal/runbook"
)

// CLIObserver implements Observer with plain text output to stdout.
type CLIObserver struct {
	AutoConfirm bool
	logBuf      bytes.Buffer
}

// LogOutput returns all captured output for logging.
func (o *CLIObserver) LogOutput() string {
	return o.logBuf.String()
}

func (o *CLIObserver) write(format string, args ...any) {
	s := fmt.Sprintf(format, args...)
	fmt.Print(s)
	o.logBuf.WriteString(s)
}

func (o *CLIObserver) writeln(w io.Writer, format string, args ...any) {
	s := fmt.Sprintf(format, args...)
	fmt.Fprint(w, s)
	o.logBuf.WriteString(s)
}

func (o *CLIObserver) OnStepStart(index int, step runbook.Step) {
	o.write("\n▸ Step %d: %s\n", index+1, step.Name)
	if step.Type != "" {
		o.write("  type: %s\n", step.Type)
	}
}

func (o *CLIObserver) OnStepOutput(_ int, line string) {
	o.write("  │ %s\n", line)
}

func (o *CLIObserver) OnStepComplete(index int, result StepResult) {
	switch result.Status {
	case StatusSuccess:
		o.write("  ✓ done (%s)\n", result.Duration.Round(100*1e6))
	case StatusFailed:
		o.write("  ✗ failed (%s): %v\n", result.Duration.Round(100*1e6), result.Error)
	case StatusSkipped:
		o.write("  - skipped\n")
	case StatusRetrying:
		o.write("  ~ retrying...\n")
	}
}

func (o *CLIObserver) OnRunComplete(result RunResult) {
	o.write("\n")
	if result.Success {
		o.write("✓ Runbook %q completed successfully (%s)\n", result.RunbookName, result.Duration.Round(100*1e6))
	} else {
		failed := 0
		for _, s := range result.Steps {
			if s.Status == StatusFailed {
				failed++
			}
		}
		o.write("✗ Runbook %q failed (%d step(s) failed, %s)\n", result.RunbookName, failed, result.Duration.Round(100*1e6))
	}
}

func (o *CLIObserver) OnPrompt(_ int, message string) (bool, error) {
	if o.AutoConfirm {
		o.write("  ? %s [auto-confirmed]\n", message)
		return true, nil
	}

	o.write("  ? %s [y/N] ", message)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes", nil
}
