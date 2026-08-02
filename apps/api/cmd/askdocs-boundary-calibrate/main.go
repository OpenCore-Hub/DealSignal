// Command askdocs-boundary-calibrate runs offline D16 boundary-band calibration.
//
// Usage:
//
//	go run ./cmd/askdocs-boundary-calibrate -in path/to/snapshots.json [-out report.md]
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/assistant/coverage"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
)

func main() {
	inPath := flag.String("in", "", "path to CalibrateInput JSON")
	outPath := flag.String("out", "", "optional markdown report path (default stdout)")
	flag.Parse()
	if strings.TrimSpace(*inPath) == "" {
		fmt.Fprintln(os.Stderr, "usage: askdocs-boundary-calibrate -in snapshots.json [-out report.md]")
		os.Exit(2)
	}
	raw, err := os.ReadFile(*inPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read input: %v\n", err)
		os.Exit(1)
	}
	in, err := coverage.ParseCalibrateInputJSON(raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse input: %v\n", err)
		os.Exit(1)
	}
	opts := coverage.OptionsFromConfig(config.AskDocsFromEnv(os.Getenv("APP_ENV")))
	rep, err := coverage.RunCalibration(in, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "calibrate: %v\n", err)
		os.Exit(1)
	}
	md := coverage.FormatCalibrateReportMarkdown(rep)
	if strings.TrimSpace(*outPath) == "" {
		fmt.Print(md)
		return
	}
	if err := os.WriteFile(*outPath, []byte(md), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write report: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", *outPath)
}
