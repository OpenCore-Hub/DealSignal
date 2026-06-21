package server

import (
	"net/http"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/auth"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/workspace"
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

	api := s.engine.Group("/api")

	if s.dbPool != nil {
		queries := db.New(s.dbPool)
		authHandler := auth.NewHandler(auth.NewService(queries))
		authHandler.RegisterRoutes(api)

		workspaceHandler := workspace.NewHandler(workspace.NewService(queries))
		workspaceHandler.RegisterRoutes(api)
	}

	s.engine.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:    "not_found",
			Message: "the requested resource does not exist",
		})
	})
}
