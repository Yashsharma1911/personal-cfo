package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yash/personal-cfo/internal/db"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the database",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := db.Init(dbPath); err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		fmt.Printf("✓ Database initialized at %s\n", dbPath)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
