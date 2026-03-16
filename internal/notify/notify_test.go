package notify

import (
	"testing"
	"time"

	"github.com/msjurset/runbook/internal/engine"
	"github.com/msjurset/runbook/internal/runbook"
)

func TestShouldNotify(t *testing.T) {
	tests := []struct {
		on      string
		success bool
		want    bool
	}{
		{"always", true, true},
		{"always", false, true},
		{"", true, true},
		{"", false, true},
		{"failure", false, true},
		{"failure", true, false},
		{"success", true, true},
		{"success", false, false},
	}

	for _, tt := range tests {
		if got := shouldNotify(tt.on, tt.success); got != tt.want {
			t.Errorf("shouldNotify(%q, %v) = %v, want %v", tt.on, tt.success, got, tt.want)
		}
	}
}

func TestFormatMessage(t *testing.T) {
	result := engine.RunResult{
		RunbookName: "deploy",
		Success:     true,
		Duration:    5 * time.Second,
		Steps: []engine.StepResult{
			{StepName: "build", Status: engine.StatusSuccess, Duration: 2 * time.Second},
			{StepName: "deploy", Status: engine.StatusSuccess, Duration: 3 * time.Second},
		},
	}

	subject, body := formatMessage("deploy", result)
	if subject == "" {
		t.Error("subject is empty")
	}
	if body == "" {
		t.Error("body is empty")
	}
	if got := subject; got != `✓ Runbook "deploy" succeeded` {
		t.Errorf("subject = %q", got)
	}
}

func TestFormatMessageFailure(t *testing.T) {
	result := engine.RunResult{
		RunbookName: "deploy",
		Success:     false,
		Duration:    1 * time.Second,
		Steps: []engine.StepResult{
			{StepName: "build", Status: engine.StatusFailed, Duration: 1 * time.Second},
		},
	}

	subject, _ := formatMessage("deploy", result)
	if got := subject; got != `✗ Runbook "deploy" failed` {
		t.Errorf("subject = %q", got)
	}
}

func TestSendNoConfig(t *testing.T) {
	book := &runbook.Runbook{Name: "test"}
	result := engine.RunResult{Success: true}

	errs := Send(book, result)
	if len(errs) != 0 {
		t.Errorf("Send() with no notify config returned errors: %v", errs)
	}
}

func TestSendSkippedByPolicy(t *testing.T) {
	book := &runbook.Runbook{
		Name: "test",
		Notify: &runbook.NotifyConfig{
			On:    "failure",
			Desktop: true,
		},
	}
	result := engine.RunResult{Success: true}

	// Should not notify on success when policy is "failure"
	errs := Send(book, result)
	if len(errs) != 0 {
		t.Errorf("Send() should have been skipped: %v", errs)
	}
}
