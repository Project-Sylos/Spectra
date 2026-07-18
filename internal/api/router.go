package api

import (
	"codeberg.org/Sylos/Spectra/internal/api/handlers"
	apimiddleware "codeberg.org/Sylos/Spectra/internal/api/middleware"
	"codeberg.org/Sylos/Spectra/sdk"
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

	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Timeout(60))
	router.Use(apimiddleware.CORS)

	healthHandler := handlers.NewHealthHandler()
	itemHandler := handlers.NewItemHandler(r.fs)
	nodeHandler := handlers.NewNodeHandler(r.fs)
	systemHandler := handlers.NewSystemHandler(r.fs)
	authHandler := handlers.NewAuthHandler(r.fs)

	router.Get("/health", healthHandler.HealthCheck)

	router.Route("/api/v1", func(api chi.Router) {
		api.Route("/auth", func(auth chi.Router) {
			auth.Post("/token", authHandler.IssueToken)
			auth.Post("/refresh", authHandler.RefreshToken)
		})

		api.Group(func(protected chi.Router) {
			protected.Use(apimiddleware.Auth(r.fs.AuthEngine()))

			protected.Route("/items", func(items chi.Router) {
				items.Post("/list", itemHandler.ListItems)
				items.Post("/get", itemHandler.GetItem)
				items.Post("/delete", itemHandler.DeleteItemByPath)
				items.Post("/folder", itemHandler.CreateFolder)
				items.Post("/file", itemHandler.UploadFile)
				items.Get("/{id}", nodeHandler.GetNode)
				items.Get("/{id}/data", itemHandler.GetFileData)
			})

			protected.Route("/node", func(node chi.Router) {
				node.Get("/{id}", nodeHandler.GetNode)
				node.Delete("/{id}", nodeHandler.DeleteNode)
			})

			protected.Post("/reset", systemHandler.Reset)
			protected.Get("/config", systemHandler.GetConfig)
			protected.Get("/stats", systemHandler.GetStats)
			protected.Get("/tables", systemHandler.GetTables)
			protected.Get("/tables/{tableName}/count", systemHandler.GetTableCount)
		})
	})

	return router
}
