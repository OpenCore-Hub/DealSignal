package ingestion

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/storage"
)

// Converter calls OnlyOffice Document Server to convert Office files to PDF.
type Converter struct {
	baseURL   string
	jwtSecret []byte
	storage   *storage.Client
	client    *http.Client
}

// NewConverter creates an OnlyOffice converter.
func NewConverter(baseURL, jwtSecret string, s *storage.Client) *Converter {
	if baseURL == "" {
		baseURL = "http://onlyoffice:80"
	}
	return &Converter{
		baseURL:   baseURL,
		jwtSecret: []byte(jwtSecret),
		storage:   s,
		client:    &http.Client{Timeout: 2 * time.Minute},
	}
}

// ConvertToPDF asks OnlyOffice to convert an Office file to PDF.
// It uses a public (non-signed) internal URL so the OnlyOffice downloader
// (which normalizes/re-encodes URLs and breaks AWS presigned signatures) can
// fetch the file. The S3 bucket must allow anonymous read access.
func (c *Converter) ConvertToPDF(ctx context.Context, sourceType, storageKey string) (string, error) {
	// Use only the document ID as the OnlyOffice cache key (full path with slashes causes error -7).
	// Append a timestamp so a previously failed conversion does not poison OnlyOffice's cache.
	parts := strings.Split(storageKey, "/")
	cacheKey := storageKey
	if len(parts) >= 6 && parts[0] == "tenants" && parts[2] == "workspaces" && parts[4] == "documents" {
		cacheKey = parts[5]
	}
	cacheKey = fmt.Sprintf("%s-%d", cacheKey, time.Now().UnixNano())

	publicURL := c.storage.PublicURLInternal(storageKey)

	payload := map[string]interface{}{
		"async":      false,
		"filetype":   sourceType,
		"outputtype": "pdf",
		"url":        publicURL,
		"key":        cacheKey,
	}

	// OnlyOffice only converts the active sheet unless spreadsheetLayout is
	// provided. The layout below triggers all sheets while keeping each sheet
	// readable: fit to 1 page wide, auto-paginate vertically, A4 landscape.
	if isSpreadsheet(sourceType) {
		payload["spreadsheetLayout"] = map[string]interface{}{
			"ignorePrintArea": true,
			"orientation":     "landscape",
			"fitToWidth":      1,
			"fitToHeight":     0,
			"scale":           100,
			"headings":        false,
			"gridLines":       false,
			"pageSize": map[string]string{
				"width":  "297mm",
				"height": "210mm",
			},
			"margins": map[string]string{
				"left":   "10mm",
				"right":  "10mm",
				"top":    "10mm",
				"bottom": "10mm",
			},
		}
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/converter", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if len(c.jwtSecret) > 0 {
		req.Header.Set("Authorization", "Bearer "+signConverterRequest(body, c.jwtSecret))
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("call onlyoffice converter: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("onlyoffice converter returned %d", resp.StatusCode)
	}

	var result struct {
		Error    int    `json:"error"`
		FileURL  string `json:"fileUrl"`
		FileType string `json:"fileType"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode converter response: %w", err)
	}
	if result.Error != 0 {
		return "", fmt.Errorf("onlyoffice conversion error code %d", result.Error)
	}
	if result.FileURL == "" {
		return "", fmt.Errorf("onlyoffice returned empty fileUrl")
	}

	return c.downloadToTemp(result.FileURL)
}

func isSpreadsheet(sourceType string) bool {
	switch sourceType {
	case "xlsx", "xls", "ods", "csv":
		return true
	default:
		return false
	}
}

func signConverterRequest(body []byte, secret []byte) string {
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	headerJSON, _ := json.Marshal(header)

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	bodyB64 := base64.RawURLEncoding.EncodeToString(body)
	signingInput := headerB64 + "." + bodyB64

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signingInput))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return signingInput + "." + signature
}

func (c *Converter) downloadToTemp(url string) (string, error) {
	resp, err := c.client.Get(url)
	if err != nil {
		return "", fmt.Errorf("download converted pdf: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download converted pdf returned %d", resp.StatusCode)
	}

	f, err := os.CreateTemp("", "converted-*.pdf")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", fmt.Errorf("write converted pdf: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("close converted pdf: %w", err)
	}
	return f.Name(), nil
}
