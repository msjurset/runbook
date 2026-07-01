package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/msjurset/runbook/internal/credentials"
	"github.com/msjurset/runbook/internal/runbook"
	"github.com/spf13/cobra"
)

var authFlags struct {
	clear bool
}

var authCmd = &cobra.Command{
	Use:   "auth [name|path]",
	Short: "Pre-resolve and cache 1Password secrets in system keychain",
	Long: `Resolve op:// variable references in a runbook via the 1Password CLI
and cache the results in the system keychain for future runs.

Use --clear to remove cached secrets for a runbook.

If no runbook name is provided, all discovered runbooks will be authenticated.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAuth,
}

func init() {
	authCmd.Flags().BoolVar(&authFlags.clear, "clear", false, "remove cached secrets instead of resolving")
	rootCmd.AddCommand(authCmd)
}

func runAuth(cmd *cobra.Command, args []string) error {
	var books []*runbook.Runbook
	if len(args) == 0 {
		var err error
		books, err = runbook.Discover(cfg.RunbookDir)
		if err != nil {
			return err
		}
		if len(books) == 0 {
			fmt.Printf("No runbooks discovered in %s\n", cfg.RunbookDir)
			return nil
		}
	} else {
		book, err := runbook.FindRunbook(args[0], cfg.RunbookDir, ".")
		if err != nil {
			return err
		}
		books = []*runbook.Runbook{book}
	}

	foundAny := false
	for _, book := range books {
		foundBook := false
		fmt.Printf("Runbook %q:\n", book.Name)

		// Cache op:// variable references
		for _, v := range book.Variables {
			if v.Default == "" || !credentials.IsOpRef(v.Default) {
				continue
			}
			foundBook = true
			foundAny = true
			key := v.Name

			if authFlags.clear {
				if err := credentials.Delete(key); err != nil {
					fmt.Fprintf(os.Stderr, "  ✗ %s: %v\n", v.Name, err)
					continue
				}
				fmt.Printf("  ✓ %s: cleared from keychain\n", v.Name)
				continue
			}

			fmt.Printf("  ▸ %s: resolving %s...\n", v.Name, truncateRef(v.Default))
			val, err := credentials.ResolveAndCache(key, v.Default)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  ✗ %s: %v\n", v.Name, err)
				continue
			}
			preview := val
			if len(preview) > 20 {
				preview = preview[:20] + "..."
			}
			fmt.Printf("  ✓ %s: cached (%s)\n", v.Name, preview)
		}

		// Cache op:// SSH keys from steps (keyed by op:// ref so all steps share cache)
		seenKeys := make(map[string]bool)
		for _, s := range book.Steps {
			if s.SSH == nil || s.SSH.KeyFile == "" || !credentials.IsOpRef(s.SSH.KeyFile) {
				continue
			}
			key := "ssh_key_" + s.SSH.KeyFile
			if seenKeys[key] {
				continue // Already cached this ref
			}
			seenKeys[key] = true
			foundBook = true
			foundAny = true

			if authFlags.clear {
				if err := credentials.Delete(key); err != nil {
					fmt.Fprintf(os.Stderr, "  ✗ ssh_key (%s): %v\n", truncateRef(s.SSH.KeyFile), err)
					continue
				}
				fmt.Printf("  ✓ ssh_key: cleared from keychain\n")
				continue
			}

			fmt.Printf("  ▸ ssh_key: resolving %s...\n", truncateRef(s.SSH.KeyFile))
			val, err := credentials.ResolveAndCache(key, s.SSH.KeyFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  ✗ ssh_key: %v\n", err)
				continue
			}
			fmt.Printf("  ✓ ssh_key: cached (%d bytes)\n", len(val))
		}

		if !foundBook {
			fmt.Printf("  No op:// references found.\n")
		}
	}

	if !foundAny && len(args) > 0 {
		fmt.Printf("No op:// references found in %q.\n", args[0])
	}
	return nil
}

func truncateRef(ref string) string {
	// Show vault/item but hide field details for security
	parts := strings.SplitN(ref, "/", 5)
	if len(parts) >= 5 {
		return strings.Join(parts[:4], "/") + "/..."
	}
	return ref
}
