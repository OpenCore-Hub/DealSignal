package ingestion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/storage"
)

// Converter calls OnlyOffice Document Server to convert Office files to PDF.
type Converter struct {
	baseURL string
	storage *storage.Client
	client  *http.Client
}

// NewConverter creates an OnlyOffice converter.
func NewConverter(baseURL string, s *storage.Client) *Converter {
	if baseURL == "" {
		baseURL = "http://onlyoffice:80"
	}
	return &Converter{
		baseURL: baseURL,
		storage: s,
		client:  &http.Client{Timeout: 2 * time.Minute},
	}
}

// ConvertToPDF downloads the original file, asks OnlyOffice to convert it, and
// returns a local temporary PDF path.
func (c *Converter) ConvertToPDF(ctx context.Context, sourceType, storageKey string) (string, error) {
	presigned, err := c.storage.PresignedGetURL(ctx, storageKey, 10*time.Minute)
	if err != nil {
		return "", fmt.Errorf("presign original: %w", err)
	}

	payload := map[string]interface{}{
		"async":      false,
		"filetype":   sourceType,
		"outputtype": "pdf",
		"url":        presigned,
		"key":        storageKey,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/converter", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

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
