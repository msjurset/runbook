package runbook

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRunbookByPath(t *testing.T) {
	dir := t.TempDir()
	content := `name: found-by-path
steps:
  - name: s1
    type: shell
    shell:
      command: "echo ok"
`
	path := filepath.Join(dir, "test.yaml")
	os.WriteFile(path, []byte(content), 0o644)

	rb, err := FindRunbook(path)
	if err != nil {
		t.Fatalf("FindRunbook(%q) error: %v", path, err)
	}
	if rb.Name != "found-by-path" {
		t.Errorf("Name = %q, want %q", rb.Name, "found-by-path")
	}
}

func TestFindRunbookByPathWithoutExtension(t *testing.T) {
	dir := t.TempDir()
	content := `name: found-no-ext
steps:
  - name: s1
    type: shell
    shell:
      command: "echo ok"
`
	os.WriteFile(filepath.Join(dir, "mybook.yaml"), []byte(content), 0o644)

	rb, err := FindRunbook(filepath.Join(dir, "mybook"))
	if err != nil {
		t.Fatalf("FindRunbook() error: %v", err)
	}
	if rb.Name != "found-no-ext" {
		t.Errorf("Name = %q, want %q", rb.Name, "found-no-ext")
	}
}

func TestFindRunbookByNameInDir(t *testing.T) {
	dir := t.TempDir()
	content := `name: my-runbook
steps:
  - name: s1
    type: shell
    shell:
      command: "echo ok"
`
	os.WriteFile(filepath.Join(dir, "my-runbook.yaml"), []byte(content), 0o644)

	rb, err := FindRunbook("my-runbook", dir)
	if err != nil {
		t.Fatalf("FindRunbook() error: %v", err)
	}
	if rb.Name != "my-runbook" {
		t.Errorf("Name = %q, want %q", rb.Name, "my-runbook")
	}
}

func TestFindRunbookNotFound(t *testing.T) {
	_, err := FindRunbook("nonexistent", "/tmp/nonexistent-dir-abc123")
	if err == nil {
		t.Error("expected error for nonexistent runbook, got nil")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	os.WriteFile(path, []byte("{{{{invalid yaml"), 0o644)

	_, err := Load(path)
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestLoadNonexistentFile(t *testing.T) {
	_, err := Load("/nonexistent/file.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

func TestValidateSSHStep(t *testing.T) {
	tests := []struct {
		name    string
		step    Step
		wantErr bool
	}{
		{
			name: "valid ssh step",
			step: Step{
				Name: "ssh-ok", Type: StepSSH,
				SSH: &SSHStep{Host: "web01", Command: "uptime"},
			},
			wantErr: false,
		},
		{
			name: "ssh missing host",
			step: Step{
				Name: "ssh-no-host", Type: StepSSH,
				SSH: &SSHStep{Command: "uptime"},
			},
			wantErr: true,
		},
		{
			name: "ssh missing command",
			step: Step{
				Name: "ssh-no-cmd", Type: StepSSH,
				SSH: &SSHStep{Host: "web01"},
			},
			wantErr: true,
		},
		{
			name:    "ssh nil config",
			step:    Step{Name: "ssh-nil", Type: StepSSH},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rb := &Runbook{Name: "test", Steps: []Step{tt.step}}
			errs := Validate(rb)
			if tt.wantErr && len(errs) == 0 {
				t.Error("expected validation error, got none")
			}
			if !tt.wantErr && len(errs) > 0 {
				t.Errorf("unexpected validation errors: %v", errs)
			}
		})
	}
}

func TestValidateHTTPStep(t *testing.T) {
	tests := []struct {
		name    string
		step    Step
		wantErr bool
	}{
		{
			name: "valid http step",
			step: Step{
				Name: "http-ok", Type: StepHTTP,
				HTTP: &HTTPStep{Method: "GET", URL: "http://localhost/health"},
			},
			wantErr: false,
		},
		{
			name: "http missing url",
			step: Step{
				Name: "http-no-url", Type: StepHTTP,
				HTTP: &HTTPStep{Method: "GET"},
			},
			wantErr: true,
		},
		{
			name:    "http nil config",
			step:    Step{Name: "http-nil", Type: StepHTTP},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rb := &Runbook{Name: "test", Steps: []Step{tt.step}}
			errs := Validate(rb)
			if tt.wantErr && len(errs) == 0 {
				t.Error("expected validation error, got none")
			}
			if !tt.wantErr && len(errs) > 0 {
				t.Errorf("unexpected validation errors: %v", errs)
			}
		})
	}
}

func TestValidateUnknownType(t *testing.T) {
	rb := &Runbook{
		Name:  "test",
		Steps: []Step{{Name: "bad", Type: StepType("ftp")}},
	}
	errs := Validate(rb)
	if len(errs) == 0 {
		t.Error("expected validation error for unknown type, got none")
	}
}

func TestValidateConfirmOnlyStep(t *testing.T) {
	rb := &Runbook{
		Name:  "test",
		Steps: []Step{{Name: "confirm-gate", Confirm: "Proceed?"}},
	}
	errs := Validate(rb)
	if len(errs) != 0 {
		t.Errorf("confirm-only step should be valid, got errors: %v", errs)
	}
}
