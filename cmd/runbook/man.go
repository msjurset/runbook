package main

import (
	"fmt"

	"github.com/msjurset/runbook/internal/manpage"
	"github.com/spf13/cobra"
)

var manCmd = &cobra.Command{
	Use:   "man",
	Short: "Display the runbook manual page",
	Long:  "Print the roff-formatted man page to stdout. Use with: runbook man | man -l -",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(manpage.Content)
	},
}

func init() {
	rootCmd.AddCommand(manCmd)
}
