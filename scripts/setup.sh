#!/bin/bash
set -e

echo "=== Personal CFO Setup ==="

# Check Go
if ! command -v go &> /dev/null; then
    echo "❌ Go not found. Install from https://go.dev/dl/"
    exit 1
fi
echo "✓ Go $(go version | awk '{print $3}')"

# Check Python
if ! command -v python3 &> /dev/null; then
    echo "❌ Python3 not found. Install from https://python.org"
    exit 1
fi
echo "✓ Python $(python3 --version | awk '{print $2}')"

# Install Python dependencies
echo "Installing Python parser dependencies..."
pip3 install -r parsers/requirements.txt --quiet
echo "✓ Python dependencies installed"

# Build Go binary
echo "Building cfo binary..."
go mod tidy
go build -o cfo .
echo "✓ Built ./cfo"

# Initialize database
./cfo init
echo ""
echo "=== Setup complete! ==="
echo ""
echo "Quick start:"
echo "  ./cfo ingest data/raw/sample_cred_march_2026.csv --source cred"
echo "  ./cfo summary --month march"
echo "  ./cfo accounts"
echo "  ./cfo categorize"
echo "  ANTHROPIC_API_KEY=sk-... ./cfo ask 'where is my money going?'"
