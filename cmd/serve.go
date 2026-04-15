package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"github.com/yash/personal-cfo/internal/db"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the MCP server over stdio",
	Long: `Starts an MCP server over stdio that exposes your financial data
as structured tools for Claude or any MCP-compatible client.

All monetary amounts are in paisa (100 paisa = ₹1).

To add to Claude Desktop, configure ~/.claude/claude_desktop_config.json:

  {
    "mcpServers": {
      "personal-cfo": {
        "command": "/path/to/cfo",
        "args": ["serve"]
      }
    }
  }`,
	RunE: runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func runServe(_ *cobra.Command, _ []string) error {
	store, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer store.Close()

	s := mcp.NewServer(&mcp.Implementation{
		Name:    "personal-cfo",
		Version: "1.0.0",
	}, nil)

	registerTools(s, store)

	return s.Run(context.Background(), &mcp.StdioTransport{})
}

func registerTools(s *mcp.Server, store *db.Store) {
	// get_summary — spending overview, optionally scoped to a month or quarter.
	type getSummaryArgs struct {
		Month   string `json:"month,omitempty" jsonschema:"month name or two-digit number, e.g. 'march' or '03'. Omit for all-time."`
		Quarter string `json:"quarter,omitempty" jsonschema:"quarter in Q1-2026 format. Omit for all-time."`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name: "get_summary",
		Description: "Return aggregate spending: total expenses, total credits, transaction count, " +
			"breakdown by category, and breakdown by account. All amounts in paisa (100 = ₹1). " +
			"Optionally filter by month (e.g. 'march') or quarter (e.g. 'Q1-2026').",
	}, func(_ context.Context, _ *mcp.CallToolRequest, args getSummaryArgs) (*mcp.CallToolResult, any, error) {
		summary, err := store.GetSummary(args.Month, args.Quarter)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(summary), nil, nil
	})

	// get_recent_transactions — latest N transactions across all accounts.
	type getRecentArgs struct {
		Limit int `json:"limit,omitempty" jsonschema:"maximum number of transactions to return (default: 50, max: 500)"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_recent_transactions",
		Description: "List recent transactions ordered by date descending. Amounts in paisa. Includes date, type (debit/credit), category, merchant, and account.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, args getRecentArgs) (*mcp.CallToolResult, any, error) {
		limit := args.Limit
		if limit <= 0 {
			limit = 50
		}
		txns, err := store.GetRecentTransactions(limit)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(txns), nil, nil
	})

	// get_account_stats — per-account totals.
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_account_stats",
		Description: "Return transaction counts, total debits, total credits, and last activity date for each account. Amounts in paisa.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		stats, err := store.GetAccountStats()
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(stats), nil, nil
	})

	// get_uncategorized — transactions without a category, for Claude to classify.
	type getUncategorizedArgs struct {
		Limit int `json:"limit,omitempty" jsonschema:"maximum transactions to return (default: 30)"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name: "get_uncategorized",
		Description: "List transactions that have no category set. Use this to identify what needs " +
			"categorizing, then call update_category for each one. " +
			"Standard categories: food_dining, groceries, transport, rent, utilities, subscriptions, " +
			"shopping, medical, travel, education, investment, insurance, personal_care, entertainment, transfers, emi, misc.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, args getUncategorizedArgs) (*mcp.CallToolResult, any, error) {
		limit := args.Limit
		if limit <= 0 {
			limit = 30
		}
		txns, err := store.GetUncategorized(limit)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(txns), nil, nil
	})

	// get_goals — active savings/investment goals.
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_goals",
		Description: "List all active financial goals with name, target amount, current progress, deadline, and priority. Amounts in paisa.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		goals, err := store.GetGoals()
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(goals), nil, nil
	})

	// add_goal — create a new financial goal.
	type addGoalArgs struct {
		Name        string `json:"name" jsonschema:"goal name, e.g. 'Emergency Fund' or 'Goa Trip'"`
		TargetPaisa int64  `json:"target_paisa,omitempty" jsonschema:"target amount in paisa (100 paisa = ₹1); e.g. 30000000 for ₹3 lakh"`
		Deadline    string `json:"deadline,omitempty" jsonschema:"deadline in YYYY-MM or YYYY-MM-DD format"`
		Priority    int    `json:"priority,omitempty" jsonschema:"priority level where 1 is highest (default: 1)"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "add_goal",
		Description: "Create a new financial savings or investment goal to track over time.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, args addGoalArgs) (*mcp.CallToolResult, any, error) {
		priority := args.Priority
		if priority == 0 {
			priority = 1
		}
		if err := store.AddGoal(db.Goal{
			Name:        args.Name,
			TargetPaisa: args.TargetPaisa,
			Deadline:    args.Deadline,
			Priority:    priority,
			Status:      "active",
		}); err != nil {
			return nil, nil, err
		}
		return textResult(fmt.Sprintf("Goal '%s' created with target ₹%.2f", args.Name, float64(args.TargetPaisa)/100)), nil, nil
	})

	// update_category — set or correct the category of a transaction.
	type updateCategoryArgs struct {
		ID       int64  `json:"id" jsonschema:"transaction ID from get_recent_transactions or get_uncategorized"`
		Category string `json:"category" jsonschema:"one of: food_dining, groceries, transport, rent, utilities, subscriptions, shopping, medical, travel, education, investment, insurance, personal_care, entertainment, transfers, emi, misc"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name: "update_category",
		Description: "Set or correct the category of a transaction by its ID. " +
			"Valid categories: food_dining, groceries, transport, rent, utilities, subscriptions, " +
			"shopping, medical, travel, education, investment, insurance, personal_care, entertainment, transfers, emi, misc.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, args updateCategoryArgs) (*mcp.CallToolResult, any, error) {
		if err := store.UpdateCategory(args.ID, args.Category); err != nil {
			return nil, nil, err
		}
		return textResult(fmt.Sprintf("Transaction %d set to category '%s'", args.ID, args.Category)), nil, nil
	})

	// insert_transaction — manually add a transaction.
	type insertTxArgs struct {
		Date           string `json:"date" jsonschema:"transaction date in YYYY-MM-DD format"`
		AmountPaisa    int64  `json:"amount_paisa" jsonschema:"amount in paisa (100 paisa = ₹1); e.g. 45050 for ₹450.50"`
		Type           string `json:"type" jsonschema:"'debit' for an expense or outflow, 'credit' for income or a refund"`
		Category       string `json:"category,omitempty" jsonschema:"spending category (optional — can be set later with update_category)"`
		Merchant       string `json:"merchant,omitempty" jsonschema:"merchant or payee name, e.g. 'Swiggy' or 'BigBasket'"`
		Account        string `json:"account" jsonschema:"account identifier, e.g. 'HDFC CC ****1234' or 'GPay'"`
		Source         string `json:"source" jsonschema:"data source slug, e.g. 'cred', 'hdfc', 'manual'"`
		RawDescription string `json:"raw_description,omitempty" jsonschema:"original transaction description from the bank or statement"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "insert_transaction",
		Description: "Manually insert a transaction into the database. Useful for transactions not covered by CSV imports (e.g. cash, corrections).",
	}, func(_ context.Context, _ *mcp.CallToolRequest, args insertTxArgs) (*mcp.CallToolResult, any, error) {
		amountRupees := float64(args.AmountPaisa) / 100
		hash := makeHash(args.Date, amountRupees, args.Merchant, args.Account)
		if err := store.InsertTransaction(db.Transaction{
			Date:           args.Date,
			AmountPaisa:    args.AmountPaisa,
			Type:           args.Type,
			Category:       args.Category,
			Merchant:       args.Merchant,
			Account:        args.Account,
			Source:         args.Source,
			RawDescription: args.RawDescription,
			Hash:           hash,
		}); err != nil {
			return nil, nil, err
		}
		return textResult(fmt.Sprintf("Inserted: %s  ₹%.2f  %s  %s  [%s]",
			args.Date, amountRupees, args.Type, args.Merchant, args.Account)), nil, nil
	})
}

// jsonResult wraps any value as indented JSON text content.
func jsonResult(v any) *mcp.CallToolResult {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: "marshal error: " + err.Error()}},
		}
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}
}

// textResult wraps a plain string as text content.
func textResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
	}
}
