package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/msjurset/runbook/internal/runbook"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available runbooks",
	Args:  cobra.NoArgs,
	RunE:  runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	books, err := runbook.Discover(cfg.RunbookDir)
	if err != nil {
		return err
	}

	// Also discover from current directory
	cwd, _ := os.Getwd()
	if cwd != cfg.RunbookDir {
		local, err := runbook.Discover(cwd)
		if err == nil {
			books = append(books, local...)
		}
	}

	if len(books) == 0 {
		fmt.Printf("No runbooks found in %s or current directory.\n", cfg.RunbookDir)
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTEPS\tDESCRIPTION\tPATH")
	for _, b := range books {
		fmt.Fprintf(w, "%s\t%d\t%s\t%s\n", b.Name, len(b.Steps), b.Description, b.FilePath)
	}
	return w.Flush()
}
