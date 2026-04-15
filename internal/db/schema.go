package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const schemaSQL = `
CREATE TABLE IF NOT EXISTS transactions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    date TEXT NOT NULL,
    amount_paisa INTEGER NOT NULL,
    type TEXT NOT NULL CHECK(type IN ('debit', 'credit')),
    category TEXT,
    subcategory TEXT,
    merchant TEXT,
    account TEXT NOT NULL,
    source TEXT NOT NULL,
    raw_description TEXT,
    notes TEXT,
    is_recurring INTEGER DEFAULT 0,
    tags TEXT,
    hash TEXT UNIQUE,
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS accounts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    type TEXT NOT NULL,
    bank TEXT,
    last_known_balance_paisa INTEGER,
    updated_at TEXT
);

CREATE TABLE IF NOT EXISTS goals (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    target_paisa INTEGER,
    current_paisa INTEGER DEFAULT 0,
    deadline TEXT,
    priority INTEGER DEFAULT 1,
    status TEXT DEFAULT 'active',
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS income (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    date TEXT NOT NULL,
    amount_paisa INTEGER NOT NULL,
    type TEXT NOT NULL,
    source TEXT,
    notes TEXT,
    tax_deducted_paisa INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS quarterly_reviews (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    quarter TEXT NOT NULL,
    total_income_paisa INTEGER,
    total_expenses_paisa INTEGER,
    savings_rate REAL,
    report_markdown TEXT,
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_tx_date ON transactions(date);
CREATE INDEX IF NOT EXISTS idx_tx_category ON transactions(category);
CREATE INDEX IF NOT EXISTS idx_tx_account ON transactions(account);
CREATE INDEX IF NOT EXISTS idx_tx_hash ON transactions(hash);
`

// Categories defines standard spending categories for Indian context.
var Categories = []string{
	"food_dining", "groceries", "transport", "rent", "utilities",
	"subscriptions", "shopping", "medical", "travel", "education",
	"investment", "insurance", "personal_care", "entertainment",
	"transfers", "emi", "misc",
}

// Store wraps a SQLite database connection.
type Store struct {
	db *sql.DB
}

// Init creates the database file and schema.
func Init(dbPath string) error {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return err
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}

	return nil
}

// Open opens an existing database.
func Open(dbPath string) (*Store, error) {
	// Auto-init if database doesn't exist
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		if err := Init(dbPath); err != nil {
			return nil, err
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA foreign_keys=ON")

	return &Store{db: db}, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying sql.DB for direct queries.
func (s *Store) DB() *sql.DB {
	return s.db
}
