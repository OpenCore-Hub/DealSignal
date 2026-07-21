package nda

import (
	"bytes"
	"fmt"
	"strings"
	"time"
)

// buildAuditTrailPDF creates a single-page PDF with the One-Click Audit Trail.
func buildAuditTrailPDF(p SealParams) ([]byte, error) {
	signedAt := p.SignedAt.UTC()
	if signedAt.IsZero() {
		signedAt = time.Now().UTC()
	}
	lines := []string{
		"ELECTRONIC SIGNATURE AUDIT TRAIL",
		"",
		"This page certifies that the preceding agreement was accepted",
		"electronically under applicable e-signature law (including ESIGN/UETA).",
		"",
		fmt.Sprintf("Certificate ID: %s", sanitizePDFText(p.CertificateID)),
		fmt.Sprintf("Agreement: %s", sanitizePDFText(p.TemplateName)),
		fmt.Sprintf("Content SHA-256: %s", sanitizePDFText(p.ContentSHA256)),
		fmt.Sprintf("Signer Name: %s", sanitizePDFText(p.SignerName)),
		fmt.Sprintf("Signer Email: %s", sanitizePDFText(p.SignerEmail)),
		fmt.Sprintf("Signed At (UTC): %s", signedAt.Format(time.RFC3339)),
		fmt.Sprintf("Link ID: %s", sanitizePDFText(p.LinkID)),
		fmt.Sprintf("IP Hash: %s", sanitizePDFText(p.IPHash)),
		fmt.Sprintf("User-Agent: %s", sanitizePDFText(truncate(p.UserAgent, 120))),
		"",
		"Acceptance method: One-Click (typed name + explicit agreement).",
	}

	var content strings.Builder
	content.WriteString("BT\n/F1 11 Tf\n50 740 Td\n14 TL\n")
	for i, line := range lines {
		if i == 0 {
			content.WriteString("/F1 14 Tf\n(")
			content.WriteString(escapePDFString(line))
			content.WriteString(") Tj\n/F1 11 Tf\nT*\n")
			continue
		}
		content.WriteString("(")
		content.WriteString(escapePDFString(line))
		content.WriteString(") Tj\nT*\n")
	}
	content.WriteString("ET\n")
	stream := content.String()

	var buf bytes.Buffer
	write := func(s string) { buf.WriteString(s) }
	write("%PDF-1.4\n")
	offsets := make([]int, 6)

	offsets[1] = buf.Len()
	write("1 0 obj<< /Type /Catalog /Pages 2 0 R >>endobj\n")
	offsets[2] = buf.Len()
	write("2 0 obj<< /Type /Pages /Kids [3 0 R] /Count 1 >>endobj\n")
	offsets[3] = buf.Len()
	write("3 0 obj<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources<< /Font<< /F1 5 0 R >> >> >>endobj\n")
	offsets[4] = buf.Len()
	write(fmt.Sprintf("4 0 obj<< /Length %d >>stream\n%s\nendstream\nendobj\n", len(stream), stream))
	offsets[5] = buf.Len()
	write("5 0 obj<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>endobj\n")

	xref := buf.Len()
	write("xref\n0 6\n")
	write("0000000000 65535 f \n")
	for i := 1; i <= 5; i++ {
		write(fmt.Sprintf("%010d 00000 n \n", offsets[i]))
	}
	write("trailer<< /Size 6 /Root 1 0 R >>\n")
	write(fmt.Sprintf("startxref\n%d\n%%%%EOF\n", xref))
	return buf.Bytes(), nil
}

func escapePDFString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "(", "\\(")
	s = strings.ReplaceAll(s, ")", "\\)")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func sanitizePDFText(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
