package engine

import (
	"context"
	"sync"
	"testing"

	"github.com/msjurset/runbook/internal/runbook"
)

type testObserver struct {
	mu        sync.Mutex
	started   []int
	completed []StepResult
	outputs   map[int][]string
	run       *RunResult
	prompts   map[int]bool // responses to prompts
}

func newTestObserver() *testObserver {
	return &testObserver{
		outputs: make(map[int][]string),
		prompts: make(map[int]bool),
	}
}

func (o *testObserver) OnStepStart(idx int, _ runbook.Step) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.started = append(o.started, idx)
}

func (o *testObserver) OnStepOutput(idx int, line string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.outputs[idx] = append(o.outputs[idx], line)
}

func (o *testObserver) OnStepComplete(_ int, result StepResult) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.completed = append(o.completed, result)
}

func (o *testObserver) OnRunComplete(result RunResult) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.run = &result
}

func (o *testObserver) OnPrompt(idx int, _ string) (bool, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if v, ok := o.prompts[idx]; ok {
		return v, nil
	}
	return true, nil
}

func TestEngineRunSimple(t *testing.T) {
	book := &runbook.Runbook{
		Name: "test",
		Steps: []runbook.Step{
			{
				Name:  "echo",
				Type:  runbook.StepShell,
				Shell: &runbook.ShellStep{Command: "echo hello"},
			},
			{
				Name:  "echo2",
				Type:  runbook.StepShell,
				Shell: &runbook.ShellStep{Command: "echo world"},
			},
		},
	}

	obs := newTestObserver()
	eng := New(book, map[string]string{}, obs)
	result := eng.Run(context.Background())

	if !result.Success {
		t.Error("expected success")
	}
	if len(obs.started) != 2 {
		t.Errorf("started %d steps, want 2", len(obs.started))
	}
	if obs.run == nil {
		t.Fatal("OnRunComplete not called")
	}
}

func TestEngineAbortOnError(t *testing.T) {
	book := &runbook.Runbook{
		Name: "test",
		Steps: []runbook.Step{
			{
				Name:    "fail",
				Type:    runbook.StepShell,
				Shell:   &runbook.ShellStep{Command: "false"},
				OnError: runbook.PolicyAbort,
			},
			{
				Name:  "should-not-run",
				Type:  runbook.StepShell,
				Shell: &runbook.ShellStep{Command: "echo unreachable"},
			},
		},
	}

	obs := newTestObserver()
	eng := New(book, map[string]string{}, obs)
	result := eng.Run(context.Background())

	if result.Success {
		t.Error("expected failure")
	}
	if len(obs.started) != 1 {
		t.Errorf("started %d steps, want 1 (second should be skipped)", len(obs.started))
	}
}

func TestEngineContinueOnError(t *testing.T) {
	book := &runbook.Runbook{
		Name: "test",
		Steps: []runbook.Step{
			{
				Name:    "fail",
				Type:    runbook.StepShell,
				Shell:   &runbook.ShellStep{Command: "false"},
				OnError: runbook.PolicyContinue,
			},
			{
				Name:  "should-run",
				Type:  runbook.StepShell,
				Shell: &runbook.ShellStep{Command: "echo continued"},
			},
		},
	}

	obs := newTestObserver()
	eng := New(book, map[string]string{}, obs)
	result := eng.Run(context.Background())

	if result.Success {
		t.Error("expected failure (first step failed)")
	}
	if len(obs.started) != 2 {
		t.Errorf("started %d steps, want 2 (should continue after failure)", len(obs.started))
	}
}

func TestEngineVariableCapture(t *testing.T) {
	book := &runbook.Runbook{
		Name: "test",
		Steps: []runbook.Step{
			{
				Name:    "produce",
				Type:    runbook.StepShell,
				Shell:   &runbook.ShellStep{Command: "echo captured-value"},
				Capture: "result",
			},
			{
				Name:  "consume",
				Type:  runbook.StepShell,
				Shell: &runbook.ShellStep{Command: "echo got={{.result}}"},
			},
		},
	}

	obs := newTestObserver()
	eng := New(book, map[string]string{}, obs)
	result := eng.Run(context.Background())

	if !result.Success {
		t.Fatalf("expected success, got errors in steps")
	}

	// Check that the second step received the captured variable
	found := false
	for _, line := range obs.outputs[1] {
		if line == "got=captured-value" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("second step did not receive captured variable, outputs: %v", obs.outputs[1])
	}
}

func TestEngineCondition(t *testing.T) {
	book := &runbook.Runbook{
		Name: "test",
		Steps: []runbook.Step{
			{
				Name:      "skipped",
				Type:      runbook.StepShell,
				Shell:     &runbook.ShellStep{Command: "echo should-not-run"},
				Condition: "false",
			},
			{
				Name:      "runs",
				Type:      runbook.StepShell,
				Shell:     &runbook.ShellStep{Command: "echo ran"},
				Condition: "true",
			},
		},
	}

	obs := newTestObserver()
	eng := New(book, map[string]string{}, obs)
	result := eng.Run(context.Background())

	if !result.Success {
		t.Error("expected success")
	}
	// Only step 1 (index 1) should have been started as an execution
	if len(obs.started) != 1 {
		t.Errorf("started %d steps, want 1", len(obs.started))
	}
}

func TestEngineConfirmDenied(t *testing.T) {
	book := &runbook.Runbook{
		Name: "test",
		Steps: []runbook.Step{
			{
				Name:    "confirm-step",
				Confirm: "Proceed?",
			},
		},
	}

	obs := newTestObserver()
	obs.prompts[0] = false // deny the prompt
	eng := New(book, map[string]string{}, obs)
	result := eng.Run(context.Background())

	if !result.Success {
		t.Error("expected success (denied confirm should skip, not fail)")
	}
}

func TestEngineSkipStep(t *testing.T) {
	book := &runbook.Runbook{
		Name: "test",
		Steps: []runbook.Step{
			{
				Name:  "step1",
				Type:  runbook.StepShell,
				Shell: &runbook.ShellStep{Command: "echo first"},
			},
			{
				Name:  "step2",
				Type:  runbook.StepShell,
				Shell: &runbook.ShellStep{Command: "echo second"},
			},
		},
	}

	obs := newTestObserver()
	eng := New(book, map[string]string{}, obs)
	eng.SkipStep(0)
	result := eng.Run(context.Background())

	if !result.Success {
		t.Error("expected success")
	}
	if len(obs.started) != 1 {
		t.Errorf("started %d steps, want 1 (first should be skipped)", len(obs.started))
	}
}

func TestEngineParallelSteps(t *testing.T) {
	book := &runbook.Runbook{
		Name: "test",
		Steps: []runbook.Step{
			{
				Name:     "parallel-1",
				Type:     runbook.StepShell,
				Shell:    &runbook.ShellStep{Command: "echo p1"},
				Parallel: true,
			},
			{
				Name:     "parallel-2",
				Type:     runbook.StepShell,
				Shell:    &runbook.ShellStep{Command: "echo p2"},
				Parallel: true,
			},
			{
				Name:  "sequential",
				Type:  runbook.StepShell,
				Shell: &runbook.ShellStep{Command: "echo seq"},
			},
		},
	}

	obs := newTestObserver()
	eng := New(book, map[string]string{}, obs)
	result := eng.Run(context.Background())

	if !result.Success {
		t.Error("expected success")
	}
	obs.mu.Lock()
	defer obs.mu.Unlock()
	if len(obs.started) != 3 {
		t.Errorf("started %d steps, want 3", len(obs.started))
	}
}

func TestEngineParallelWithCapture(t *testing.T) {
	book := &runbook.Runbook{
		Name: "test",
		Steps: []runbook.Step{
			{
				Name:     "p1",
				Type:     runbook.StepShell,
				Shell:    &runbook.ShellStep{Command: "echo val1"},
				Parallel: true,
				Capture:  "out1",
			},
			{
				Name:     "p2",
				Type:     runbook.StepShell,
				Shell:    &runbook.ShellStep{Command: "echo val2"},
				Parallel: true,
				Capture:  "out2",
			},
			{
				Name:  "use-captured",
				Type:  runbook.StepShell,
				Shell: &runbook.ShellStep{Command: "echo {{.out1}}-{{.out2}}"},
			},
		},
	}

	obs := newTestObserver()
	eng := New(book, map[string]string{}, obs)
	result := eng.Run(context.Background())

	if !result.Success {
		t.Error("expected success")
	}

	obs.mu.Lock()
	defer obs.mu.Unlock()
	found := false
	for _, line := range obs.outputs[2] {
		if line == "val1-val2" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("sequential step did not receive parallel captured values, outputs: %v", obs.outputs[2])
	}
}

func TestEngineParallelAbortOnFailure(t *testing.T) {
	book := &runbook.Runbook{
		Name: "test",
		Steps: []runbook.Step{
			{
				Name:     "p-ok",
				Type:     runbook.StepShell,
				Shell:    &runbook.ShellStep{Command: "echo ok"},
				Parallel: true,
				OnError:  runbook.PolicyContinue,
			},
			{
				Name:     "p-fail",
				Type:     runbook.StepShell,
				Shell:    &runbook.ShellStep{Command: "false"},
				Parallel: true,
				OnError:  runbook.PolicyAbort,
			},
			{
				Name:  "after-parallel",
				Type:  runbook.StepShell,
				Shell: &runbook.ShellStep{Command: "echo should-be-skipped"},
			},
		},
	}

	obs := newTestObserver()
	eng := New(book, map[string]string{}, obs)
	result := eng.Run(context.Background())

	if result.Success {
		t.Error("expected failure")
	}
	// The step after the parallel group should be skipped
	if result.Steps[2].Status != StatusSkipped {
		t.Errorf("step 2 status = %s, want skipped", result.Steps[2].Status)
	}
}

func TestEngineCollectParallelGroup(t *testing.T) {
	book := &runbook.Runbook{
		Name: "test",
		Steps: []runbook.Step{
			{Name: "s0", Parallel: true},
			{Name: "s1", Parallel: true},
			{Name: "s2", Parallel: true},
			{Name: "s3"},
			{Name: "s4", Parallel: true},
		},
	}

	eng := New(book, map[string]string{}, newTestObserver())

	group0 := eng.collectParallelGroup(0)
	if len(group0) != 3 {
		t.Errorf("group at 0 = %v, want 3 items", group0)
	}

	group3 := eng.collectParallelGroup(3)
	if len(group3) != 1 {
		t.Errorf("group at 3 = %v, want 1 item", group3)
	}

	group4 := eng.collectParallelGroup(4)
	if len(group4) != 1 {
		t.Errorf("group at 4 = %v, want 1 item (last step, no following parallel)", group4)
	}
}

func TestEngineRunFrom(t *testing.T) {
	book := &runbook.Runbook{
		Name: "test",
		Steps: []runbook.Step{
			{
				Name:  "step1",
				Type:  runbook.StepShell,
				Shell: &runbook.ShellStep{Command: "echo first"},
			},
			{
				Name:    "step2-fail",
				Type:    runbook.StepShell,
				Shell:   &runbook.ShellStep{Command: "false"},
				OnError: runbook.PolicyAbort,
			},
			{
				Name:  "step3",
				Type:  runbook.StepShell,
				Shell: &runbook.ShellStep{Command: "echo third"},
			},
		},
	}

	obs := newTestObserver()
	eng := New(book, map[string]string{}, obs)

	// First run: step2 fails, step3 skipped
	result := eng.Run(context.Background())
	if result.Success {
		t.Fatal("expected failure on first run")
	}
	if result.Steps[2].Status != StatusSkipped {
		t.Errorf("step3 status = %s, want skipped", result.Steps[2].Status)
	}

	// Now fix the command and re-run from step 2
	book.Steps[1].Shell.Command = "echo fixed"
	obs2 := newTestObserver()
	eng.observer = obs2

	result2 := eng.RunFrom(context.Background(), 1)
	if !result2.Success {
		t.Error("expected success on RunFrom")
	}

	// Step 0 should retain its original result (not re-run)
	obs2.mu.Lock()
	defer obs2.mu.Unlock()
	for _, idx := range obs2.started {
		if idx == 0 {
			t.Error("step 0 should not have been re-run")
		}
	}

	// Steps 1 and 2 should have run
	if len(obs2.started) != 2 {
		t.Errorf("started %d steps, want 2 (steps 1 and 2)", len(obs2.started))
	}
}

func TestEngineRunFromPreservesCapture(t *testing.T) {
	book := &runbook.Runbook{
		Name: "test",
		Steps: []runbook.Step{
			{
				Name:    "produce",
				Type:    runbook.StepShell,
				Shell:   &runbook.ShellStep{Command: "echo myval"},
				Capture: "val",
			},
			{
				Name:    "fail",
				Type:    runbook.StepShell,
				Shell:   &runbook.ShellStep{Command: "false"},
				OnError: runbook.PolicyAbort,
			},
			{
				Name:  "use-val",
				Type:  runbook.StepShell,
				Shell: &runbook.ShellStep{Command: "echo got={{.val}}"},
			},
		},
	}

	obs := newTestObserver()
	eng := New(book, map[string]string{}, obs)
	eng.Run(context.Background())

	// Fix step 2 and re-run from it
	book.Steps[1].Shell.Command = "echo ok"
	obs2 := newTestObserver()
	eng.observer = obs2
	result := eng.RunFrom(context.Background(), 1)

	if !result.Success {
		t.Fatal("expected success")
	}

	// The captured variable from step 0 should still be available
	obs2.mu.Lock()
	defer obs2.mu.Unlock()
	found := false
	for _, line := range obs2.outputs[2] {
		if line == "got=myval" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("step 2 did not receive captured variable from step 0, outputs: %v", obs2.outputs[2])
	}
}
