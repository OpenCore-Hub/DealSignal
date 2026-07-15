package link

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/watermark"
)

const watermarkQueryParam = "wm"
const watermarkSigQueryParam = "wmsig"

// SignDownloadResource generates an HMAC-signed proxy URL for a downloadable
// storage resource. It extends SignResource with an optional signed watermark
// text parameter. When the watermark is present, the file-serving endpoint can
// apply a visible watermark before streaming the file to the visitor.
func SignDownloadResource(secret, storageKey, publicToken, visitorID, baseURL string, ttl time.Duration, watermark string) string {
	u, err := url.Parse(SignResource(secret, storageKey, publicToken, visitorID, baseURL, ttl))
	if err != nil {
		return ""
	}
	if watermark == "" {
		return u.String()
	}

	expiresStr := u.Query().Get("expires")
	expires, _ := strconv.ParseInt(expiresStr, 10, 64)

	q := u.Query()
	q.Set(watermarkQueryParam, watermark)
	q.Set(watermarkSigQueryParam, computeDownloadWatermarkSignature(secret, storageKey, publicToken, visitorID, expires, watermark))
	u.RawQuery = q.Encode()
	return u.String()
}

// VerifyDownloadWatermark validates the signed watermark text parameter on a
// download URL. It returns nil if the watermark parameter is absent or valid,
// and an error if it is present but tampered with.
func VerifyDownloadWatermark(secret, storageKey, publicToken, visitorID string, expires int64, watermark, sig string) error {
	if watermark == "" {
		return nil
	}
	expected := computeDownloadWatermarkSignature(secret, storageKey, publicToken, visitorID, expires, watermark)
	if !hmac.Equal([]byte(expected), []byte(sig)) {
		return fmt.Errorf("invalid watermark signature")
	}
	return nil
}

func computeDownloadWatermarkSignature(secret, storageKey, publicToken, visitorID string, expires int64, watermark string) string {
	payload := fmt.Sprintf("%s|%s|%s|%d|%s", storageKey, publicToken, visitorID, expires, watermark)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// shouldApplyServerWatermark reports whether the requested key supports a
// server-side watermark. For now this is limited to PDF downloads.
func shouldApplyServerWatermark(key string) bool {
	return watermark.ShouldApply(key)
}

// applyPDFWatermark overlays text diagonally across every page of the PDF
// read from r and writes the resulting PDF to w.
func applyPDFWatermark(r io.Reader, w io.Writer, text string) error {
	return watermark.ApplyPDF(r, w, text)
}
