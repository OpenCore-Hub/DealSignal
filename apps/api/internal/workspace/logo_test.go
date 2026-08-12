package workspace

import (
	"context"
	"testing"
)

func TestNormalizeLogoContentType(t *testing.T) {
	t.Parallel()
	cases := []struct {
		contentType string
		filename    string
		want        string
		wantErr     error
	}{
		{contentType: "image/png", filename: "logo.png", want: "image/png"},
		{contentType: "image/jpeg; charset=binary", filename: "a.jpg", want: "image/jpeg"},
		{contentType: "", filename: "brand.webp", want: "image/webp"},
		{contentType: "text/plain", filename: "notes.txt", wantErr: ErrInvalidLogoType},
		{contentType: "image/svg+xml", filename: "logo.svg", want: "image/svg+xml"},
		{contentType: "image/svg", filename: "logo.svg", want: "image/svg+xml"},
		{contentType: "", filename: "mark.svg", want: "image/svg+xml"},
	}
	for _, tc := range cases {
		got, err := normalizeLogoContentType(tc.contentType, tc.filename)
		if tc.wantErr != nil {
			if err != tc.wantErr {
				t.Fatalf("%s/%s: error = %v, want %v", tc.contentType, tc.filename, err, tc.wantErr)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s/%s: unexpected error %v", tc.contentType, tc.filename, err)
		}
		if got != tc.want {
			t.Fatalf("%s/%s: got %s, want %s", tc.contentType, tc.filename, got, tc.want)
		}
	}
}

func TestUploadLogoRequiresStorage(t *testing.T) {
	t.Parallel()
	svc := NewService(nil)
	_, err := svc.UploadLogo(context.Background(), "ws", "tenant", nil, nil)
	if err != ErrLogoStorageUnavailable {
		t.Fatalf("expected ErrLogoStorageUnavailable, got %v", err)
	}
}
