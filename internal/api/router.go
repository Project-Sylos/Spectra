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
	folderHandler := handlers.NewFolderHandler(r.fs)
	fileHandler := handlers.NewFileHandler(r.fs)
	nodeHandler := handlers.NewNodeHandler(r.fs)
	systemHandler := handlers.NewSystemHandler(r.fs)

	// Health check
	router.Get("/health", healthHandler.HealthCheck)

	// API routes
	router.Route("/api/v1", func(api chi.Router) {
		// Folder operations
		api.Route("/folder", func(folder chi.Router) {
			folder.Post("/list", folderHandler.ListChildren)
			folder.Post("/create", folderHandler.CreateFolder)
			folder.Get("/{id}", nodeHandler.GetNode) // Reuse node handler for getting folder info
		})

		// File operations
		api.Route("/file", func(file chi.Router) {
			file.Post("/upload", fileHandler.UploadFile)
			file.Get("/{id}", nodeHandler.GetNode) // Reuse node handler for getting file info
			file.Get("/{id}/data", fileHandler.GetFileData)
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
