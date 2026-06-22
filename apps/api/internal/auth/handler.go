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

type authResponse struct {
	User  User   `json:"user"`
	Token string `json:"token"`
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
}

// Register handles user registration.
func (h *Handler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}

	user, token, err := h.service.Register(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		switch err {
		case ErrEmailExists:
			c.JSON(http.StatusConflict, gin.H{"code": "email_conflict", "message": err.Error()})
		case ErrInvalidEmail:
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_email", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, authResponse{User: user, Token: token})
}

// Login handles user login.
func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}

	user, token, err := h.service.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, authResponse{User: user, Token: token})
}
