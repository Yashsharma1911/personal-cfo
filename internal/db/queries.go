package db

import (
	"fmt"
	"strings"
)

// Transaction represents a single financial transaction.
type Transaction struct {
	ID             int64
	Date           string
	AmountPaisa    int64
	Type           string // "debit" or "credit"
	Category       string
	Merchant       string
	Account        string
	Source         string
	RawDescription string
	Hash           string
}

// AccountStats holds aggregated stats for an account.
type AccountStats struct {
	Name         string
	TxCount      int
	TotalDebits  int64
	TotalCredits int64
	LastDate     string
}

// Goal represents a financial goal.
type Goal struct {
	ID           int64
	Name         string
	TargetPaisa  int64
	CurrentPaisa int64
	Deadline     string
	Priority     int
	Status       string
}

// Summary holds aggregated financial data for a period.
type Summary struct {
	TotalExpenses int64
	TotalIncome   int64
	TxCount       int
	ByCategory    map[string]int64
	ByAccount     map[string]AccountSummary
}

// AccountSummary holds per-account totals within a summary.
type AccountSummary struct {
	Amount int64
	Count  int
}

// InsertTransaction inserts a transaction, returning error on duplicate hash.
func (s *Store) InsertTransaction(tx Transaction) error {
	_, err := s.db.Exec(`
		INSERT INTO transactions (date, amount_paisa, type, category, merchant, account, source, raw_description, hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tx.Date, tx.AmountPaisa, tx.Type, tx.Category, tx.Merchant,
		tx.Account, tx.Source, tx.RawDescription, tx.Hash)
	return err
}

// GetSummary returns spending summary for a given period.
func (s *Store) GetSummary(month, quarter string) (*Summary, error) {
	where, params := buildDateFilter(month, quarter)

	summary := &Summary{
		ByCategory: make(map[string]int64),
		ByAccount:  make(map[string]AccountSummary),
	}

	// Total expenses
	row := s.db.QueryRow(
		fmt.Sprintf("SELECT COALESCE(SUM(amount_paisa),0) FROM transactions WHERE type='debit' AND %s", where),
		params...)
	row.Scan(&summary.TotalExpenses)

	// Total income
	row = s.db.QueryRow(
		fmt.Sprintf("SELECT COALESCE(SUM(amount_paisa),0) FROM transactions WHERE type='credit' AND %s", where),
		params...)
	row.Scan(&summary.TotalIncome)

	// Count
	row = s.db.QueryRow(
		fmt.Sprintf("SELECT COUNT(*) FROM transactions WHERE %s", where),
		params...)
	row.Scan(&summary.TxCount)

	// By category
	rows, err := s.db.Query(
		fmt.Sprintf(`SELECT COALESCE(category,''), SUM(amount_paisa) 
			FROM transactions WHERE type='debit' AND %s 
			GROUP BY category ORDER BY SUM(amount_paisa) DESC`, where),
		params...)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var cat string
			var amount int64
			rows.Scan(&cat, &amount)
			summary.ByCategory[cat] = amount
		}
	}

	// By account
	rows, err = s.db.Query(
		fmt.Sprintf(`SELECT account, SUM(amount_paisa), COUNT(*) 
			FROM transactions WHERE type='debit' AND %s 
			GROUP BY account ORDER BY SUM(amount_paisa) DESC`, where),
		params...)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name string
			var amount int64
			var count int
			rows.Scan(&name, &amount, &count)
			summary.ByAccount[name] = AccountSummary{Amount: amount, Count: count}
		}
	}

	return summary, nil
}

// GetAccountStats returns stats for all accounts.
func (s *Store) GetAccountStats() ([]AccountStats, error) {
	rows, err := s.db.Query(`
		SELECT 
			account,
			COUNT(*),
			COALESCE(SUM(CASE WHEN type='debit' THEN amount_paisa ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN type='credit' THEN amount_paisa ELSE 0 END), 0),
			MAX(date)
		FROM transactions GROUP BY account ORDER BY COUNT(*) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []AccountStats
	for rows.Next() {
		var a AccountStats
		if err := rows.Scan(&a.Name, &a.TxCount, &a.TotalDebits, &a.TotalCredits, &a.LastDate); err != nil {
			continue
		}
		stats = append(stats, a)
	}
	return stats, nil
}

// GetUncategorized returns transactions without a category.
func (s *Store) GetUncategorized(limit int) ([]Transaction, error) {
	rows, err := s.db.Query(`
		SELECT id, date, amount_paisa, type, merchant, account, raw_description
		FROM transactions WHERE category IS NULL OR category = ''
		ORDER BY date DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txns []Transaction
	for rows.Next() {
		var tx Transaction
		if err := rows.Scan(&tx.ID, &tx.Date, &tx.AmountPaisa, &tx.Type,
			&tx.Merchant, &tx.Account, &tx.RawDescription); err != nil {
			continue
		}
		txns = append(txns, tx)
	}
	return txns, nil
}

// UpdateCategory sets the category for a transaction.
func (s *Store) UpdateCategory(id int64, category string) error {
	_, err := s.db.Exec("UPDATE transactions SET category = ? WHERE id = ?", category, id)
	return err
}

// GetRecentTransactions returns recent transactions for AI analysis.
func (s *Store) GetRecentTransactions(limit int) ([]Transaction, error) {
	rows, err := s.db.Query(`
		SELECT id, date, amount_paisa, type, category, merchant, account
		FROM transactions ORDER BY date DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txns []Transaction
	for rows.Next() {
		var tx Transaction
		var cat, merchant *string
		if err := rows.Scan(&tx.ID, &tx.Date, &tx.AmountPaisa, &tx.Type,
			&cat, &merchant, &tx.Account); err != nil {
			continue
		}
		if cat != nil {
			tx.Category = *cat
		}
		if merchant != nil {
			tx.Merchant = *merchant
		}
		txns = append(txns, tx)
	}
	return txns, nil
}

// GetGoals returns all active goals.
func (s *Store) GetGoals() ([]Goal, error) {
	rows, err := s.db.Query(`
		SELECT id, name, COALESCE(target_paisa,0), COALESCE(current_paisa,0),
			COALESCE(deadline,''), priority, status
		FROM goals WHERE status = 'active' ORDER BY priority ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var goals []Goal
	for rows.Next() {
		var g Goal
		if err := rows.Scan(&g.ID, &g.Name, &g.TargetPaisa, &g.CurrentPaisa,
			&g.Deadline, &g.Priority, &g.Status); err != nil {
			continue
		}
		goals = append(goals, g)
	}
	return goals, nil
}

// AddGoal inserts a new goal.
func (s *Store) AddGoal(g Goal) error {
	_, err := s.db.Exec(`
		INSERT INTO goals (name, target_paisa, deadline, priority, status)
		VALUES (?, ?, ?, ?, ?)`,
		g.Name, g.TargetPaisa, g.Deadline, g.Priority, g.Status)
	return err
}

// buildDateFilter creates SQL WHERE clause for date filtering.
func buildDateFilter(month, quarter string) (string, []interface{}) {
	if quarter != "" {
		parts := strings.Split(quarter, "-")
		if len(parts) == 2 {
			q := strings.TrimPrefix(parts[0], "Q")
			year := parts[1]
			monthRanges := map[string][2]string{
				"1": {"01", "03"}, "2": {"04", "06"},
				"3": {"07", "09"}, "4": {"10", "12"},
			}
			if r, ok := monthRanges[q]; ok {
				return "date >= ? AND date <= ?",
					[]interface{}{fmt.Sprintf("%s-%s-01", year, r[0]), fmt.Sprintf("%s-%s-31", year, r[1])}
			}
		}
	}

	if month != "" {
		monthMap := map[string]string{
			"january": "01", "february": "02", "march": "03", "april": "04",
			"may": "05", "june": "06", "july": "07", "august": "08",
			"september": "09", "october": "10", "november": "11", "december": "12",
		}
		m := month
		if mapped, ok := monthMap[strings.ToLower(month)]; ok {
			m = mapped
		}
		if len(m) == 1 {
			m = "0" + m
		}
		return "strftime('%m', date) = ?", []interface{}{m}
	}

	return "1=1", nil
}
