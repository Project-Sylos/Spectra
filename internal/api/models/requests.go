package models

// ListChildrenRequest represents the request to list children of a parent node
// Supports both ID-based and Path+TableName-based lookups
type ListChildrenRequest struct {
	ParentID   string `json:"parent_id,omitempty"`   // Parent node ID
	ParentPath string `json:"parent_path,omitempty"` // Parent node path
	TableName  string `json:"table_name,omitempty"`  // Required when using ParentPath
	Depth      *int   `json:"depth,omitempty"`       // Required for ephemeral mode
}

// CreateFolderRequest represents the request to create a new folder
// Supports both ID-based and Path+TableName-based lookups
type CreateFolderRequest struct {
	ParentID   string `json:"parent_id,omitempty"`   // Parent node ID
	ParentPath string `json:"parent_path,omitempty"` // Parent node path
	TableName  string `json:"table_name,omitempty"`  // Required when using ParentPath
	Name       string `json:"name"`                  // Name of the folder to create
}

// UploadFileRequest represents the request to upload a file
// Supports both ID-based and Path+TableName-based lookups
type UploadFileRequest struct {
	ParentID   string `json:"parent_id,omitempty"`   // Parent node ID
	ParentPath string `json:"parent_path,omitempty"` // Parent node path
	TableName  string `json:"table_name,omitempty"`  // Required when using ParentPath
	Name       string `json:"name"`                  // Name of the file to upload
	Data       []byte `json:"data"`                  // File content (base64 encoded in JSON)
}

// GetNodeRequest represents the request to get a node by ID or path
type GetNodeRequest struct {
	ID        string `json:"id,omitempty"`
	Path      string `json:"path,omitempty"`
	TableName string `json:"table_name,omitempty"`
}

// DeleteNodeRequest represents the request to delete a node by path
type DeleteNodeRequest struct {
	Path      string `json:"path,omitempty"`
	TableName string `json:"table_name,omitempty"`
}
