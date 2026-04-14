package api

import (
	"github.com/allopze/reform-lab/apps/api/internal/api/handlers"
	"github.com/allopze/reform-lab/apps/api/internal/api/middleware"
	"github.com/allopze/reform-lab/apps/api/internal/auth"
	"github.com/allopze/reform-lab/apps/api/internal/email"
	"github.com/allopze/reform-lab/apps/api/internal/observability"
	"github.com/allopze/reform-lab/apps/api/internal/orchestrator"
	"github.com/allopze/reform-lab/apps/api/internal/queue"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/allopze/reform-lab/apps/api/internal/storage"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

// Deps groups all dependencies needed to wire up routes.
type Deps struct {
	Logger                         zerolog.Logger
	Metrics                        *observability.Metrics
	Store                          storage.Store
	Files                          repository.FileRepository
	Jobs                           repository.JobRepository
	Artifacts                      repository.ArtifactRepository
	Audit                          repository.AuditRepository
	Users                          repository.UserRepository
	Dashboard                      repository.DashboardRepository
	SiteSettings                   repository.SiteSettingRepository
	EmailTemplates                 repository.EmailTemplateRepository
	Webhooks                       repository.WebhookRepository
	EmailService                   *email.Service
	Queue                          queue.JobQueue
	Orchestrator                   *orchestrator.Service
	AuthService                    *auth.Service
	CORSOrigin                     string
	ExposeMetrics                  bool
	MetricsToken                   string
	TrustProxyHeaders              bool
	ArtifactTTLHours               int
	ArtifactTTLByFamily            map[string]int
	UserUploadsPerMinute           int
	UserUploadBurst                int
	UserConversionsPerMinute       int
	UserConversionBurst            int
	MaxActiveJobsPerGuestSession   int
	GuestCumulativeQuotaBytes      int64
	RegisteredCumulativeQuotaBytes int64
}

// NewRouter creates the chi router with all middleware and routes.
func NewRouter(d Deps) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.Recoverer)
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.RequestID)
	r.Use(middleware.Logging(d.Logger))
	r.Use(middleware.MetricsMiddleware(d.Metrics))
	r.Use(middleware.CORS(d.CORSOrigin))
	r.Use(middleware.RateLimit(20, 40, d.TrustProxyHeaders, d.Metrics))
	r.Use(middleware.MaxBodySize(500 * 1024 * 1024)) // 500 MB global limit

	if d.ExposeMetrics {
		metricsHandler := promhttp.Handler()
		if d.MetricsToken != "" {
			r.Handle("/metrics", middleware.BearerTokenAuth(d.MetricsToken)(metricsHandler))
		} else {
			r.Handle("/metrics", metricsHandler)
		}
	}

	// API routes
	r.Route("/api", func(r chi.Router) {
		r.Get("/health", handlers.PublicHealth())
		footer := &handlers.FooterHandler{Settings: d.SiteSettings}
		uploadPolicy := &handlers.UploadPolicyHandler{
			Settings:                       d.SiteSettings,
			Files:                          d.Files,
			GuestCumulativeQuotaBytes:      d.GuestCumulativeQuotaBytes,
			RegisteredCumulativeQuotaBytes: d.RegisteredCumulativeQuotaBytes,
		}
		r.Get("/footer-message", footer.Get)

		// Auth routes (public)
		authH := &handlers.AuthHandler{
			Auth:   d.AuthService,
			Email:  d.EmailService,
			Queue:  d.Queue,
			Logger: d.Logger,
		}
		r.With(middleware.RateLimit(1, 5, d.TrustProxyHeaders, d.Metrics)).Post("/auth/register", authH.Register)
		r.With(middleware.RateLimit(1, 5, d.TrustProxyHeaders, d.Metrics)).Post("/auth/login", authH.Login)
		r.Post("/auth/logout", authH.Logout)

		// File and conversion routes — optional authentication keeps ownership when
		// available but still allows anonymous use of the public app flow.
		r.Group(func(r chi.Router) {
			r.Use(middleware.OptionalAuth(d.AuthService, d.Users))
			r.Use(middleware.OptionalGuestSession)
			uploadQuota := middleware.UserOrIPRateLimit(d.UserUploadsPerMinute, d.UserUploadBurst, d.TrustProxyHeaders, d.Metrics)
			conversionQuota := middleware.UserOrIPRateLimit(d.UserConversionsPerMinute, d.UserConversionBurst, d.TrustProxyHeaders, d.Metrics)

			upload := &handlers.UploadHandler{
				Settings:                       d.SiteSettings,
				Store:                          d.Store,
				Files:                          d.Files,
				Audit:                          d.Audit,
				Logger:                         d.Logger,
				Metrics:                        d.Metrics,
				GuestCumulativeQuotaBytes:      d.GuestCumulativeQuotaBytes,
				RegisteredCumulativeQuotaBytes: d.RegisteredCumulativeQuotaBytes,
			}
			r.Get("/upload-policy", uploadPolicy.Get)
			r.With(middleware.RateLimit(1, 2, d.TrustProxyHeaders, d.Metrics), uploadQuota).Post("/files", upload.Handle)

			caps := &handlers.CapabilitiesHandler{
				Files: d.Files,
			}
			r.Get("/files/{fileId}/capabilities", caps.Handle)
			r.Post("/files/capabilities/batch", caps.HandleBatch)

			conv := &handlers.ConversionHandler{
				Files:                        d.Files,
				Jobs:                         d.Jobs,
				Store:                        d.Store,
				Orchestrator:                 d.Orchestrator,
				Logger:                       d.Logger,
				MaxActiveJobsPerGuestSession: d.MaxActiveJobsPerGuestSession,
			}
			r.With(middleware.RateLimit(1, 3, d.TrustProxyHeaders, d.Metrics), conversionQuota).Post("/conversions", conv.Handle)
			r.With(middleware.RateLimit(1, 3, d.TrustProxyHeaders, d.Metrics), conversionQuota).Post("/conversions/batch", conv.HandleBatch)

			jobs := &handlers.JobHandler{
				Orchestrator: d.Orchestrator,
				Artifacts:    d.Artifacts,
				Files:        d.Files,
				Store:        d.Store,
			}
			r.Get("/jobs/{jobId}", jobs.Handle)
			r.Post("/jobs/{jobId}/cancel", jobs.Cancel)
			r.Post("/jobs/batch/cancel", jobs.CancelBatch)
			r.With(conversionQuota).Post("/jobs/{jobId}/retry", jobs.Retry)
			r.With(conversionQuota).Post("/jobs/batch/retry", jobs.RetryBatch)

			art := &handlers.ArtifactHandler{
				Artifacts: d.Artifacts,
				Files:     d.Files,
				Store:     d.Store,
			}
			r.Get("/artifacts/{artifactId}/download", art.Handle)
		})

		// Auth-protected routes (require authentication)
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(d.AuthService, d.Users))
			r.Get("/auth/me", authH.Me)

			dashboard := &handlers.DashboardHandler{Dashboard: d.Dashboard, Audit: d.Audit}
			r.Get("/dashboard/me", dashboard.Me)

			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireAdmin)
				r.Get("/admin/health", handlers.DetailedHealth(d.ArtifactTTLHours, d.ArtifactTTLByFamily))
				r.Get("/admin/overview", dashboard.AdminOverview)
				r.Get("/admin/engines", dashboard.AdminEngines)
				r.Put("/admin/footer-message", footer.Update)
				r.Put("/admin/upload-policy", uploadPolicy.Update)

				// SMTP settings
				smtpH := &handlers.SMTPSettingsHandler{
					Email:    d.EmailService,
					Settings: d.SiteSettings,
				}
				r.Get("/admin/smtp-settings", smtpH.Get)
				r.Put("/admin/smtp-settings", smtpH.Update)
				r.Post("/admin/smtp-test", smtpH.Test)

				// Email templates
				emailTmplH := &handlers.EmailTemplateHandler{
					Email:     d.EmailService,
					Templates: d.EmailTemplates,
				}
				webhookH := &handlers.WebhookHandler{Webhooks: d.Webhooks}
				r.Get("/admin/email-templates", emailTmplH.List)
				r.Get("/admin/email-templates/{key}", emailTmplH.Get)
				r.Put("/admin/email-templates/{key}", emailTmplH.Update)
				r.Post("/admin/email-templates/{key}/preview", emailTmplH.Preview)
				r.Get("/admin/webhooks", webhookH.List)
				r.Post("/admin/webhooks", webhookH.Create)
				r.Put("/admin/webhooks/{webhookId}", webhookH.Update)
				r.Delete("/admin/webhooks/{webhookId}", webhookH.Delete)
			})
		})
	})

	return r
}
