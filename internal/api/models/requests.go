package models

// ListChildrenRequest represents the request to list children of a parent node
type ListChildrenRequest struct {
	ParentID string `json:"parent_id"`
}

// CreateFolderRequest represents the request to create a new folder
type CreateFolderRequest struct {
	ParentID string `json:"parent_id"`
	Name     string `json:"name"`
}

// UploadFileRequest represents the request to upload a file
type UploadFileRequest struct {
	ParentID string `json:"parent_id"`
	Name     string `json:"name"`
	Data     []byte `json:"data"`
}
