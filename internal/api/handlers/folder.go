package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Project-Sylos/Spectra/internal/api/models"
	"github.com/Project-Sylos/Spectra/internal/types"
	"github.com/Project-Sylos/Spectra/sdk"
)

// FolderHandler handles folder-related endpoints
type FolderHandler struct {
	BaseHandler
	fs *sdk.SpectraFS
}

// NewFolderHandler creates a new folder handler
func NewFolderHandler(fs *sdk.SpectraFS) *FolderHandler {
	return &FolderHandler{
		fs: fs,
	}
}

// ListChildren handles the list children endpoint
func (h *FolderHandler) ListChildren(w http.ResponseWriter, req *http.Request) {
	var request models.ListChildrenRequest
	if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if request.ParentID == "" {
		h.sendError(w, http.StatusBadRequest, "parent_id is required")
		return
	}

	result, err := h.fs.ListChildren(request.ParentID)
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list children: %v", err))
		return
	}

	h.sendJSON(w, http.StatusOK, result)
}

// CreateFolder handles the create folder endpoint
func (h *FolderHandler) CreateFolder(w http.ResponseWriter, req *http.Request) {
	var request models.CreateFolderRequest
	if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if request.ParentID == "" || request.Name == "" {
		h.sendError(w, http.StatusBadRequest, "parent_id and name are required")
		return
	}

	// Create folder using the proper CreateFolder method
	folder, err := h.fs.CreateFolder(request.ParentID, request.Name)
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create folder: %v", err))
		return
	}

	h.sendJSON(w, http.StatusCreated, types.APIResponse{
		Success: true,
		Message: "Folder created successfully",
		Data:    folder,
	})
}
