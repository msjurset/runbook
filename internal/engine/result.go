package engine

import "time"

// StepStatus represents the current state of a step.
type StepStatus int

const (
	StatusPending StepStatus = iota
	StatusRunning
	StatusSuccess
	StatusFailed
	StatusSkipped
	StatusRetrying
)

func (s StepStatus) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusRunning:
		return "running"
	case StatusSuccess:
		return "success"
	case StatusFailed:
		return "failed"
	case StatusSkipped:
		return "skipped"
	case StatusRetrying:
		return "retrying"
	default:
		return "unknown"
	}
}

// StepResult holds the outcome of a single step execution.
type StepResult struct {
	StepIndex int
	StepName  string
	Status    StepStatus
	Output    string
	Error     error
	Duration  time.Duration
	StartedAt time.Time
}

// RunResult holds the outcome of a complete runbook execution.
type RunResult struct {
	RunbookName string
	Steps       []StepResult
	StartedAt   time.Time
	Duration    time.Duration
	Success     bool
}
