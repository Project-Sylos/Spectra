package handlers

import (
	"fmt"
	"net/http"

	"github.com/Project-Sylos/Spectra/sdk"
	"github.com/go-chi/chi/v5"
)

// SystemHandler handles system-related endpoints
type SystemHandler struct {
	BaseHandler
	fs *sdk.SpectraFS
}

// NewSystemHandler creates a new system handler
func NewSystemHandler(fs *sdk.SpectraFS) *SystemHandler {
	return &SystemHandler{
		fs: fs,
	}
}

// Reset handles the reset endpoint
func (h *SystemHandler) Reset(w http.ResponseWriter, req *http.Request) {
	if err := h.fs.Reset(); err != nil {
		h.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to reset filesystem: %v", err))
		return
	}

	h.sendSuccess(w, "Filesystem reset successfully", nil)
}

// GetTables handles the get tables endpoint
func (h *SystemHandler) GetTables(w http.ResponseWriter, req *http.Request) {
	tables, err := h.fs.GetTableInfo()
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get table info: %v", err))
		return
	}

	h.sendSuccess(w, "Tables retrieved successfully", tables)
}

// GetTableCount handles the get table count endpoint
func (h *SystemHandler) GetTableCount(w http.ResponseWriter, req *http.Request) {
	tableName := chi.URLParam(req, "tableName")
	if tableName == "" {
		h.sendError(w, http.StatusBadRequest, "table name is required")
		return
	}

	count, err := h.fs.GetNodeCount(tableName)
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get table count: %v", err))
		return
	}

	response := map[string]any{
		"table_name": tableName,
		"count":      count,
	}

	h.sendSuccess(w, "Table count retrieved successfully", response)
}

// GetConfig handles the get config endpoint
func (h *SystemHandler) GetConfig(w http.ResponseWriter, req *http.Request) {
	config := h.fs.GetConfig()
	h.sendSuccess(w, "Config retrieved successfully", config)
}

// GetStats handles the get stats endpoint
func (h *SystemHandler) GetStats(w http.ResponseWriter, req *http.Request) {
	stats, err := h.fs.GetStats()
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get stats: %v", err))
		return
	}

	h.sendSuccess(w, "Stats retrieved successfully", stats)
}
