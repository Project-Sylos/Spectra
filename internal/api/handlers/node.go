package handlers

import (
	"fmt"
	"net/http"

	spectrafsmodels "github.com/Project-Sylos/Spectra/internal/spectrafs/models"
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

	// Create request struct from URL parameter
	request := &spectrafsmodels.GetNodeRequest{
		ID: id,
	}

	node, err := h.fs.GetNode(request)
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

	// Prevent deletion of root node (check will also happen in SDK, but this is a fast path)
	if id == "root" || (len(id) > 5 && id[len(id)-5:] == "-root") {
		h.sendError(w, http.StatusBadRequest, "Cannot delete root node")
		return
	}

	// Create request struct from URL parameter
	request := &spectrafsmodels.DeleteNodeRequest{
		ID: id,
	}

	if err := h.fs.DeleteNode(request); err != nil {
		h.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete node: %v", err))
		return
	}

	h.sendSuccess(w, "Node deleted successfully", nil)
}
