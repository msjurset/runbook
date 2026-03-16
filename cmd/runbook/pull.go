package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/msjurset/runbook/internal/pull"
	"github.com/msjurset/runbook/internal/runbook"
	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:   "pull <repo-url|file-url>",
	Short: "Pull runbooks from a git repo or download a single file",
	Long: `Clone a git repository into ~/.runbook/books/ so its runbook YAML files
become discoverable, or download a single YAML file by URL.

If the repo was previously pulled, it updates to the latest version.

Examples:
  runbook pull github.com/user/runbooks
  runbook pull https://example.com/deploy.yaml`,
	Args: cobra.ExactArgs(1),
	RunE: runPull,
}

var pullListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pulled repositories",
	Args:  cobra.NoArgs,
	RunE:  runPullList,
}

var pullRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a pulled repository",
	Args:  cobra.ExactArgs(1),
	RunE:  runPullRemove,
}

func init() {
	pullCmd.AddCommand(pullListCmd)
	pullCmd.AddCommand(pullRemoveCmd)
	rootCmd.AddCommand(pullCmd)
}

func runPull(cmd *cobra.Command, args []string) error {
	url := args[0]

	// Detect if this is a single file URL (ends in .yaml/.yml)
	lower := strings.ToLower(url)
	if strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml") {
		dest, err := pull.SingleFile(url, cfg.RunbookDir)
		if err != nil {
			return err
		}
		fmt.Printf("✓ Downloaded to %s\n", dest)
		return nil
	}

	// Git repo
	cloned, err := pull.GitRepo(url, cfg.RunbookDir)
	if err != nil {
		return err
	}

	if cloned {
		fmt.Printf("✓ Cloned %s\n", url)
	} else {
		fmt.Printf("✓ Updated %s\n", url)
	}

	// Show what runbooks were found
	books, err := runbook.Discover(cfg.RunbookDir)
	if err == nil && len(books) > 0 {
		fmt.Printf("  %d runbook(s) available\n", len(books))
	}
	return nil
}

func runPullList(cmd *cobra.Command, args []string) error {
	repos, err := pull.ListRepos(cfg.RunbookDir)
	if err != nil {
		return err
	}

	if len(repos) == 0 {
		fmt.Println("No pulled repositories.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "REPOSITORY\tRUNBOOKS")
	for _, name := range repos {
		books, _ := runbook.Discover(fmt.Sprintf("%s/%s", cfg.RunbookDir, name))
		fmt.Fprintf(w, "%s\t%d\n", name, len(books))
	}
	return w.Flush()
}

func runPullRemove(cmd *cobra.Command, args []string) error {
	if err := pull.RemoveRepo(args[0], cfg.RunbookDir); err != nil {
		return err
	}
	fmt.Printf("✓ Removed %q\n", args[0])
	return nil
}
