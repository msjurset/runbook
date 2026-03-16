package runbook

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDurationUnmarshal(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"5m", "5m0s"},
		{"30s", "30s"},
		{"1h30m", "1h30m0s"},
		{"", "0s"},
	}

	for _, tt := range tests {
		var d Duration
		err := yaml.Unmarshal([]byte(tt.input), &d)
		if err != nil {
			t.Errorf("Unmarshal(%q) error: %v", tt.input, err)
			continue
		}
		if got := d.Duration.String(); got != tt.want {
			t.Errorf("Unmarshal(%q) = %s, want %s", tt.input, got, tt.want)
		}
	}
}

func TestLoadRunbook(t *testing.T) {
	content := `
name: test-book
description: A test runbook
variables:
  - name: host
    default: "localhost"
    required: false
steps:
  - name: Check status
    type: shell
    shell:
      command: "echo ok"
    timeout: 10s
    capture: status
`
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	rb, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if rb.Name != "test-book" {
		t.Errorf("Name = %q, want %q", rb.Name, "test-book")
	}
	if len(rb.Variables) != 1 {
		t.Fatalf("len(Variables) = %d, want 1", len(rb.Variables))
	}
	if rb.Variables[0].Name != "host" {
		t.Errorf("Variables[0].Name = %q, want %q", rb.Variables[0].Name, "host")
	}
	if len(rb.Steps) != 1 {
		t.Fatalf("len(Steps) = %d, want 1", len(rb.Steps))
	}
	if rb.Steps[0].Type != StepShell {
		t.Errorf("Steps[0].Type = %q, want %q", rb.Steps[0].Type, StepShell)
	}
	if rb.Steps[0].Capture != "status" {
		t.Errorf("Steps[0].Capture = %q, want %q", rb.Steps[0].Capture, "status")
	}
	if rb.Steps[0].Timeout.Duration.Seconds() != 10 {
		t.Errorf("Steps[0].Timeout = %v, want 10s", rb.Steps[0].Timeout.Duration)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		book    Runbook
		wantErr int
	}{
		{
			name:    "empty name",
			book:    Runbook{Steps: []Step{{Name: "s", Type: StepShell, Shell: &ShellStep{Command: "echo"}}}},
			wantErr: 1,
		},
		{
			name:    "no steps",
			book:    Runbook{Name: "test"},
			wantErr: 1,
		},
		{
			name: "shell step missing command",
			book: Runbook{
				Name:  "test",
				Steps: []Step{{Name: "s", Type: StepShell, Shell: &ShellStep{}}},
			},
			wantErr: 1,
		},
		{
			name: "retry without retries count",
			book: Runbook{
				Name: "test",
				Steps: []Step{{
					Name: "s", Type: StepShell,
					Shell:   &ShellStep{Command: "echo"},
					OnError: PolicyRetry, Retries: 0,
				}},
			},
			wantErr: 1,
		},
		{
			name: "valid runbook",
			book: Runbook{
				Name: "test",
				Steps: []Step{{
					Name: "s", Type: StepShell,
					Shell: &ShellStep{Command: "echo ok"},
				}},
			},
			wantErr: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := Validate(&tt.book)
			if len(errs) != tt.wantErr {
				t.Errorf("Validate() returned %d errors, want %d: %v", len(errs), tt.wantErr, errs)
			}
		})
	}
}

func TestDiscover(t *testing.T) {
	dir := t.TempDir()

	content := `name: book1
steps:
  - name: s1
    type: shell
    shell:
      command: "echo 1"
`
	os.WriteFile(filepath.Join(dir, "book1.yaml"), []byte(content), 0o644)
	os.WriteFile(filepath.Join(dir, "book2.yml"), []byte(content), 0o644)
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not a runbook"), 0o644)

	books, err := Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(books) != 2 {
		t.Errorf("Discover() found %d books, want 2", len(books))
	}
}

func TestDiscoverNonexistent(t *testing.T) {
	books, err := Discover("/nonexistent/path")
	if err != nil {
		t.Errorf("Discover() unexpected error for nonexistent dir: %v", err)
	}
	if books != nil {
		t.Errorf("Discover() = %v, want nil", books)
	}
}
