package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var dbPath string

var rootCmd = &cobra.Command{
	Use:   "cfo",
	Short: "Personal CFO — AI-powered financial intelligence",
	Long: `Personal CFO is a local-first CLI tool that ingests your financial data
(CRED exports, bank statements, UPI history) and uses AI to give you
actionable insights — spending analysis, quarterly reviews, goal tracking,
and savings advice.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	home, _ := os.UserHomeDir()
	defaultDB := filepath.Join(home, ".cfo", "cfo.db")
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", defaultDB, "path to SQLite database")
}
