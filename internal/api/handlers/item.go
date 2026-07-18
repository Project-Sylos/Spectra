package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	apimodels "codeberg.org/Sylos/Spectra/internal/api/models"
	"codeberg.org/Sylos/Spectra/internal/chaos"
	spectrafsmodels "codeberg.org/Sylos/Spectra/internal/spectrafs/models"
	"codeberg.org/Sylos/Spectra/internal/types"
	"codeberg.org/Sylos/Spectra/sdk"
	"github.com/go-chi/chi/v5"
)
type ItemHandler struct {
	BaseHandler
	fs *sdk.SpectraFS
}

// NewItemHandler creates a new item handler
func NewItemHandler(fs *sdk.SpectraFS) *ItemHandler {
	return &ItemHandler{
		fs: fs,
	}
}

// ListItems handles the list items endpoint (replaces ListChildren)
func (h *ItemHandler) ListItems(w http.ResponseWriter, req *http.Request) {
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
		Depth:      apiRequest.Depth,
	}

	result, err := h.fs.ListChildren(spectrafsRequest)
	if err != nil {
		if rl, ok := sdk.IsRateLimited(err); ok {
			chaos.WriteRateLimitedResponse(w, rl)
			return
		}
		if _, ok := sdk.IsUnauthorized(err); ok {
			h.sendError(w, http.StatusUnauthorized, err.Error())
			return
		}
		h.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list items: %v", err))
		return
	}

	h.sendJSON(w, http.StatusOK, result)
}

// CreateFolder handles the create folder endpoint
func (h *ItemHandler) CreateFolder(w http.ResponseWriter, req *http.Request) {
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

// UploadFile handles the upload file endpoint
func (h *ItemHandler) UploadFile(w http.ResponseWriter, req *http.Request) {
	var apiRequest apimodels.UploadFileRequest
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
	spectrafsRequest := &spectrafsmodels.UploadFileRequest{
		ParentID:   apiRequest.ParentID,
		ParentPath: apiRequest.ParentPath,
		TableName:  apiRequest.TableName,
		Name:       apiRequest.Name,
		Data:       apiRequest.Data,
	}

	file, err := h.fs.UploadFile(spectrafsRequest)
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to upload file: %v", err))
		return
	}

	h.sendJSON(w, http.StatusCreated, types.APIResponse{
		Success: true,
		Message: "File uploaded successfully",
		Data:    file,
	})
}

// GetFileData handles the get file data endpoint
func (h *ItemHandler) GetFileData(w http.ResponseWriter, req *http.Request) {
	id := chi.URLParam(req, "id")
	if id == "" {
		h.sendError(w, http.StatusBadRequest, "file id is required")
		return
	}
	world := req.URL.Query().Get("table_name")
	if world == "" {
		world = req.URL.Query().Get("world")
	}
	if world == "" {
		world = "primary"
	}

	data, checksum, err := h.fs.GetFileData(id, world)
	if err != nil {
		if _, ok := sdk.IsUnauthorized(err); ok {
			h.sendError(w, http.StatusUnauthorized, err.Error())
			return
		}
		h.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get file data: %v", err))
		return
	}

	response := map[string]any{
		"data":     data,
		"checksum": checksum,
		"size":     len(data),
	}

	h.sendSuccess(w, "File data retrieved successfully", response)
}

// GetItem handles path or ID based node lookup
func (h *ItemHandler) GetItem(w http.ResponseWriter, req *http.Request) {
	var apiRequest apimodels.GetNodeRequest
	if err := json.NewDecoder(req.Body).Decode(&apiRequest); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if apiRequest.ID == "" && apiRequest.Path == "" {
		h.sendError(w, http.StatusBadRequest, "either id or path is required")
		return
	}
	if apiRequest.ID == "" && apiRequest.TableName == "" {
		h.sendError(w, http.StatusBadRequest, "table_name is required when using path")
		return
	}

	spectrafsRequest := &spectrafsmodels.GetNodeRequest{
		ID:        apiRequest.ID,
		Path:      apiRequest.Path,
		TableName: apiRequest.TableName,
	}

	node, err := h.fs.GetNode(spectrafsRequest)
	if err != nil {
		h.sendError(w, http.StatusNotFound, fmt.Sprintf("Node not found: %v", err))
		return
	}

	h.sendSuccess(w, "Node retrieved successfully", node)
}

// DeleteItemByPath handles deleting a node by path and table name
func (h *ItemHandler) DeleteItemByPath(w http.ResponseWriter, req *http.Request) {
	var apiRequest apimodels.DeleteNodeRequest
	if err := json.NewDecoder(req.Body).Decode(&apiRequest); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if apiRequest.Path == "" || apiRequest.TableName == "" {
		h.sendError(w, http.StatusBadRequest, "path and table_name are required")
		return
	}

	deleteReq := &spectrafsmodels.DeleteNodeRequest{
		Path:      apiRequest.Path,
		TableName: apiRequest.TableName,
	}

	if err := h.fs.DeleteNode(deleteReq); err != nil {
		h.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete node: %v", err))
		return
	}

	h.sendSuccess(w, "Node deleted successfully", nil)
}
