package auth

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/gin-gonic/gin"
)

func TestSetAuthCookiesDevelopment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	h := &Handler{cfg: &config.Config{AppEnv: "development"}}
	h.setAuthCookies(c, TokenPair{AccessToken: "access", RefreshToken: "refresh", ExpiresIn: 123})

	cookies := w.Result().Cookies()
	var access, refresh, session *http.Cookie
	for _, ck := range cookies {
		switch ck.Name {
		case accessTokenCookie:
			access = ck
		case refreshTokenCookie:
			refresh = ck
		case authSessionCookie:
			session = ck
		}
	}
	if access == nil || access.Value != "access" || !access.HttpOnly || access.Secure || access.Path != "/" {
		t.Fatalf("unexpected access cookie: %+v", access)
	}
	if refresh == nil || refresh.Value != "refresh" || !refresh.HttpOnly || refresh.Secure || refresh.Path != "/" {
		t.Fatalf("unexpected refresh cookie: %+v", refresh)
	}
	if session == nil || session.Value != "1" || session.HttpOnly || session.Secure || session.Path != "/" {
		t.Fatalf("unexpected session cookie: %+v", session)
	}
	if access.SameSite != http.SameSiteLaxMode {
		t.Fatalf("expected SameSite=Lax in development, got %v", access.SameSite)
	}
	if access.MaxAge != 123 {
		t.Fatalf("expected MaxAge 123, got %d", access.MaxAge)
	}
}

func TestSetAuthCookiesProduction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "https://example.com/api/login", nil)
	os.Setenv("APP_ENV", "production")
	defer os.Unsetenv("APP_ENV")

	h := &Handler{cfg: &config.Config{AppEnv: "production"}}
	h.setAuthCookies(c, TokenPair{AccessToken: "access", RefreshToken: "refresh", ExpiresIn: 123})

	cookies := w.Result().Cookies()
	var access *http.Cookie
	for _, ck := range cookies {
		if ck.Name == accessTokenCookie {
			access = ck
		}
	}
	if access == nil || !access.Secure {
		t.Fatalf("expected Secure cookie in production, got %+v", access)
	}
	if access.SameSite != http.SameSiteNoneMode {
		t.Fatalf("expected SameSite=None in production, got %v", access.SameSite)
	}
}

func TestSetAuthCookiesForwardedProto(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "http://example.com/api/login", nil)
	c.Request.Header.Set("X-Forwarded-Proto", "https")

	h := &Handler{cfg: &config.Config{AppEnv: "staging"}}
	h.setAuthCookies(c, TokenPair{AccessToken: "access", RefreshToken: "refresh", ExpiresIn: 123})

	cookies := w.Result().Cookies()
	var access *http.Cookie
	for _, ck := range cookies {
		if ck.Name == accessTokenCookie {
			access = ck
		}
	}
	if access == nil || !access.Secure {
		t.Fatalf("expected Secure cookie with X-Forwarded-Proto=https, got %+v", access)
	}
	if access.SameSite != http.SameSiteNoneMode {
		t.Fatalf("expected SameSite=None with X-Forwarded-Proto=https, got %v", access.SameSite)
	}
}

func TestClearAuthCookies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	h := &Handler{cfg: &config.Config{AppEnv: "development"}}
	h.clearAuthCookies(c)

	cookies := w.Result().Cookies()
	if len(cookies) != 3 {
		t.Fatalf("expected 3 cleared cookies, got %d", len(cookies))
	}
	for _, ck := range cookies {
		if ck.MaxAge != -1 || ck.Value != "" {
			t.Fatalf("cookie %s was not cleared: MaxAge=%d Value=%q", ck.Name, ck.MaxAge, ck.Value)
		}
	}
}
