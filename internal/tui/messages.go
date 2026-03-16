package tui

import (
	"github.com/msjurset/runbook/internal/engine"
	"github.com/msjurset/runbook/internal/runbook"
)

// stepStartedMsg signals that a step has begun executing.
type stepStartedMsg struct {
	index int
	step  runbook.Step
}

// stepOutputMsg delivers a line of output from a running step.
type stepOutputMsg struct {
	index int
	line  string
}

// stepCompletedMsg signals that a step has finished.
type stepCompletedMsg struct {
	index  int
	result engine.StepResult
}

// runCompletedMsg signals that the entire runbook has finished.
type runCompletedMsg struct {
	result engine.RunResult
}

// promptMsg asks the user to confirm before proceeding.
type promptMsg struct {
	index   int
	message string
	respCh  chan bool
}

// runStartMsg triggers the engine to begin execution.
type runStartMsg struct{}
