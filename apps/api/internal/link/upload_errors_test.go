package link

import (
	"errors"
	"net/http"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/upload"
)

func TestClassifyLinkUploadError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"not file request", ErrNotFileRequestLink, http.StatusForbidden, "not_file_request_link"},
		{"sidecar filename", upload.ErrUnsupportedUpload, http.StatusBadRequest, "unsupported_upload"},
		{"empty file sentinel", ErrLinkUploadEmpty, http.StatusBadRequest, "file_empty"},
		{"empty file upload pkg", upload.ErrEmptyFile, http.StatusBadRequest, "file_empty"},
		{"too large", ErrLinkUploadTooLarge, http.StatusRequestEntityTooLarge, "file_too_large"},
		{"unsupported type", ErrLinkUploadUnsupportedType, http.StatusUnsupportedMediaType, "unsupported_type"},
		{"plan upload cap", plan.ErrLimitUpload, http.StatusForbidden, plan.CodeLimitUpload},
		{"plan storage cap", plan.ErrLimitStorage, http.StatusForbidden, plan.CodeLimitStorage},
		{"unknown", errors.New("store upload: boom"), http.StatusInternalServerError, "internal_error"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, code := classifyLinkUploadError(tc.err)
			if status != tc.wantStatus || code != tc.wantCode {
				t.Fatalf("classifyLinkUploadError(%v) = (%d, %q), want (%d, %q)",
					tc.err, status, code, tc.wantStatus, tc.wantCode)
			}
		})
	}
}
