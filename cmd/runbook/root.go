package main

import (
	"github.com/msjurset/runbook/internal/config"
	"github.com/spf13/cobra"
)

var cfg = config.Default()

var rootCmd = &cobra.Command{
	Use:   "runbook",
	Short: "Personal command center and runbook engine",
	Long:  "Define, manage, and execute multi-step operational runbooks.",
	SilenceUsage: true,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfg.RunbookDir, "dir", cfg.RunbookDir, "runbook directory")
}
