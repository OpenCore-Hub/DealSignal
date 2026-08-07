package ingestion

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestSheetContentWidthMM(t *testing.T) {
	src := filepath.Join(t.TempDir(), "wide.xlsx")
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetColWidth("Sheet1", "A", "A", 38); err != nil {
		t.Fatal(err)
	}
	if err := f.SetColWidth("Sheet1", "B", "N", 16); err != nil {
		t.Fatal(err)
	}
	_ = f.SetCellValue("Sheet1", "A1", "label")
	for _, col := range []string{"B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N"} {
		_ = f.SetCellValue("Sheet1", col+"1", "x")
	}
	if err := f.SaveAs(src); err != nil {
		t.Fatal(err)
	}

	got, err := sheetContentWidthMM(src, "Sheet1")
	if err != nil {
		t.Fatal(err)
	}
	want := columnWidthToMM(38) + 13*columnWidthToMM(16)
	if math.Abs(got-want) > 0.01 {
		t.Fatalf("content width=%f want %f", got, want)
	}
}

func TestSheetContentWidthMMSkipsHiddenColumns(t *testing.T) {
	src := filepath.Join(t.TempDir(), "hidden.xlsx")
	f := excelize.NewFile()
	defer f.Close()
	_ = f.SetColWidth("Sheet1", "A", "B", 16)
	_ = f.SetCellValue("Sheet1", "A1", "a")
	_ = f.SetCellValue("Sheet1", "B1", "b")
	if err := f.SetColVisible("Sheet1", "B", false); err != nil {
		t.Fatal(err)
	}
	if err := f.SaveAs(src); err != nil {
		t.Fatal(err)
	}

	got, err := sheetContentWidthMM(src, "Sheet1")
	if err != nil {
		t.Fatal(err)
	}
	want := columnWidthToMM(16)
	if math.Abs(got-want) > 0.01 {
		t.Fatalf("hidden column width=%f want %f", got, want)
	}
}

func TestChooseSheetPreviewLayout(t *testing.T) {
	cases := []struct {
		name        string
		contentMM   float64
		widthMM     float64
		heightMM    float64
		fitToWidth  int
		fitToHeight int
	}{
		{"narrow uses A4", 100, 297, 210, 1, 0},
		{"medium uses A3", 500, 420, 297, 1, 0},
		{"wide uses A2", 800, 594, 420, 1, 0},
		{"very wide uses custom width", 900, 650, 297, 1, 0},
		{"extreme falls back to horizontal pagination", 2000, 297, 210, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := chooseSheetPreviewLayout(tc.contentMM)
			if got.pageWidthMM != tc.widthMM || got.pageHeightMM != tc.heightMM ||
				got.fitToWidth != tc.fitToWidth || got.fitToHeight != tc.fitToHeight {
				t.Fatalf("layout=%+v", got)
			}
		})
	}
}

func TestSpreadsheetLayoutPayloadUsesChosenPageSize(t *testing.T) {
	layout := chooseSheetPreviewLayout(500)
	payload := spreadsheetLayoutPayload(layout)
	pageSize, ok := payload["pageSize"].(map[string]string)
	if !ok {
		t.Fatalf("pageSize not string map: %T", payload["pageSize"])
	}
	if pageSize["width"] != "420mm" || pageSize["height"] != "297mm" {
		t.Fatalf("pageSize=%v", pageSize)
	}
	if payload["fitToWidth"] != 1 {
		t.Fatalf("fitToWidth=%v", payload["fitToWidth"])
	}
}
