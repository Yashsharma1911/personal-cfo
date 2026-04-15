package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/yash/personal-cfo/internal/db"
)

var accountsCmd = &cobra.Command{
	Use:   "accounts",
	Short: "Show all accounts and their transaction stats",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := db.Open(dbPath)
		if err != nil {
			return err
		}
		defer store.Close()

		stats, err := store.GetAccountStats()
		if err != nil {
			return err
		}

		if len(stats) == 0 {
			fmt.Println("No accounts found. Run `cfo ingest` first.")
			return nil
		}

		fmt.Println()
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "  ACCOUNT\tTRANSACTIONS\tDEBITS\tCREDITS\tLAST ACTIVITY\n")
		fmt.Fprintf(w, "  ───────\t────────────\t──────\t───────\t─────────────\n")
		for _, a := range stats {
			fmt.Fprintf(w, "  %-25s\t%d\t₹%s\t₹%s\t%s\n",
				a.Name, a.TxCount,
				formatPaisa(a.TotalDebits), formatPaisa(a.TotalCredits),
				a.LastDate)
		}
		w.Flush()
		fmt.Println()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(accountsCmd)
}
