// Package watermark provides server-side PDF watermarking for the download
// pipeline. It is used by both public link downloads and authenticated
// workspace downloads.
package watermark

import (
	"bytes"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// ShouldApply reports whether the requested key supports a server-side
// watermark. For now this is limited to PDF downloads.
func ShouldApply(key string) bool {
	return strings.EqualFold(path.Ext(key), ".pdf")
}

// ApplyPDF overlays text diagonally across every page of the PDF read from r
// and writes the resulting PDF to w.
func ApplyPDF(r io.Reader, w io.Writer, text string) error {
	conf := model.NewDefaultConfiguration()

	// Center, 45-degree, semi-transparent gray text scaled to fit each page.
	desc := "pos:c, scale:.9 abs, rot:45, op:.35, col:.5 .5 .5"
	wm, err := api.TextWatermark(text, desc, true, false, types.POINTS)
	if err != nil {
		return fmt.Errorf("create text watermark: %w", err)
	}

	// pdfcpu requires a ReadSeeker. We buffer the input because S3 returns a
	// plain ReadCloser and the PDF needs random access during processing.
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read pdf: %w", err)
	}

	if err := api.AddWatermarks(bytes.NewReader(data), w, nil, wm, conf); err != nil {
		return fmt.Errorf("apply watermark: %w", err)
	}
	return nil
}
