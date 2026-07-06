package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/redis"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// DBPool is the minimal interface required by sqlc generated queries and transactions.
type DBPool interface {
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
	Ping(ctx context.Context) error
}

// worker is a background process that must be started and stopped with the server.
type worker interface {
	Start(ctx context.Context)
	Stop()
}

// Server wraps the gin engine and application config.
type Server struct {
	engine      *gin.Engine
	cfg         *config.Config
	dbPool      DBPool
	redisClient *redis.Client
	workers     []worker
	// shutdownCtx is cancelled before worker Stop() so workers that check
	// ctx.Done() can exit their processing loops early.
	shutdownCtx    context.Context
	cancelShutdown context.CancelFunc
}

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dealsignal_http_requests_total",
			Help: "Total number of HTTP requests by method, path, and status.",
		},
		[]string{"method", "path", "status"},
	)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "dealsignal_http_request_duration_seconds",
			Help:    "HTTP request latencies in seconds by method and path.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
)

func init() {
	_ = prometheus.Register(httpRequestsTotal)
	_ = prometheus.Register(httpRequestDuration)
}

// New creates a configured HTTP server without a database connection (for tests).
func New(cfg *config.Config) *Server {
	return NewWithDB(cfg, nil)
}

// NewWithDB creates a configured HTTP server with a database connection.
func NewWithDB(cfg *config.Config, dbPool DBPool) *Server {
	logger.Init(cfg.LogLevel)

	s := &Server{cfg: cfg, dbPool: dbPool}
	s.shutdownCtx, s.cancelShutdown = context.WithCancel(context.Background())
	if cfg.RedisURL != "" {
		var err error
		s.redisClient, err = redis.NewClient(cfg.RedisURL)
		if err != nil {
			logger.ErrorCtx(context.Background(), "redis connection failed", err)
			os.Exit(1)
		}
	}
	if cfg.LogLevel == "info" {
		gin.SetMode(gin.ReleaseMode)
	} else if cfg.LogLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.TestMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware(cfg.CORSAllowedOrigins))
	r.Use(requestIDMiddleware())
	r.Use(requestLogger())
	r.Use(metricsMiddleware())
	r.Use(middleware.Locale())
	if s.redisClient != nil {
		r.Use(middleware.RateLimitMiddleware(s.redisClient, cfg))
		r.Use(middleware.IdempotencyMiddleware(s.redisClient, cfg))
	}
	r.Use(securityHeadersMiddleware())

	s.engine = r
	s.registerRoutes()
	s.registerObservabilityRoutes()
	return s
}

// Engine exposes the underlying gin engine for testing.
func (s *Server) Engine() *gin.Engine {
	return s.engine
}

// registerWorker records a background worker so it can be gracefully stopped.
func (s *Server) registerWorker(w worker) {
	s.workers = append(s.workers, w)
}

// Run starts the HTTP server on the configured port and blocks until an interrupt
// signal is received or the server fails to start. Shutdown allows in-flight
// requests to complete before background workers are stopped.
func (s *Server) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{
		Addr:           fmt.Sprintf("0.0.0.0:%s", s.cfg.Port),
		Handler:        s.engine,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	// Cancel the shared worker context so workers that respect ctx.Done() can
	// exit their processing loops before Stop() blocks on the current iteration.
	s.cancelShutdown()

	workerStopCtx, workerCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer workerCancel()

	done := make(chan struct{})
	go func() {
		for _, w := range s.workers {
			w.Stop()
		}
		close(done)
	}()

	select {
	case <-done:
	case <-workerStopCtx.Done():
		logger.ErrorCtx(context.Background(), "worker shutdown timed out", nil,
			logger.Attr("timeout", "15s"),
		)
	}
	return nil
}

func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}
		c.Set("requestID", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
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
		requestID := c.GetString("requestID")
		if raw != "" {
			path = path + "?" + raw
		}

		ctx := logger.WithRequestID(c.Request.Context(), requestID)
		logger.InfoCtx(ctx, "http_request",
			logger.Attr("request_id", requestID),
			logger.Attr("client_ip", clientIP),
			logger.Attr("method", method),
			logger.Attr("path", path),
			logger.Attr("status", statusCode),
			logger.Attr("latency_ms", latency.Milliseconds()),
		)
	}
}

func metricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		method := c.Request.Method

		c.Next()

		status := strconv.Itoa(c.Writer.Status())
		httpRequestsTotal.WithLabelValues(method, path, status).Inc()
		httpRequestDuration.WithLabelValues(method, path).Observe(time.Since(start).Seconds())
	}
}

func corsMiddleware(allowedOrigins string) gin.HandlerFunc {
	origins := parseOrigins(allowedOrigins)
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && isAllowedOrigin(origin, origins) {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		}
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, Accept-Language, Idempotency-Key, X-Link-Access, X-Link-Session")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "X-Request-ID, X-RateLimit-Limit, X-RateLimit-Remaining, Retry-After, X-Idempotency-Key, X-Link-Session-Refresh")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// securityHeadersMiddleware adds best-practice HTTP security headers to every response.
func securityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
		c.Writer.Header().Set("X-Frame-Options", "DENY")
		c.Writer.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Writer.Header().Set("X-XSS-Protection", "0") // deprecated but harmless; set to 0 because modern browsers use CSP
		c.Writer.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Next()
	}
}

func parseOrigins(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		o := strings.TrimSpace(p)
		if o != "" {
			out = append(out, o)
		}
	}
	return out
}

func isAllowedOrigin(origin string, allowed []string) bool {
	for _, a := range allowed {
		if a == origin {
			return true
		}
		if strings.HasPrefix(a, "*.") && strings.HasSuffix(origin, a[1:]) {
			return true
		}
	}
	return false
}

// registerObservabilityRoutes mounts /metrics and /debug/pprof when enabled.
// debug/pprof endpoints expose sensitive runtime data and must only be accessible
// from trusted networks in production.
func (s *Server) registerObservabilityRoutes() {
	if s.cfg.MetricsEnabled {
		s.engine.GET("/metrics", gin.WrapH(promhttp.Handler()))
	}
	if s.cfg.PprofEnabled {
		pp := s.engine.Group("/debug/pprof")
		// Restrict pprof to localhost in production. PprofEnabled is designed to
		// be left off in production; this is a defense-in-depth measure for when
		// it is accidentally enabled.
		//
		// We use Request.RemoteAddr (the TCP peer address) instead of
		// ClientIP() which trusts X-Forwarded-For. This prevents IP spoofing
		// through untrusted reverse-proxy headers.
		pp.Use(func(c *gin.Context) {
			remoteIP := c.Request.RemoteAddr
			// Strip port if present (RemoteAddr is "ip:port").
			if idx := strings.LastIndex(remoteIP, ":"); idx != -1 {
				remoteIP = remoteIP[:idx]
			}
			// Remove brackets from IPv6 addresses.
			remoteIP = strings.TrimPrefix(remoteIP, "[")
			remoteIP = strings.TrimSuffix(remoteIP, "]")
			if remoteIP != "::1" && remoteIP != "127.0.0.1" {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.Next()
		})
		pp.GET("/", gin.WrapF(pprof.Index))
		pp.GET("/cmdline", gin.WrapF(pprof.Cmdline))
		pp.GET("/profile", gin.WrapF(pprof.Profile))
		pp.GET("/symbol", gin.WrapF(pprof.Symbol))
		pp.POST("/Symbol", gin.WrapF(pprof.Symbol))
		pp.GET("/trace", gin.WrapF(pprof.Trace))
		pp.GET("/heap", gin.WrapF(pprof.Handler("heap").ServeHTTP))
		pp.GET("/goroutine", gin.WrapF(pprof.Handler("goroutine").ServeHTTP))
		pp.GET("/allocs", gin.WrapF(pprof.Handler("allocs").ServeHTTP))
		pp.GET("/threadcreate", gin.WrapF(pprof.Handler("threadcreate").ServeHTTP))
		pp.GET("/block", gin.WrapF(pprof.Handler("block").ServeHTTP))
		pp.GET("/mutex", gin.WrapF(pprof.Handler("mutex").ServeHTTP))
	}
}
