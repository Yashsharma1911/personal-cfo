package cmd

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yash/personal-cfo/internal/db"
)

// ParserTransaction is the JSON format output by Python parsers.
type ParserTransaction struct {
	Date           string  `json:"date"`
	Amount         float64 `json:"amount"`
	Type           string  `json:"type"`
	Category       string  `json:"category,omitempty"`
	Merchant       string  `json:"merchant,omitempty"`
	Account        string  `json:"account,omitempty"`
	RawDescription string  `json:"raw_description,omitempty"`
}

var (
	ingestSource  string
	ingestAccount string
	ingestDryRun  bool
)

var ingestCmd = &cobra.Command{
	Use:   "ingest <file-or-directory>",
	Short: "Ingest transaction data from CSV or PDF",
	Long: `Ingest calls the appropriate Python parser to extract transactions
from the given file, then inserts them into the database.

Supported formats: .csv, .pdf
Sources: cred, hdfc, icici, sbi, gpay, manual`,
	Args: cobra.ExactArgs(1),
	RunE: runIngest,
}

func init() {
	ingestCmd.Flags().StringVarP(&ingestSource, "source", "s", "", "data source (cred, hdfc, icici, sbi, gpay, manual)")
	ingestCmd.Flags().StringVarP(&ingestAccount, "account", "a", "", "account name (auto-detected if possible)")
	ingestCmd.Flags().BoolVar(&ingestDryRun, "dry-run", false, "preview without inserting")
	ingestCmd.MarkFlagRequired("source")
	rootCmd.AddCommand(ingestCmd)
}

func runIngest(cmd *cobra.Command, args []string) error {
	path := args[0]

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot access %s: %w", path, err)
	}

	// If directory, find all CSV/PDF files
	var files []string
	if info.IsDir() {
		entries, _ := os.ReadDir(path)
		for _, e := range entries {
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if ext == ".csv" || ext == ".pdf" {
				files = append(files, filepath.Join(path, e.Name()))
			}
		}
		if len(files) == 0 {
			return fmt.Errorf("no CSV or PDF files found in %s", path)
		}
	} else {
		files = []string{path}
	}

	totalInserted := 0
	totalSkipped := 0

	for _, f := range files {
		inserted, skipped, err := ingestFile(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠ Error processing %s: %v\n", filepath.Base(f), err)
			continue
		}
		totalInserted += inserted
		totalSkipped += skipped
		fmt.Printf("  %s: %d inserted, %d duplicates skipped\n", filepath.Base(f), inserted, skipped)
	}

	fmt.Printf("\n✓ Total: %d inserted, %d skipped\n", totalInserted, totalSkipped)
	return nil
}

func ingestFile(filePath string) (inserted, skipped int, err error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	// Determine which Python parser to call
	var parserScript string
	switch ext {
	case ".csv":
		parserScript = "csv_parser.py"
	case ".pdf":
		parserScript = "pdf_parser.py"
	default:
		return 0, 0, fmt.Errorf("unsupported file type: %s", ext)
	}

	// Find the parsers directory (relative to the binary or cwd)
	parsersDir := findParsersDir()
	scriptPath := filepath.Join(parsersDir, parserScript)

	// Call Python parser
	pyCmd := exec.Command("python3", scriptPath, filePath, "--source", ingestSource)
	if ingestAccount != "" {
		pyCmd.Args = append(pyCmd.Args, "--account", ingestAccount)
	}
	pyCmd.Stderr = os.Stderr

	output, err := pyCmd.Output()
	if err != nil {
		return 0, 0, fmt.Errorf("parser failed: %w", err)
	}

	// Parse JSON output
	var transactions []ParserTransaction
	if err := json.Unmarshal(output, &transactions); err != nil {
		return 0, 0, fmt.Errorf("failed to parse JSON from parser: %w", err)
	}

	if ingestDryRun {
		fmt.Printf("\n  Preview: %d transactions from %s\n", len(transactions), filepath.Base(filePath))
		for i, tx := range transactions {
			if i >= 15 {
				fmt.Printf("  ... and %d more\n", len(transactions)-15)
				break
			}
			fmt.Printf("  %s | ₹%.2f | %-6s | %s | %s\n",
				tx.Date, tx.Amount, tx.Type, tx.Merchant, tx.Account)
		}
		return 0, 0, nil
	}

	// Insert into database
	store, err := db.Open(dbPath)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to open database: %w", err)
	}
	defer store.Close()

	for _, tx := range transactions {
		amountPaisa := int64(math.Round(tx.Amount * 100))
		hash := makeHash(tx.Date, tx.Amount, tx.Merchant, tx.Account)

		account := tx.Account
		if account == "" {
			account = ingestSource
		}

		err := store.InsertTransaction(db.Transaction{
			Date:           tx.Date,
			AmountPaisa:    amountPaisa,
			Type:           tx.Type,
			Category:       tx.Category,
			Merchant:       tx.Merchant,
			Account:        account,
			Source:          ingestSource,
			RawDescription: tx.RawDescription,
			Hash:           hash,
		})
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint") {
				skipped++
			} else {
				fmt.Fprintf(os.Stderr, "  insert error: %v\n", err)
			}
			continue
		}
		inserted++
	}

	return inserted, skipped, nil
}

func makeHash(date string, amount float64, merchant, account string) string {
	raw := fmt.Sprintf("%s|%.2f|%s|%s", date, amount,
		strings.ToLower(strings.TrimSpace(merchant)),
		strings.ToLower(strings.TrimSpace(account)))
	h := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", h[:8])
}

func findParsersDir() string {
	// Check relative to current working directory first
	candidates := []string{
		"parsers",
		filepath.Join("..", "parsers"),
	}

	// Check relative to executable
	if ex, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(ex), "parsers"))
	}

	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			abs, _ := filepath.Abs(dir)
			return abs
		}
	}

	return "parsers" // fallback
}
