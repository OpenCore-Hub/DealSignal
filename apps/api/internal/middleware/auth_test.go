package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/auth"
	"github.com/gin-gonic/gin"
)

func TestAuthMiddleware(t *testing.T) {
	auth.InitJWT("test-secret")
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name       string
		header     string
		wantStatus int
		wantUserID string
	}{
		{
			name:       "missing header",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid scheme",
			header:     "Basic token",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid token",
			header:     "Bearer not-a-token",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "valid token",
			header:     "Bearer " + mustToken(t, "user-123"),
			wantStatus: http.StatusOK,
			wantUserID: "user-123",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			validator := auth.NewService(nil, auth.NewMemoryTokenStore())
			r.GET("/me", Auth(validator), func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"user_id": UserIDFrom(c)})
			})

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, "/me", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			r.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d", tc.wantStatus, w.Code)
			}
		})
	}
}

func TestUserIDFromWithoutMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	if got := UserIDFrom(c); got != "" {
		t.Fatalf("expected empty user id, got %s", got)
	}
}

func mustToken(t *testing.T, userID string) string {
	t.Helper()
	tok, err := auth.GenerateToken(userID, time.Hour)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return tok
}
