package ingestion

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestRowsToTableChunks_HeaderAndSkipEmpty(t *testing.T) {
	rows := [][]string{
		{"Name", "ARR"},
		{"Acme", "100"},
		{"", ""},
		{"Beta", "200"},
	}
	got, warns := rowsToTableChunks("Fin", rows, 5000, 20000)
	if len(warns) != 0 {
		t.Fatalf("warns=%v", warns)
	}
	if len(got) != 2 {
		t.Fatalf("rows=%d", len(got))
	}
	if got[0].Meta.Sheet != "Fin" || got[0].Meta.Row != 1 {
		t.Fatalf("meta=%+v", got[0].Meta)
	}
	if !strings.Contains(got[0].Text, "Name: Acme") || !strings.Contains(got[0].Text, "ARR: 100") {
		t.Fatalf("text=%q", got[0].Text)
	}
	if got[0].Meta.Kind != chunkTypeTableRow {
		t.Fatalf("kind=%q", got[0].Meta.Kind)
	}
}

func TestRowsToTableChunks_SoftLimits(t *testing.T) {
	rows := [][]string{{"A"}}
	for i := 0; i < 10; i++ {
		rows = append(rows, []string{strings.Repeat("x", 3)})
	}
	got, warns := rowsToTableChunks("S", rows, 3, 100)
	if len(got) != 3 {
		t.Fatalf("got=%d", len(got))
	}
	if len(warns) == 0 {
		t.Fatal("expected truncation warning")
	}
}

func TestExtractTableRowsFromCSV(t *testing.T) {
	in := "tenant,units\nA,1\nB,2\n"
	res, err := ExtractTableRowsFromCSV(strings.NewReader(in), TableIngestLimits{
		MaxSheets: 20, MaxRowsPerSheet: 5000, MaxRowsPerFile: 20000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 2 {
		t.Fatalf("rows=%d", len(res.Rows))
	}
}

func TestExtractTableRowsFromXLSX(t *testing.T) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	idx, err := f.NewSheet("Cap")
	if err != nil {
		t.Fatal(err)
	}
	f.SetActiveSheet(idx)
	_ = f.SetCellValue("Cap", "A1", "Investor")
	_ = f.SetCellValue("Cap", "B1", "Shares")
	_ = f.SetCellValue("Cap", "A2", "Alpha")
	_ = f.SetCellValue("Cap", "B2", "1000")
	path := filepath.Join(t.TempDir(), "cap.xlsx")
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	res, err := ExtractTableRowsFromXLSX(path, TableIngestLimits{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) < 1 {
		t.Fatalf("rows=%d", len(res.Rows))
	}
	found := false
	for _, row := range res.Rows {
		if row.Meta.Sheet == "Cap" && strings.Contains(row.Text, "Alpha") {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing Cap/Alpha row: %+v", res.Rows)
	}
}

func TestIsTableSourceType(t *testing.T) {
	if !isTableSourceType("xlsx") || !isTableSourceType("csv") {
		t.Fatal("expected true")
	}
	if isTableSourceType("pdf") {
		t.Fatal("pdf must be false")
	}
}
