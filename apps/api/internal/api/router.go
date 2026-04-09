package api

import (
	"github.com/allopze/reform-lab/apps/api/internal/api/handlers"
	"github.com/allopze/reform-lab/apps/api/internal/api/middleware"
	"github.com/allopze/reform-lab/apps/api/internal/auth"
	"github.com/allopze/reform-lab/apps/api/internal/observability"
	"github.com/allopze/reform-lab/apps/api/internal/orchestrator"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/allopze/reform-lab/apps/api/internal/storage"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

// Deps groups all dependencies needed to wire up routes.
type Deps struct {
	Logger                   zerolog.Logger
	Metrics                  *observability.Metrics
	Store                    storage.Store
	Files                    repository.FileRepository
	Jobs                     repository.JobRepository
	Artifacts                repository.ArtifactRepository
	Audit                    repository.AuditRepository
	Users                    repository.UserRepository
	Dashboard                repository.DashboardRepository
	Orchestrator             *orchestrator.Service
	AuthService              *auth.Service
	CORSOrigin               string
	ExposeMetrics            bool
	TrustProxyHeaders        bool
	ArtifactTTLHours         int
	ArtifactTTLByFamily      map[string]int
	UserUploadsPerMinute     int
	UserUploadBurst          int
	UserConversionsPerMinute int
	UserConversionBurst      int
}

// NewRouter creates the chi router with all middleware and routes.
func NewRouter(d Deps) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.Recoverer)
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.RequestID)
	r.Use(middleware.Logging(d.Logger))
	r.Use(middleware.CORS(d.CORSOrigin))
	r.Use(middleware.RateLimit(20, 40, d.TrustProxyHeaders))
	r.Use(middleware.MaxBodySize(500 * 1024 * 1024)) // 500 MB global limit

	if d.ExposeMetrics {
		r.Handle("/metrics", promhttp.Handler())
	}

	// API routes
	r.Route("/api", func(r chi.Router) {
		r.Get("/health", handlers.PublicHealth())

		// Auth routes (public)
		authH := &handlers.AuthHandler{Auth: d.AuthService}
		r.With(middleware.RateLimit(1, 5, d.TrustProxyHeaders)).Post("/auth/register", authH.Register)
		r.With(middleware.RateLimit(1, 5, d.TrustProxyHeaders)).Post("/auth/login", authH.Login)
		r.Post("/auth/logout", authH.Logout)

		// File and conversion routes — optional authentication keeps ownership when
		// available but still allows anonymous use of the public app flow.
		r.Group(func(r chi.Router) {
			r.Use(middleware.OptionalAuth(d.AuthService, d.Users))
			uploadQuota := middleware.UserOrIPRateLimit(d.UserUploadsPerMinute, d.UserUploadBurst, d.TrustProxyHeaders)
			conversionQuota := middleware.UserOrIPRateLimit(d.UserConversionsPerMinute, d.UserConversionBurst, d.TrustProxyHeaders)

			upload := &handlers.UploadHandler{
				Store:   d.Store,
				Files:   d.Files,
				Audit:   d.Audit,
				Logger:  d.Logger,
				Metrics: d.Metrics,
			}
			r.With(middleware.RateLimit(1, 2, d.TrustProxyHeaders), uploadQuota).Post("/files", upload.Handle)

			caps := &handlers.CapabilitiesHandler{
				Files: d.Files,
			}
			r.Get("/files/{fileId}/capabilities", caps.Handle)

			conv := &handlers.ConversionHandler{
				Files:        d.Files,
				Store:        d.Store,
				Orchestrator: d.Orchestrator,
				Logger:       d.Logger,
			}
			r.With(middleware.RateLimit(1, 3, d.TrustProxyHeaders), conversionQuota).Post("/conversions", conv.Handle)

			jobs := &handlers.JobHandler{
				Orchestrator: d.Orchestrator,
				Artifacts:    d.Artifacts,
				Files:        d.Files,
				Store:        d.Store,
			}
			r.Get("/jobs/{jobId}", jobs.Handle)
			r.Post("/jobs/{jobId}/cancel", jobs.Cancel)
			r.With(conversionQuota).Post("/jobs/{jobId}/retry", jobs.Retry)

			art := &handlers.ArtifactHandler{
				Artifacts: d.Artifacts,
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
			})
		})
	})

	return r
}
