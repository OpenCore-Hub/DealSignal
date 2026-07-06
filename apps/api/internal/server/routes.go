package server

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/analytics"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/assistant"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/auth"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/contact"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/dealroom"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/domain"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/evidence"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/ingestion"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/integration"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/link"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/llm"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/mailer"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/marketing"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/notification"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/signal"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/storage"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/suggestions"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/upload"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/workspace"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type notificationAdapter struct {
	svc *notification.Service
}

// closerWorker releases mailer-held resources (e.g. SMTP connection pool) on
// server shutdown. It does no background work between Start and Stop.
type closerWorker struct {
	closer mailer.Closer
}

func (c *closerWorker) Start(_ context.Context) {}
func (c *closerWorker) Stop()                   { _ = c.closer.Close() }

func (a *notificationAdapter) Enqueue(ctx context.Context, workspaceID, userID, channel, subject, body string) error {
	_, err := a.svc.Enqueue(ctx, workspaceID, userID, channel, subject, body)
	return err
}

// ErrorResponse is the standard JSON error shape.
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// HealthResponse is returned by the health check endpoint.
type HealthResponse struct {
	Status  string            `json:"status"`
	Version string            `json:"version"`
	Checks  map[string]string `json:"checks,omitempty"`
}

func (s *Server) registerRoutes() {
	s.engine.GET("/healthz", s.handleHealthz)

	api := s.engine.Group("/api")

	if s.dbPool != nil {
		queries := db.New(s.dbPool)
		var tokenStore auth.TokenStore
		if s.redisClient != nil {
			tokenStore = s.redisClient
		} else {
			tokenStore = auth.NewMemoryTokenStore()
		}

		actualMailer := mailer.New(s.cfg)
		if c, ok := actualMailer.(mailer.Closer); ok {
			s.registerWorker(&closerWorker{closer: c})
		}
		var appMailer mailer.Mailer = actualMailer
		if s.cfg.EmailQueueEnabled && s.redisClient != nil {
			queue := mailer.NewRedisQueue(s.redisClient.RDB(), s.cfg.EmailQueueStream)
			appMailer = mailer.NewQueuedMailer(queue, queries, mailer.ProviderForConfig(s.cfg), s.cfg.EmailQueueMaxAttempts, s.cfg.DefaultBrandName, s.cfg.VerificationTokenTTLHours, mailer.DefaultTemplates())
			emailWorker := mailer.NewWorker(queue, actualMailer, queries, mailer.ProviderForConfig(s.cfg), s.cfg.EmailWorkerCount, s.cfg.EmailWorkerBatchSize, s.cfg.EmailWorkerInterval, s.cfg.RetryBackoffBase, s.cfg.RetryBackoffMax)
			s.registerWorker(emailWorker)
			emailWorker.Start(s.shutdownCtx)

			scheduler := mailer.NewScheduler(s.redisClient.RDB(), s.cfg.EmailQueueStream, s.cfg.EmailWorkerInterval)
			s.registerWorker(scheduler)
			scheduler.Start(s.shutdownCtx)
		}

		if s.cfg.ResendAPIKey != "" && s.cfg.ResendWebhookSecret != "" {
			webhookHandler := mailer.NewResendWebhookHandler(queries, s.cfg.ResendWebhookSecret)
			webhookHandler.RegisterRoutes(api)
		}

		authSvc := auth.NewService(queries, tokenStore,
			auth.WithMailer(appMailer),
			auth.WithAppBaseURL(s.cfg.FrontendURL),
			auth.WithVerificationTokenTTL(time.Duration(s.cfg.VerificationTokenTTLHours)*time.Hour),
		)
		authHandler := auth.NewHandler(authSvc)
		authHandler.RegisterRoutes(api)

		workspaceSvc := workspace.NewService(queries,
			workspace.WithDBPool(s.dbPool),
			workspace.WithMailer(appMailer),
			workspace.WithFrontendURL(s.cfg.FrontendURL),
		)
		workspaceHandler := workspace.NewHandler(workspaceSvc, authSvc)
		workspaceHandler.RegisterRoutes(api)

		domainSvc := domain.NewService(queries, certProvider(s.cfg.CertProvider), s.cfg.CNAMETarget)
		domainHandler := domain.NewHandler(domainSvc, workspaceSvc, authSvc)
		domainHandler.RegisterRoutes(api)

		s.engine.Use(middleware.HostMiddleware(s.cfg.BaseDomain, hostLookup(domainSvc, s.cfg.BaseDomain)))

		public := s.engine.Group("/api/v1/public")
		tracker := mailer.NewTracker(queries, s.cfg.AppBaseURL, s.cfg.EmailTrackingSecret, s.cfg.EmailTrackingTTL, mailer.WithRedis(s.redisClient))
		tracker.RegisterRoutes(public)

		if s.cfg.S3Bucket != "" {
			storageClient, err := storage.NewS3Client(s.cfg)
			if err != nil {
				panic(err)
			}

			var llmClient *llm.Client
			var ingestionEmbedder ingestion.Embedder
			var searchEmbedder search.Embedder
			var chatCompleter assistant.ChatCompleter
			if s.cfg.OpenAIAPIKey != "" {
				llmClient, err = llm.NewClient(llm.Config{
					APIKey:         s.cfg.OpenAIAPIKey,
					BaseURL:        s.cfg.OpenAIBaseURL,
					EmbeddingModel: s.cfg.OpenAIEmbeddingModel,
					ChatModel:      s.cfg.OpenAIChatModel,
					Referer:        s.cfg.OpenAIReferer,
					AppTitle:       s.cfg.OpenAIAppTitle,
				})
				if err != nil {
					panic(err)
				}
				ingestionEmbedder = llmClient
				searchEmbedder = llmClient
				chatCompleter = llmClient
			}

			converter := ingestion.NewConverter(s.cfg.OnlyOfficeURL, s.cfg.OnlyOfficeJWTSecret, storageClient)
			ingestionSvc := ingestion.NewService(queries, storageClient, converter, ingestionEmbedder)
			uploadSvc := upload.NewService(queries, storageClient)
			uploadHandler := upload.NewHandler(uploadSvc, storageClient)

			ingestionWorker := ingestion.NewWorker(ingestionSvc, 1*time.Second)
			s.registerWorker(ingestionWorker)
			ingestionWorker.Start(s.shutdownCtx)

			searchSvc := search.NewService(queries, searchEmbedder)
			searchHandler := search.NewHandler(searchSvc)

			evidenceFormatter := evidence.NewFormatter()
			assistantSvc := assistant.NewService(queries, searchSvc, evidenceFormatter, chatCompleter)
			assistantHandler := assistant.NewHandler(assistantSvc)

			linkSvc := link.NewService(queries, s.dbPool, s.redisClient, appMailer, s.cfg.ViewerBaseURL)
			var dedupChecker analytics.DedupChecker
			if s.redisClient != nil && s.cfg.DedupRedisEnabled {
				dedupChecker = analytics.NewFailoverDedupChecker(s.redisClient, queries, s.cfg.LinkOpenDedupWindow, s.cfg.PageViewDedupWindow)
			} else {
				dedupChecker = analytics.NewFailoverDedupChecker(nil, queries, s.cfg.LinkOpenDedupWindow, s.cfg.PageViewDedupWindow)
			}
			analyticsSvc := analytics.NewService(queries, dedupChecker)
			notificationSvc := notification.NewService(queries, appMailer, s.cfg)
			suggestionSvc := suggestions.NewService(queries, &notificationAdapter{notificationSvc})
			linkHandler := link.NewHandler(linkSvc, analyticsSvc, suggestionSvc, storageClient, s.cfg)
			analyticsHandler := analytics.NewHandler(analyticsSvc, s.cfg)
			assistantPublicHandler := assistant.NewPublicHandler(assistantSvc, linkSvc, s.cfg)

			dealroomSvc := dealroom.NewService(queries, s.dbPool)
			dealroomHandler := dealroom.NewHandler(dealroomSvc)

			suggestionHandler := suggestions.NewHandler(suggestionSvc)
			signalSvc := signal.NewService(queries)
			signalHandler := signal.NewHandler(signalSvc)

			contactSvc := contact.NewService(queries)
			contactHandler := contact.NewHandler(contactSvc)

			marketingSvc := marketing.NewService(queries, appMailer, mailer.ProviderForConfig(s.cfg))
			marketingHandler := marketing.NewHandler(marketingSvc)

			ws := api.Group("/workspaces/:workspaceSlug")
			ws.Use(middleware.Auth(authSvc))
			ws.Use(workspace.AuthMiddleware(workspaceSvc))
			uploadHandler.RegisterRoutes(ws)
			searchHandler.RegisterRoutes(ws)
			assistantHandler.RegisterRoutes(ws)
			linkHandler.RegisterWorkspaceRoutes(ws)
			analyticsHandler.RegisterWorkspaceRoutes(ws)
			dealroomHandler.RegisterWorkspaceRoutes(ws)
			suggestionHandler.RegisterRoutes(ws)
			signalHandler.RegisterRoutes(ws)
			contactHandler.RegisterRoutes(ws)
			marketingHandler.RegisterRoutes(ws)

			notificationWorker := notification.NewWorker(notificationSvc, 30*time.Second)
			s.registerWorker(notificationWorker)
			notificationWorker.Start(s.shutdownCtx)

			renewalWorker := domain.NewRenewalWorker(domainSvc, 1*time.Hour, 7*24*time.Hour)
			s.registerWorker(renewalWorker)
			renewalWorker.Start(s.shutdownCtx)

			integrationSvc := integration.NewService(queries, s.cfg)
			integrationHandler := integration.NewHandler(integrationSvc)
			integrationHandler.RegisterRoutes(ws)

			hubSpotWorker := integration.NewWorker(integrationSvc, 30*time.Second)
			s.registerWorker(hubSpotWorker)
			hubSpotWorker.Start(s.shutdownCtx)

			integrationHandler.RegisterOAuthRoutes(public)
			linkHandler.RegisterPublicRoutes(public)
			dealroomHandler.RegisterPublicRoutes(public)
			assistantPublicHandler.RegisterPublicRoutes(public)
		}
	}

	s.engine.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:    "not_found",
			Message: "the requested resource does not exist",
		})
	})
}

func hostLookup(svc *domain.Service, baseDomain string) middleware.HostLookup {
	return func(ctx context.Context, host string) (string, error) {
		if suffix := "." + baseDomain; strings.HasSuffix(host, suffix) {
			slug := strings.TrimSuffix(host, suffix)
			t, err := svc.GetTenantBySlug(ctx, slug)
			if err != nil {
				return "", err
			}
			return uuid.UUID(t.ID.Bytes).String(), nil
		}
		return svc.ResolveHost(ctx, host)
	}
}

func certProvider(name string) domain.CertificateProvider {
	if name == "selfsigned" {
		return domain.SelfSignedProvider{}
	}
	return domain.NoopProvider{}
}

func (s *Server) handleHealthz(c *gin.Context) {
	checks := make(map[string]string)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	status := "ok"
	if s.dbPool != nil {
		if err := s.dbPool.Ping(ctx); err != nil {
			checks["database"] = "error: " + err.Error()
			status = "degraded"
		} else {
			checks["database"] = "ok"
		}
	}
	if s.redisClient != nil {
		if err := s.redisClient.RDB().Ping(ctx).Err(); err != nil {
			checks["redis"] = "error: " + err.Error()
			status = "degraded"
		} else {
			checks["redis"] = "ok"
		}
	}
	if s.cfg.ResendAPIKey != "" {
		if err := checkResend(ctx, s.cfg); err != nil {
			checks["resend"] = "error: " + err.Error()
			status = "degraded"
		} else {
			checks["resend"] = "ok"
		}
	}
	if s.cfg.SMTPHost != "" {
		if err := checkSMTP(ctx, s.cfg); err != nil {
			checks["smtp"] = "error: " + err.Error()
			status = "degraded"
		} else {
			checks["smtp"] = "ok"
		}
	}

	code := http.StatusOK
	if status != "ok" {
		code = http.StatusServiceUnavailable
	}
	c.JSON(code, HealthResponse{Status: status, Version: s.cfg.Version, Checks: checks})
}

func checkResend(ctx context.Context, cfg *config.Config) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.resend.com/emails", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.ResendAPIKey)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func checkSMTP(ctx context.Context, cfg *config.Config) error {
	addr := net.JoinHostPort(cfg.SMTPHost, cfg.SMTPPort)
	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	return conn.Close()
}
