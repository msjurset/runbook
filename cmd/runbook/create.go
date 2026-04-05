package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/msjurset/runbook/internal/runbook"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new runbook",
	Long:  "Create a new runbook, optionally from a template.",
	Args:  cobra.ExactArgs(1),
	RunE:  runCreate,
}

func init() {
	createCmd.Flags().String("from", "", "Template name to copy from")
	rootCmd.AddCommand(createCmd)
}

func runCreate(cmd *cobra.Command, args []string) error {
	name := args[0]
	dest := filepath.Join(cfg.RunbookDir, name+".yaml")

	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("runbook %q already exists at %s", name, dest)
	}

	if err := os.MkdirAll(cfg.RunbookDir, 0o755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	fromName, _ := cmd.Flags().GetString("from")
	var content string

	if fromName != "" {
		// Find the template
		templates, err := runbook.DiscoverTemplates(cfg.RunbookDir)
		if err != nil {
			return fmt.Errorf("discovering templates: %w", err)
		}
		var tmpl *runbook.Runbook
		for _, t := range templates {
			if t.Name == fromName {
				tmpl = t
				break
			}
		}
		if tmpl == nil {
			return fmt.Errorf("template %q not found (use 'runbook list --templates' to see available templates)", fromName)
		}

		data, err := os.ReadFile(tmpl.FilePath)
		if err != nil {
			return fmt.Errorf("reading template: %w", err)
		}
		// Replace the name field
		content = replaceName(string(data), name)
	} else {
		content = fmt.Sprintf(`name: %s
description: ""
steps:
  - name: step-1
    type: shell
    shell:
      command: echo "hello"
`, name)
	}

	if err := os.WriteFile(dest, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing runbook: %w", err)
	}

	fmt.Printf("Created %s\n", dest)

	// Open in $EDITOR if set
	editor := os.Getenv("EDITOR")
	if editor == "" {
		return nil
	}
	editorCmd := exec.Command(editor, dest)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr
	return editorCmd.Run()
}

// replaceName swaps the name: field value in raw YAML.
func replaceName(yaml, newName string) string {
	lines := strings.Split(yaml, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "name:") {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			lines[i] = indent + "name: " + newName
			break
		}
	}
	return strings.Join(lines, "\n")
}
