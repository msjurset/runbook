package runbook

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/msjurset/runbook/internal/credentials"
)

// ResolveVariables merges variable values from defaults, environment, and CLI
// overrides (in ascending priority order) and returns the final variable map.
// Variables with op:// values are resolved through the platform keychain and
// 1Password CLI. Returns any variables that still need interactive prompting.
func ResolveVariables(defs []VariableDef, cliVars map[string]string) (map[string]string, []VariableDef, error) {
	vars := make(map[string]string, len(defs))
	var needPrompt []VariableDef

	for _, d := range defs {
		// Layer 1: default
		if d.Default != "" {
			vars[d.Name] = d.Default
		}

		// Layer 2: environment (RUNBOOK_VAR_<NAME>)
		envKey := "RUNBOOK_VAR_" + strings.ToUpper(d.Name)
		if v, ok := os.LookupEnv(envKey); ok {
			vars[d.Name] = v
		}

		// Layer 3: CLI override
		if v, ok := cliVars[d.Name]; ok {
			vars[d.Name] = v
		}

		// Layer 4: resolve op:// references via keychain + 1Password
		if val, ok := vars[d.Name]; ok && credentials.IsOpRef(val) {
			keychainKey := d.Name
			resolved, err := credentials.LoadOrResolve(keychainKey, val)
			if err != nil {
				return nil, nil, fmt.Errorf("resolving secret %q: %w", d.Name, err)
			}
			vars[d.Name] = resolved
			continue
		}

		// If still empty and has a prompt, mark for prompting
		if _, ok := vars[d.Name]; !ok {
			if d.Prompt != "" {
				needPrompt = append(needPrompt, d)
				continue
			}
			if d.Required {
				return nil, nil, fmt.Errorf("required variable %q has no value (set via --var, env RUNBOOK_VAR_%s, or add a default)",
					d.Name, strings.ToUpper(d.Name))
			}
		}
	}

	return vars, needPrompt, nil
}

// Expand applies Go template expansion to s using the given variable map.
func Expand(s string, vars map[string]string) (string, error) {
	if !strings.Contains(s, "{{") {
		return s, nil
	}
	tmpl, err := template.New("").Option("missingkey=zero").Parse(s)
	if err != nil {
		return "", fmt.Errorf("template parse error: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("template exec error: %w", err)
	}
	return buf.String(), nil
}
