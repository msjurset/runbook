package executor

import (
	"testing"

	"github.com/msjurset/runbook/internal/runbook"
)

func TestNewExecutor(t *testing.T) {
	tests := []struct {
		name    string
		step    runbook.Step
		wantErr bool
	}{
		{
			name: "shell executor",
			step: runbook.Step{
				Name: "s", Type: runbook.StepShell,
				Shell: &runbook.ShellStep{Command: "echo"},
			},
			wantErr: false,
		},
		{
			name: "shell nil config",
			step: runbook.Step{
				Name: "s", Type: runbook.StepShell,
			},
			wantErr: true,
		},
		{
			name: "ssh executor",
			step: runbook.Step{
				Name: "s", Type: runbook.StepSSH,
				SSH:  &runbook.SSHStep{Host: "h", Command: "c"},
			},
			wantErr: false,
		},
		{
			name: "ssh nil config",
			step: runbook.Step{
				Name: "s", Type: runbook.StepSSH,
			},
			wantErr: true,
		},
		{
			name: "http executor",
			step: runbook.Step{
				Name: "s", Type: runbook.StepHTTP,
				HTTP: &runbook.HTTPStep{URL: "http://x"},
			},
			wantErr: false,
		},
		{
			name: "http nil config",
			step: runbook.Step{
				Name: "s", Type: runbook.StepHTTP,
			},
			wantErr: true,
		},
		{
			name:    "unknown type",
			step:    runbook.Step{Name: "s", Type: runbook.StepType("ftp")},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec, err := New(tt.step)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if exec == nil {
				t.Error("expected non-nil executor")
			}
		})
	}
}

func TestExecResultOutput(t *testing.T) {
	tests := []struct {
		name   string
		result ExecResult
		want   string
	}{
		{
			name:   "stdout only",
			result: ExecResult{Stdout: "hello"},
			want:   "hello",
		},
		{
			name:   "body takes precedence",
			result: ExecResult{Stdout: "hello", Body: "response"},
			want:   "response",
		},
		{
			name:   "empty",
			result: ExecResult{},
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.Output(); got != tt.want {
				t.Errorf("Output() = %q, want %q", got, tt.want)
			}
		})
	}
}
