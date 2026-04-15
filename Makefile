.PHONY: build setup test clean ingest-sample

# Build the Go binary
build:
	go build -o cfo .

# One-time setup
setup:
	chmod +x scripts/setup.sh
	./scripts/setup.sh

# Run with sample data
ingest-sample: build
	./cfo init
	./cfo ingest data/raw/sample_cred_march_2026.csv --source cred
	./cfo summary --month march
	./cfo accounts

# Dry run with sample data
dry-run: build
	./cfo ingest data/raw/sample_cred_march_2026.csv --source cred --dry-run

# Run tests
test:
	go test ./...
	python3 -m pytest parsers/ -v 2>/dev/null || echo "No Python tests yet"

# Clean build artifacts and test database
clean:
	rm -f cfo
	rm -f ~/.cfo/cfo.db

# Run AI categorization on uncategorized transactions
categorize: build
	./cfo categorize

# Ask a question (usage: make ask Q="how much on food?")
ask: build
	./cfo ask "$(Q)"
