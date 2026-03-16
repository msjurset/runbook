package main

import (
	"fmt"

	"github.com/msjurset/runbook/internal/runbook"
	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show <name|path>",
	Short: "Show runbook details",
	Args:  cobra.ExactArgs(1),
	RunE:  runShow,
}

func init() {
	rootCmd.AddCommand(showCmd)
}

func runShow(cmd *cobra.Command, args []string) error {
	book, err := runbook.FindRunbook(args[0], cfg.RunbookDir, ".")
	if err != nil {
		return err
	}

	fmt.Printf("Name:        %s\n", book.Name)
	if book.Description != "" {
		fmt.Printf("Description: %s\n", book.Description)
	}
	fmt.Printf("File:        %s\n", book.FilePath)

	if len(book.Variables) > 0 {
		fmt.Printf("\nVariables:\n")
		for _, v := range book.Variables {
			req := ""
			if v.Required {
				req = " (required)"
			}
			def := ""
			if v.Default != "" {
				def = fmt.Sprintf(" [default: %s]", v.Default)
			}
			fmt.Printf("  %s%s%s\n", v.Name, req, def)
		}
	}

	fmt.Printf("\nSteps (%d):\n", len(book.Steps))
	for i, s := range book.Steps {
		typ := string(s.Type)
		if typ == "" {
			typ = "confirm"
		}
		extra := ""
		if s.OnError != "" {
			extra += fmt.Sprintf(" on_error:%s", s.OnError)
		}
		if s.Timeout.Duration > 0 {
			extra += fmt.Sprintf(" timeout:%s", s.Timeout.Duration)
		}
		if s.Capture != "" {
			extra += fmt.Sprintf(" capture:%s", s.Capture)
		}
		if s.Condition != "" {
			extra += " conditional"
		}
		fmt.Printf("  %d. [%s] %s%s\n", i+1, typ, s.Name, extra)
	}

	return nil
}
