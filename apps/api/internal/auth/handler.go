package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type registerRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type authResponse struct {
	User         User   `json:"user"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// Handler exposes auth HTTP endpoints.
type Handler struct {
	service *Service
}

// NewHandler creates an auth handler.
func NewHandler(s *Service) *Handler {
	return &Handler{service: s}
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

func pairResponse(u User, pair TokenPair) authResponse {
	return authResponse{
		User:         u,
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresIn:    pair.ExpiresIn,
	}
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

	c.JSON(http.StatusCreated, pairResponse(user, pair))
}

// Login handles user login.
func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}

	user, pair, err := h.service.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "message": "invalid email or password"})
		return
	}

	c.JSON(http.StatusOK, pairResponse(user, pair))
}

// Refresh issues a new token pair from a refresh token.
func (h *Handler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}

	pair, err := h.service.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "message": "invalid or expired refresh token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"expires_in":    pair.ExpiresIn,
	})
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

// Logout revokes the current access and refresh tokens.
func (h *Handler) Logout(c *gin.Context) {
	var req logoutRequest
	_ = c.ShouldBindJSON(&req)

	accessToken := ""
	if header := c.GetHeader("Authorization"); len(header) > 7 {
		accessToken = header[7:]
	}
	if accessToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_request", "message": "missing access token"})
		return
	}

	if err := h.service.Logout(c.Request.Context(), accessToken, req.RefreshToken); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "message": "invalid token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": "ok", "message": "logged out"})
}
