package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/Project-Sylos/Spectra/internal/types"
)

// BaseHandler provides common functionality for all API handlers
type BaseHandler struct{}

// sendJSON sends a JSON response with the given status code and data
func (h *BaseHandler) sendJSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// sendError sends an error response with the given status code and message
func (h *BaseHandler) sendError(w http.ResponseWriter, statusCode int, message string) {
	h.sendJSON(w, statusCode, types.APIResponse{
		Success: false,
		Message: message,
	})
}

// sendSuccess sends a success response with the given data
func (h *BaseHandler) sendSuccess(w http.ResponseWriter, message string, data any) {
	h.sendJSON(w, http.StatusOK, types.APIResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}
