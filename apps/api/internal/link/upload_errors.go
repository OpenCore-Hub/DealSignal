package link

import (
	"errors"
	"net/http"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/upload"
)

var (
	ErrNotFileRequestLink        = errors.New("link is not a file request link")
	ErrLinkUploadEmpty           = errors.New("file is empty")
	ErrLinkUploadTooLarge        = errors.New("file too large")
	ErrLinkUploadUnsupportedType = errors.New("unsupported file type")
)

// classifyLinkUploadError maps link file-request upload failures to HTTP status/code.
func classifyLinkUploadError(err error) (status int, code string) {
	if status, code, ok := plan.HTTPError(err); ok {
		return status, code
	}
	switch {
	case errors.Is(err, ErrNotFileRequestLink):
		return http.StatusForbidden, "not_file_request_link"
	case errors.Is(err, upload.ErrUnsupportedUpload):
		return http.StatusBadRequest, "unsupported_upload"
	case errors.Is(err, upload.ErrEmptyFile), errors.Is(err, ErrLinkUploadEmpty):
		return http.StatusBadRequest, "file_empty"
	case errors.Is(err, ErrLinkUploadTooLarge):
		return http.StatusRequestEntityTooLarge, "file_too_large"
	case errors.Is(err, ErrLinkUploadUnsupportedType):
		return http.StatusUnsupportedMediaType, "unsupported_type"
	default:
		return http.StatusInternalServerError, "internal_error"
	}
}
