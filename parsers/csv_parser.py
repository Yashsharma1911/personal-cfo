#!/usr/bin/env python3
"""Parse CSV files from CRED, banks, etc. and output JSON to stdout.

Usage:
    python3 csv_parser.py <file> --source cred [--account "HDFC CC"]

Output: JSON array of transaction objects to stdout.
Errors go to stderr.
"""

import argparse
import csv
import json
import sys
from pathlib import Path


def detect_columns(headers: list[str]) -> dict:
    """Auto-detect column mapping from header names."""
    mapping = {}
    lower = [h.lower().strip() for h in headers]

    date_candidates = ["date", "transaction date", "txn date", "posting date", "value date"]
    for c in date_candidates:
        if c in lower:
            mapping["date"] = headers[lower.index(c)]
            break

    amount_candidates = ["amount", "transaction amount", "txn amount", "amount (inr)", "amount(inr)", "value"]
    for c in amount_candidates:
        if c in lower:
            mapping["amount"] = headers[lower.index(c)]
            break

    merchant_candidates = ["description", "narration", "particulars", "merchant",
                           "transaction details", "details", "remarks"]
    for c in merchant_candidates:
        if c in lower:
            mapping["merchant"] = headers[lower.index(c)]
            break

    type_candidates = ["type", "transaction type", "txn type", "dr/cr", "debit/credit"]
    for c in type_candidates:
        if c in lower:
            mapping["type"] = headers[lower.index(c)]
            break

    category_candidates = ["category", "expense category", "tag"]
    for c in category_candidates:
        if c in lower:
            mapping["category"] = headers[lower.index(c)]
            break

    account_candidates = ["card", "account", "card/account", "account number"]
    for c in account_candidates:
        if c in lower:
            mapping["account"] = headers[lower.index(c)]
            break

    # Separate debit/credit columns (HDFC, SBI format)
    for c in ["debit", "withdrawal", "debit amount"]:
        if c in lower:
            mapping["debit_col"] = headers[lower.index(c)]
    for c in ["credit", "deposit", "credit amount"]:
        if c in lower:
            mapping["credit_col"] = headers[lower.index(c)]

    return mapping


def clean_amount(raw: str) -> float:
    """Parse amount strings like '₹1,234.56' or '-450.00'."""
    cleaned = raw.replace("₹", "").replace(",", "").replace(" ", "").strip()
    if not cleaned or cleaned == "-":
        return 0.0
    return float(cleaned)


def infer_type(amount: float, raw_type: str | None, row: dict, mapping: dict) -> str:
    """Determine if debit or credit."""
    if raw_type:
        t = raw_type.lower().strip()
        if t in ("debit", "dr", "dr.", "expense", "payment"):
            return "debit"
        if t in ("credit", "cr", "cr.", "income", "refund"):
            return "credit"

    if "debit_col" in mapping and "credit_col" in mapping:
        debit_val = row.get(mapping["debit_col"], "").strip()
        credit_val = row.get(mapping["credit_col"], "").strip()
        if debit_val and debit_val not in ("0", "0.00", ""):
            return "debit"
        if credit_val and credit_val not in ("0", "0.00", ""):
            return "credit"

    return "credit" if amount < 0 else "debit"


def parse_csv(file_path: str, source: str, account: str | None) -> list[dict]:
    """Parse a CSV and return list of transaction dicts."""
    with open(file_path, "r", encoding="utf-8-sig") as f:
        lines = f.readlines()

    # Find header row (skip bank preamble lines)
    header_idx = 0
    for i, line in enumerate(lines):
        if line.count(",") >= 2 and any(
            kw in line.lower()
            for kw in ["date", "amount", "description", "narration", "transaction"]
        ):
            header_idx = i
            break

    reader = csv.DictReader(lines[header_idx:])
    headers = reader.fieldnames or []

    if not headers:
        print(f"Error: no headers detected in {file_path}", file=sys.stderr)
        return []

    mapping = detect_columns(headers)

    if "date" not in mapping:
        print(f"Error: no date column found. Headers: {headers}", file=sys.stderr)
        return []
    if "amount" not in mapping and "debit_col" not in mapping:
        print(f"Error: no amount column found. Headers: {headers}", file=sys.stderr)
        return []

    transactions = []
    for row in reader:
        try:
            date = row.get(mapping.get("date", ""), "").strip()
            if not date:
                continue

            if "amount" in mapping:
                raw_amount = row.get(mapping["amount"], "").strip()
                if not raw_amount:
                    continue
                amount = abs(clean_amount(raw_amount))
            elif "debit_col" in mapping:
                debit_val = row.get(mapping["debit_col"], "").strip()
                credit_val = row.get(mapping.get("credit_col", ""), "").strip()
                if debit_val and debit_val not in ("0", "0.00", ""):
                    amount = abs(clean_amount(debit_val))
                elif credit_val and credit_val not in ("0", "0.00", ""):
                    amount = abs(clean_amount(credit_val))
                else:
                    continue
            else:
                continue

            if amount == 0:
                continue

            merchant = row.get(mapping.get("merchant", ""), "").strip()
            raw_type = row.get(mapping.get("type", ""), "")
            tx_type = infer_type(amount, raw_type, row, mapping)
            category = row.get(mapping.get("category", ""), "").strip() or None
            tx_account = row.get(mapping.get("account", ""), "").strip() or account or source

            transactions.append({
                "date": date,
                "amount": round(amount, 2),
                "type": tx_type,
                "category": category,
                "merchant": merchant,
                "account": tx_account,
                "raw_description": merchant,
            })
        except (ValueError, KeyError) as e:
            print(f"Warning: skipping row: {e}", file=sys.stderr)
            continue

    return transactions


def main():
    parser = argparse.ArgumentParser(description="Parse CSV to JSON transactions")
    parser.add_argument("file", help="CSV file path")
    parser.add_argument("--source", required=True, help="Data source name")
    parser.add_argument("--account", default=None, help="Account name override")
    args = parser.parse_args()

    if not Path(args.file).exists():
        print(f"Error: file not found: {args.file}", file=sys.stderr)
        sys.exit(1)

    transactions = parse_csv(args.file, args.source, args.account)
    json.dump(transactions, sys.stdout, indent=2)


if __name__ == "__main__":
    main()
