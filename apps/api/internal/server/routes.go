package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/analytics"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/assistant"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/auth"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/dealroom"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/domain"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/evidence"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/ingestion"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/link"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/llm"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/suggestions"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/storage"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/upload"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/workspace"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

		workspaceSvc := workspace.NewService(queries)
		workspaceHandler := workspace.NewHandler(workspaceSvc)
		workspaceHandler.RegisterRoutes(api)

		domainSvc := domain.NewService(queries, certProvider(s.cfg.CertProvider), s.cfg.CNAMETarget)
		domainHandler := domain.NewHandler(domainSvc, workspaceSvc)
		domainHandler.RegisterRoutes(api)

		s.engine.Use(middleware.HostMiddleware(s.cfg.BaseDomain, hostLookup(domainSvc, s.cfg.BaseDomain)))

		if s.cfg.S3Bucket != "" {
			storageClient, err := storage.NewS3Client(s.cfg)
			if err != nil {
				panic(err)
			}

			var llmClient *llm.Client
			if s.cfg.OpenAIAPIKey != "" {
				llmClient, err = llm.NewClient(llm.Config{
					APIKey:         s.cfg.OpenAIAPIKey,
					BaseURL:        s.cfg.OpenAIBaseURL,
					EmbeddingModel: s.cfg.OpenAIEmbeddingModel,
					ChatModel:      s.cfg.OpenAIChatModel,
				})
				if err != nil {
					panic(err)
				}
			}

			converter := ingestion.NewConverter(s.cfg.OnlyOfficeURL, storageClient)
			ingestionSvc := ingestion.NewService(queries, storageClient, converter, llmClient)
			uploadSvc := upload.NewService(queries, storageClient)
			uploadHandler := upload.NewHandler(uploadSvc, ingestionSvc, storageClient)

			searchSvc := search.NewService(queries, llmClient)
			searchHandler := search.NewHandler(searchSvc)

			evidenceFormatter := evidence.NewFormatter()
			assistantSvc := assistant.NewService(queries, searchSvc, evidenceFormatter, llmClient)
			assistantHandler := assistant.NewHandler(assistantSvc)

			linkSvc := link.NewService(queries)
			analyticsSvc := analytics.NewService(queries)
			linkHandler := link.NewHandler(linkSvc, analyticsSvc)
			analyticsHandler := analytics.NewHandler(analyticsSvc)

			dealroomSvc := dealroom.NewService(queries)
			dealroomHandler := dealroom.NewHandler(dealroomSvc)

			ws := api.Group("/workspaces/:workspaceSlug")
			ws.Use(middleware.Auth())
			ws.Use(workspace.AuthMiddleware(workspaceSvc))
			uploadHandler.RegisterRoutes(ws)
			searchHandler.RegisterRoutes(ws)
			assistantHandler.RegisterRoutes(ws)
			linkHandler.RegisterWorkspaceRoutes(ws)
			analyticsHandler.RegisterWorkspaceRoutes(ws)
			dealroomHandler.RegisterWorkspaceRoutes(ws)

			suggestionSvc := suggestions.NewService(queries)
			suggestionHandler := suggestions.NewHandler(suggestionSvc)
			suggestionHandler.RegisterRoutes(ws)

			public := s.engine.Group("/api/v1/public")
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
