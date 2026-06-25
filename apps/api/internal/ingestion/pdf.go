package ingestion

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/ledongthuc/pdf"
)

// PageInfo holds extracted data for a single PDF page.
type PageInfo struct {
	Number int
	Width  int
	Height int
	Text   string
	Blocks []TextBlock
}

// TextBlock is a text segment with a normalized bounding box.
type TextBlock struct {
	Text string
	Bbox NormalizedBBox
}

// NormalizedBBox is a bounding box in PAGE_IMAGE_NORMALIZED coordinate space (0-1).
type NormalizedBBox struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// Chunk holds a text segment extracted from a page.
type Chunk struct {
	Text string
	Bbox []byte
}

// pageBbox returns a JSON bounding box for the whole page in normalized coordinates (0-1).
func pageBbox(width, height int) []byte {
	// Use normalized coordinates so downstream consumers (search, highlight)
	// can consistently map to any image resolution.
	return []byte(`{"x":0,"y":0,"w":1,"h":1}`)
}

// ExtractPages extracts page count, dimensions, text and precise bbox from a PDF file.
// It first tries pdftotext -bbox-layout for precise word-level coordinates,
// falling back to the ledongthuc/pdf library for text-only extraction.
func ExtractPages(ctx context.Context, filePath string) ([]PageInfo, error) {
	// Try precise bbox extraction with pdftotext first
	if pages, err := extractPagesWithBBox(filePath); err == nil {
		return pages, nil
	}
	// Fallback to text-only extraction
	return extractPagesTextOnly(filePath)
}

// extractPagesWithBBox uses pdftotext -bbox-layout to get word-level coordinates.
func extractPagesWithBBox(filePath string) ([]PageInfo, error) {
	cmd := exec.Command("pdftotext", "-bbox-layout", filePath, "-")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("pdftotext bbox-layout: %w", err)
	}

	return parseBBoxHTML(stdout.Bytes())
}

// bboxWord represents a <word> element in pdftotext -bbox-layout output.
type bboxWord struct {
	XMin float64
	YMin float64
	XMax float64
	YMax float64
	Text string
}

func parseBBoxHTML(data []byte) ([]PageInfo, error) {
	// pdftotext -bbox-layout outputs <html><body><doc><page>...<word>text</word>...</page></doc></body></html>
	// The <word> elements are nested inside <flow><block><line>, so encoding/xml
	// cannot directly extract them with a flat struct tag. We use regex to extract
	// each <page> block and its <word> elements.

	// Extract all <page ...>...</page> blocks
	pageRegex := regexp.MustCompile(`(?s)<page\s+width="([\d.]+)"\s+height="([\d.]+)"\s*>(.*?)</page>`)
	pageMatches := pageRegex.FindAllSubmatch(data, -1)
	if len(pageMatches) == 0 {
		return nil, fmt.Errorf("no <page> elements found in pdftotext output")
	}

	// Regex to extract word attributes and text
	wordRegex := regexp.MustCompile(`(?s)<word\s+xMin="([\d.]+)"\s+yMin="([\d.]+)"\s+xMax="([\d.]+)"\s+yMax="([\d.]+)"\s*>(.*?)</word>`)

	pages := make([]PageInfo, 0, len(pageMatches))
	for i, pm := range pageMatches {
		pageNum := i + 1
		pageW, _ := strconv.ParseFloat(string(pm[1]), 64)
		pageH, _ := strconv.ParseFloat(string(pm[2]), 64)
		pageContent := pm[3]

		pageWi := int(pageW)
		pageHi := int(pageH)
		if pageWi <= 0 {
			pageWi = 612
		}
		if pageHi <= 0 {
			pageHi = 792
		}

		// Extract all words from this page
		wordMatches := wordRegex.FindAllSubmatch(pageContent, -1)
		words := make([]bboxWord, 0, len(wordMatches))
		for _, wm := range wordMatches {
			xMin, _ := strconv.ParseFloat(string(wm[1]), 64)
			yMin, _ := strconv.ParseFloat(string(wm[2]), 64)
			xMax, _ := strconv.ParseFloat(string(wm[3]), 64)
			yMax, _ := strconv.ParseFloat(string(wm[4]), 64)
			text := strings.TrimSpace(string(wm[5]))
			// Decode HTML entities
			text = decodeHTMLEntities(text)
			words = append(words, bboxWord{
				XMin: xMin, YMin: yMin, XMax: xMax, YMax: yMax, Text: text,
			})
		}

		blocks := groupWordsIntoBlocks(words, pageW, pageH)
		var fullText strings.Builder
		for _, b := range blocks {
			fullText.WriteString(b.Text)
			fullText.WriteString("\n")
		}

		pages = append(pages, PageInfo{
			Number: pageNum,
			Width:  pageWi,
			Height: pageHi,
			Text:   strings.TrimSpace(fullText.String()),
			Blocks: blocks,
		})
	}
	return pages, nil
}

// decodeHTMLEntities decodes common HTML entities produced by pdftotext.
func decodeHTMLEntities(s string) string {
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&apos;", "'")
	s = strings.ReplaceAll(s, "&#39;", "'")
	return s
}

// groupWordsIntoBlocks clusters words into text blocks (paragraphs) based on vertical proximity.
// Words on the same line are grouped; lines close together form blocks.
func groupWordsIntoBlocks(words []bboxWord, pageW, pageH float64) []TextBlock {
	if len(words) == 0 {
		return nil
	}
	if pageW <= 0 {
		pageW = 612
	}
	if pageH <= 0 {
		pageH = 792
	}

	// Sort words by Y then X
	sort.Slice(words, func(i, j int) bool {
		if abs(words[i].YMin-words[j].YMin) > 3 {
			return words[i].YMin < words[j].YMin
		}
		return words[i].XMin < words[j].XMin
	})

	// Group words into lines (words with similar Y)
	type line struct {
		words []bboxWord
		yMin  float64
		yMax  float64
	}
	var lines []line
	for _, w := range words {
		if len(lines) > 0 && abs(lines[len(lines)-1].yMin-w.YMin) < 3 {
			lines[len(lines)-1].words = append(lines[len(lines)-1].words, w)
			if w.YMax > lines[len(lines)-1].yMax {
				lines[len(lines)-1].yMax = w.YMax
			}
		} else {
			lines = append(lines, line{
				words: []bboxWord{w},
				yMin:  w.YMin,
				yMax:  w.YMax,
			})
		}
	}

	// Group lines into blocks (lines with small Y gap form a paragraph)
	var blocks []TextBlock
	type blockAccum struct {
		texts []string
		xMin  float64
		yMin  float64
		xMax  float64
		yMax  float64
	}
	var cur *blockAccum
	const lineGapThreshold = 5.0 // points

	for _, ln := range lines {
		var lineText strings.Builder
		var lxMin, lyMin, lxMax, lyMax float64
		for i, w := range ln.words {
			if i > 0 {
				lineText.WriteString(" ")
			}
			lineText.WriteString(strings.TrimSpace(w.Text))
			if i == 0 || w.XMin < lxMin {
				lxMin = w.XMin
			}
			if i == 0 || w.YMin < lyMin {
				lyMin = w.YMin
			}
			if w.XMax > lxMax {
				lxMax = w.XMax
			}
			if w.YMax > lyMax {
				lyMax = w.YMax
			}
		}
		lt := strings.TrimSpace(lineText.String())
		if lt == "" {
			continue
		}

		if cur != nil && abs(lyMin-cur.yMax) <= lineGapThreshold {
			cur.texts = append(cur.texts, lt)
			if lxMin < cur.xMin {
				cur.xMin = lxMin
			}
			if lyMin < cur.yMin {
				cur.yMin = lyMin
			}
			if lxMax > cur.xMax {
				cur.xMax = lxMax
			}
			if lyMax > cur.yMax {
				cur.yMax = lyMax
			}
		} else {
			if cur != nil {
				blocks = append(blocks, TextBlock{
					Text: strings.Join(cur.texts, "\n"),
					Bbox: NormalizedBBox{
						X: cur.xMin / pageW,
						Y: cur.yMin / pageH,
						W: (cur.xMax - cur.xMin) / pageW,
						H: (cur.yMax - cur.yMin) / pageH,
					},
				})
			}
			cur = &blockAccum{
				texts: []string{lt},
				xMin:  lxMin,
				yMin:  lyMin,
				xMax:  lxMax,
				yMax:  lyMax,
			}
		}
	}
	if cur != nil {
		blocks = append(blocks, TextBlock{
			Text: strings.Join(cur.texts, "\n"),
			Bbox: NormalizedBBox{
				X: cur.xMin / pageW,
				Y: cur.yMin / pageH,
				W: (cur.xMax - cur.xMin) / pageW,
				H: (cur.yMax - cur.yMin) / pageH,
			},
		})
	}
	return blocks
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

// extractPagesTextOnly falls back to the ledongthuc/pdf library for text extraction without bbox.
func extractPagesTextOnly(filePath string) ([]PageInfo, error) {
	f, r, err := pdf.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open pdf: %w", err)
	}
	defer f.Close()

	total := r.NumPage()
	pages := make([]PageInfo, 0, total)
	for i := 1; i <= total; i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}

		w, h := pageDimensions(page)
		text, err := page.GetPlainText(nil)
		if err != nil {
			text = ""
		}

		// Without bbox, create a single block with whole-page bbox
		blocks := []TextBlock{
			{Text: text, Bbox: NormalizedBBox{X: 0, Y: 0, W: 1, H: 1}},
		}
		pages = append(pages, PageInfo{
			Number: i,
			Width:  w,
			Height: h,
			Text:   text,
			Blocks: blocks,
		})
	}
	return pages, nil
}

func pageDimensions(p pdf.Page) (int, int) {
	box := p.V.Key("MediaBox")
	if box.IsNull() {
		box = p.V.Key("CropBox")
	}
	if box.IsNull() {
		return 612, 792
	}
	if box.Len() < 4 {
		return 612, 792
	}
	w := box.Index(2).Float64() - box.Index(0).Float64()
	h := box.Index(3).Float64() - box.Index(1).Float64()
	if w <= 0 || h <= 0 {
		return 612, 792
	}

	const maxEdge = 2048
	scale := 1.0
	if w > h && w > maxEdge {
		scale = maxEdge / w
	} else if h > maxEdge {
		scale = maxEdge / h
	}
	return int(w * scale), int(h * scale)
}

// renderPage creates a PNG image for a page and returns its pixel bounds.
// It tries to render the real PDF page via pdftoppm (poppler-utils).
// If pdftoppm is unavailable, it falls back to a placeholder image.
func renderPage(p PageInfo, pdfPath string) ([]byte, image.Rectangle, error) {
	// Try real PDF rendering with pdftoppm
	if img, bounds, err := renderPDFPageWithPdftoppm(pdfPath, p.Number); err == nil {
		return img, bounds, nil
	}
	// Fallback to placeholder
	img, err := renderPlaceholderPage(p)
	if err != nil {
		return nil, image.Rectangle{}, err
	}
	bounds, err := pngBounds(img)
	if err != nil {
		return nil, image.Rectangle{}, err
	}
	return img, bounds, nil
}

func pngBounds(data []byte) (image.Rectangle, error) {
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return image.Rectangle{}, fmt.Errorf("decode png config: %w", err)
	}
	return image.Rect(0, 0, cfg.Width, cfg.Height), nil
}

// renderPDFPageWithPdftoppm renders a single PDF page to PNG using pdftoppm.
func renderPDFPageWithPdftoppm(pdfPath string, pageNumber int) ([]byte, image.Rectangle, error) {
	tmpDir, err := os.MkdirTemp("", "pdfrender-*")
	if err != nil {
		return nil, image.Rectangle{}, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Use 200 DPI so that fine text and edge content remain readable after the
	// frontend fits the page into the viewport. PDF dimensions are in points
	// (1/72 inch), so 200 DPI gives ~2.78x scale.
	dpi := 200

	outputPrefix := filepath.Join(tmpDir, "page")
	cmd := exec.Command("pdftoppm",
		"-png",
		"-r", strconv.Itoa(dpi),
		"-f", strconv.Itoa(pageNumber),
		"-l", strconv.Itoa(pageNumber),
		"-singlefile",
		pdfPath,
		outputPrefix,
	)
	if err := cmd.Run(); err != nil {
		return nil, image.Rectangle{}, fmt.Errorf("pdftoppm render: %w", err)
	}

	outputFile := outputPrefix + ".png"
	data, err := os.ReadFile(outputFile)
	if err != nil {
		return nil, image.Rectangle{}, fmt.Errorf("read rendered page: %w", err)
	}

	bounds, err := pngBounds(data)
	if err != nil {
		return nil, image.Rectangle{}, fmt.Errorf("bounds: %w", err)
	}

	return data, bounds, nil
}

// renderPlaceholderPage creates a white background with "Page N" label.
func renderPlaceholderPage(p PageInfo) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, p.Width, p.Height))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	label := "Page " + strconv.Itoa(p.Number)
	drawLabel(img, label)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	return buf.Bytes(), nil
}

func drawLabel(img *image.RGBA, label string) {
	startX, startY := 20, 40
	x := startX
	for _, ch := range label {
		glyph := font5x7(ch)
		for row, bits := range glyph {
			for col := 0; col < 5; col++ {
				if bits&(1<<(4-col)) != 0 {
					img.Set(x+col, startY+row, color.Black)
				}
			}
		}
		x += 6
	}
}

func font5x7(r rune) [7]byte {
	switch {
	case r >= '0' && r <= '9':
		return digitFont[r-'0']
	case r == ' ':
		return [7]byte{0, 0, 0, 0, 0, 0, 0}
	case r >= 'A' && r <= 'Z':
		return letterFont[r-'A']
	case r >= 'a' && r <= 'z':
		return letterFont[r-'a']
	}
	return [7]byte{0, 0, 0, 0, 0, 0, 0}
}

var digitFont = [10][7]byte{
	{0x3E, 0x51, 0x49, 0x45, 0x3E, 0x00, 0x00},
	{0x00, 0x42, 0x7F, 0x40, 0x00, 0x00, 0x00},
	{0x42, 0x61, 0x51, 0x49, 0x46, 0x00, 0x00},
	{0x21, 0x41, 0x45, 0x4B, 0x31, 0x00, 0x00},
	{0x18, 0x14, 0x12, 0x7F, 0x10, 0x00, 0x00},
	{0x27, 0x45, 0x45, 0x45, 0x39, 0x00, 0x00},
	{0x3C, 0x4A, 0x49, 0x49, 0x30, 0x00, 0x00},
	{0x01, 0x71, 0x09, 0x05, 0x03, 0x00, 0x00},
	{0x36, 0x49, 0x49, 0x49, 0x36, 0x00, 0x00},
	{0x06, 0x49, 0x49, 0x29, 0x1E, 0x00, 0x00},
}

var letterFont = [26][7]byte{
	{0x7E, 0x11, 0x11, 0x11, 0x7E, 0x00, 0x00},
	{0x7F, 0x49, 0x49, 0x49, 0x36, 0x00, 0x00},
	{0x3E, 0x41, 0x41, 0x41, 0x22, 0x00, 0x00},
	{0x7F, 0x41, 0x41, 0x22, 0x1C, 0x00, 0x00},
	{0x7F, 0x49, 0x49, 0x49, 0x41, 0x00, 0x00},
	{0x7F, 0x09, 0x09, 0x09, 0x01, 0x00, 0x00},
	{0x3E, 0x41, 0x49, 0x49, 0x7A, 0x00, 0x00},
	{0x7F, 0x08, 0x08, 0x08, 0x7F, 0x00, 0x00},
	{0x00, 0x41, 0x7F, 0x41, 0x00, 0x00, 0x00},
	{0x20, 0x40, 0x41, 0x3F, 0x01, 0x00, 0x00},
	{0x7F, 0x08, 0x14, 0x22, 0x41, 0x00, 0x00},
	{0x7F, 0x40, 0x40, 0x40, 0x40, 0x00, 0x00},
	{0x7F, 0x02, 0x0C, 0x02, 0x7F, 0x00, 0x00},
	{0x7F, 0x04, 0x08, 0x10, 0x7F, 0x00, 0x00},
	{0x3E, 0x41, 0x41, 0x41, 0x3E, 0x00, 0x00},
	{0x7F, 0x09, 0x09, 0x09, 0x06, 0x00, 0x00},
	{0x3E, 0x41, 0x51, 0x21, 0x5E, 0x00, 0x00},
	{0x7F, 0x09, 0x19, 0x29, 0x46, 0x00, 0x00},
	{0x46, 0x49, 0x49, 0x49, 0x31, 0x00, 0x00},
	{0x01, 0x01, 0x7F, 0x01, 0x01, 0x00, 0x00},
	{0x3F, 0x40, 0x40, 0x40, 0x3F, 0x00, 0x00},
	{0x1F, 0x20, 0x40, 0x20, 0x1F, 0x00, 0x00},
	{0x3F, 0x40, 0x38, 0x40, 0x3F, 0x00, 0x00},
	{0x63, 0x14, 0x08, 0x14, 0x63, 0x00, 0x00},
	{0x07, 0x08, 0x70, 0x08, 0x07, 0x00, 0x00},
	{0x61, 0x51, 0x49, 0x45, 0x43, 0x00, 0x00},
}

// splitTextChunks converts page TextBlocks into Chunks with bbox JSON.
// If blocks have precise bbox, each block becomes a chunk.
// Otherwise falls back to paragraph splitting with whole-page bbox.
func splitTextChunks(page PageInfo) []Chunk {
	if len(page.Blocks) > 0 {
		chunks := make([]Chunk, 0, len(page.Blocks))
		for _, b := range page.Blocks {
			if strings.TrimSpace(b.Text) == "" {
				continue
			}
			bboxJSON := []byte(fmt.Sprintf(`{"x":%.6f,"y":%.6f,"w":%.6f,"h":%.6f}`, b.Bbox.X, b.Bbox.Y, b.Bbox.W, b.Bbox.H))
			chunks = append(chunks, Chunk{Text: b.Text, Bbox: bboxJSON})
		}
		return chunks
	}
	// Fallback: split by newlines with whole-page bbox
	paragraphs := strings.Split(strings.TrimSpace(page.Text), "\n")
	var chunks []Chunk
	bbox := pageBbox(page.Width, page.Height)
	for i, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		chunks = append(chunks, Chunk{Text: para, Bbox: bbox})
		if i > 100 {
			break
		}
	}
	return chunks
}

// normalizeText creates a normalized version of text for exact/fuzzy search.
func normalizeText(text string) string {
	lower := strings.ToLower(text)
	reg := regexp.MustCompile(`[^a-z0-9\x{4e00}-\x{9fff}]`)
	return strings.TrimSpace(reg.ReplaceAllString(lower, " "))
}
