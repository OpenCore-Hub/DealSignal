// Command askdocs-table-ingest-smoke runs P3.1a xlsx/csv → table_row extraction offline.
//
// Usage:
//
//	go run ./cmd/askdocs-table-ingest-smoke -in /path/to/file.xlsx
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/ingestion"
)

func main() {
	inPath := flag.String("in", "", "path to .xlsx or .csv")
	flag.Parse()
	if strings.TrimSpace(*inPath) == "" {
		fmt.Fprintln(os.Stderr, "usage: askdocs-table-ingest-smoke -in file.xlsx")
		os.Exit(2)
	}

	cfg := config.AskDocsFromEnv(os.Getenv("APP_ENV"))
	limits := ingestion.TableIngestLimits{
		MaxSheets:       cfg.TableMaxSheets,
		MaxRowsPerSheet: cfg.TableMaxRowsPerSheet,
		MaxRowsPerFile:  cfg.TableMaxRowsPerFile,
	}

	ext := strings.ToLower(filepath.Ext(*inPath))
	var res ingestion.TableIngestResult
	var err error
	switch ext {
	case ".xlsx":
		res, err = ingestion.ExtractTableRowsFromXLSX(*inPath, limits)
	case ".csv":
		f, openErr := os.Open(*inPath)
		if openErr != nil {
			fmt.Fprintf(os.Stderr, "open: %v\n", openErr)
			os.Exit(1)
		}
		defer f.Close()
		res, err = ingestion.ExtractTableRowsFromCSV(f, limits)
	default:
		fmt.Fprintf(os.Stderr, "unsupported extension %q (want .xlsx or .csv)\n", ext)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "extract failed: %v\n", err)
		os.Exit(1)
	}

	bySheet := map[string]int{}
	for _, r := range res.Rows {
		bySheet[r.Meta.Sheet]++
	}
	sheets := make([]string, 0, len(bySheet))
	for s := range bySheet {
		sheets = append(sheets, s)
	}
	sort.Strings(sheets)

	fmt.Printf("file: %s\n", *inPath)
	fmt.Printf("ASK_DOCS_TABLE_INGEST default (APP_ENV=%q): %v\n", os.Getenv("APP_ENV"), cfg.TableIngestEnabled)
	fmt.Printf("limits: sheets=%d rows/sheet=%d rows/file=%d\n", limits.MaxSheets, limits.MaxRowsPerSheet, limits.MaxRowsPerFile)
	fmt.Printf("table_row chunks: %d\n", len(res.Rows))
	fmt.Printf("sheets with data rows: %d\n", len(sheets))
	for _, s := range sheets {
		fmt.Printf("  - %s: %d rows\n", s, bySheet[s])
	}
	fmt.Printf("warnings: %d\n", len(res.Warnings))
	for _, w := range res.Warnings {
		fmt.Printf("  ! %s\n", w)
	}
	if len(res.Rows) == 0 {
		return
	}
	printSample("first", res.Rows[0])
	printSample("last", res.Rows[len(res.Rows)-1])
}

func printSample(label string, row ingestion.TableRowChunk) {
	q := row.Text
	runes := []rune(q)
	if len(runes) > 200 {
		q = string(runes[:200]) + "…"
	}
	fmt.Printf("sample[%s]: sheet=%q row=%d headers=%d quote=%q\n",
		label, row.Meta.Sheet, row.Meta.Row, len(row.Meta.Headers), q)
}
