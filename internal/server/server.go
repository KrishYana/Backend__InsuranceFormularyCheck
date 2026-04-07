package server

import (
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/kyanaman/formularycheck/ent"
	"github.com/kyanaman/formularycheck/internal/handler"
	"github.com/kyanaman/formularycheck/internal/middleware"
)

// New creates a configured Chi router with all routes and middleware.
func New(db *ent.Client, cfg Config) *chi.Mux {
	r := chi.NewRouter()

	// Auth middleware
	auth := middleware.NewAuthMiddleware(db, cfg.SupabaseURL, cfg.SupabaseJWTSecret)

	// Global middleware
	r.Use(chimw.RequestID)
	r.Use(middleware.Logging)
	r.Use(middleware.CORS(cfg.CORSOrigins))
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))

	// Handlers
	h := handler.New(db)

	r.Route("/v1", func(r chi.Router) {
		// Public routes (guest-accessible)
		r.Group(func(r chi.Router) {
			r.Use(auth.OptionalAuth)

			// Insurers & plans
			r.Get("/states/{code}/insurers", h.GetInsurers)
			r.Get("/insurers/{id}/plans", h.GetPlans)
			r.Get("/plans/lookup/medicare", h.LookupMedicarePlan)
			r.Get("/plans/lookup/hios", h.LookupHiosPlan)
			r.Get("/plans/lookup/group", h.LookupGroupPlan)

			// Drug search & coverage
			r.Get("/drugs/search", h.SearchDrugs)
			r.Get("/plans/{id}/drugs/{drugId}/coverage", h.GetCoverage)
			r.Get("/drugs/{id}/coverage", h.GetCoverageMulti)
			r.Get("/drugs/{id}/alternatives", h.GetAlternatives)
			r.Get("/coverage/{entryId}/prior-auth", h.GetPriorAuthCriteria)

			// Discover
			r.Get("/discover/articles", h.GetArticles)
		})

		// Protected routes (require valid Supabase JWT)
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAuth)

			// Auth handshake
			r.Post("/auth/callback", h.AuthCallback)

			// Physician profile
			r.Get("/physicians/me", h.GetProfile)
			r.Patch("/physicians/me", h.UpdateProfile)

			// Saved lookups
			r.Get("/saved-lookups", h.ListSavedLookups)
			r.Post("/saved-lookups", h.CreateSavedLookup)
			r.Patch("/saved-lookups/{id}", h.UpdateSavedLookup)
			r.Delete("/saved-lookups/{id}", h.DeleteSavedLookup)

			// Search history
			r.Get("/search-history", h.ListSearchHistory)
			r.Delete("/search-history/{id}", h.DeleteSearchHistoryEntry)
			r.Delete("/search-history", h.ClearSearchHistory)

			// Insights
			r.Get("/insights/summary", h.GetInsightsSummary)
			r.Get("/insights/trends", h.GetInsightsTrends)
		})
	})

	return r
}
