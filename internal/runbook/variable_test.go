package runbook

import (
	"os"
	"testing"
)

func TestResolveVariables(t *testing.T) {
	tests := []struct {
		name      string
		defs      []VariableDef
		cliVars   map[string]string
		envVars   map[string]string
		wantVars  map[string]string
		wantErr   bool
		wantPrompt int
	}{
		{
			name: "defaults only",
			defs: []VariableDef{
				{Name: "host", Default: "localhost"},
				{Name: "port", Default: "8080"},
			},
			wantVars: map[string]string{"host": "localhost", "port": "8080"},
		},
		{
			name: "cli overrides default",
			defs: []VariableDef{
				{Name: "host", Default: "localhost"},
			},
			cliVars:  map[string]string{"host": "prod-01"},
			wantVars: map[string]string{"host": "prod-01"},
		},
		{
			name: "env overrides default",
			defs: []VariableDef{
				{Name: "host", Default: "localhost"},
			},
			envVars:  map[string]string{"RUNBOOK_VAR_HOST": "staging-01"},
			wantVars: map[string]string{"host": "staging-01"},
		},
		{
			name: "cli overrides env",
			defs: []VariableDef{
				{Name: "host", Default: "localhost"},
			},
			envVars:  map[string]string{"RUNBOOK_VAR_HOST": "staging-01"},
			cliVars:  map[string]string{"host": "prod-01"},
			wantVars: map[string]string{"host": "prod-01"},
		},
		{
			name: "required without value errors",
			defs: []VariableDef{
				{Name: "token", Required: true},
			},
			wantErr: true,
		},
		{
			name: "required with prompt defers",
			defs: []VariableDef{
				{Name: "token", Required: true, Prompt: "Enter token"},
			},
			wantVars:   map[string]string{},
			wantPrompt: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env vars
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			vars, needPrompt, err := ResolveVariables(tt.defs, tt.cliVars)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(needPrompt) != tt.wantPrompt {
				t.Errorf("needPrompt = %d, want %d", len(needPrompt), tt.wantPrompt)
			}

			for k, want := range tt.wantVars {
				if got := vars[k]; got != want {
					t.Errorf("vars[%q] = %q, want %q", k, got, want)
				}
			}
		})
	}
}

func TestExpand(t *testing.T) {
	tests := []struct {
		input string
		vars  map[string]string
		want  string
	}{
		{"no templates", nil, "no templates"},
		{"Hello {{.name}}", map[string]string{"name": "World"}, "Hello World"},
		{"{{.host}}:{{.port}}", map[string]string{"host": "localhost", "port": "8080"}, "localhost:8080"},
		{"missing {{.missing}} var", map[string]string{}, "missing  var"},
	}

	for _, tt := range tests {
		got, err := Expand(tt.input, tt.vars)
		if err != nil {
			t.Errorf("Expand(%q) error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Expand(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
