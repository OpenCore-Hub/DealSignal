package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHostMiddlewareSetsTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)

	lookup := func(_ context.Context, host string) (string, error) {
		if host == "acme.dealsignal.com" {
			return "tenant-123", nil
		}
		return "", nil
	}

	r := gin.New()
	r.Use(HostMiddleware("dealsignal.com", lookup))
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"tenant_id": TenantIDFrom(c)})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.Host = "acme.dealsignal.com"
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "tenant-123") {
		t.Fatalf("expected tenant id in response, got %s", w.Body.String())
	}
}

func TestHostMiddlewareIgnoresBaseDomain(t *testing.T) {
	gin.SetMode(gin.TestMode)

	lookup := func(_ context.Context, _ string) (string, error) {
		t.Fatal("lookup should not be called for base domain")
		return "", nil
	}

	r := gin.New()
	r.Use(HostMiddleware("dealsignal.com", lookup))
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"tenant_id": TenantIDFrom(c)})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.Host = "dealsignal.com"
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}

