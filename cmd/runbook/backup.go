package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/msjurset/runbook/internal/backup"
	"github.com/msjurset/runbook/internal/runbook"
	"github.com/spf13/cobra"
)

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Manage runbook backups (per-YAML and full snapshots)",
	Long: `Manage files in ~/.runbook/backups/.

Two kinds of backups coexist there:
  - Per-YAML backups written by the Mac app before each save/delete.
    Filename: <name>-<ISO timestamp>.yaml
  - Full-state snapshots created via 'runbook backup snapshot'.
    Filename: runbook-<ISO timestamp>.tar.gz

The snapshot command is goback-friendly: paired with a goback 'local'
job that picks up the tarball, you get scheduled backups of the entire
runbook home directory.`,
}

var backupListCmd = &cobra.Command{
	Use:   "list [name]",
	Short: "List backup files (per-YAML and snapshots), newest first",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runBackupList,
}

var backupShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Print the contents of a backup (or a tarball file listing)",
	Args:  cobra.ExactArgs(1),
	RunE:  runBackupShow,
}

var backupRestoreCmd = &cobra.Command{
	Use:   "restore <name>",
	Short: "Restore a per-YAML backup over the current file (saves a fresh backup of the current state first)",
	Args:  cobra.ExactArgs(1),
	RunE:  runBackupRestore,
}

var backupDiffCmd = &cobra.Command{
	Use:   "diff <name>",
	Short: "Show a unified diff between a backup and the current file",
	Args:  cobra.ExactArgs(1),
	RunE:  runBackupDiff,
}

var backupPruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Delete old backups by count and/or age",
	Args:  cobra.NoArgs,
	RunE:  runBackupPrune,
}

var backupSnapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Create a full-state tarball at ~/.runbook/backups/runbook-<ts>.tar.gz",
	Args:  cobra.NoArgs,
	RunE:  runBackupSnapshot,
}

func init() {
	backupShowCmd.Flags().String("at", "", "Match a specific timestamp prefix (e.g. 2026-04-29 or 2026-04-29T08)")
	backupRestoreCmd.Flags().String("at", "", "Match a specific timestamp prefix")
	backupDiffCmd.Flags().String("at", "", "Match a specific timestamp prefix")
	backupPruneCmd.Flags().Int("keep", 10, "Max backups to keep per (name, kind). 0 disables this rule.")
	backupPruneCmd.Flags().String("older-than", "", "Delete backups older than this duration (e.g. 7d, 30d, 24h)")
	backupPruneCmd.Flags().Bool("dry-run", false, "Print what would be deleted without deleting")

	backupCmd.AddCommand(backupListCmd)
	backupCmd.AddCommand(backupShowCmd)
	backupCmd.AddCommand(backupRestoreCmd)
	backupCmd.AddCommand(backupDiffCmd)
	backupCmd.AddCommand(backupPruneCmd)
	backupCmd.AddCommand(backupSnapshotCmd)

	rootCmd.AddCommand(backupCmd)
}

func runBackupList(cmd *cobra.Command, args []string) error {
	dir := backup.DefaultDir()
	name := ""
	if len(args) == 1 {
		name = args[0]
	}
	entries, err := backup.List(dir, name)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		if name != "" {
			fmt.Printf("No backups for %q.\n", name)
		} else {
			fmt.Println("No backups found.")
		}
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "KIND\tNAME\tTIMESTAMP\tSIZE\tPATH")
	for _, e := range entries {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			e.Kind, e.RunbookName,
			e.Timestamp.Local().Format("2006-01-02 15:04:05"),
			humanSize(e.Size),
			e.Path,
		)
	}
	return w.Flush()
}

func runBackupShow(cmd *cobra.Command, args []string) error {
	entry, err := resolveEntry(args[0], cmd)
	if err != nil {
		return err
	}
	out, err := backup.Show(entry.Path)
	if err != nil {
		return err
	}
	os.Stdout.Write(out)
	if len(out) > 0 && out[len(out)-1] != '\n' {
		fmt.Println()
	}
	return nil
}

func runBackupRestore(cmd *cobra.Command, args []string) error {
	name := args[0]
	entry, err := resolveEntry(name, cmd)
	if err != nil {
		return err
	}

	// Find the runbook on disk to restore over.
	book, err := runbook.FindRunbook(name, cfg.RunbookDir, ".")
	if err != nil {
		return fmt.Errorf("could not locate the current runbook %q: %w (the file may have been deleted; copy %s to %s manually)",
			name, err, entry.Path, filepath.Join(cfg.RunbookDir, name+".yaml"))
	}

	if err := backup.Restore(entry.Path, book.FilePath, backup.DefaultDir()); err != nil {
		return err
	}
	fmt.Printf("✓ Restored %s from %s\n", book.FilePath, filepath.Base(entry.Path))
	fmt.Printf("  A backup of the previous state was saved to %s\n", backup.DefaultDir())
	return nil
}

func runBackupDiff(cmd *cobra.Command, args []string) error {
	name := args[0]
	entry, err := resolveEntry(name, cmd)
	if err != nil {
		return err
	}
	book, err := runbook.FindRunbook(name, cfg.RunbookDir, ".")
	if err != nil {
		return fmt.Errorf("could not locate the current runbook %q: %w", name, err)
	}
	out, err := backup.Diff(entry.Path, book.FilePath)
	if err != nil {
		return err
	}
	if len(out) == 0 {
		fmt.Printf("No differences between %s and %s.\n", filepath.Base(entry.Path), book.FilePath)
		return nil
	}
	os.Stdout.Write(out)
	return nil
}

func runBackupPrune(cmd *cobra.Command, args []string) error {
	keep, _ := cmd.Flags().GetInt("keep")
	olderStr, _ := cmd.Flags().GetString("older-than")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	var olderThan time.Duration
	if olderStr != "" {
		d, err := parseDayDuration(olderStr)
		if err != nil {
			return fmt.Errorf("invalid --older-than %q: %w", olderStr, err)
		}
		olderThan = d
	}
	if keep == 0 && olderThan == 0 {
		return fmt.Errorf("specify at least one of --keep N or --older-than DURATION (use --keep 0 + --older-than to disable count rule)")
	}

	deleted, err := backup.Prune(backup.DefaultDir(), keep, olderThan, dryRun)
	if err != nil {
		return err
	}
	if len(deleted) == 0 {
		fmt.Println("No backups to prune.")
		return nil
	}
	verb := "Deleted"
	if dryRun {
		verb = "Would delete"
	}
	fmt.Printf("%s %d backup(s):\n", verb, len(deleted))
	for _, p := range deleted {
		fmt.Printf("  %s\n", p)
	}
	return nil
}

func runBackupSnapshot(cmd *cobra.Command, args []string) error {
	out, err := backup.Snapshot(backup.HomeDir(), backup.DefaultDir())
	if err != nil {
		return err
	}
	info, _ := os.Stat(out)
	size := int64(0)
	if info != nil {
		size = info.Size()
	}
	fmt.Printf("✓ Created %s (%s)\n", out, humanSize(size))
	return nil
}

// resolveEntry looks up a backup by name and (optional) --at timestamp prefix.
// Returns a usable error message if the name is unknown.
func resolveEntry(name string, cmd *cobra.Command) (*backup.Entry, error) {
	tsPrefix, _ := cmd.Flags().GetString("at")
	entries, err := backup.List(backup.DefaultDir(), name)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no backups for %q in %s", name, backup.DefaultDir())
	}
	hit := backup.Find(entries, name, tsPrefix)
	if hit == nil {
		if tsPrefix == "" {
			return nil, fmt.Errorf("no backups for %q", name)
		}
		return nil, fmt.Errorf("no backups for %q matching --at %q", name, tsPrefix)
	}
	return hit, nil
}

// parseDayDuration extends time.ParseDuration to accept "Nd" for days.
// Pure stdlib durations like "24h" / "7d" / "30m" all work.
var dayDurRe = regexp.MustCompile(`^(\d+)d$`)

func parseDayDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if m := dayDurRe.FindStringSubmatch(s); m != nil {
		days, err := strconv.Atoi(m[1])
		if err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

// humanSize renders a byte count as a short string.
func humanSize(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%dB", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1fK", float64(n)/1024)
	case n < 1024*1024*1024:
		return fmt.Sprintf("%.1fM", float64(n)/1024/1024)
	default:
		return fmt.Sprintf("%.1fG", float64(n)/1024/1024/1024)
	}
}
