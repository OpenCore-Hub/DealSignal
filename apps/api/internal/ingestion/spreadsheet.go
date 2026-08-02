package ingestion

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/xuri/excelize/v2"
)

const (
	chunkTypeParagraph = "paragraph"
	chunkTypeTableRow  = "table_row"
)

// TableIngestLimits are soft caps for spreadsheet → table_row (P3.1a / K10).
type TableIngestLimits struct {
	MaxSheets       int
	MaxRowsPerSheet int
	MaxRowsPerFile  int
}

func (l TableIngestLimits) normalized() TableIngestLimits {
	out := l
	if out.MaxSheets <= 0 {
		out.MaxSheets = 20
	}
	if out.MaxRowsPerSheet <= 0 {
		out.MaxRowsPerSheet = 5000
	}
	if out.MaxRowsPerFile <= 0 {
		out.MaxRowsPerFile = 20000
	}
	return out
}

// TableRowMeta is stored in chunk.bbox for table_row (not a PDF bbox).
type TableRowMeta struct {
	Kind    string            `json:"kind"`
	Sheet   string            `json:"sheet"`
	Row     int               `json:"row"` // 1-based data row index (header is row 0 conceptually; data starts at 1)
	Headers []string          `json:"headers"`
	Cells   map[string]string `json:"cells"`
}

// TableRowChunk is one spreadsheet data row ready to persist as a chunk.
type TableRowChunk struct {
	Text string
	Meta TableRowMeta
	BBox []byte
}

// TableIngestResult holds parsed rows plus truncation warnings (K10).
type TableIngestResult struct {
	Rows     []TableRowChunk
	Warnings []string
}

// ExtractTableRowsFromXLSX parses an .xlsx into table_row chunks (K3/K6/K9).
func ExtractTableRowsFromXLSX(path string, limits TableIngestLimits) (TableIngestResult, error) {
	limits = limits.normalized()
	f, err := excelize.OpenFile(path)
	if err != nil {
		return TableIngestResult{}, fmt.Errorf("open xlsx: %w", err)
	}
	defer func() { _ = f.Close() }()

	sheets := f.GetSheetList()
	out := TableIngestResult{}
	if len(sheets) > limits.MaxSheets {
		out.Warnings = append(out.Warnings, fmt.Sprintf("truncated sheets: keeping first %d of %d", limits.MaxSheets, len(sheets)))
		sheets = sheets[:limits.MaxSheets]
	}

	fileRows := 0
	for _, sheet := range sheets {
		if fileRows >= limits.MaxRowsPerFile {
			out.Warnings = append(out.Warnings, fmt.Sprintf("truncated file rows at %d", limits.MaxRowsPerFile))
			break
		}
		rows, err := f.GetRows(sheet)
		if err != nil {
			return TableIngestResult{}, fmt.Errorf("sheet %q: %w", sheet, err)
		}
		parsed, warns := rowsToTableChunks(sheet, rows, limits.MaxRowsPerSheet, limits.MaxRowsPerFile-fileRows)
		out.Warnings = append(out.Warnings, warns...)
		out.Rows = append(out.Rows, parsed...)
		fileRows += len(parsed)
	}
	return out, nil
}

// ExtractTableRowsFromCSV parses a CSV into table_row chunks (single logical sheet).
func ExtractTableRowsFromCSV(r io.Reader, limits TableIngestLimits) (TableIngestResult, error) {
	limits = limits.normalized()
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true
	records, err := cr.ReadAll()
	if err != nil {
		return TableIngestResult{}, fmt.Errorf("parse csv: %w", err)
	}
	parsed, warns := rowsToTableChunks("Sheet1", records, limits.MaxRowsPerSheet, limits.MaxRowsPerFile)
	return TableIngestResult{Rows: parsed, Warnings: warns}, nil
}

func rowsToTableChunks(sheet string, rows [][]string, maxPerSheet, maxRemaining int) ([]TableRowChunk, []string) {
	var warnings []string
	if len(rows) == 0 {
		return nil, nil
	}
	headers := normalizeHeaderRow(rows[0])
	data := rows[1:]
	if len(data) > maxPerSheet {
		warnings = append(warnings, fmt.Sprintf("sheet %q: truncated rows keeping first %d of %d", sheet, maxPerSheet, len(data)))
		data = data[:maxPerSheet]
	}
	if len(data) > maxRemaining {
		warnings = append(warnings, fmt.Sprintf("sheet %q: truncated to remaining file budget %d", sheet, maxRemaining))
		data = data[:maxRemaining]
	}

	out := make([]TableRowChunk, 0, len(data))
	for i, row := range data {
		if rowEmpty(row) {
			continue
		}
		cells := map[string]string{}
		parts := make([]string, 0, len(headers))
		for c, h := range headers {
			val := ""
			if c < len(row) {
				val = strings.TrimSpace(row[c])
			}
			if h == "" {
				continue
			}
			cells[h] = val
			if val != "" {
				parts = append(parts, h+": "+val)
			}
		}
		if len(parts) == 0 {
			continue
		}
		meta := TableRowMeta{
			Kind:    chunkTypeTableRow,
			Sheet:   sheet,
			Row:     i + 1,
			Headers: headers,
			Cells:   cells,
		}
		raw, err := json.Marshal(meta)
		if err != nil {
			continue
		}
		out = append(out, TableRowChunk{
			Text: strings.Join(parts, " | "),
			Meta: meta,
			BBox: raw,
		})
	}
	return out, warnings
}

func normalizeHeaderRow(row []string) []string {
	out := make([]string, len(row))
	seen := map[string]int{}
	for i, cell := range row {
		h := strings.TrimSpace(cell)
		if h == "" {
			h = fmt.Sprintf("Col%d", i+1)
		}
		if n, ok := seen[h]; ok {
			seen[h] = n + 1
			h = fmt.Sprintf("%s_%d", h, n+1)
		} else {
			seen[h] = 1
		}
		out[i] = h
	}
	if len(out) == 0 {
		return []string{"Col1"}
	}
	return out
}

func rowEmpty(row []string) bool {
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

func isTableSourceType(sourceType string) bool {
	switch strings.ToLower(strings.TrimSpace(sourceType)) {
	case "xlsx", "csv":
		return true
	default:
		return false
	}
}
