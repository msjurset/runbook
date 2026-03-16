package engine

import "testing"

func TestStepStatusString(t *testing.T) {
	tests := []struct {
		status StepStatus
		want   string
	}{
		{StatusPending, "pending"},
		{StatusRunning, "running"},
		{StatusSuccess, "success"},
		{StatusFailed, "failed"},
		{StatusSkipped, "skipped"},
		{StatusRetrying, "retrying"},
		{StepStatus(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Errorf("StepStatus(%d).String() = %q, want %q", tt.status, got, tt.want)
		}
	}
}
