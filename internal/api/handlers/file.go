package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Project-Sylos/Spectra/internal/api/models"
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
	var request models.UploadFileRequest
	if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if request.ParentID == "" || request.Name == "" {
		h.sendError(w, http.StatusBadRequest, "parent_id and name are required")
		return
	}

	file, err := h.fs.UploadFile(request.ParentID, request.Name, request.Data)
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
