package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/msjurset/runbook/internal/history"
	"github.com/spf13/cobra"
)

var historyFlags struct {
	limit int
	name  string
}

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Show runbook execution history",
	Args:  cobra.NoArgs,
	RunE:  runHistory,
}

func init() {
	historyCmd.Flags().IntVarP(&historyFlags.limit, "limit", "n", 20, "max records to show")
	historyCmd.Flags().StringVar(&historyFlags.name, "runbook", "", "filter by runbook name")
	rootCmd.AddCommand(historyCmd)
}

func runHistory(cmd *cobra.Command, args []string) error {
	store := history.NewStore(cfg.HistoryDir)
	records, err := store.List(historyFlags.limit, historyFlags.name)
	if err != nil {
		return err
	}

	if len(records) == 0 {
		fmt.Println("No run history found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "TIME\tRUNBOOK\tSTATUS\tSTEPS\tDURATION")
	for _, r := range records {
		status := "✓"
		if !r.Success {
			status = "✗"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
			r.StartedAt.Local().Format("2006-01-02 15:04:05"),
			r.RunbookName,
			status,
			r.StepCount,
			r.Duration,
		)
	}
	return w.Flush()
}
