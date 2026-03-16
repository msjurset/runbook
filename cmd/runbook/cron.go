package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/msjurset/runbook/internal/cron"
	"github.com/msjurset/runbook/internal/runbook"
	"github.com/spf13/cobra"
)

var cronCmd = &cobra.Command{
	Use:   "cron",
	Short: "Manage scheduled runbook execution via crontab",
}

var cronAddCmd = &cobra.Command{
	Use:   "add <name|path> <schedule>",
	Short: "Schedule a runbook to run on a cron schedule",
	Long: `Add a crontab entry to run a runbook on a schedule.

The schedule uses standard cron syntax (5 fields):
  minute hour day-of-month month day-of-week

Examples:
  "0 3 * * 0"     Every Sunday at 3:00 AM
  "*/15 * * * *"   Every 15 minutes
  "0 9 1 * *"     First of every month at 9:00 AM
  "30 2 * * 1-5"  Weekdays at 2:30 AM`,
	Args: cobra.ExactArgs(2),
	RunE: runCronAdd,
}

var cronRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a scheduled runbook from crontab",
	Args:  cobra.ExactArgs(1),
	RunE:  runCronRemove,
}

var cronListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all scheduled runbooks",
	Args:  cobra.NoArgs,
	RunE:  runCronList,
}

func init() {
	cronCmd.AddCommand(cronAddCmd)
	cronCmd.AddCommand(cronRemoveCmd)
	cronCmd.AddCommand(cronListCmd)
	rootCmd.AddCommand(cronCmd)
}

func runCronAdd(cmd *cobra.Command, args []string) error {
	nameOrPath := args[0]
	schedule := args[1]

	// Validate the runbook exists
	book, err := runbook.FindRunbook(nameOrPath, cfg.RunbookDir, ".")
	if err != nil {
		return err
	}

	logDir := cfg.HistoryDir // reuse history dir for logs

	if err := cron.Add(book.Name, schedule, logDir); err != nil {
		return err
	}

	fmt.Printf("✓ Scheduled %q: %s\n", book.Name, schedule)
	fmt.Printf("  Logs: %s/%s.log\n", logDir, book.Name)
	return nil
}

func runCronRemove(cmd *cobra.Command, args []string) error {
	if err := cron.Remove(args[0]); err != nil {
		return err
	}
	fmt.Printf("✓ Removed cron schedule for %q\n", args[0])
	return nil
}

func runCronList(cmd *cobra.Command, args []string) error {
	entries, err := cron.List()
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		fmt.Println("No scheduled runbooks.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "RUNBOOK\tSCHEDULE\tCOMMAND")
	for _, e := range entries {
		fmt.Fprintf(w, "%s\t%s\t%s\n", e.Name, e.Schedule, e.Command)
	}
	return w.Flush()
}
