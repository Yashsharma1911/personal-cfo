package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/yash/personal-cfo/internal/db"
)

var (
	summaryMonth   string
	summaryQuarter string
)

var summaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Show spending summary for a period",
	RunE:  runSummary,
}

func init() {
	summaryCmd.Flags().StringVarP(&summaryMonth, "month", "m", "", "month (e.g., march, 03)")
	summaryCmd.Flags().StringVarP(&summaryQuarter, "quarter", "q", "", "quarter (e.g., Q1-2026)")
	rootCmd.AddCommand(summaryCmd)
}

func runSummary(cmd *cobra.Command, args []string) error {
	store, err := db.Open(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()

	summary, err := store.GetSummary(summaryMonth, summaryQuarter)
	if err != nil {
		return err
	}

	if summary.TxCount == 0 {
		fmt.Println("No transactions found for this period.")
		return nil
	}

	// Header
	fmt.Println()
	fmt.Printf("  Total expenses:   ₹%s\n", formatPaisa(summary.TotalExpenses))
	fmt.Printf("  Total credits:    ₹%s\n", formatPaisa(summary.TotalIncome))
	if summary.TotalIncome > 0 {
		savingsRate := float64(summary.TotalIncome-summary.TotalExpenses) / float64(summary.TotalIncome) * 100
		fmt.Printf("  Savings rate:     %.1f%%\n", savingsRate)
	}
	fmt.Printf("  Transactions:     %d\n", summary.TxCount)

	// By category
	if len(summary.ByCategory) > 0 {
		fmt.Println("\n  Spending by category:")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

		type catEntry struct {
			name   string
			amount int64
		}
		var entries []catEntry
		for name, amount := range summary.ByCategory {
			if name == "" {
				name = "uncategorized"
			}
			entries = append(entries, catEntry{name, amount})
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].amount > entries[j].amount })

		for _, e := range entries {
			pct := float64(e.amount) / float64(summary.TotalExpenses) * 100
			bar := strings.Repeat("█", int(pct/5))
			fmt.Fprintf(w, "    %-18s\t₹%s\t%5.1f%%\t%s\n", e.name, formatPaisa(e.amount), pct, bar)
		}
		w.Flush()
	}

	// By account
	if len(summary.ByAccount) > 0 {
		fmt.Println("\n  By account:")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		for name, stats := range summary.ByAccount {
			fmt.Fprintf(w, "    %-25s\t₹%s\t%d txns\n", name, formatPaisa(stats.Amount), stats.Count)
		}
		w.Flush()
	}

	fmt.Println()
	return nil
}

// formatPaisa converts paisa (int64) to a formatted rupee string.
func formatPaisa(paisa int64) string {
	rupees := float64(paisa) / 100.0
	// Indian number formatting (lakhs, crores)
	if rupees >= 10000000 {
		return fmt.Sprintf("%.2f Cr", rupees/10000000)
	}
	if rupees >= 100000 {
		return fmt.Sprintf("%.2f L", rupees/100000)
	}
	// Simple comma formatting for smaller amounts
	whole := paisa / 100
	frac := paisa % 100
	s := fmt.Sprintf("%d", whole)

	// Add commas (Indian style: 1,23,456)
	if len(s) > 3 {
		result := s[len(s)-3:]
		s = s[:len(s)-3]
		for len(s) > 2 {
			result = s[len(s)-2:] + "," + result
			s = s[:len(s)-2]
		}
		if len(s) > 0 {
			result = s + "," + result
		}
		return fmt.Sprintf("%s.%02d", result, frac)
	}
	return fmt.Sprintf("%s.%02d", s, frac)
}
