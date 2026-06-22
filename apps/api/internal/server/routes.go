package server

import (
	"net/http"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/analytics"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/assistant"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/auth"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/evidence"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/ingestion"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/link"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/llm"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/storage"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/upload"
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

		workspaceSvc := workspace.NewService(queries)
		workspaceHandler := workspace.NewHandler(workspaceSvc)
		workspaceHandler.RegisterRoutes(api)

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

			ws := api.Group("/workspaces/:workspaceSlug")
			ws.Use(middleware.Auth())
			ws.Use(workspace.AuthMiddleware(workspaceSvc))
			uploadHandler.RegisterRoutes(ws)
			searchHandler.RegisterRoutes(ws)
			assistantHandler.RegisterRoutes(ws)
			linkHandler.RegisterWorkspaceRoutes(ws)
			analyticsHandler.RegisterWorkspaceRoutes(ws)

			public := s.engine.Group("/api/v1/public")
			linkHandler.RegisterPublicRoutes(public)
		}
	}

	s.engine.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:    "not_found",
			Message: "the requested resource does not exist",
		})
	})
}
