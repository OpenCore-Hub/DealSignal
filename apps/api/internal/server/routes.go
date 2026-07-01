package server

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/analytics"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/assistant"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/auth"
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
		var tokenStore auth.TokenStore
		if s.redisClient != nil {
			tokenStore = s.redisClient
		} else {
			tokenStore = auth.NewMemoryTokenStore()
		}
		authSvc := auth.NewService(queries, tokenStore,
			auth.WithMailer(mailer.New(s.cfg)),
			auth.WithAppBaseURL(s.cfg.FrontendURL),
		)
		authHandler := auth.NewHandler(authSvc)
		authHandler.RegisterRoutes(api)

		workspaceSvc := workspace.NewService(queries)
		workspaceHandler := workspace.NewHandler(workspaceSvc, authSvc)
		workspaceHandler.RegisterRoutes(api)

		domainSvc := domain.NewService(queries, certProvider(s.cfg.CertProvider), s.cfg.CNAMETarget)
		domainHandler := domain.NewHandler(domainSvc, workspaceSvc, authSvc)
		domainHandler.RegisterRoutes(api)

		s.engine.Use(middleware.HostMiddleware(s.cfg.BaseDomain, hostLookup(domainSvc, s.cfg.BaseDomain)))

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
			ingestionWorker.Start(context.Background())

			searchSvc := search.NewService(queries, searchEmbedder)
			searchHandler := search.NewHandler(searchSvc)

			evidenceFormatter := evidence.NewFormatter()
			assistantSvc := assistant.NewService(queries, searchSvc, evidenceFormatter, chatCompleter)
			assistantHandler := assistant.NewHandler(assistantSvc)

			linkSvc := link.NewService(queries, s.dbPool, s.redisClient, mailer.New(s.cfg), s.cfg.ViewerBaseURL)
			analyticsSvc := analytics.NewService(queries)
			notificationSvc := notification.NewService(queries, s.cfg)
			suggestionSvc := suggestions.NewService(queries, &notificationAdapter{notificationSvc})
			linkHandler := link.NewHandler(linkSvc, analyticsSvc, suggestionSvc, storageClient, s.cfg)
			analyticsHandler := analytics.NewHandler(analyticsSvc, s.cfg)

			dealroomSvc := dealroom.NewService(queries, s.dbPool)
			dealroomHandler := dealroom.NewHandler(dealroomSvc)

			suggestionHandler := suggestions.NewHandler(suggestionSvc)
			signalSvc := signal.NewService(queries)
			signalHandler := signal.NewHandler(signalSvc)

			contactSvc := contact.NewService(queries)
			contactHandler := contact.NewHandler(contactSvc)

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

			notificationWorker := notification.NewWorker(notificationSvc, 30*time.Second)
			s.registerWorker(notificationWorker)
			notificationWorker.Start(context.Background())

			renewalWorker := domain.NewRenewalWorker(domainSvc, 1*time.Hour, 7*24*time.Hour)
			s.registerWorker(renewalWorker)
			renewalWorker.Start(context.Background())

			integrationSvc := integration.NewService(queries, s.cfg)
			integrationHandler := integration.NewHandler(integrationSvc)
			integrationHandler.RegisterRoutes(ws)

			hubSpotWorker := integration.NewWorker(integrationSvc, 30*time.Second)
			s.registerWorker(hubSpotWorker)
			hubSpotWorker.Start(context.Background())

			public := s.engine.Group("/api/v1/public")
			integrationHandler.RegisterOAuthRoutes(public)
			linkHandler.RegisterPublicRoutes(public)
			dealroomHandler.RegisterPublicRoutes(public)
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
