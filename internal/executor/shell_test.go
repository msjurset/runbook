package executor

import (
	"bytes"
	"context"
	"testing"

	"github.com/msjurset/runbook/internal/runbook"
)

func TestShellExecutor(t *testing.T) {
	tests := []struct {
		name       string
		step       *runbook.ShellStep
		vars       map[string]string
		wantOutput string
		wantErr    bool
	}{
		{
			name:       "simple echo",
			step:       &runbook.ShellStep{Command: "echo hello"},
			wantOutput: "hello\n",
		},
		{
			name:       "template expansion",
			step:       &runbook.ShellStep{Command: "echo {{.greeting}}"},
			vars:       map[string]string{"greeting": "howdy"},
			wantOutput: "howdy\n",
		},
		{
			name:    "failing command",
			step:    &runbook.ShellStep{Command: "false"},
			wantErr: true,
		},
		{
			name:       "working directory",
			step:       &runbook.ShellStep{Command: "pwd -P", Dir: "/tmp"},
			wantOutput: "/private/tmp\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &ShellExecutor{Step: tt.step}
			vars := tt.vars
			if vars == nil {
				vars = make(map[string]string)
			}

			var stdout, stderr bytes.Buffer
			result, err := e.Execute(context.Background(), vars, &stdout, &stderr)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Stdout != tt.wantOutput {
				t.Errorf("stdout = %q, want %q", result.Stdout, tt.wantOutput)
			}
		})
	}
}

func TestShellExecutorCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	e := &ShellExecutor{Step: &runbook.ShellStep{Command: "sleep 10"}}
	_, err := e.Execute(ctx, map[string]string{}, nil, nil)
	if err == nil {
		t.Error("expected error from cancelled context, got nil")
	}
}
