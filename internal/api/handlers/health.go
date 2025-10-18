package handlers

import (
	"net/http"
)

// HealthHandler handles health check endpoints
type HealthHandler struct {
	BaseHandler
}

// NewHealthHandler creates a new health handler
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// HealthCheck handles the health check endpoint
func (h *HealthHandler) HealthCheck(w http.ResponseWriter, req *http.Request) {
	h.sendSuccess(w, "Spectra API is healthy", nil)
}
