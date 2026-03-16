package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/msjurset/runbook/internal/engine"
	"github.com/msjurset/runbook/internal/history"
	"github.com/msjurset/runbook/internal/notify"
	"github.com/msjurset/runbook/internal/runbook"
	"github.com/msjurset/runbook/internal/tui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var runFlags struct {
	vars   []string
	dryRun bool
	yes    bool
	noTUI  bool
}

var runCmd = &cobra.Command{
	Use:   "run <name|path>",
	Short: "Execute a runbook",
	Args:  cobra.ExactArgs(1),
	RunE:  runRun,
}

func init() {
	runCmd.Flags().StringArrayVar(&runFlags.vars, "var", nil, "set variable (key=value, repeatable)")
	runCmd.Flags().BoolVar(&runFlags.dryRun, "dry-run", false, "validate and show steps without executing")
	runCmd.Flags().BoolVar(&runFlags.yes, "yes", false, "auto-confirm all prompts")
	runCmd.Flags().BoolVar(&runFlags.noTUI, "no-tui", false, "disable TUI mode")
	rootCmd.AddCommand(runCmd)
}

func runRun(cmd *cobra.Command, args []string) error {
	book, err := runbook.FindRunbook(args[0], cfg.RunbookDir, ".")
	if err != nil {
		return err
	}

	// Validate
	if errs := runbook.Validate(book); len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  error: %s\n", e)
		}
		return fmt.Errorf("runbook %q has validation errors", book.Name)
	}

	// Parse CLI variables
	cliVars := make(map[string]string)
	for _, v := range runFlags.vars {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid variable format %q (expected key=value)", v)
		}
		cliVars[parts[0]] = parts[1]
	}

	// Resolve variables
	vars, needPrompt, err := runbook.ResolveVariables(book.Variables, cliVars)
	if err != nil {
		return err
	}

	// Prompt for missing variables (before TUI starts)
	for _, vd := range needPrompt {
		fmt.Printf("  ? %s: ", vd.Prompt)
		var input string
		if _, err := fmt.Scanln(&input); err != nil {
			return fmt.Errorf("reading variable %q: %w", vd.Name, err)
		}
		vars[vd.Name] = strings.TrimSpace(input)
	}

	if runFlags.dryRun {
		fmt.Printf("Runbook: %s\n", book.Name)
		if book.Description != "" {
			fmt.Printf("  %s\n", book.Description)
		}
		fmt.Printf("\nVariables:\n")
		for k, v := range vars {
			fmt.Printf("  %s = %s\n", k, v)
		}
		fmt.Printf("\nSteps:\n")
		for i, s := range book.Steps {
			fmt.Printf("  %d. [%s] %s\n", i+1, s.Type, s.Name)
		}
		return nil
	}

	// Set up context with signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Auto-detect TUI: use TUI when stdout is a terminal and --no-tui is not set
	useTUI := !runFlags.noTUI && term.IsTerminal(int(os.Stdout.Fd()))

	if useTUI {
		tuiResult, err := tui.Run(book, vars, ctx, cancel)
		if tuiResult != nil {
			saveHistory(book, *tuiResult)
			sendNotifications(book, *tuiResult)
		}
		return err
	}

	// Plain CLI mode
	observer := &engine.CLIObserver{AutoConfirm: runFlags.yes}

	fmt.Printf("Running: %s", book.Name)
	if book.Description != "" {
		fmt.Printf(" — %s", book.Description)
	}
	fmt.Printf(" (%d steps)\n", len(book.Steps))

	eng := engine.New(book, vars, observer)
	result := eng.Run(ctx)

	saveHistory(book, result)
	sendNotifications(book, result)

	if !result.Success {
		return fmt.Errorf("runbook failed")
	}
	return nil
}

func saveHistory(book *runbook.Runbook, result engine.RunResult) {
	store := history.NewStore(cfg.HistoryDir)
	rec := history.Record{
		RunbookName: result.RunbookName,
		FilePath:    book.FilePath,
		StartedAt:   result.StartedAt,
		Duration:    result.Duration.Round(100 * 1e6).String(),
		Success:     result.Success,
		StepCount:   len(result.Steps),
	}
	for _, s := range result.Steps {
		sr := history.StepRecord{
			Name:     s.StepName,
			Status:   s.Status.String(),
			Duration: s.Duration.Round(100 * 1e6).String(),
		}
		if s.Error != nil {
			sr.Error = s.Error.Error()
		}
		rec.Steps = append(rec.Steps, sr)
	}
	// Best-effort save — don't fail the run if history write fails
	_ = store.Save(rec)
}

func sendNotifications(book *runbook.Runbook, result engine.RunResult) {
	if errs := notify.Send(book, result); len(errs) > 0 {
		for _, err := range errs {
			fmt.Fprintf(os.Stderr, "notification error: %v\n", err)
		}
	}
}
