package api

import (
	"github.com/Project-Sylos/Spectra/internal/api/handlers"
	apimiddleware "github.com/Project-Sylos/Spectra/internal/api/middleware"
	"github.com/Project-Sylos/Spectra/sdk"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Router represents the HTTP API router
type Router struct {
	fs *sdk.SpectraFS
}

// NewRouter creates a new API router
func NewRouter(fs *sdk.SpectraFS) *Router {
	return &Router{fs: fs}
}

// SetupRoutes configures all API routes using modular handlers
func (r *Router) SetupRoutes() *chi.Mux {
	router := chi.NewRouter()

	// Standard middleware
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Timeout(60))

	// Custom middleware
	router.Use(apimiddleware.CORS)

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler()
	itemHandler := handlers.NewItemHandler(r.fs)
	nodeHandler := handlers.NewNodeHandler(r.fs)
	systemHandler := handlers.NewSystemHandler(r.fs)

	// Health check
	router.Get("/health", healthHandler.HealthCheck)

	// API routes
	router.Route("/api/v1", func(api chi.Router) {
		// Item operations (files and folders)
		api.Route("/items", func(items chi.Router) {
			items.Post("/list", itemHandler.ListItems)
			items.Post("/folder", itemHandler.CreateFolder)
			items.Post("/file", itemHandler.UploadFile)
			items.Get("/{id}", nodeHandler.GetNode) // Reuse node handler for getting item info
			items.Get("/{id}/data", itemHandler.GetFileData)
		})

		// Node operations
		api.Route("/node", func(node chi.Router) {
			node.Get("/{id}", nodeHandler.GetNode)
			node.Delete("/{id}", nodeHandler.DeleteNode)
		})

		// System operations
		api.Post("/reset", systemHandler.Reset)
		api.Get("/config", systemHandler.GetConfig)
		api.Get("/tables", systemHandler.GetTables)
		api.Get("/tables/{tableName}/count", systemHandler.GetTableCount)
	})

	return router
}
