package engine

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/msjurset/runbook/internal/executor"
	"github.com/msjurset/runbook/internal/runbook"
)

// ansiRe matches ANSI escape sequences (CSI sequences, OSC, etc.)
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x1b]*\x1b\\|\x1b\[[\?]?[0-9;]*[hlm]`)

// Observer receives events from the engine during execution.
type Observer interface {
	OnStepStart(stepIndex int, step runbook.Step)
	OnStepOutput(stepIndex int, line string)
	OnStepComplete(stepIndex int, result StepResult)
	OnRunComplete(result RunResult)
	OnPrompt(stepIndex int, message string) (bool, error)
}

// Engine orchestrates the execution of a runbook.
type Engine struct {
	book     *runbook.Runbook
	vars     map[string]string
	mu       sync.Mutex // protects vars during parallel execution
	observer Observer
	results  []StepResult
	skipped  map[int]bool
}

// New creates a new engine for the given runbook.
func New(book *runbook.Runbook, vars map[string]string, obs Observer) *Engine {
	return &Engine{
		book:     book,
		vars:     vars,
		observer: obs,
		results:  make([]StepResult, len(book.Steps)),
		skipped:  make(map[int]bool),
	}
}

// Run executes steps, running parallel groups concurrently.
func (e *Engine) Run(ctx context.Context) RunResult {
	start := time.Now()
	allSuccess := true

	i := 0
	for i < len(e.book.Steps) {
		if ctx.Err() != nil {
			for j := i; j < len(e.book.Steps); j++ {
				e.results[j] = StepResult{
					StepIndex: j,
					StepName:  e.book.Steps[j].Name,
					Status:    StatusSkipped,
				}
			}
			break
		}

		// Collect a parallel group: consecutive steps with parallel: true
		group := e.collectParallelGroup(i)

		if len(group) == 1 {
			// Single step — run sequentially
			idx := group[0]
			step := e.book.Steps[idx]

			if e.skipped[idx] {
				e.results[idx] = StepResult{
					StepIndex: idx,
					StepName:  step.Name,
					Status:    StatusSkipped,
				}
				e.observer.OnStepComplete(idx, e.results[idx])
				i++
				continue
			}

			result := e.runStep(ctx, idx, step)
			e.results[idx] = result

			if result.Status == StatusFailed {
				allSuccess = false
				if step.OnError == runbook.PolicyAbort || step.OnError == "" {
					for j := i + 1; j < len(e.book.Steps); j++ {
						e.results[j] = StepResult{
							StepIndex: j,
							StepName:  e.book.Steps[j].Name,
							Status:    StatusSkipped,
						}
					}
					i = len(e.book.Steps)
					continue
				}
			}
			i++
		} else {
			// Parallel group — run concurrently
			failed := e.runParallelGroup(ctx, group)
			if failed {
				allSuccess = false
				// Check if any step in the group has abort policy
				shouldAbort := false
				for _, idx := range group {
					step := e.book.Steps[idx]
					if e.results[idx].Status == StatusFailed &&
						(step.OnError == runbook.PolicyAbort || step.OnError == "") {
						shouldAbort = true
						break
					}
				}
				if shouldAbort {
					lastIdx := group[len(group)-1]
					for j := lastIdx + 1; j < len(e.book.Steps); j++ {
						e.results[j] = StepResult{
							StepIndex: j,
							StepName:  e.book.Steps[j].Name,
							Status:    StatusSkipped,
						}
					}
					i = len(e.book.Steps)
					continue
				}
			}
			i += len(group)
		}
	}

	rr := RunResult{
		RunbookName: e.book.Name,
		Steps:       e.results,
		StartedAt:   start,
		Duration:    time.Since(start),
		Success:     allSuccess,
	}
	e.observer.OnRunComplete(rr)
	return rr
}

// SkipStep marks a step to be skipped.
func (e *Engine) SkipStep(index int) {
	e.skipped[index] = true
}

// RunFrom re-executes the runbook starting from the given step index.
// Steps before fromIndex retain their previous results.
func (e *Engine) RunFrom(ctx context.Context, fromIndex int) RunResult {
	start := time.Now()
	allSuccess := true

	// Check previous results for steps we're not re-running
	for j := 0; j < fromIndex && j < len(e.results); j++ {
		if e.results[j].Status == StatusFailed {
			allSuccess = false
		}
	}

	// Reset results and skipped state for steps we're re-running
	for j := fromIndex; j < len(e.book.Steps); j++ {
		e.results[j] = StepResult{}
		delete(e.skipped, j)
	}

	i := fromIndex
	for i < len(e.book.Steps) {
		if ctx.Err() != nil {
			for j := i; j < len(e.book.Steps); j++ {
				e.results[j] = StepResult{
					StepIndex: j,
					StepName:  e.book.Steps[j].Name,
					Status:    StatusSkipped,
				}
			}
			break
		}

		group := e.collectParallelGroup(i)

		if len(group) == 1 {
			idx := group[0]
			step := e.book.Steps[idx]

			if e.skipped[idx] {
				e.results[idx] = StepResult{
					StepIndex: idx,
					StepName:  step.Name,
					Status:    StatusSkipped,
				}
				e.observer.OnStepComplete(idx, e.results[idx])
				i++
				continue
			}

			result := e.runStep(ctx, idx, step)
			e.results[idx] = result

			if result.Status == StatusFailed {
				allSuccess = false
				if step.OnError == runbook.PolicyAbort || step.OnError == "" {
					for j := i + 1; j < len(e.book.Steps); j++ {
						e.results[j] = StepResult{
							StepIndex: j,
							StepName:  e.book.Steps[j].Name,
							Status:    StatusSkipped,
						}
					}
					i = len(e.book.Steps)
					continue
				}
			}
			i++
		} else {
			failed := e.runParallelGroup(ctx, group)
			if failed {
				allSuccess = false
				shouldAbort := false
				for _, idx := range group {
					step := e.book.Steps[idx]
					if e.results[idx].Status == StatusFailed &&
						(step.OnError == runbook.PolicyAbort || step.OnError == "") {
						shouldAbort = true
						break
					}
				}
				if shouldAbort {
					lastIdx := group[len(group)-1]
					for j := lastIdx + 1; j < len(e.book.Steps); j++ {
						e.results[j] = StepResult{
							StepIndex: j,
							StepName:  e.book.Steps[j].Name,
							Status:    StatusSkipped,
						}
					}
					i = len(e.book.Steps)
					continue
				}
			}
			i += len(group)
		}
	}

	rr := RunResult{
		RunbookName: e.book.Name,
		Steps:       e.results,
		StartedAt:   start,
		Duration:    time.Since(start),
		Success:     allSuccess,
	}
	e.observer.OnRunComplete(rr)
	return rr
}

// collectParallelGroup returns indices of consecutive steps starting at i
// that should run in parallel. A single non-parallel step returns a group of 1.
func (e *Engine) collectParallelGroup(start int) []int {
	if !e.book.Steps[start].Parallel {
		return []int{start}
	}
	group := []int{start}
	for j := start + 1; j < len(e.book.Steps) && e.book.Steps[j].Parallel; j++ {
		group = append(group, j)
	}
	return group
}

// runParallelGroup executes a group of steps concurrently. Returns true if any failed.
func (e *Engine) runParallelGroup(ctx context.Context, indices []int) bool {
	var wg sync.WaitGroup
	results := make([]StepResult, len(indices))

	for gi, idx := range indices {
		if e.skipped[idx] {
			results[gi] = StepResult{
				StepIndex: idx,
				StepName:  e.book.Steps[idx].Name,
				Status:    StatusSkipped,
			}
			e.observer.OnStepComplete(idx, results[gi])
			continue
		}

		wg.Add(1)
		go func(gi, idx int) {
			defer wg.Done()
			step := e.book.Steps[idx]
			results[gi] = e.runStep(ctx, idx, step)
		}(gi, idx)
	}

	wg.Wait()

	anyFailed := false
	for gi, idx := range indices {
		e.results[idx] = results[gi]
		if results[gi].Status == StatusFailed {
			anyFailed = true
		}
	}
	return anyFailed
}

func (e *Engine) runStep(ctx context.Context, index int, step runbook.Step) StepResult {
	start := time.Now()

	// Evaluate condition
	if step.Condition != "" {
		val, err := runbook.Expand(step.Condition, e.vars)
		if err != nil || strings.TrimSpace(val) != "true" {
			result := StepResult{
				StepIndex: index,
				StepName:  step.Name,
				Status:    StatusSkipped,
				StartedAt: start,
				Duration:  time.Since(start),
			}
			e.observer.OnStepComplete(index, result)
			return result
		}
	}

	// Handle confirmation prompt
	if step.Confirm != "" {
		msg, _ := runbook.Expand(step.Confirm, e.vars)
		ok, err := e.observer.OnPrompt(index, msg)
		if err != nil || !ok {
			result := StepResult{
				StepIndex: index,
				StepName:  step.Name,
				Status:    StatusSkipped,
				StartedAt: start,
				Duration:  time.Since(start),
			}
			e.observer.OnStepComplete(index, result)
			return result
		}
	}

	// Confirm-only steps (no type) are done after confirmation
	if step.Type == "" {
		result := StepResult{
			StepIndex: index,
			StepName:  step.Name,
			Status:    StatusSuccess,
			StartedAt: start,
			Duration:  time.Since(start),
		}
		e.observer.OnStepComplete(index, result)
		return result
	}

	maxAttempts := 1
	if step.OnError == runbook.PolicyRetry && step.Retries > 0 {
		maxAttempts = step.Retries + 1
	}

	var lastResult StepResult
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			e.observer.OnStepOutput(index, fmt.Sprintf("--- retry %d/%d ---", attempt, step.Retries))
		}

		lastResult = e.executeStep(ctx, index, step, start)
		if lastResult.Status == StatusSuccess {
			return lastResult
		}

		if attempt < maxAttempts-1 {
			lastResult.Status = StatusRetrying
			e.observer.OnStepComplete(index, lastResult)
		}
	}

	return lastResult
}

func (e *Engine) executeStep(ctx context.Context, index int, step runbook.Step, start time.Time) StepResult {
	e.observer.OnStepStart(index, step)

	// Apply timeout
	if step.Timeout.Duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, step.Timeout.Duration)
		defer cancel()
	}

	exec, err := executor.New(step)
	if err != nil {
		e.observer.OnStepOutput(index, fmt.Sprintf("error: %v", err))
		result := StepResult{
			StepIndex: index,
			StepName:  step.Name,
			Status:    StatusFailed,
			Error:     err,
			StartedAt: start,
			Duration:  time.Since(start),
		}
		e.observer.OnStepComplete(index, result)
		return result
	}

	// Create streaming writers
	stdoutW := &observerWriter{index: index, observer: e.observer}
	stderrW := &observerWriter{index: index, observer: e.observer}

	execResult, err := exec.Execute(ctx, e.vars, stdoutW, stderrW)
	stdoutW.Flush()
	stderrW.Flush()

	result := StepResult{
		StepIndex: index,
		StepName:  step.Name,
		StartedAt: start,
		Duration:  time.Since(start),
	}

	if err != nil {
		result.Status = StatusFailed
		result.Error = err
		if execResult != nil {
			result.Output = execResult.Output()
			if execResult.Stderr != "" {
				e.observer.OnStepOutput(index, execResult.Stderr)
			}
		}
		e.observer.OnStepOutput(index, fmt.Sprintf("error: %v", err))
	} else {
		result.Status = StatusSuccess
		result.Output = execResult.Output()

		// Capture output to variable
		if step.Capture != "" {
			output := strings.TrimSpace(execResult.Output())
			e.mu.Lock()
			e.vars[step.Capture] = output
			e.mu.Unlock()
		}
	}

	e.observer.OnStepComplete(index, result)
	return result
}

// observerWriter sends written data to the observer line by line.
// It sanitizes output by stripping ANSI escape sequences and handling
// carriage returns so that raw terminal output doesn't break the TUI.
type observerWriter struct {
	index    int
	observer Observer
	buf      []byte
}

func (w *observerWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		// Split on \n or \r\n
		idx := indexOfAny(w.buf, '\n', '\r')
		if idx < 0 {
			break
		}
		line := string(w.buf[:idx])
		// Skip past \r\n or \r or \n
		advance := 1
		if idx < len(w.buf)-1 && w.buf[idx] == '\r' && w.buf[idx+1] == '\n' {
			advance = 2
		}
		w.buf = w.buf[idx+advance:]

		// Strip ANSI escape sequences
		line = sanitizeLine(line)
		line = strings.TrimRight(line, " ")
		if line == "" {
			continue
		}
		w.observer.OnStepOutput(w.index, line)
	}
	return len(p), nil
}

// Flush sends any remaining buffered data.
func (w *observerWriter) Flush() {
	if len(w.buf) > 0 {
		line := sanitizeLine(string(w.buf))
		line = strings.TrimRight(line, " ")
		if line != "" {
			w.observer.OnStepOutput(w.index, line)
		}
		w.buf = nil
	}
}

var _ io.Writer = (*observerWriter)(nil)

// sanitizeLine strips ANSI escape sequences and control characters from a line.
func sanitizeLine(s string) string {
	// Strip ANSI escape sequences
	s = ansiRe.ReplaceAllString(s, "")
	// Strip remaining control characters except tab
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\t' || r >= 32 {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func indexOfAny(b []byte, chars ...byte) int {
	for i, v := range b {
		for _, c := range chars {
			if v == c {
				return i
			}
		}
	}
	return -1
}
