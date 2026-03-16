package executor

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/msjurset/runbook/internal/runbook"
)

// ExecResult holds the output of a step execution.
type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int

	// HTTP-specific fields
	StatusCode int
	Headers    http.Header
	Body       string
}

// Output returns the primary output: Body for HTTP steps, Stdout for others.
func (r *ExecResult) Output() string {
	if r.Body != "" {
		return r.Body
	}
	return r.Stdout
}

// StepExecutor executes a single step.
type StepExecutor interface {
	Execute(ctx context.Context, vars map[string]string, stdout, stderr io.Writer) (*ExecResult, error)
}

// New creates the appropriate executor for a step.
func New(step runbook.Step) (StepExecutor, error) {
	switch step.Type {
	case runbook.StepShell:
		if step.Shell == nil {
			return nil, fmt.Errorf("shell step %q has no shell config", step.Name)
		}
		return &ShellExecutor{Step: step.Shell}, nil
	case runbook.StepSSH:
		if step.SSH == nil {
			return nil, fmt.Errorf("ssh step %q has no ssh config", step.Name)
		}
		return &SSHExecutor{Step: step.SSH}, nil
	case runbook.StepHTTP:
		if step.HTTP == nil {
			return nil, fmt.Errorf("http step %q has no http config", step.Name)
		}
		return &HTTPExecutor{Step: step.HTTP}, nil
	default:
		return nil, fmt.Errorf("unknown step type %q", step.Type)
	}
}
