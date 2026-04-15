package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/yash/personal-cfo/internal/db"
)

var (
	goalTarget   float64
	goalDeadline string
	goalPriority int
)

var goalsCmd = &cobra.Command{
	Use:   "goals",
	Short: "Show financial goals and progress",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := db.Open(dbPath)
		if err != nil {
			return err
		}
		defer store.Close()

		goals, err := store.GetGoals()
		if err != nil {
			return err
		}

		if len(goals) == 0 {
			fmt.Println("No goals set. Use `cfo goals add` to create one.")
			return nil
		}

		fmt.Println()
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "  GOAL\tPROGRESS\tTARGET\tDEADLINE\tSTATUS\n")
		fmt.Fprintf(w, "  ────\t────────\t──────\t────────\t──────\n")
		for _, g := range goals {
			pct := float64(0)
			if g.TargetPaisa > 0 {
				pct = float64(g.CurrentPaisa) / float64(g.TargetPaisa) * 100
			}
			fmt.Fprintf(w, "  %-20s\t₹%s (%.0f%%)\t₹%s\t%s\t%s\n",
				g.Name,
				formatPaisa(g.CurrentPaisa), pct,
				formatPaisa(g.TargetPaisa),
				g.Deadline, g.Status)
		}
		w.Flush()
		fmt.Println()
		return nil
	},
}

var goalsAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new financial goal",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := db.Open(dbPath)
		if err != nil {
			return err
		}
		defer store.Close()

		targetPaisa := int64(goalTarget * 100)

		err = store.AddGoal(db.Goal{
			Name:        args[0],
			TargetPaisa: targetPaisa,
			Deadline:    goalDeadline,
			Priority:    goalPriority,
			Status:      "active",
		})
		if err != nil {
			return err
		}

		fmt.Printf("✓ Goal added: %s (target ₹%s", args[0], formatPaisa(targetPaisa))
		if goalDeadline != "" {
			fmt.Printf(", deadline %s", goalDeadline)
		}
		fmt.Println(")")
		return nil
	},
}

func init() {
	goalsAddCmd.Flags().Float64VarP(&goalTarget, "target", "t", 0, "target amount in rupees")
	goalsAddCmd.Flags().StringVarP(&goalDeadline, "deadline", "d", "", "deadline (YYYY-MM-DD)")
	goalsAddCmd.Flags().IntVarP(&goalPriority, "priority", "p", 1, "priority (1=highest)")
	goalsAddCmd.MarkFlagRequired("target")

	goalsCmd.AddCommand(goalsAddCmd)
	rootCmd.AddCommand(goalsCmd)
}
