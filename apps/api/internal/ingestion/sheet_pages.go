package ingestion

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/xuri/excelize/v2"
)

// SheetPageRange maps one workbook sheet onto a contiguous viewer page span
// (1-based, inclusive) in the merged preview PDF.
//
// Ranges are keyed by sheet *name* (matching docling ContentLocus.sheet), so
// lookup for citation jumps does not depend on sheet index.
type SheetPageRange struct {
	SheetName string
	PageStart int
	PageEnd   int
}

// ConvertSpreadsheetWithSheetRanges converts each XLSX worksheet to PDF
// separately (active-sheet OnlyOffice convert), merges them in workbook order,
// and returns the merged PDF path plus sheet→page ranges. Caller must remove
// the temp PDF.
//
// Sheet order matches excelize workbook order restricted to worksheets (same
// class of sheet docling stamps with locus.sheet). Chartsheets are skipped.
func (c *Converter) ConvertSpreadsheetWithSheetRanges(
	ctx context.Context,
	sourceType, storageKey, localPath string,
) (mergedPDF string, ranges []SheetPageRange, err error) {
	if !strings.EqualFold(sourceType, "xlsx") {
		return "", nil, fmt.Errorf("per-sheet convert supports xlsx only")
	}
	sheets, err := listWorksheetNames(localPath)
	if err != nil {
		return "", nil, err
	}
	if len(sheets) == 0 {
		return "", nil, fmt.Errorf("workbook has no worksheets")
	}

	var partPDFs []string
	var tempKeys []string
	defer func() {
		for _, p := range partPDFs {
			_ = os.Remove(p)
		}
		for _, key := range tempKeys {
			if delErr := c.storage.DeleteObject(ctx, key); delErr != nil {
				logger.InfoCtx(ctx, "cleanup temp sheet workbook failed",
					logger.Attr("key", key),
					logger.Attr("error", delErr.Error()),
				)
			}
		}
	}()

	pageCounts := make([]int, 0, len(sheets))
	widths, widthErr := sheetContentWidthsMM(localPath, sheets)
	if widthErr != nil {
		logger.InfoCtx(ctx, "sheet width probe failed; using default layout",
			logger.Attr("error", widthErr.Error()),
		)
	}
	for _, sheet := range sheets {
		layout := defaultSheetPreviewLayout()
		if widthErr == nil {
			layout = chooseSheetPreviewLayout(widths[sheet])
		}

		partKey, err := c.uploadSingleSheetWorkbook(ctx, storageKey, localPath, sheet, layout)
		if err != nil {
			return "", nil, fmt.Errorf("materialize sheet %q: %w", sheet, err)
		}
		tempKeys = append(tempKeys, partKey)
		pdfPath, convErr := c.ConvertToPDFActiveSheetWithLayout(ctx, "xlsx", partKey, layout)
		if convErr != nil {
			return "", nil, fmt.Errorf("convert sheet %q: %w", sheet, convErr)
		}
		partPDFs = append(partPDFs, pdfPath)
		n, countErr := api.PageCountFile(pdfPath)
		if countErr != nil || n <= 0 {
			if countErr == nil {
				countErr = fmt.Errorf("zero pages")
			}
			return "", nil, fmt.Errorf("page count for sheet %q: %w", sheet, countErr)
		}
		pageCounts = append(pageCounts, n)
	}

	ranges, err = buildSheetPageRanges(sheets, pageCounts)
	if err != nil {
		return "", nil, err
	}

	out, err := os.CreateTemp("", "merged-sheets-*.pdf")
	if err != nil {
		return "", nil, err
	}
	outPath := out.Name()
	_ = out.Close()
	if err := api.MergeCreateFile(partPDFs, outPath, false, model.NewDefaultConfiguration()); err != nil {
		_ = os.Remove(outPath)
		return "", nil, fmt.Errorf("merge sheet pdfs: %w", err)
	}
	return outPath, ranges, nil
}

// buildSheetPageRanges assigns contiguous 1-based page spans in sheet order.
func buildSheetPageRanges(sheets []string, pageCounts []int) ([]SheetPageRange, error) {
	if len(sheets) != len(pageCounts) {
		return nil, fmt.Errorf("sheets (%d) != pageCounts (%d)", len(sheets), len(pageCounts))
	}
	out := make([]SheetPageRange, 0, len(sheets))
	pageCursor := 1
	for i, sheet := range sheets {
		n := pageCounts[i]
		if n <= 0 {
			return nil, fmt.Errorf("sheet %q: non-positive page count %d", sheet, n)
		}
		if sheet == "" {
			return nil, fmt.Errorf("empty sheet name at index %d", i)
		}
		out = append(out, SheetPageRange{
			SheetName: sheet,
			PageStart: pageCursor,
			PageEnd:   pageCursor + n - 1,
		})
		pageCursor += n
	}
	return out, nil
}

// listWorksheetNames returns workbook-order worksheet names (skips chartsheets).
// Names align with docling locus.sheet for ordinary worksheets.
func listWorksheetNames(path string) ([]string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var sheets []string
	for _, name := range f.GetSheetList() {
		if !isWorksheet(f, name) {
			continue
		}
		sheets = append(sheets, name)
	}
	return sheets, nil
}

// isWorksheet reports whether name is a cell worksheet (not a chartsheet).
// excelize GetRows succeeds on worksheets (including empty ones) and fails on chartsheets.
func isWorksheet(f *excelize.File, name string) bool {
	_, err := f.GetRows(name)
	return err == nil
}

func (c *Converter) uploadSingleSheetWorkbook(
	ctx context.Context,
	origKey, localPath, sheet string,
	layout sheetPreviewLayout,
) (string, error) {
	tmp, err := materializeActiveSheetXLSX(localPath, sheet, layout)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(tmp)
	_ = os.Remove(tmp)
	if err != nil {
		return "", err
	}
	objectKey := path.Join(
		path.Dir(origKey),
		fmt.Sprintf(".sheet-%s-%d.xlsx", sanitizeSheetKey(sheet), time.Now().UnixNano()),
	)
	if err := c.storage.PutObject(
		ctx,
		objectKey,
		bytes.NewReader(data),
		int64(len(data)),
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	); err != nil {
		return "", fmt.Errorf("upload single-sheet workbook: %w", err)
	}
	return objectKey, nil
}

func materializeActiveSheetXLSX(srcPath, sheetName string, layout sheetPreviewLayout) (string, error) {
	f, err := excelize.OpenFile(srcPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	found := false
	names := append([]string(nil), f.GetSheetList()...)
	for _, name := range names {
		if name == sheetName {
			found = true
			continue
		}
		// Delete other sheets so OnlyOffice active-sheet convert cannot leak them.
		if err := f.DeleteSheet(name); err != nil {
			return "", fmt.Errorf("delete sheet %q: %w", name, err)
		}
	}
	if !found {
		return "", fmt.Errorf("sheet %q not found", sheetName)
	}
	idx, err := f.GetSheetIndex(sheetName)
	if err != nil || idx < 0 {
		return "", fmt.Errorf("sheet %q index: %w", sheetName, err)
	}
	f.SetActiveSheet(idx)

	// Mirror the chosen page setup into the workbook as well, so OnlyOffice
	// versions that prefer workbook settings over spreadsheetLayout still get
	// a wide landscape page instead of clipping the right-hand columns.
	paperSize := sheetPaperSize(layout)
	orientation := "landscape"
	fitToWidth := layout.fitToWidth
	fitToHeight := layout.fitToHeight
	fitToPage := layout.fitToWidth > 0 || layout.fitToHeight > 0
	margin10mm := 0.3937007874
	if err := f.SetSheetProps(sheetName, &excelize.SheetPropsOptions{FitToPage: &fitToPage}); err != nil {
		return "", fmt.Errorf("set sheet %q fit-to-page: %w", sheetName, err)
	}
	if err := f.SetPageLayout(sheetName, &excelize.PageLayoutOptions{
		Size:        &paperSize,
		Orientation: &orientation,
		FitToWidth:  &fitToWidth,
		FitToHeight: &fitToHeight,
	}); err != nil {
		return "", fmt.Errorf("set sheet %q page layout: %w", sheetName, err)
	}
	if err := f.SetPageMargins(sheetName, &excelize.PageLayoutMarginsOptions{
		Left:   &margin10mm,
		Right:  &margin10mm,
		Top:    &margin10mm,
		Bottom: &margin10mm,
	}); err != nil {
		return "", fmt.Errorf("set sheet %q page margins: %w", sheetName, err)
	}

	out, err := os.CreateTemp("", "sheet-*.xlsx")
	if err != nil {
		return "", err
	}
	outPath := out.Name()
	_ = out.Close()
	if err := f.SaveAs(outPath); err != nil {
		_ = os.Remove(outPath)
		return "", err
	}
	return outPath, nil
}

func sheetPaperSize(layout sheetPreviewLayout) int {
	switch {
	case layout.pageWidthMM > 420:
		return 66 // A2
	case layout.pageWidthMM > 297:
		return 8 // A3
	default:
		return 9 // A4
	}
}

func sanitizeSheetKey(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	s := b.String()
	if s == "" {
		return "sheet"
	}
	if len(s) > 40 {
		return s[:40]
	}
	return s
}
