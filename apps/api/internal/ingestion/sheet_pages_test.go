package ingestion

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestMaterializeActiveSheetXLSX(t *testing.T) {
	src := filepath.Join(t.TempDir(), "multi.xlsx")
	f := excelize.NewFile()
	defer f.Close()
	_ = f.SetSheetName("Sheet1", "损益表")
	_, _ = f.NewSheet("Cashflow")
	_ = f.SetCellValue("损益表", "A1", "revenue")
	_ = f.SetCellValue("Cashflow", "A1", "cash")
	if err := f.SaveAs(src); err != nil {
		t.Fatal(err)
	}

	out, err := materializeActiveSheetXLSX(src, "Cashflow")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(out)

	got, err := excelize.OpenFile(out)
	if err != nil {
		t.Fatal(err)
	}
	defer got.Close()
	sheets := got.GetSheetList()
	if len(sheets) != 1 || sheets[0] != "Cashflow" {
		t.Fatalf("want only Cashflow, got %v", sheets)
	}
}

func TestListWorksheetNamesOrder(t *testing.T) {
	src := filepath.Join(t.TempDir(), "ordered.xlsx")
	f := excelize.NewFile()
	defer f.Close()
	_ = f.SetSheetName("Sheet1", "Alpha")
	_, _ = f.NewSheet("Beta")
	_, _ = f.NewSheet("Gamma")
	if err := f.SaveAs(src); err != nil {
		t.Fatal(err)
	}
	names, err := listWorksheetNames(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 3 || names[0] != "Alpha" || names[1] != "Beta" || names[2] != "Gamma" {
		t.Fatalf("workbook order mismatch: %v", names)
	}
}

func TestBuildSheetPageRanges(t *testing.T) {
	ranges, err := buildSheetPageRanges(
		[]string{"A", "B", "C"},
		[]int{2, 1, 3},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := []SheetPageRange{
		{SheetName: "A", PageStart: 1, PageEnd: 2},
		{SheetName: "B", PageStart: 3, PageEnd: 3},
		{SheetName: "C", PageStart: 4, PageEnd: 6},
	}
	if len(ranges) != len(want) {
		t.Fatalf("len=%d", len(ranges))
	}
	for i := range want {
		if ranges[i] != want[i] {
			t.Fatalf("[%d]=%+v want %+v", i, ranges[i], want[i])
		}
	}
	// Name-keyed lookup (order-independent for BFF).
	byName := map[string]int{}
	for _, r := range ranges {
		byName[r.SheetName] = r.PageStart
	}
	if byName["B"] != 3 {
		t.Fatalf("B start=%d", byName["B"])
	}
}

func TestSanitizeSheetKey(t *testing.T) {
	if got := sanitizeSheetKey("损益表"); got == "" {
		t.Fatal("empty")
	}
	if got := sanitizeSheetKey("A/B"); got != "A_B" {
		t.Fatalf("got %q", got)
	}
}
