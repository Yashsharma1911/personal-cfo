# CLAUDE.md — Context for Claude Code

## Project overview
Personal CFO: A local-first CLI tool for personal finance intelligence.
- **Go** — Core CLI, SQLite operations, Claude API calls, analysis, reports, goal tracking
- **Python** — CSV/PDF parsing and data extraction (called by Go as subprocess)

Python parsers output JSON to stdout. Go CLI reads that JSON and inserts into SQLite.
This keeps the boundary clean: Python only touches files, Go owns the database and AI.

Read PROJECT_SPEC.md for full architecture, schema, and phase plan.

## Project structure
```
personal-cfo/
├── go.mod / go.sum              # Go module
├── main.go                      # Entry point
├── cmd/                         # Cobra CLI commands
│   ├── root.go
│   ├── init.go
│   ├── ingest.go
│   ├── summary.go
│   ├── ask.go
│   ├── categorize.go
│   ├── accounts.go
│   └── goals.go
├── internal/
│   ├── db/
│   │   ├── schema.go            # SQLite schema + init
│   │   └── queries.go           # Query helpers
│   ├── analysis/
│   │   ├── chat.go              # Claude API — natural language queries
│   │   └── categorizer.go       # Claude API — transaction categorization
│   └── output/
│       └── report.go            # Markdown/terminal report generation
├── parsers/                     # Python data extraction
│   ├── requirements.txt
│   ├── csv_parser.py            # Parse any bank/CRED CSV → JSON
│   ├── pdf_parser.py            # Extract transactions from PDF statements
│   └── normalizer.py            # LLM-assisted normalization for weird formats
├── data/
│   └── raw/                     # Drop CSV/PDF files here
├── templates/                   # Prompt templates for Claude API
└── scripts/
    └── setup.sh                 # One-time setup (pip install, go build)
```

## Data flow
```
CSV/PDF file
    → Python parser (subprocess) 
    → JSON to stdout
    → Go reads JSON
    → Dedup by hash
    → Insert into SQLite
    → Go queries DB
    → Claude API for analysis
    → Terminal output / markdown report
```

## Tech decisions
- Go 1.22+ with Cobra CLI
- modernc.org/sqlite (pure Go SQLite — no CGO needed)
- github.com/anthropics/anthropic-sdk-go for Claude API
- Python 3.11+ only for parsers/ directory (pdfplumber, pandas)
- All amounts stored as integers (paisa) to avoid float issues: ₹450.50 = 45050
- JSON interchange format between Python and Go

## Go conventions
- Use standard library where possible
- Errors returned, not panicked
- Context passed through for cancellation
- Table-driven tests
- No unnecessary interfaces — concrete types first

## Python conventions (parsers only)
- Scripts are standalone — no importing between them
- Output is JSON array to stdout, errors to stderr
- Exit code 0 = success, 1 = error
- Keep dependencies minimal: pandas, pdfplumber, that's it

## Running
```bash
# One-time setup
./scripts/setup.sh

# Or manually:
cd parsers && pip install -r requirements.txt && cd ..
go build -o cfo .

# Usage
./cfo init
./cfo ingest data/raw/cred_march.csv --source cred
./cfo summary --month march
./cfo categorize
./cfo ask "how much did I spend on food?"
./cfo accounts
./cfo goals add "Emergency fund" --target 300000
```

## Current status
Project scaffold created. Start with: schema, CSV parser, ingest command, summary command.
