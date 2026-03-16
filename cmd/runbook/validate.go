package main

import (
	"fmt"

	"github.com/msjurset/runbook/internal/runbook"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate <name|path>",
	Short: "Validate a runbook without executing",
	Args:  cobra.ExactArgs(1),
	RunE:  runValidate,
}

func init() {
	rootCmd.AddCommand(validateCmd)
}

func runValidate(cmd *cobra.Command, args []string) error {
	book, err := runbook.FindRunbook(args[0], cfg.RunbookDir, ".")
	if err != nil {
		return err
	}

	errs := runbook.Validate(book)
	if len(errs) == 0 {
		fmt.Printf("✓ %s is valid (%d steps)\n", book.Name, len(book.Steps))
		return nil
	}

	fmt.Printf("✗ %s has %d error(s):\n", book.Name, len(errs))
	for _, e := range errs {
		fmt.Printf("  - %s\n", e)
	}
	return fmt.Errorf("validation failed")
}
