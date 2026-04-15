#!/usr/bin/env python3
"""Parse PDF bank statements and output JSON to stdout.

Usage:
    python3 pdf_parser.py <file.pdf> --source hdfc [--account "HDFC Savings"]

This is a Phase 2 feature. Currently handles:
- HDFC bank statements
- ICICI bank statements
- Generic tabular PDF statements

Output: JSON array of transaction objects to stdout.
"""

import argparse
import json
import sys
from pathlib import Path

try:
    import pdfplumber
except ImportError:
    print("Error: pdfplumber not installed. Run: pip install pdfplumber", file=sys.stderr)
    sys.exit(1)


def parse_pdf(file_path: str, source: str, account: str | None) -> list[dict]:
    """Extract transactions from a PDF bank statement."""
    transactions = []

    with pdfplumber.open(file_path) as pdf:
        for page in pdf.pages:
            # Try extracting tables first (works for most bank statements)
            tables = page.extract_tables()
            for table in tables:
                if not table or len(table) < 2:
                    continue

                # First row is likely headers
                headers = [str(h).lower().strip() if h else "" for h in table[0]]

                # Check if this looks like a transaction table
                has_date = any("date" in h for h in headers)
                has_amount = any(h in ("amount", "debit", "credit", "withdrawal", "deposit") for h in headers)

                if not (has_date and has_amount):
                    continue

                for row in table[1:]:
                    if not row or not any(row):
                        continue
                    # TODO: Map columns based on detected headers
                    # This needs per-bank customization
                    print(f"Found row: {row}", file=sys.stderr)

            # Fallback: extract raw text for LLM-assisted parsing
            text = page.extract_text()
            if text and not tables:
                print(f"Page {page.page_number}: no tables found, raw text available ({len(text)} chars)",
                      file=sys.stderr)

    if not transactions:
        print(f"Warning: could not extract transactions from {file_path}. "
              "PDF parsing is still in development — try exporting as CSV instead.",
              file=sys.stderr)

    return transactions


def main():
    parser = argparse.ArgumentParser(description="Parse PDF statement to JSON transactions")
    parser.add_argument("file", help="PDF file path")
    parser.add_argument("--source", required=True, help="Data source name")
    parser.add_argument("--account", default=None, help="Account name override")
    args = parser.parse_args()

    if not Path(args.file).exists():
        print(f"Error: file not found: {args.file}", file=sys.stderr)
        sys.exit(1)

    transactions = parse_pdf(args.file, args.source, args.account)
    json.dump(transactions, sys.stdout, indent=2)


if __name__ == "__main__":
    main()
