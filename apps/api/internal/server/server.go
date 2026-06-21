package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/gin-gonic/gin"
)

// Server wraps the gin engine and application config.
type Server struct {
	engine *gin.Engine
	cfg    *config.Config
}

// New creates a configured HTTP server.
func New(cfg *config.Config) *Server {
	if cfg.LogLevel == "info" {
		gin.SetMode(gin.ReleaseMode)
	} else if cfg.LogLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.TestMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(requestLogger())
	r.Use(corsMiddleware())

	s := &Server{engine: r, cfg: cfg}
	s.registerRoutes()
	return s
}

// Engine exposes the underlying gin engine for testing.
func (s *Server) Engine() *gin.Engine {
	return s.engine
}

// Run starts the HTTP server on the configured port.
func (s *Server) Run() error {
	addr := fmt.Sprintf("0.0.0.0:%s", s.cfg.Port)
	return s.engine.Run(addr)
}

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		c.Next()

		latency := time.Since(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()
		if raw != "" {
			path = path + "?" + raw
		}

		fmt.Printf(`{"time":"%s","level":"info","client_ip":"%s","method":"%s","path":"%s","status":%d,"latency_ms":%d}%s`,
			start.Format(time.RFC3339Nano),
			clientIP,
			method,
			path,
			statusCode,
			latency.Milliseconds(),
			"\n",
		)
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
