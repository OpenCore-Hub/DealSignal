package ingestion

import (
	"fmt"
	"math"
	"strings"

	"github.com/xuri/excelize/v2"
)

// sheetPreviewLayout describes the OnlyOffice page layout used for one XLSX
// sheet preview. Widths are chosen from the sheet content instead of assuming
// A4, so wide sheets stay complete without shrinking below a readable scale.
type sheetPreviewLayout struct {
	pageWidthMM  float64
	pageHeightMM float64
	fitToWidth   int
	fitToHeight  int
	scale        int
}

const (
	sheetPreviewMarginMM       = 10.0
	sheetPreviewMinScale       = 0.7
	sheetPreviewMaxPageWidthMM = 1000.0
	sheetPreviewCustomHeightMM = 297.0
)

var sheetPreviewStandardSizes = []struct{ widthMM, heightMM float64 }{
	{297, 210}, // A4 landscape
	{420, 297}, // A3 landscape
	{594, 420}, // A2 landscape
}

func defaultSheetPreviewLayout() sheetPreviewLayout {
	return sheetPreviewLayout{
		pageWidthMM:  297,
		pageHeightMM: 210,
		fitToWidth:   1,
		fitToHeight:  0,
		scale:        100,
	}
}

// chooseSheetPreviewLayout picks the smallest standard page that keeps the
// width scale at or above sheetPreviewMinScale, then falls back to a capped
// custom width and finally to horizontal pagination for extreme sheets.
func chooseSheetPreviewLayout(contentWidthMM float64) sheetPreviewLayout {
	if contentWidthMM < 0 {
		contentWidthMM = 0
	}
	neededUsableMM := contentWidthMM * sheetPreviewMinScale
	for _, size := range sheetPreviewStandardSizes {
		if size.widthMM-2*sheetPreviewMarginMM >= neededUsableMM {
			return sheetPreviewLayout{
				pageWidthMM:  size.widthMM,
				pageHeightMM: size.heightMM,
				fitToWidth:   1,
				fitToHeight:  0,
				scale:        100,
			}
		}
	}

	customWidthMM := math.Ceil(contentWidthMM*sheetPreviewMinScale + 2*sheetPreviewMarginMM)
	if customWidthMM <= sheetPreviewMaxPageWidthMM {
		return sheetPreviewLayout{
			pageWidthMM:  customWidthMM,
			pageHeightMM: sheetPreviewCustomHeightMM,
			fitToWidth:   1,
			fitToHeight:  0,
			scale:        100,
		}
	}

	return sheetPreviewLayout{
		pageWidthMM:  297,
		pageHeightMM: 210,
		fitToWidth:   0,
		fitToHeight:  0,
		scale:        100,
	}
}

// sheetContentWidthMM returns the total visible column width of one worksheet
// in millimetres.
func sheetContentWidthMM(path, sheet string) (float64, error) {
	widths, err := sheetContentWidthsMM(path, []string{sheet})
	if err != nil {
		return 0, err
	}
	return widths[sheet], nil
}

// sheetContentWidthsMM returns the total visible column width of every
// worksheet in millimetres, based on the used range and configured widths.
func sheetContentWidthsMM(path string, sheets []string) (map[string]float64, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := make(map[string]float64, len(sheets))
	for _, sheet := range sheets {
		maxCol, err := sheetMaxColumnIndex(f, sheet)
		if err != nil {
			return nil, fmt.Errorf("sheet %q dimension: %w", sheet, err)
		}
		if maxCol <= 0 {
			out[sheet] = 0
			continue
		}

		var totalMM float64
		for col := 1; col <= maxCol; col++ {
			name, err := excelize.ColumnNumberToName(col)
			if err != nil {
				return nil, err
			}
			visible, err := f.GetColVisible(sheet, name)
			if err != nil {
				return nil, fmt.Errorf("sheet %q column %s visibility: %w", sheet, name, err)
			}
			if !visible {
				continue
			}
			width, err := f.GetColWidth(sheet, name)
			if err != nil {
				return nil, fmt.Errorf("sheet %q column %s width: %w", sheet, name, err)
			}
			if width <= 0 {
				continue
			}
			totalMM += columnWidthToMM(width)
		}
		out[sheet] = totalMM
	}
	return out, nil
}

func sheetMaxColumnIndex(f *excelize.File, sheet string) (int, error) {
	ref, err := f.GetSheetDimension(sheet)
	if err != nil {
		return 0, err
	}
	maxCol := 0
	if ref != "" {
		ref = strings.ReplaceAll(ref, "$", "")
		if i := strings.LastIndex(ref, ":"); i >= 0 {
			ref = ref[i+1:]
		}
		colName, _, err := excelize.SplitCellName(ref)
		if err != nil {
			return 0, err
		}
		maxCol, err = excelize.ColumnNameToNumber(colName)
		if err != nil {
			return 0, err
		}
	}
	if maxCol > 1 {
		return maxCol, nil
	}

	// Some files keep a stale A1 dimension; fall back to the actual row cells
	// so configured-but-empty trailing columns are not mistaken for content.
	rows, err := f.GetRows(sheet)
	if err != nil {
		return 0, err
	}
	for _, row := range rows {
		if len(row) > maxCol {
			maxCol = len(row)
		}
	}
	return maxCol, nil
}

// columnWidthToMM mirrors Excel's column-width pixel rounding (7px per
// character plus 5px padding) and converts pixels to millimetres at 96 DPI.
func columnWidthToMM(width float64) float64 {
	if width <= 0 {
		return 0
	}
	var pixels float64
	if width < 1 {
		pixels = math.Ceil(width*12 + 0.5)
	} else {
		pixels = math.Ceil(width*7 + 0.5 + 5)
	}
	return pixels * 25.4 / 96
}
