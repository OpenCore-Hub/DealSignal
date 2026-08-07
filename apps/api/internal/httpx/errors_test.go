package httpx

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestIsInfrastructure(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"domain", errors.New("invalid slug"), false},
		{"no rows", pgx.ErrNoRows, true},
		{"pg error", &pgconn.PgError{Code: "23503", Message: "fk"}, true},
		{"sqlstate text", errors.New(`ERROR: insert violates foreign key (SQLSTATE 23503)`), true},
		{"wrapped no rows", errors.Join(errors.New("lookup"), pgx.ErrNoRows), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsInfrastructure(tc.err); got != tc.want {
				t.Fatalf("IsInfrastructure(%v)=%v want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestPublicMessage(t *testing.T) {
	t.Parallel()

	if got := PublicMessage(errors.New("invalid slug"), "fallback"); got != "invalid slug" {
		t.Fatalf("domain message = %q", got)
	}
	if got := PublicMessage(pgx.ErrNoRows, "workspace not found"); got != "workspace not found" {
		t.Fatalf("infra fallback = %q", got)
	}
	if got := PublicMessage(&pgconn.PgError{Code: "23503", Message: "fk boom"}, ""); got != MsgInternal {
		t.Fatalf("infra default = %q", got)
	}
}

func TestSafeMessage(t *testing.T) {
	t.Parallel()

	if got := SafeMessage("internal_error", &pgconn.PgError{Code: "23503", Message: "fk boom"}); got != MsgInternal {
		t.Fatalf("internal_error = %q", got)
	}
	if got := SafeMessage("workspace_not_found", pgx.ErrNoRows); got != "workspace not found" {
		t.Fatalf("workspace_not_found = %q", got)
	}
	if got := SafeMessage("invalid_slug", errors.New("slug must be lowercase")); got != "slug must be lowercase" {
		t.Fatalf("domain pass-through = %q", got)
	}
	if got := SafeMessage("storage_error", errors.New("minio: NoSuchKey")); got != "storage operation failed" {
		t.Fatalf("storage_error = %q", got)
	}
	smtpErr := fmt.Errorf("%w: smtp close data: 550 \"Queueing failed\"", errors.New("verification code could not be sent"))
	if got := SafeMessage("access_code_send_failed", smtpErr); got != "could not send verification code" {
		t.Fatalf("access_code_send_failed = %q", got)
	}
}

func TestInternalDoesNotLeak(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/workspaces", nil)

	Internal(c, &pgconn.PgError{Code: "23503", Message: "fk boom", ConstraintName: "workspace_members_user_id_fkey"}, "create workspace")

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", w.Code)
	}
	body := w.Body.String()
	if !containsAll(body, `"code":"internal_error"`, MsgInternal) {
		t.Fatalf("body=%s", body)
	}
	if containsAny(body, "23503", "workspace_members", "fk boom") {
		t.Fatalf("leaked infrastructure details: %s", body)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !contains(s, p) {
			return false
		}
	}
	return true
}

func containsAny(s string, parts ...string) bool {
	for _, p := range parts {
		if contains(s, p) {
			return true
		}
	}
	return false
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
