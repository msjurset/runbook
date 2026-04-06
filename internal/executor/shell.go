package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"runtime"

	"github.com/msjurset/runbook/internal/runbook"
)

// ShellExecutor runs a command in the local shell.
type ShellExecutor struct {
	Step *runbook.ShellStep
}

func (e *ShellExecutor) Execute(ctx context.Context, vars map[string]string, stdout, stderr io.Writer) (*ExecResult, error) {
	command, err := runbook.Expand(e.Step.Command, vars)
	if err != nil {
		return nil, fmt.Errorf("expanding command: %w", err)
	}

	shell, flag := shellCmd()
	cmd := exec.CommandContext(ctx, shell, flag, command)

	if e.Step.Dir != "" {
		dir, err := runbook.Expand(e.Step.Dir, vars)
		if err != nil {
			return nil, fmt.Errorf("expanding dir: %w", err)
		}
		cmd.Dir = dir
	}

	// Capture output while also streaming to the provided writers
	var stdoutBuf, stderrBuf bytes.Buffer
	if stdout != nil {
		cmd.Stdout = io.MultiWriter(&stdoutBuf, stdout)
	} else {
		cmd.Stdout = &stdoutBuf
	}
	if stderr != nil {
		cmd.Stderr = io.MultiWriter(&stderrBuf, stderr)
	} else {
		cmd.Stderr = &stderrBuf
	}

	// Pass variables as environment
	cmd.Env = envWithVars(vars)

	err = cmd.Run()
	result := &ExecResult{
		Stdout: stdoutBuf.String(),
		Stderr: stderrBuf.String(),
	}

	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	if err != nil {
		return result, fmt.Errorf("command failed (exit %d): %w", result.ExitCode, err)
	}
	return result, nil
}

func shellCmd() (string, string) {
	if runtime.GOOS == "windows" {
		return "cmd", "/C"
	}
	return "sh", "-c"
}

func envWithVars(vars map[string]string) []string {
	// Build set of keys we'll override
	overrides := make(map[string]string, len(vars))
	for k, v := range vars {
		overrides["RUNBOOK_"+k] = v
	}

	// Copy parent env, skipping keys we'll override
	var env []string
	for _, e := range os.Environ() {
		key, _, _ := strings.Cut(e, "=")
		if _, ok := overrides[key]; !ok {
			env = append(env, e)
		}
	}

	// Append our overrides
	for k, v := range overrides {
		env = append(env, k+"="+v)
	}
	return env
}
