package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	apimodels "github.com/Project-Sylos/Spectra/internal/api/models"
	spectrafsmodels "github.com/Project-Sylos/Spectra/internal/spectrafs/models"
	"github.com/Project-Sylos/Spectra/internal/types"
	"github.com/Project-Sylos/Spectra/sdk"
	"github.com/go-chi/chi/v5"
)

// FileHandler handles file-related endpoints
type FileHandler struct {
	BaseHandler
	fs *sdk.SpectraFS
}

// NewFileHandler creates a new file handler
func NewFileHandler(fs *sdk.SpectraFS) *FileHandler {
	return &FileHandler{
		fs: fs,
	}
}

// UploadFile handles the upload file endpoint
func (h *FileHandler) UploadFile(w http.ResponseWriter, req *http.Request) {
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
func (h *FileHandler) GetFileData(w http.ResponseWriter, req *http.Request) {
	id := chi.URLParam(req, "id")
	if id == "" {
		h.sendError(w, http.StatusBadRequest, "file id is required")
		return
	}

	data, checksum, err := h.fs.GetFileData(id)
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get file data: %v", err))
		return
	}

	response := map[string]interface{}{
		"data":     data,
		"checksum": checksum,
		"size":     len(data),
	}

	h.sendSuccess(w, "File data retrieved successfully", response)
}
