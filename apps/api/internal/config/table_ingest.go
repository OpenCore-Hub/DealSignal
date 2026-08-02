package config

import (
	"os"
	"strconv"
	"strings"
)

const (
	defaultTableMaxSheets       = 20
	defaultTableMaxRowsPerSheet = 5000
	defaultTableMaxRowsPerFile  = 20000
)

// TableIngestConfig holds TABLE_INGEST_* limits for spreadsheet → table_row chunking (P3.1a).
type TableIngestConfig struct {
	Enabled         bool
	MaxSheets       int
	MaxRowsPerSheet int
	MaxRowsPerFile  int
}

// TableIngestFromEnv parses TABLE_INGEST_* env vars.
// Enabled defaults to on in non-production, off in production|prod when unset.
func TableIngestFromEnv(appEnv string) TableIngestConfig {
	env := strings.ToLower(strings.TrimSpace(appEnv))
	prod := env == "production" || env == "prod"

	o := TableIngestConfig{
		Enabled:         !prod,
		MaxSheets:       defaultTableMaxSheets,
		MaxRowsPerSheet: defaultTableMaxRowsPerSheet,
		MaxRowsPerFile:  defaultTableMaxRowsPerFile,
	}

	if v := strings.TrimSpace(os.Getenv("TABLE_INGEST_ENABLED")); v != "" {
		o.Enabled = tableIngestEnvTruthy(v)
	}

	if v := strings.TrimSpace(os.Getenv("TABLE_MAX_SHEETS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			o.MaxSheets = n
		}
	}

	if v := strings.TrimSpace(os.Getenv("TABLE_MAX_ROWS_PER_SHEET")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			o.MaxRowsPerSheet = n
		}
	}

	if v := strings.TrimSpace(os.Getenv("TABLE_MAX_ROWS_PER_FILE")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			o.MaxRowsPerFile = n
		}
	}

	return o
}

func tableIngestEnvTruthy(v string) bool {
	return strings.EqualFold(v, "true") || v == "1" || strings.EqualFold(v, "on")
}
