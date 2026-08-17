package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/turnstile"
	"github.com/gin-gonic/gin"
)

func TestMeUnauthorizedWithoutToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{cfg: &config.Config{AppEnv: "development"}, service: NewService(nil, NewMemoryTokenStore())}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	h.Me(c)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

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

type stubCaptcha struct {
	enabled bool
	siteKey string
	err     error
	token   string
	ip      string
}

func (s *stubCaptcha) Enabled() bool   { return s.enabled }
func (s *stubCaptcha) SiteKey() string { return s.siteKey }
func (s *stubCaptcha) Verify(_ context.Context, token, remoteIP string) error {
	s.token = token
	s.ip = remoteIP
	return s.err
}

func TestCaptchaReturnsSiteKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(nil, &config.Config{AppEnv: "development"})
	h.SetTurnstile(&stubCaptcha{enabled: true, siteKey: "1x00000000000000000000AA"})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/auth/captcha", nil)
	h.Captcha(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "1x00000000000000000000AA") {
		t.Fatalf("body=%s", w.Body.String())
	}
}

func TestRegisterRequiresCaptchaWhenEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(NewService(nil, NewMemoryTokenStore()), &config.Config{AppEnv: "development"})
	h.SetTurnstile(&stubCaptcha{enabled: true, siteKey: "site", err: turnstile.ErrMissing})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{"email":"a@example.com","password":"Password123!"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h.Register(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "captcha_required") {
		t.Fatalf("body=%s", w.Body.String())
	}
}

func TestRegisterCaptchaUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(NewService(nil, NewMemoryTokenStore()), &config.Config{AppEnv: "development"})
	h.SetTurnstile(&stubCaptcha{enabled: true, siteKey: "site", err: turnstile.ErrUnavailable})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{"email":"a@example.com","password":"Password123!","turnstile_token":"tok"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h.Register(c)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "captcha_unavailable") {
		t.Fatalf("body=%s", w.Body.String())
	}
}

func testHandler(autoVerify bool) *Handler {
	svc, _, _, _ := testAuthService(autoVerify)
	return NewHandler(svc, &config.Config{AppEnv: "development"})
}

func TestRegisterOmitsCookiesWhenVerificationRequired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := testHandler(false)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{"email":"a@example.com","password":"Password123!"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h.Register(c)
	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"verification_required":true`) {
		t.Fatalf("body=%s", w.Body.String())
	}
	for _, ck := range w.Result().Cookies() {
		if ck.Name == accessTokenCookie && ck.Value != "" {
			t.Fatalf("register must not set access cookie: %+v", ck)
		}
	}
}

func TestLoginUnverifiedReturns403(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := testHandler(false)
	_, err := h.service.Register(context.Background(), "a@example.com", "Password123!")
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"a@example.com","password":"Password123!"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h.Login(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "email_not_verified") {
		t.Fatalf("body=%s", w.Body.String())
	}
	for _, ck := range w.Result().Cookies() {
		if ck.Name == accessTokenCookie && ck.Value != "" {
			t.Fatalf("unverified login must not set cookies: %+v", ck)
		}
	}
}

func TestResendVerificationAlwaysOK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := testHandler(false)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/resend-verification", strings.NewReader(`{"email":"missing@example.com"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h.ResendVerification(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestResendVerificationRequiresCaptchaWhenEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := testHandler(false)
	h.SetTurnstile(&stubCaptcha{enabled: true, siteKey: "site", err: turnstile.ErrMissing})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/resend-verification", strings.NewReader(`{"email":"a@example.com"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h.ResendVerification(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "captcha_required") {
		t.Fatalf("body=%s", w.Body.String())
	}
}

func TestVerifyEmailPOSTSetsCookies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := testHandler(false)
	result, err := h.service.Register(context.Background(), "a@example.com", "Password123!")
	if err != nil {
		t.Fatal(err)
	}
	token, err := h.service.verifyStore.CreateVerificationToken(context.Background(), result.User.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/verify-email", strings.NewReader(`{"token":"`+token+`"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h.VerifyEmail(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var access *http.Cookie
	for _, ck := range w.Result().Cookies() {
		if ck.Name == accessTokenCookie {
			access = ck
		}
	}
	if access == nil || access.Value == "" {
		t.Fatal("verify must set access cookie")
	}
}

func TestRefreshUnverifiedClearsCookies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := testHandler(false)
	result, err := h.service.Register(context.Background(), "a@example.com", "Password123!")
	if err != nil {
		t.Fatal(err)
	}
	pair, err := GenerateTokenPair(result.User.ID, accessTokenDuration, refreshTokenDuration)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.service.tokenStore.StoreRefreshToken(context.Background(), result.User.ID, pair.RefreshToken, refreshTokenDuration); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/refresh", strings.NewReader(`{"refresh_token":"`+pair.RefreshToken+`"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h.Refresh(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	cleared := false
	for _, ck := range w.Result().Cookies() {
		if ck.Name == accessTokenCookie && ck.MaxAge == -1 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatal("refresh must clear cookies for unverified subject")
	}
}

func TestForgotPasswordAlwaysOK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := testHandler(false)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/forgot-password", strings.NewReader(`{"email":"missing@example.com"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h.ForgotPassword(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestForgotPasswordRequiresCaptchaWhenEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := testHandler(false)
	h.SetTurnstile(&stubCaptcha{enabled: true, siteKey: "site", err: turnstile.ErrMissing})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/forgot-password", strings.NewReader(`{"email":"a@example.com"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h.ForgotPassword(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "captcha_required") {
		t.Fatalf("body=%s", w.Body.String())
	}
}

func TestResetPasswordDoesNotSetCookies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, accounts, mail, _ := testAuthService(false)
	h := NewHandler(svc, &config.Config{AppEnv: "development"})
	seedVerified(t, accounts, "user@example.com", "OldPass123!")
	svc.ForgotPassword(context.Background(), "user@example.com")
	waitForMail(t, mail)
	token := mail.lastLink[strings.LastIndex(mail.lastLink, "/")+1:]

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/reset-password", strings.NewReader(`{"token":"`+token+`","password":"NewPass123!"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h.ResetPassword(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	for _, ck := range w.Result().Cookies() {
		if (ck.Name == accessTokenCookie || ck.Name == refreshTokenCookie || ck.Name == authSessionCookie) && ck.Value != "" && ck.MaxAge != -1 {
			t.Fatalf("reset must not set session cookies: %+v", ck)
		}
	}
}

func TestResetPasswordWeakPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, accounts, mail, _ := testAuthService(false)
	h := NewHandler(svc, &config.Config{AppEnv: "development"})
	seedVerified(t, accounts, "user@example.com", "OldPass123!")
	svc.ForgotPassword(context.Background(), "user@example.com")
	waitForMail(t, mail)
	token := mail.lastLink[strings.LastIndex(mail.lastLink, "/")+1:]

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/reset-password", strings.NewReader(`{"token":"`+token+`","password":"short"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h.ResetPassword(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "weak_password") {
		t.Fatalf("body=%s", w.Body.String())
	}
}

func TestResetPasswordInvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := testHandler(false)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/reset-password", strings.NewReader(`{"token":"missing","password":"NewPass123!"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h.ResetPassword(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "invalid_or_expired_token") {
		t.Fatalf("body=%s", w.Body.String())
	}
}
