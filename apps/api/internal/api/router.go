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
	Logger              zerolog.Logger
	Metrics             *observability.Metrics
	Store               storage.Store
	Files               repository.FileRepository
	Jobs                repository.JobRepository
	Artifacts           repository.ArtifactRepository
	Audit               repository.AuditRepository
	Users               repository.UserRepository
	Dashboard           repository.DashboardRepository
	Orchestrator        *orchestrator.Service
	AuthService         *auth.Service
	CORSOrigin          string
	ArtifactTTLHours    int
	ArtifactTTLByFamily map[string]int
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
	r.Use(middleware.RateLimit(100, 200))            // 100 req/s, burst 200
	r.Use(middleware.MaxBodySize(500 * 1024 * 1024)) // 500 MB global limit

	// Prometheus metrics endpoint
	r.Handle("/metrics", promhttp.Handler())

	// API routes
	r.Route("/api", func(r chi.Router) {
		r.Get("/health", handlers.Health(d.ArtifactTTLHours, d.ArtifactTTLByFamily))

		// Auth routes (public)
		authH := &handlers.AuthHandler{Auth: d.AuthService}
		r.Post("/auth/register", authH.Register)
		r.Post("/auth/login", authH.Login)

		// Auth-protected route
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(d.AuthService, d.Users))
			r.Get("/auth/me", authH.Me)

			upload := &handlers.UploadHandler{
				Store:   d.Store,
				Files:   d.Files,
				Audit:   d.Audit,
				Logger:  d.Logger,
				Metrics: d.Metrics,
			}
			r.Post("/files", upload.Handle)

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
			r.Post("/conversions", conv.Handle)

			jobs := &handlers.JobHandler{
				Orchestrator: d.Orchestrator,
				Files:        d.Files,
				Store:        d.Store,
			}
			r.Get("/jobs/{jobId}", jobs.Handle)
			r.Post("/jobs/{jobId}/cancel", jobs.Cancel)
			r.Post("/jobs/{jobId}/retry", jobs.Retry)

			art := &handlers.ArtifactHandler{
				Artifacts: d.Artifacts,
				Store:     d.Store,
			}
			r.Get("/artifacts/{artifactId}/download", art.Handle)

			dashboard := &handlers.DashboardHandler{Dashboard: d.Dashboard, Audit: d.Audit}
			r.Get("/dashboard/me", dashboard.Me)

			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireAdmin)
				r.Get("/admin/overview", dashboard.AdminOverview)
				r.Get("/admin/engines", dashboard.AdminEngines)
			})
		})
	})

	return r
}
