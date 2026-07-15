package auth

import (
	"net/http"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/gin-gonic/gin"
)

const (
	accessTokenCookie  = "access_token"
	refreshTokenCookie = "refresh_token"
	authSessionCookie  = "auth_session"
)

type registerRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type loginRequest struct {
	Email    string `json:"email" binding:"omitempty,email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// Handler exposes auth HTTP endpoints.
type Handler struct {
	service *Service
	cfg     *config.Config
}

// NewHandler creates an auth handler.
func NewHandler(s *Service, cfg *config.Config) *Handler {
	return &Handler{service: s, cfg: cfg}
}

// RegisterRoutes mounts auth routes.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/auth")
	g.POST("/register", h.Register)
	g.POST("/login", h.Login)
	g.POST("/refresh", h.Refresh)
	g.POST("/logout", h.Logout)
	g.GET("/verify-email/:token", h.VerifyEmail)
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
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}

	user, pair, err := h.service.Register(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		switch err {
		case ErrEmailExists:
			c.JSON(http.StatusConflict, gin.H{"code": "email_conflict", "message": "email already registered"})
		case ErrInvalidEmail:
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_email", "message": "invalid email address"})
		case ErrWeakPassword:
			c.JSON(http.StatusBadRequest, gin.H{"code": "weak_password", "message": "password must be at least 8 characters and include uppercase, lowercase, digit and special character"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "registration failed"})
		}
		return
	}

	h.setAuthCookies(c, pair)
	c.JSON(http.StatusCreated, gin.H{"user": user, "expires_in": pair.ExpiresIn})
}

// Login handles user login.
func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}
	if req.Email == "" || req.Password == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "message": "missing credentials"})
		return
	}

	user, pair, err := h.service.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "message": "invalid email or password"})
		return
	}

	h.setAuthCookies(c, pair)
	c.JSON(http.StatusOK, gin.H{"user": user, "expires_in": pair.ExpiresIn})
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
		c.JSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "message": "invalid or expired refresh token"})
		return
	}

	h.setAuthCookies(c, pair)
	c.JSON(http.StatusOK, gin.H{"expires_in": pair.ExpiresIn})
}

// VerifyEmail verifies a user's email address using a single-use token.
func (h *Handler) VerifyEmail(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_token", "message": "verification token is required"})
		return
	}

	if err := h.service.VerifyEmailByToken(c.Request.Context(), token); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_or_expired_token", "message": "verification link is invalid or has expired"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": "verified", "message": "email verified successfully"})
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
