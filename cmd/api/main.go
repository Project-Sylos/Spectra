package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Project-Sylos/Spectra/internal/api"
	"github.com/Project-Sylos/Spectra/sdk"
)

func main() {
	fmt.Println("Spectra API Server")
	fmt.Println("==================")

	// Load configuration
	configPath := getConfigPath()
	fmt.Printf("Loading configuration from: %s\n", configPath)

	// Initialize SpectraFS
	fmt.Println("Initializing SpectraFS...")
	fs, err := sdk.New(configPath)
	if err != nil {
		log.Fatalf("Failed to initialize SpectraFS: %v", err)
	}
	fmt.Println("SpectraFS initialized successfully")

	// Get configuration
	cfg := fs.GetConfig()
	fmt.Printf("API config: Host=%s, Port=%d\n", cfg.API.Host, cfg.API.Port)

	// Create API server
	fmt.Println("Creating API server...")
	server := api.NewServer(fs, &cfg.API)
	fmt.Println("API server created successfully")

	// Create HTTP server with timeout
	addr := fmt.Sprintf("%s:%d", cfg.API.Host, cfg.API.Port)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      server.GetRouter(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nShutting down server...")

		// Create shutdown context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Shutdown HTTP server
		if err := httpServer.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down HTTP server: %v", err)
		}

		// Close filesystem
		if err := fs.Close(); err != nil {
			log.Printf("Error closing filesystem: %v", err)
		}

		fmt.Println("Server shutdown complete")
		os.Exit(0)
	}()

	// Start server
	fmt.Printf("Starting HTTP server on %s\n", addr)
	fmt.Printf("API endpoints available at http://%s/api/v1/\n", addr)
	fmt.Printf("Health check available at http://%s/health\n", addr)
	fmt.Println("Press Ctrl+C to stop the server")

	// I am here to serve.
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// getConfigPath returns the configuration file path
func getConfigPath() string {
	if len(os.Args) > 1 {
		return os.Args[1]
	}
	return "internal/config/default.json"
}
