package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	apimodels "github.com/Project-Sylos/Spectra/internal/api/models"
	spectrafsmodels "github.com/Project-Sylos/Spectra/internal/spectrafs/models"
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
	var apiRequest apimodels.ListChildrenRequest
	if err := json.NewDecoder(req.Body).Decode(&apiRequest); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate that either parent_id or (parent_path + table_name) is provided
	if apiRequest.ParentID == "" && (apiRequest.ParentPath == "" || apiRequest.TableName == "") {
		h.sendError(w, http.StatusBadRequest, "either parent_id or (parent_path + table_name) are required")
		return
	}

	// Convert API model to spectrafs request model
	spectrafsRequest := &spectrafsmodels.ListChildrenRequest{
		ParentID:   apiRequest.ParentID,
		ParentPath: apiRequest.ParentPath,
		TableName:  apiRequest.TableName,
	}

	result, err := h.fs.ListChildren(spectrafsRequest)
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list children: %v", err))
		return
	}

	h.sendJSON(w, http.StatusOK, result)
}

// CreateFolder handles the create folder endpoint
func (h *FolderHandler) CreateFolder(w http.ResponseWriter, req *http.Request) {
	var apiRequest apimodels.CreateFolderRequest
	if err := json.NewDecoder(req.Body).Decode(&apiRequest); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate that either parent_id or (parent_path + table_name) is provided
	if apiRequest.ParentID == "" && (apiRequest.ParentPath == "" || apiRequest.TableName == "") {
		h.sendError(w, http.StatusBadRequest, "either parent_id or (parent_path + table_name) are required")
		return
	}

	if apiRequest.Name == "" {
		h.sendError(w, http.StatusBadRequest, "name is required")
		return
	}

	// Convert API model to spectrafs request model
	spectrafsRequest := &spectrafsmodels.CreateFolderRequest{
		ParentID:   apiRequest.ParentID,
		ParentPath: apiRequest.ParentPath,
		TableName:  apiRequest.TableName,
		Name:       apiRequest.Name,
	}

	folder, err := h.fs.CreateFolder(spectrafsRequest)
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
