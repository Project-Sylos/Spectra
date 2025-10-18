package api

import (
	"fmt"
	"net/http"

	"github.com/Project-Sylos/Spectra/internal/types"
	"github.com/Project-Sylos/Spectra/sdk"
	"github.com/go-chi/chi/v5"
)

// Server represents the HTTP API server
type Server struct {
	router *chi.Mux
	fs     *sdk.SpectraFS
	config *types.APIConfig
}

// NewServer creates a new API server
func NewServer(fs *sdk.SpectraFS, config *types.APIConfig) *Server {
	router := NewRouter(fs)

	return &Server{
		router: router.SetupRoutes(),
		fs:     fs,
		config: config,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	fmt.Printf("Starting Spectra API server on %s\n", addr)
	fmt.Printf("API endpoints available at http://%s/api/v1/\n", addr)
	fmt.Printf("Health check available at http://%s/health\n", addr)

	return http.ListenAndServe(addr, s.router)
}

// GetRouter returns the configured router
func (s *Server) GetRouter() *chi.Mux {
	return s.router
}

// Stop gracefully stops the server (placeholder for future implementation)
func (s *Server) Stop() error {
	// Close the filesystem connection
	return s.fs.Close()
}
