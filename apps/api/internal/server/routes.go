package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorResponse is the standard JSON error shape.
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// HealthResponse is returned by the health check endpoint.
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

func (s *Server) registerRoutes() {
	s.engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, HealthResponse{Status: "ok", Version: s.cfg.Version})
	})

	s.engine.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:    "not_found",
			Message: "the requested resource does not exist",
		})
	})
}
