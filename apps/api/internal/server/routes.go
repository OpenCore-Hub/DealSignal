package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/action"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/analytics"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/auth"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/compliance"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/contact"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/crm"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/dealroom"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/docling"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/domain"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/events"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/ingestion"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/integration"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/knowledge"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/link"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/llm"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/mailer"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/marketing"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/nda"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/notification"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/radar"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/signal"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/sse"
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

// analyticsSignalSyncer adapts the signal service to the analytics SignalSyncer
// interface so that dashboard stats always reflect the latest synced signals.
type analyticsSignalSyncer struct {
	svc *signal.Service
}

func (a *analyticsSignalSyncer) GetFeed(ctx context.Context, workspaceID, userID string) (analytics.SignalFeed, error) {
	feed, err := a.svc.GetFeed(ctx, workspaceID, userID)
	if err != nil {
		return analytics.SignalFeed{}, err
	}
	return analytics.SignalFeed{Signals: feed.Signals, Actions: feed.Actions}, nil
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

func (s *Server) registerRoutes() error {
	s.engine.GET("/healthz", s.handleHealthz)
	s.engine.GET("/readyz", s.handleReadyz)

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

		authMailTimeout := s.cfg.SMTPTimeout
		if s.cfg.ResendAPIKey != "" && s.cfg.ResendTimeout > 0 {
			authMailTimeout = s.cfg.ResendTimeout
		}
		if authMailTimeout <= 0 {
			authMailTimeout = 10 * time.Second
		}
		// Background-only budget (Register does not wait). Headroom for provider retries.
		authMailTimeout *= 3
		authSvc := auth.NewService(queries, tokenStore,
			auth.WithMailer(appMailer),
			auth.WithAppBaseURL(s.cfg.FrontendURL),
			auth.WithVerificationTokenTTL(time.Duration(s.cfg.VerificationTokenTTLHours)*time.Hour),
			auth.WithSendTimeout(authMailTimeout),
		)
		authHandler := auth.NewHandler(authSvc, s.cfg)
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
				return fmt.Errorf("s3 client: %w", err)
			}

			var llmClient *llm.Client
			if s.cfg.OpenAIAPIKey != "" {
				llmClient, err = llm.NewClient(llm.Config{
					APIKey:    s.cfg.OpenAIAPIKey,
					BaseURL:   s.cfg.OpenAIBaseURL,
					ChatModel: s.cfg.OpenAIChatModel,
					Referer:   s.cfg.OpenAIReferer,
					AppTitle:  s.cfg.OpenAIAppTitle,
				})
				if err != nil {
					return fmt.Errorf("llm client: %w", err)
				}
				slog.Info("llm client configured",
					"chat_model", s.cfg.OpenAIChatModel,
					"base_url_set", s.cfg.OpenAIBaseURL != "",
				)
			}
			converter := ingestion.NewConverter(s.cfg.OnlyOfficeURL, s.cfg.OnlyOfficeJWTSecret, storageClient)
			ingestionSvc := ingestion.NewService(queries, storageClient, converter).
				WithTableIngest(s.cfg.TableIngest.Enabled, ingestion.TableIngestLimits{
					MaxSheets:       s.cfg.TableIngest.MaxSheets,
					MaxRowsPerSheet: s.cfg.TableIngest.MaxRowsPerSheet,
					MaxRowsPerFile:  s.cfg.TableIngest.MaxRowsPerFile,
				})
			uploadSvc := upload.NewService(queries, storageClient, s.dbPool)
			uploadHandler := upload.NewHandler(uploadSvc, storageClient, workspaceSvc, s.cfg.AppBaseURL)

			ingestionWorker := ingestion.NewWorker(ingestionSvc, 1*time.Second)
			s.registerWorker(ingestionWorker)
			ingestionWorker.Start(s.shutdownCtx)

			notificationSvc := notification.NewService(s.dbPool, queries, appMailer, s.cfg)
			notificationSvc.SetRuleEngine(notification.NewRuleEngine(queries, func(ctx context.Context, wsID, userID, channel, subject, body string, opts ...notification.EnqueueOption) error {
				_, err := notificationSvc.Enqueue(ctx, wsID, userID, channel, subject, body, opts...)
				return err
			}))

			var suggestionEnricher suggestions.Enricher
			if llmClient != nil {
				suggestionEnricher = suggestions.NewLLMEnricher(llmClient)
			}
			suggestionRuleEngine, err := suggestions.NewRuleEngine(s.cfg.SignalRulesPath)
			if err != nil {
				return fmt.Errorf("suggestion rule engine: %w", err)
			}

			featureStore := suggestions.NewFeatureStore(queries)
			suggestionSvc := suggestions.NewService(queries, &notificationAdapter{notificationSvc}, suggestionRuleEngine,
				suggestions.WithEnricher(suggestionEnricher),
				suggestions.WithFeatureStore(featureStore),
			)

			if s.cfg.FeatureWorkerEnabled {
				featureWorker := suggestions.NewFeatureWorker(featureStore, s.cfg.FeatureWorkerInterval, 100)
				s.registerWorker(featureWorker)
				featureWorker.Start(s.shutdownCtx)
			}

			var eventBus events.Bus = events.NewNoOpBus()
			if s.cfg.EventsEnabled && s.redisClient != nil {
				eventBus = events.NewRedisBus(s.redisClient.GoRedis(), s.cfg.EventsStreamName, s.cfg.EventsConsumerGroup)
			}

			suggestionWorker := suggestions.NewWorker(suggestionSvc, s.dbPool, eventBus, suggestions.DefaultWorkerConfig())
			s.registerWorker(suggestionWorker)
			suggestionWorker.Start(s.shutdownCtx)

			actionSyncer := action.NewSyncer(queries)

			ndaSvc := nda.NewService(queries, storageClient, appMailer)
			linkSvc := link.NewService(queries, s.dbPool, s.redisClient, appMailer, s.cfg.ViewerBaseURL, s.cfg, notificationSvc, nil,
				link.WithActionSyncer(actionSyncer),
				link.WithNDAService(ndaSvc),
				link.WithFormalAskInsights(link.FormalAskInsightsAdapter{Suggestions: suggestionSvc}),
			)
			ndaHandler := nda.NewHandler(ndaSvc)
			var dedupChecker analytics.DedupChecker
			if s.redisClient != nil && s.cfg.DedupRedisEnabled {
				dedupChecker = analytics.NewFailoverDedupChecker(s.redisClient, queries, s.cfg.LinkOpenDedupWindow, s.cfg.PageViewDedupWindow)
			} else {
				dedupChecker = analytics.NewFailoverDedupChecker(nil, queries, s.cfg.LinkOpenDedupWindow, s.cfg.PageViewDedupWindow)
			}
			signalSvc := signal.NewService(queries, signal.WithActionSyncer(actionSyncer))

			if s.cfg.EventsEnabled {
				signalConsumer := events.NewSignalConsumer(signalSvc)
				consumerWorker := events.NewConsumerWorker(eventBus, signalConsumer.Handle)
				s.registerWorker(consumerWorker)
				consumerWorker.Start(s.shutdownCtx)
			}

			signalSyncer := &analyticsSignalSyncer{svc: signalSvc}
			analyticsSvc := analytics.NewService(queries, dedupChecker, s.cfg, signalSyncer)
			if s.redisClient != nil {
				analyticsSvc.WithCache(analytics.NewRedisCache(s.redisClient))
			}
			linkSvc.SetAskSecurityRecorder(link.AnalyticsSecurityRecorder{Svc: analyticsSvc})
			linkHandler := link.NewHandler(linkSvc, analyticsSvc, suggestionSvc, storageClient, s.cfg)
			expiryReminder := link.NewExpiryReminder(queries, notificationSvc, 6*time.Hour)
			s.registerWorker(expiryReminder)
			expiryReminder.Start(s.shutdownCtx)

			analyticsRetention := analytics.NewRetentionCleaner(s.dbPool, queries, s.cfg.AccessLogsRetentionDays, s.cfg.PageViewsRetentionDays, s.cfg.SecurityEventsRetentionDays)
			s.registerWorker(analyticsRetention)
			analyticsRetention.Start(s.shutdownCtx)

			knowledgeRetention := knowledge.NewRetentionCleaner(queries, storageClient, s.cfg.KnowledgeQARetentionDays)
			s.registerWorker(knowledgeRetention)
			knowledgeRetention.Start(s.shutdownCtx)

			heatScoreWorker := analytics.NewHeatScoreRefreshWorker(s.dbPool, s.cfg.HeatScoreRefreshInterval)
			s.registerWorker(heatScoreWorker)
			heatScoreWorker.Start(s.shutdownCtx)

			formalPublishWorker := link.NewFormalPublishWorker(
				linkSvc,
				s.cfg.FormalPublishInterval,
				int32(s.cfg.FormalPublishBatchSize),
			)
			s.registerWorker(formalPublishWorker)
			formalPublishWorker.Start(s.shutdownCtx)

			// SSE realtime push
			sseHub := sse.NewHub(s.redisClient.GoRedis())
			sseHandler := sse.NewHandler(sseHub)
			ssePublisher := sse.NewLinkPublisher(sseHub)
			linkHandler.SetEventPublisher(ssePublisher)

			// CRM aggregation worker (30min window).
			crmAggregator := crm.NewWindowAggregator(queries, 30*time.Minute)
			s.registerWorker(crmAggregator)
			crmAggregator.Start(s.shutdownCtx)

			// CRM webhook endpoint (CRM → DealSignal deal stage changes).
			webhookHandler := crm.NewWebhookHandler()
			public.POST("/webhooks/crm/deal-stage", webhookHandler.HandleDealStageChange)
			analyticsHandler := analytics.NewHandler(analyticsSvc, s.cfg)

			doclingClient := docling.NewClient(
				s.cfg.DoclingRAG.BaseURL,
				s.cfg.DoclingRAG.PlatformAdminKey,
				s.cfg.DoclingRAG.HTTPTimeout,
			)
			linkSvc.SetFormalAskEntitlement(doclingFormalAskEntitlement{
				client:    doclingClient,
				planCodes: formalPlanCodeSet(s.cfg.FormalAskEntitledPlanCodes),
				stubPlan:  strings.ToLower(strings.TrimSpace(s.cfg.FormalAskEntitlementStubPlan)),
				appEnv:    s.cfg.AppEnv,
			})
			knowledgeSvc := knowledge.NewService(
				queries,
				s.cfg.DoclingRAG,
				doclingClient,
				storageClient,
				s.cfg.URLSigningSecret,
			).WithDBPool(s.dbPool).WithPreviewPDFConverter(converter).
				WithRetentionDays(s.cfg.KnowledgeQARetentionDays)
			if llmClient != nil {
				knowledgeSvc = knowledgeSvc.WithFollowUpLLM(llmClient)
			}
			knowledgeSvc = knowledgeSvc.WithQueryRewrite(s.cfg.KnowledgeQARewriteEnabled)
			knowledgeSvc = knowledgeSvc.WithTableLane(s.cfg.KnowledgeQATableLaneEnabled)
			knowledgeSvc = knowledgeSvc.WithMultiHop(s.cfg.KnowledgeQAMultiHopEnabled)
			if s.cfg.KnowledgeQARewriteCacheEnabled {
				if s.redisClient != nil {
					knowledgeSvc = knowledgeSvc.WithRewriteCache(knowledge.NewKVRewriteCache(s.redisClient))
				} else {
					knowledgeSvc = knowledgeSvc.WithRewriteCache(knowledge.NewMemoryRewriteCache())
				}
			}
			var knowledgeOpts []knowledge.HandlerOption
			if s.redisClient != nil {
				knowledgeOpts = append(knowledgeOpts,
					knowledge.WithAskAdmission(
						knowledge.NewRedisAskAdmission(s.redisClient, s.redisClient, s.cfg.KnowledgeQAMemberRPM),
					),
					knowledge.WithFollowUpAdmission(
						knowledge.NewRedisFollowUpAdmission(s.redisClient, s.redisClient, s.cfg.KnowledgeQAFollowUpRPM),
					),
				)
			} else {
				knowledgeOpts = append(knowledgeOpts,
					knowledge.WithAskAdmission(
						knowledge.NewMemoryAskAdmission(s.cfg.KnowledgeQAMemberRPM),
					),
					knowledge.WithFollowUpAdmission(
						knowledge.NewMemoryFollowUpAdmission(s.cfg.KnowledgeQAFollowUpRPM),
					),
				)
			}
			knowledgeHandler := knowledge.NewHandler(knowledgeSvc,
				append(knowledgeOpts, knowledge.WithHTTPWriteTimeout(s.cfg.HTTPWriteTimeout))...,
			)
			if knowledgeSvc.Enabled() {
				knowledgeWorker := knowledge.NewWorker(knowledgeSvc, 3*time.Second)
				s.registerWorker(knowledgeWorker)
				knowledgeWorker.Start(s.shutdownCtx)
			}
			linkHandler.SetKnowledgeService(knowledgeSvc)

			dealroomOpts := []dealroom.ServiceOption{
				dealroom.WithActionSyncer(actionSyncer),
				dealroom.WithRateLimiter(s.redisClient),
				dealroom.WithKnowledgeEnqueuer(knowledgeSvc),
			}
			if s.redisClient != nil {
				dealroomOpts = append(dealroomOpts, dealroom.WithListCache(dealroom.NewRedisListCache(s.redisClient)))
			}
			dealroomSvc := dealroom.NewService(queries, s.dbPool, s.cfg, dealroomOpts...)
			dealroomHandler := dealroom.NewHandler(dealroomSvc)
			// Access-log and Q&A writers soft-invalidate list cards (debounced).
			analyticsSvc.WithRoomListInvalidator(dealroomSvc)
			linkSvc.WithRoomListInvalidator(dealroomSvc)

			complianceSvc := compliance.NewService(queries, s.dbPool, s.cfg)
			complianceHandler := compliance.NewHandler(complianceSvc, workspaceSvc)

			suggestionHandler := suggestions.NewHandler(suggestionSvc)
			signalHandler := signal.NewHandler(signalSvc)
			radarHandler := radar.NewHandler(radar.NewService(queries, signalSvc))

			contactOpts := []contact.ServiceOption{}
			if s.redisClient != nil {
				contactOpts = append(contactOpts, contact.WithCache(analytics.NewRedisCache(s.redisClient)))
			}
			contactSvc := contact.NewService(queries, contactOpts...)
			contactHandler := contact.NewHandler(contactSvc)

			marketingSvc := marketing.NewService(queries, appMailer, mailer.ProviderForConfig(s.cfg))
			marketingHandler := marketing.NewHandler(marketingSvc)

			ws := api.Group("/workspaces/:workspaceSlug")
			ws.Use(middleware.Auth(authSvc))
			ws.Use(workspace.AuthMiddleware(workspaceSvc))
			uploadHandler.RegisterRoutes(ws)
			linkHandler.RegisterWorkspaceRoutes(ws)
			ndaHandler.RegisterRoutes(ws)
			analyticsHandler.RegisterWorkspaceRoutes(ws)
			ws.GET("/reverse-funnel", linkHandler.ReverseFunnel)
			ws.GET("/events", sseHandler.StreamEvents)
			dealroomHandler.RegisterWorkspaceRoutes(ws)
			knowledgeHandler.RegisterWorkspaceRoutes(ws)
			complianceHandler.RegisterRoutes(ws)
			suggestionHandler.RegisterRoutes(ws)
			signalHandler.RegisterRoutes(ws)
			radarHandler.RegisterRoutes(ws)
			contactHandler.RegisterRoutes(ws)
			marketingHandler.RegisterRoutes(ws)

			notificationWorker := notification.NewWorker(notificationSvc, 30*time.Second)
			s.registerWorker(notificationWorker)
			notificationWorker.Start(s.shutdownCtx)

			digestRunner := notification.NewDigestRunner(
				queries,
				notificationSvc,
				notification.NewInsightsOverviewAdapter(func(ctx context.Context, workspaceID string, days int) (notification.DigestOverview, error) {
					ov, err := analyticsSvc.InsightsOverview(ctx, workspaceID, days)
					if err != nil {
						return notification.DigestOverview{}, err
					}
					docs := make([]string, 0, 3)
					for i, d := range ov.TopDocuments {
						if i >= 3 {
							break
						}
						if d.Title != "" {
							docs = append(docs, d.Title)
						}
					}
					contacts := make([]string, 0, 3)
					for i, c := range ov.TopContacts {
						if i >= 3 {
							break
						}
						if c.Email != "" {
							contacts = append(contacts, c.Email)
						}
					}
					out := notification.DigestOverview{
						PeriodOpens:                 ov.PeriodOpens,
						PreviousPeriodOpens:         ov.PreviousPeriodOpens,
						PeriodUniqueVisitors:        ov.PeriodUniqueVisitors,
						PeriodMedianDurationSeconds: ov.PeriodMedianDurationSeconds,
						HotLinks:                    ov.TierCounts["hot"],
						WarmLinks:                   ov.TierCounts["warm"],
						TopDocuments:                docs,
						TopContacts:                 contacts,
					}
					if pack := ov.ScenarioPack; pack != nil {
						out.Scenario = pack.Scenario
						out.ScenarioDepth = pack.Depth
						out.ScenarioRoomCount = pack.RoomCount
						out.ScenarioLabel = pack.Label
						out.ScenarioLead = pack.DigestLead
						for _, kpi := range pack.KPIs {
							out.ScenarioKPIs = append(out.ScenarioKPIs, notification.DigestScenarioKPI{
								ID: kpi.ID, Value: kpi.Value,
							})
						}
					}
					return out, nil
				}),
				s.cfg.InsightsDigestHourUTC,
			)
			digestWorker := notification.NewDigestWorker(digestRunner, s.cfg.InsightsDigestInterval)
			s.registerWorker(digestWorker)
			digestWorker.Start(s.shutdownCtx)

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
		}
	}

	s.engine.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:    "not_found",
			Message: "the requested resource does not exist",
		})
	})
	return nil
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

func (s *Server) handleReadyz(c *gin.Context) {
	status := "ready"
	code := http.StatusOK

	if s.dbPool != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := s.dbPool.Ping(ctx); err != nil {
			status = "not_ready"
			code = http.StatusServiceUnavailable
		}
	}

	c.JSON(code, HealthResponse{Status: status, Version: s.cfg.Version})
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

type doclingFormalAskEntitlement struct {
	client    *docling.Client
	planCodes map[string]struct{}
	stubPlan  string
	appEnv    string
}

func (e doclingFormalAskEntitlement) stubEntitled() (bool, bool) {
	if e.stubPlan == "" || strings.ToLower(strings.TrimSpace(e.appEnv)) == "production" {
		return false, false
	}
	_, ok := e.planCodes[e.stubPlan]
	return ok, true
}

func (e doclingFormalAskEntitlement) IsFormalAskEntitled(ctx context.Context, tenantSlug string) (bool, error) {
	if e.client == nil || !e.client.Enabled() {
		if ok, used := e.stubEntitled(); used {
			return ok, nil
		}
		return false, nil
	}
	ent, err := e.client.GetEntitlements(ctx, tenantSlug)
	if err != nil {
		if ok, used := e.stubEntitled(); used {
			return ok, nil
		}
		return false, err
	}
	_, ok := e.planCodes[strings.ToLower(strings.TrimSpace(ent.PlanCode))]
	return ok, nil
}

func formalPlanCodeSet(codes []string) map[string]struct{} {
	out := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		code = strings.ToLower(strings.TrimSpace(code))
		if code != "" {
			out[code] = struct{}{}
		}
	}
	return out
}
