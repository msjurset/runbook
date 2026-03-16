package main

import (
	"fmt"
	"time"

	"github.com/msjurset/runbook/internal/engine"
	"github.com/msjurset/runbook/internal/notify"
	"github.com/msjurset/runbook/internal/runbook"
	"github.com/spf13/cobra"
)

var notifyFlags struct {
	fail bool
}

var notifyCmd = &cobra.Command{
	Use:   "notify <name|path>",
	Short: "Send a test notification using a runbook's notify config",
	Args:  cobra.ExactArgs(1),
	RunE:  runNotify,
}

func init() {
	notifyCmd.Flags().BoolVar(&notifyFlags.fail, "fail", false, "simulate a failed run")
	rootCmd.AddCommand(notifyCmd)
}

func runNotify(cmd *cobra.Command, args []string) error {
	book, err := runbook.FindRunbook(args[0], cfg.RunbookDir, ".")
	if err != nil {
		return err
	}

	if book.Notify == nil {
		return fmt.Errorf("runbook %q has no notify config", book.Name)
	}

	// Build a fake result
	status := engine.StatusSuccess
	success := true
	if notifyFlags.fail {
		status = engine.StatusFailed
		success = false
	}

	result := engine.RunResult{
		RunbookName: book.Name,
		Success:     success,
		Duration:    2*time.Second + 500*time.Millisecond,
		StartedAt:   time.Now().Add(-2*time.Second - 500*time.Millisecond),
		Steps: []engine.StepResult{
			{StepName: "test step 1", Status: engine.StatusSuccess, Duration: 1 * time.Second},
			{StepName: "test step 2", Status: status, Duration: 1500 * time.Millisecond},
		},
	}
	if notifyFlags.fail {
		result.Steps[1].Error = fmt.Errorf("simulated failure")
	}

	fmt.Printf("Sending test notification (%s)...\n", map[bool]string{true: "success", false: "failure"}[success])

	errs := notify.Send(book, result)
	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Printf("  ✗ %v\n", e)
		}
		return fmt.Errorf("notification errors occurred")
	}

	fmt.Println("✓ Notification sent")
	return nil
}
