package main

import (
	"fmt"
	"os"

	"github.com/msjurset/runbook/internal/runbook"
	"github.com/spf13/cobra"
)

var completionNamesCmd = &cobra.Command{
	Use:    "completion-names",
	Short:  "Print runbook names for shell completion",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE:   runCompletionNames,
}

func init() {
	rootCmd.AddCommand(completionNamesCmd)
}

func runCompletionNames(cmd *cobra.Command, args []string) error {
	books, _ := runbook.Discover(cfg.RunbookDir)

	cwd, _ := os.Getwd()
	if cwd != cfg.RunbookDir {
		local, _ := runbook.Discover(cwd)
		books = append(books, local...)
	}

	seen := make(map[string]bool)
	for _, b := range books {
		if !seen[b.Name] {
			fmt.Println(b.Name)
			seen[b.Name] = true
		}
	}
	return nil
}
