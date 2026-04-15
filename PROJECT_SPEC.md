# Personal CFO — AI-Powered Personal Finance Tracker

## What this project is

A local-first, privacy-first personal finance tool that ingests transaction data from
existing apps (CRED exports, bank statements, UPI history, Fidelity RSU data, salary slips)
and uses Claude API to generate actionable financial intelligence — quarterly reviews,
goal tracking, spending analysis, and savings advice.

**This is NOT another expense tracker.** CRED, FinArt, Walnut etc. already handle transaction
capture via SMS/email/notification parsing. This project is the **intelligence layer on top**.

## Architecture: Go + Python hybrid

- **Go**: Core CLI, database, AI analysis, report generation, goal tracking
- **Python**: CSV/PDF parsing only (called as subprocess, outputs JSON to stdout)

Why hybrid:
- Python has unmatched libraries for parsing messy Indian bank CSVs and PDFs (pdfplumber, pandas)
- Go gives a single binary, strong typing, excellent CLI (Cobra), and is the builder's preferred language
- Clean boundary: Python only reads files → outputs JSON. Go owns everything else.

## Database schema (SQLite)

All amounts stored as INTEGER in paisa (₹450.50 = 45050) to avoid float issues.

```sql
CREATE TABLE transactions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    date TEXT NOT NULL,                    -- YYYY-MM-DD
    amount_paisa INTEGER NOT NULL,         -- amount in paisa
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

CREATE TABLE accounts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    type TEXT NOT NULL,                    -- savings, credit_card, upi_wallet, investment
    bank TEXT,
    last_known_balance_paisa INTEGER,
    updated_at TEXT
);

CREATE TABLE goals (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    target_paisa INTEGER,
    current_paisa INTEGER DEFAULT 0,
    deadline TEXT,
    priority INTEGER DEFAULT 1,
    status TEXT DEFAULT 'active',
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE income (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    date TEXT NOT NULL,
    amount_paisa INTEGER NOT NULL,
    type TEXT NOT NULL,                    -- salary, rsu_vest, rsu_sale, freelance, sponsored
    source TEXT,
    notes TEXT,
    tax_deducted_paisa INTEGER DEFAULT 0
);

CREATE TABLE quarterly_reviews (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    quarter TEXT NOT NULL,
    total_income_paisa INTEGER,
    total_expenses_paisa INTEGER,
    savings_rate REAL,
    report_markdown TEXT,
    created_at TEXT DEFAULT (datetime('now'))
);
```

## Standard categories

```
food_dining, groceries, transport, rent, utilities, subscriptions,
shopping, medical, travel, education, investment, insurance,
personal_care, entertainment, transfers, emi, misc
```

## CLI commands

```bash
cfo init                                    # Create database
cfo ingest <file> --source cred             # Ingest CSV/PDF
cfo ingest data/raw/ --auto                 # Auto-detect all files
cfo summary --month march                   # Monthly summary
cfo summary --quarter Q1-2026               # Quarterly breakdown
cfo accounts                                # All accounts + stats
cfo categorize                              # AI-categorize uncategorized txns
cfo ask "how much on food vs last quarter?" # Natural language query
cfo goals                                   # Show goal progress
cfo goals add "Emergency fund" --target 300000 --deadline 2026-12
cfo review --quarter Q1-2026                # Full AI quarterly report
cfo recurring                               # Show recurring payments
```

## Python parser JSON output format

Each Python parser outputs a JSON array to stdout:

```json
[
  {
    "date": "2026-03-15",
    "amount": 450.00,
    "type": "debit",
    "category": "Food & Dining",
    "merchant": "Swiggy Order #12345",
    "account": "HDFC CC ****1234",
    "raw_description": "Swiggy Order #12345"
  }
]
```

Go reads this, converts amount to paisa, generates hash, deduplicates, and inserts.

## Phase plan

### Phase 1 (MVP)
- [x] Project scaffold (Go + Python)
- [ ] SQLite schema + init command
- [ ] Python CSV parser (CRED + generic bank format)
- [ ] Go ingest command (call Python, read JSON, insert)
- [ ] Go summary command (by month, quarter, category, account)
- [ ] Go ask command (Claude API natural language queries)

### Phase 2
- [ ] Python PDF parser (bank statements, salary slips)
- [ ] Go categorize command (Claude API batch categorization)
- [ ] Go quarterly review generation
- [ ] Goal tracking (add, update, progress)
- [ ] Recurring payment detection
- [ ] Account-level analytics

### Phase 3
- [ ] Web dashboard (Go HTTP server + htmx or React)
- [ ] Income tracking (salary + RSU)
- [ ] Tax summaries for ITR filing
- [ ] Historical trend charts

### Phase 4
- [ ] Account Aggregator API integration (via Finvu/OneMoney TSP)
- [ ] Telegram bot for quick queries
- [ ] Android companion app
