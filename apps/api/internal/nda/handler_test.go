package nda

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestWorkspaceID_UsesMiddlewareNotRouteParam(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("reads workspaceID from middleware context", func(t *testing.T) {
		var gotID, gotSlug, gotLegacyParam string
		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Set("workspaceID", "ws-uuid-123")
			c.Next()
		})
		r.GET("/workspaces/:workspaceSlug/nda/templates", func(c *gin.Context) {
			gotID = workspaceID(c)
			gotSlug = c.Param("workspaceSlug")
			gotLegacyParam = c.Param("workspaceId")
			c.Status(http.StatusNoContent)
		})

		req := httptest.NewRequest(http.MethodGet, "/workspaces/acme-capital/nda/templates", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", w.Code)
		}
		if gotID != "ws-uuid-123" {
			t.Fatalf("workspaceID() = %q, want ws-uuid-123", gotID)
		}
		if gotSlug != "acme-capital" {
			t.Fatalf("workspaceSlug = %q, want acme-capital", gotSlug)
		}
		if gotLegacyParam != "" {
			t.Fatalf("legacy workspaceId param = %q, want empty (route uses workspaceSlug)", gotLegacyParam)
		}
	})

	t.Run("empty when middleware did not inject workspace", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "workspaceSlug", Value: "acme-capital"}}
		c.Request = httptest.NewRequest(http.MethodGet, "/workspaces/acme-capital/nda/templates", nil)
		if got := workspaceID(c); got != "" {
			t.Fatalf("workspaceID() = %q, want empty", got)
		}
	})
}
