package main

import (
	"fmt"
	"os"
	"text/tabwriter"

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

	// Variables
	if len(book.Variables) > 0 {
		fmt.Printf("\nVariables:\n")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "  NAME\tDEFAULT\tREQUIRED\tSECRET")
		for _, v := range book.Variables {
			req := "no"
			if v.Required {
				req = "yes"
			}
			secret := ""
			if v.Secret {
				secret = "yes"
			}
			def := v.Default
			if def == "" {
				def = "—"
			}
			fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", v.Name, def, req, secret)
		}
		w.Flush()
	}

	// Steps
	fmt.Printf("\nSteps (%d):\n", len(book.Steps))
	for i, s := range book.Steps {
		typ := string(s.Type)
		if typ == "" {
			typ = "confirm"
		}

		fmt.Printf("  %d. [%s] %s\n", i+1, typ, s.Name)

		// Step details
		switch s.Type {
		case runbook.StepShell:
			if s.Shell != nil {
				fmt.Printf("     command: %s\n", truncate(s.Shell.Command, 80))
				if s.Shell.Dir != "" {
					fmt.Printf("     dir: %s\n", s.Shell.Dir)
				}
			}
		case runbook.StepSSH:
			if s.SSH != nil {
				host := s.SSH.Host
				if s.SSH.User != "" {
					host = s.SSH.User + "@" + host
				}
				if s.SSH.Port > 0 {
					host = fmt.Sprintf("%s:%d", host, s.SSH.Port)
				}
				fmt.Printf("     host: %s\n", host)
				fmt.Printf("     command: %s\n", truncate(s.SSH.Command, 80))
			}
		case runbook.StepHTTP:
			if s.HTTP != nil {
				method := s.HTTP.Method
				if method == "" {
					method = "GET"
				}
				fmt.Printf("     %s %s\n", method, s.HTTP.URL)
			}
		default:
			if s.Confirm != "" {
				fmt.Printf("     prompt: %s\n", truncate(s.Confirm, 80))
			}
		}

		// Step options
		var opts []string
		if s.OnError != "" {
			opts = append(opts, fmt.Sprintf("on_error:%s", s.OnError))
		}
		if s.Timeout.Duration > 0 {
			opts = append(opts, fmt.Sprintf("timeout:%s", s.Timeout.Duration))
		}
		if s.Retries > 0 {
			opts = append(opts, fmt.Sprintf("retries:%d", s.Retries))
		}
		if s.Capture != "" {
			opts = append(opts, fmt.Sprintf("capture:%s", s.Capture))
		}
		if s.Condition != "" {
			opts = append(opts, "conditional")
		}
		if s.Parallel {
			opts = append(opts, "parallel")
		}
		if len(opts) > 0 {
			fmt.Printf("     ")
			for i, o := range opts {
				if i > 0 {
					fmt.Printf("  ")
				}
				fmt.Printf("[%s]", o)
			}
			fmt.Println()
		}
	}

	// Notifications
	if book.Notify != nil {
		fmt.Printf("\nNotifications:\n")
		fmt.Printf("  trigger: %s\n", notifyTrigger(book.Notify.On))
		if book.Notify.Desktop {
			fmt.Printf("  desktop: yes\n")
		}
		if book.Notify.Slack != nil {
			fmt.Printf("  slack: %s\n", book.Notify.Slack.Webhook)
			if book.Notify.Slack.Channel != "" {
				fmt.Printf("    channel: %s\n", book.Notify.Slack.Channel)
			}
		}
		if book.Notify.Email != nil {
			fmt.Printf("  email: %s → %s\n", book.Notify.Email.From, book.Notify.Email.To)
		}
	}

	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func notifyTrigger(on string) string {
	if on == "" {
		return "always"
	}
	return on
}
