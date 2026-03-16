package executor

import (
	"testing"

	"github.com/msjurset/runbook/internal/runbook"
)

func TestSSHExecutorAuthMethodsNoAuth(t *testing.T) {
	// Test that auth methods returns error when no auth is available
	e := &SSHExecutor{
		Step: &runbook.SSHStep{
			Host:    "localhost",
			Command: "echo",
		},
	}

	_, err := e.authMethods("/nonexistent/key", "")
	if err == nil {
		t.Error("expected error for nonexistent key file, got nil")
	}
}

func TestSSHExecutorAuthMethodsAgentNoSocket(t *testing.T) {
	// With agent_auth but no SSH_AUTH_SOCK, should fall through to default keys
	e := &SSHExecutor{
		Step: &runbook.SSHStep{
			Host:      "localhost",
			Command:   "echo",
			AgentAuth: true,
		},
	}

	// This may or may not fail depending on whether default keys exist,
	// but it should not panic
	_, _ = e.authMethods("", "")
}
