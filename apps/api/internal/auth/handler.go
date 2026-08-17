package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/httpx"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/turnstile"
)

const (
	accessTokenCookie  = "access_token"
	refreshTokenCookie = "refresh_token"
	authSessionCookie  = "auth_session"
)

type registerRequest struct {
	Email          string `json:"email" binding:"required,email"`
	Password       string `json:"password" binding:"required,min=8"`
	TurnstileToken string `json:"turnstile_token"`
}

type loginRequest struct {
	Email       string `json:"email" binding:"omitempty,email"`
	Password    string `json:"password"`
	InviteToken string `json:"invite_token"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type verifyEmailRequest struct {
	Token string `json:"token" binding:"required"`
}

type resendVerificationRequest struct {
	Email          string `json:"email" binding:"required,email"`
	TurnstileToken string `json:"turnstile_token"`
}

type forgotPasswordRequest struct {
	Email          string `json:"email" binding:"required,email"`
	TurnstileToken string `json:"turnstile_token"`
}

type resetPasswordRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password"`
}

// Handler exposes auth HTTP endpoints.
type Handler struct {
	service *Service
	cfg     *config.Config
	captcha turnstile.Verifier
}

// NewHandler creates an auth handler.
func NewHandler(s *Service, cfg *config.Config) *Handler {
	return &Handler{service: s, cfg: cfg, captcha: turnstile.New("", "")}
}

// SetTurnstile attaches the register CAPTCHA verifier.
func (h *Handler) SetTurnstile(v turnstile.Verifier) {
	if h == nil {
		return
	}
	if v == nil {
		h.captcha = turnstile.New("", "")
		return
	}
	h.captcha = v
}

// RegisterRoutes mounts auth routes.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/auth")
	g.POST("/register", h.Register)
	g.POST("/login", h.Login)
	g.POST("/refresh", h.Refresh)
	g.POST("/logout", h.Logout)
	g.POST("/verify-email", h.VerifyEmail)
	g.POST("/resend-verification", h.ResendVerification)
	g.POST("/forgot-password", h.ForgotPassword)
	g.POST("/reset-password", h.ResetPassword)
	g.GET("/captcha", h.Captcha)
	g.GET("/me", h.Me)
}

func isRequestSecure(c *gin.Context) bool {
	if c.Request == nil {
		return false
	}
	if c.Request.TLS != nil {
		return true
	}
	return strings.ToLower(c.GetHeader("X-Forwarded-Proto")) == "https"
}

func (h *Handler) cookieSettings(c *gin.Context) (secure bool, sameSite http.SameSite) {
	secure = strings.ToLower(h.cfg.AppEnv) == "production" || isRequestSecure(c)
	if secure {
		return true, http.SameSiteNoneMode
	}
	return false, http.SameSiteLaxMode
}

func (h *Handler) setAuthCookies(c *gin.Context, pair TokenPair) {
	secure, sameSite := h.cookieSettings(c)
	c.SetSameSite(sameSite)
	c.SetCookie(accessTokenCookie, pair.AccessToken, int(pair.ExpiresIn), "/", "", secure, true)
	c.SetCookie(refreshTokenCookie, pair.RefreshToken, int(refreshTokenDuration.Seconds()), "/", "", secure, true)
	c.SetCookie(authSessionCookie, "1", int(refreshTokenDuration.Seconds()), "/", "", secure, false)
}

func (h *Handler) clearAuthCookies(c *gin.Context) {
	secure, sameSite := h.cookieSettings(c)
	c.SetSameSite(sameSite)
	c.SetCookie(accessTokenCookie, "", -1, "/", "", secure, true)
	c.SetCookie(refreshTokenCookie, "", -1, "/", "", secure, true)
	c.SetCookie(authSessionCookie, "", -1, "/", "", secure, false)
}

func accessTokenFromRequest(c *gin.Context) string {
	if header := c.GetHeader("Authorization"); len(header) > 7 {
		return header[7:]
	}
	token, _ := c.Cookie(accessTokenCookie)
	return token
}

func refreshTokenFromRequest(c *gin.Context) string {
	if token, err := c.Cookie(refreshTokenCookie); err == nil && token != "" {
		return token
	}
	var req refreshRequest
	_ = c.ShouldBindJSON(&req)
	return req.RefreshToken
}

// Register handles user registration.
func (h *Handler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}

	if !h.verifyTurnstile(c, req.TurnstileToken) {
		return
	}

	result, err := h.service.Register(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, ErrEmailExists):
			c.JSON(http.StatusConflict, gin.H{"code": "email_conflict", "message": "email already registered"})
		case errors.Is(err, ErrInvalidEmail):
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_email", "message": "invalid email address"})
		case errors.Is(err, ErrDisposableEmail):
			c.JSON(http.StatusBadRequest, gin.H{"code": "disposable_email", "message": httpx.SafeMessage("disposable_email", err)})
		case errors.Is(err, ErrWeakPassword):
			c.JSON(http.StatusBadRequest, gin.H{"code": "weak_password", "message": "password must be at least 8 characters and include uppercase, lowercase, digit and special character"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "registration failed"})
		}
		return
	}

	if result.VerificationRequired || result.Pair.AccessToken == "" {
		c.JSON(http.StatusCreated, gin.H{"user": result.User, "verification_required": true})
		return
	}

	h.setAuthCookies(c, result.Pair)
	c.JSON(http.StatusCreated, gin.H{"user": result.User, "expires_in": result.Pair.ExpiresIn})
}

func (h *Handler) verifyTurnstile(c *gin.Context, token string) bool {
	if h.captcha == nil || !h.captcha.Enabled() {
		return true
	}
	switch err := h.captcha.Verify(c.Request.Context(), token, c.ClientIP()); {
	case errors.Is(err, turnstile.ErrMissing):
		c.JSON(http.StatusBadRequest, gin.H{"code": "captcha_required", "message": httpx.SafeMessage("captcha_required", err)})
		return false
	case errors.Is(err, turnstile.ErrUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "captcha_unavailable", "message": httpx.SafeMessage("captcha_unavailable", err)})
		return false
	case err != nil:
		c.JSON(http.StatusBadRequest, gin.H{"code": "captcha_failed", "message": httpx.SafeMessage("captcha_failed", err)})
		return false
	}
	return true
}

// Login handles user login.
func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}
	if req.Email == "" || req.Password == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "message": "missing credentials"})
		return
	}

	user, pair, err := h.service.Login(c.Request.Context(), req.Email, req.Password, strings.TrimSpace(req.InviteToken))
	if err != nil {
		if errors.Is(err, ErrEmailNotVerified) {
			c.JSON(http.StatusForbidden, gin.H{"code": "email_not_verified", "message": "email not verified"})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "message": "invalid email or password"})
		return
	}

	h.setAuthCookies(c, pair)
	c.JSON(http.StatusOK, gin.H{"user": user, "expires_in": pair.ExpiresIn})
}

// Me returns the authenticated user's public profile (login email for owner watermark, etc.).
func (h *Handler) Me(c *gin.Context) {
	token := accessTokenFromRequest(c)
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "message": "missing authorization"})
		return
	}
	claims, err := h.service.ValidateAccessToken(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "message": "invalid or expired token"})
		return
	}
	user, err := h.service.GetUser(c.Request.Context(), claims.Subject)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "message": "user not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": user})
}

// Refresh issues a new token pair from the refresh cookie.
func (h *Handler) Refresh(c *gin.Context) {
	refreshToken := refreshTokenFromRequest(c)
	if refreshToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "message": "missing refresh token"})
		return
	}

	pair, err := h.service.Refresh(c.Request.Context(), refreshToken)
	if err != nil {
		if errors.Is(err, ErrEmailNotVerified) {
			h.clearAuthCookies(c)
			c.JSON(http.StatusForbidden, gin.H{"code": "email_not_verified", "message": "email not verified"})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "message": "invalid or expired refresh token"})
		return
	}

	h.setAuthCookies(c, pair)
	c.JSON(http.StatusOK, gin.H{"expires_in": pair.ExpiresIn})
}

// VerifyEmail verifies a user's email address using a single-use token and
// completes registration by issuing a session.
func (h *Handler) VerifyEmail(c *gin.Context) {
	var req verifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Token) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_token", "message": "verification token is required"})
		return
	}

	user, pair, err := h.service.VerifyEmailByToken(c.Request.Context(), strings.TrimSpace(req.Token))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_or_expired_token", "message": "verification link is invalid or has expired"})
		return
	}

	h.setAuthCookies(c, pair)
	c.JSON(http.StatusOK, gin.H{
		"code":       "verified",
		"message":    "email verified successfully",
		"user":       user,
		"expires_in": pair.ExpiresIn,
	})
}

// ResendVerification always returns 200 to avoid mailbox enumeration.
func (h *Handler) ResendVerification(c *gin.Context) {
	var req resendVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}
	if !h.verifyTurnstile(c, req.TurnstileToken) {
		return
	}
	h.service.ResendVerification(c.Request.Context(), req.Email)
	c.JSON(http.StatusOK, gin.H{"code": "ok"})
}

// ForgotPassword always returns 200 to avoid mailbox enumeration.
func (h *Handler) ForgotPassword(c *gin.Context) {
	var req forgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}
	if !h.verifyTurnstile(c, req.TurnstileToken) {
		return
	}
	h.service.ForgotPassword(c.Request.Context(), req.Email)
	c.JSON(http.StatusOK, gin.H{"code": "ok"})
}

// ResetPassword updates the password from a one-time email token.
// It never sets session cookies: a reset URL can leak via Referer or history.
func (h *Handler) ResetPassword(c *gin.Context) {
	var req resetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Token) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_or_expired_token", "message": "reset link is invalid or has expired"})
		return
	}
	if err := h.service.ResetPassword(c.Request.Context(), strings.TrimSpace(req.Token), req.Password); err != nil {
		switch {
		case errors.Is(err, ErrWeakPassword):
			c.JSON(http.StatusBadRequest, gin.H{"code": "weak_password", "message": "password must be at least 8 characters and include uppercase, lowercase, digit and special character"})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_or_expired_token", "message": "reset link is invalid or has expired"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "ok"})
}

// Logout revokes the current access and refresh tokens and clears cookies.
func (h *Handler) Logout(c *gin.Context) {
	accessToken := accessTokenFromRequest(c)
	if accessToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_request", "message": "missing access token"})
		return
	}

	refreshToken := refreshTokenFromRequest(c)

	if err := h.service.Logout(c.Request.Context(), accessToken, refreshToken); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "message": "invalid token"})
		return
	}

	h.clearAuthCookies(c)
	c.JSON(http.StatusOK, gin.H{"code": "ok", "message": "logged out"})
}

// Captcha returns the public Turnstile site key. Empty key means the widget is off.
func (h *Handler) Captcha(c *gin.Context) {
	siteKey := ""
	if h.captcha != nil {
		siteKey = h.captcha.SiteKey()
	}
	c.Header("Cache-Control", "public, max-age=60")
	c.JSON(http.StatusOK, gin.H{"turnstile_site_key": siteKey})
}
