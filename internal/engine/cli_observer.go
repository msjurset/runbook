package engine

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/msjurset/runbook/internal/runbook"
)

// heartbeatSilence is how long a step can go without any output before the
// observer prints a "still running" liveness line. Anchors at the last write
// (start banner, output line, prior heartbeat) so a chatty step never prints
// one, but a silent multi-minute step does. Picked at 60s because that's
// longer than typical shell output bursts and short enough that a user
// staring at the screen doesn't wonder if the runner is wedged.
const heartbeatSilence = 60 * time.Second

// heartbeatTick is the goroutine's wakeup cadence. It only emits if the
// silence threshold has been crossed, so a tighter tick just trims jitter
// in when the heartbeat fires.
const heartbeatTick = 15 * time.Second

// CLIObserver implements Observer with plain text output to stdout.
type CLIObserver struct {
	AutoConfirm bool

	mu       sync.Mutex
	logBuf   bytes.Buffer
	lastOut  time.Time
	hbCancel context.CancelFunc
}

// LogOutput returns all captured output for logging.
func (o *CLIObserver) LogOutput() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.logBuf.String()
}

func (o *CLIObserver) write(format string, args ...any) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.writeLocked(format, args...)
}

// writeLocked is the inner write that the caller must hold o.mu for. Every
// write resets lastOut so the heartbeat clock restarts from the most recent
// visible activity — emitted lines from the step, start banners, even prior
// heartbeats themselves all count as liveness.
func (o *CLIObserver) writeLocked(format string, args ...any) {
	s := fmt.Sprintf(format, args...)
	fmt.Print(s)
	o.logBuf.WriteString(s)
	o.lastOut = time.Now()
}

func (o *CLIObserver) writeln(w io.Writer, format string, args ...any) {
	o.mu.Lock()
	defer o.mu.Unlock()
	s := fmt.Sprintf(format, args...)
	fmt.Fprint(w, s)
	o.logBuf.WriteString(s)
	o.lastOut = time.Now()
}

func (o *CLIObserver) OnStepStart(index int, step runbook.Step) {
	o.write("\n▸ Step %d: %s\n", index+1, step.Name)
	if step.Type != "" {
		o.write("  type: %s\n", step.Type)
	}
	o.startHeartbeat(time.Now())
}

func (o *CLIObserver) OnStepOutput(_ int, line string) {
	o.write("  │ %s\n", line)
}

func (o *CLIObserver) OnStepComplete(index int, result StepResult) {
	o.stopHeartbeat()
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

// startHeartbeat begins a background goroutine that prints a liveness line
// once a step has been silent for heartbeatSilence. start is the step's
// start time, used for the elapsed-time column in the heartbeat line.
func (o *CLIObserver) startHeartbeat(start time.Time) {
	ctx, cancel := context.WithCancel(context.Background())
	o.mu.Lock()
	o.hbCancel = cancel
	o.mu.Unlock()

	go func() {
		ticker := time.NewTicker(heartbeatTick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				o.mu.Lock()
				silent := now.Sub(o.lastOut)
				if silent >= heartbeatSilence {
					elapsed := now.Sub(start).Round(time.Second)
					o.writeLocked("  · still running (%s elapsed)\n", elapsed)
				}
				o.mu.Unlock()
			}
		}
	}()
}

func (o *CLIObserver) stopHeartbeat() {
	o.mu.Lock()
	cancel := o.hbCancel
	o.hbCancel = nil
	o.mu.Unlock()
	if cancel != nil {
		cancel()
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
