package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/msjurset/runbook/internal/cron"
	"github.com/msjurset/runbook/internal/launchd"
	"github.com/msjurset/runbook/internal/runbook"
	"github.com/spf13/cobra"
)

var cronCmd = &cobra.Command{
	Use:   "cron",
	Short: "Manage scheduled runbook execution via crontab",
}

var cronAddFlags struct {
	vars []string
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
  "30 2 * * 1-5"  Weekdays at 2:30 AM

Use --var (repeatable) to bake CLI variables into the scheduled run.
Same runbook + different schedule + different vars is supported and
common — e.g. a daily and a monthly variant of one report:

  runbook cron add my-report "0 6 * * *" \
    --var report_type=daily --var path=/reports/daily.csv
  runbook cron add my-report "0 7 1 * *" \
    --var report_type=monthly --var path=/reports/monthly.csv`,
	Args: cobra.ExactArgs(2),
	RunE: runCronAdd,
}

var cronRemoveCmd = &cobra.Command{
	Use:   "remove <name> [schedule]",
	Short: "Remove a scheduled runbook from crontab",
	Long: `Remove cron entries for a runbook.

  runbook cron remove my-runbook              # remove ALL schedules
  runbook cron remove my-runbook "0 3 * * 0"  # remove specific schedule`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runCronRemove,
}

var cronListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all scheduled runbooks",
	Args:  cobra.NoArgs,
	RunE:  runCronList,
}

func init() {
	cronAddCmd.Flags().StringArrayVar(&cronAddFlags.vars, "var", nil, "set variable for the scheduled run (key=value, repeatable)")
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

	// Pick the backend automatically: runbooks that resolve op:// secrets
	// can't run from cron's launchd session on macOS (no keychain access),
	// so install them as user LaunchAgents instead. Cron is the default for
	// everything else and the only option on non-macOS.
	backend := cron.BackendCron
	backendNote := ""
	if book.NeedsKeychain() && launchd.Available() {
		backend = cron.BackendLaunchd
		backendNote = " (via launchd — keychain-required)"
	} else if book.NeedsKeychain() && !launchd.Available() {
		fmt.Fprintln(os.Stderr, "warning: runbook references op:// secrets but the LaunchAgent backend isn't available on this platform; scheduled runs may fail to resolve secrets")
	}

	if err := cron.Add(book.Name, schedule, logDir, backend, cronAddFlags.vars); err != nil {
		return err
	}

	fmt.Printf("✓ Scheduled %q: %s%s\n", book.Name, schedule, backendNote)
	if len(cronAddFlags.vars) > 0 {
		fmt.Printf("  Vars: %s\n", strings.Join(cronAddFlags.vars, " "))
	}
	fmt.Printf("  Logs: %s/%s.log\n", logDir, book.Name)
	return nil
}

func runCronRemove(cmd *cobra.Command, args []string) error {
	name := args[0]
	schedule := ""
	if len(args) > 1 {
		schedule = args[1]
	}
	if err := cron.RemoveSchedule(name, schedule); err != nil {
		return err
	}
	if schedule != "" {
		fmt.Printf("✓ Removed schedule %q for %q\n", schedule, name)
	} else {
		fmt.Printf("✓ Removed all schedules for %q\n", name)
	}
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
	fmt.Fprintln(w, "RUNBOOK\tSCHEDULE\tBACKEND\tVARS")
	for _, e := range entries {
		vars := "-"
		if len(e.Vars) > 0 {
			vars = strings.Join(e.Vars, " ")
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", e.Name, e.Schedule, e.Backend, vars)
	}
	return w.Flush()
}
