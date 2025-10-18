package handlers

import (
	"fmt"
	"net/http"

	"github.com/Project-Sylos/Spectra/sdk"
	"github.com/go-chi/chi/v5"
)

// NodeHandler handles node-related endpoints
type NodeHandler struct {
	BaseHandler
	fs *sdk.SpectraFS
}

// NewNodeHandler creates a new node handler
func NewNodeHandler(fs *sdk.SpectraFS) *NodeHandler {
	return &NodeHandler{
		fs: fs,
	}
}

// GetNode handles the get node endpoint
func (h *NodeHandler) GetNode(w http.ResponseWriter, req *http.Request) {
	id := chi.URLParam(req, "id")
	if id == "" {
		h.sendError(w, http.StatusBadRequest, "node id is required")
		return
	}

	node, err := h.fs.GetNode(id)
	if err != nil {
		h.sendError(w, http.StatusNotFound, fmt.Sprintf("Node not found: %v", err))
		return
	}

	h.sendSuccess(w, "Node retrieved successfully", node)
}

// DeleteNode handles the delete node endpoint
func (h *NodeHandler) DeleteNode(w http.ResponseWriter, req *http.Request) {
	id := chi.URLParam(req, "id")
	if id == "" {
		h.sendError(w, http.StatusBadRequest, "node id is required")
		return
	}

	// Prevent deletion of root node
	if id == "p-root" {
		h.sendError(w, http.StatusBadRequest, "Cannot delete root node")
		return
	}

	if err := h.fs.DeleteNode(id); err != nil {
		h.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete node: %v", err))
		return
	}

	h.sendSuccess(w, "Node deleted successfully", nil)
}
