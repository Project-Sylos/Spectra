package models

// GetNodeRequest represents the request to get a node
// You can specify either:
//   - ID: Direct node ID (e.g., "root", "s1-abc123")
//   - Path + TableName: Lookup by path in a specific table (TableName should be "primary", "s1", "s2", etc.)
//
// If ID is provided, Path and TableName are ignored.
// If Path is provided, TableName is required.
//
// This struct implements NodeIdentifier, allowing it to be used with the interface-based API.
type GetNodeRequest struct {
	ID        string `json:"id,omitempty"`
	Path      string `json:"path,omitempty"`
	TableName string `json:"table_name,omitempty"`
}

// GetID implements NodeIdentifier
func (r *GetNodeRequest) GetID() string { return r.ID }

// GetPath implements NodeIdentifier
func (r *GetNodeRequest) GetPath() string { return r.Path }

// GetTableName implements NodeIdentifier
func (r *GetNodeRequest) GetTableName() string { return r.TableName }

// ListChildrenRequest represents the request to list children of a parent node
// You can specify either:
//   - ParentID: Direct parent node ID (e.g., "root", "s1-abc123")
//   - ParentPath + TableName: Lookup by path in a specific table
//
// If ParentID is provided, ParentPath and TableName are ignored.
// If ParentPath is provided, TableName is required.
//
// This struct implements ParentIdentifier.
type ListChildrenRequest struct {
	ParentID   string `json:"parent_id,omitempty"`
	ParentPath string `json:"parent_path,omitempty"`
	TableName  string `json:"table_name,omitempty"`
}

// GetParentID implements ParentIdentifier
func (r *ListChildrenRequest) GetParentID() string { return r.ParentID }

// GetParentPath implements ParentIdentifier
func (r *ListChildrenRequest) GetParentPath() string { return r.ParentPath }

// GetTableName implements ParentIdentifier
func (r *ListChildrenRequest) GetTableName() string { return r.TableName }

// CreateFolderRequest represents the request to create a new folder
// You can specify either:
//   - ParentID: Direct parent node ID
//   - ParentPath + TableName: Lookup by path in a specific table
//
// Name is required.
//
// This struct implements ParentIdentifier and NamedRequest.
type CreateFolderRequest struct {
	ParentID   string `json:"parent_id,omitempty"`
	ParentPath string `json:"parent_path,omitempty"`
	TableName  string `json:"table_name,omitempty"`
	Name       string `json:"name"`
}

// GetParentID implements ParentIdentifier
func (r *CreateFolderRequest) GetParentID() string { return r.ParentID }

// GetParentPath implements ParentIdentifier
func (r *CreateFolderRequest) GetParentPath() string { return r.ParentPath }

// GetTableName implements ParentIdentifier
func (r *CreateFolderRequest) GetTableName() string { return r.TableName }

// GetName implements NamedRequest
func (r *CreateFolderRequest) GetName() string { return r.Name }

// UploadFileRequest represents the request to upload a file
// You can specify either:
//   - ParentID: Direct parent node ID
//   - ParentPath + TableName: Lookup by path in a specific table
//
// Name and Data are required.
//
// This struct implements ParentIdentifier, NamedRequest, and DataRequest.
type UploadFileRequest struct {
	ParentID   string `json:"parent_id,omitempty"`
	ParentPath string `json:"parent_path,omitempty"`
	TableName  string `json:"table_name,omitempty"`
	Name       string `json:"name"`
	Data       []byte `json:"data"`
}

// GetParentID implements ParentIdentifier
func (r *UploadFileRequest) GetParentID() string { return r.ParentID }

// GetParentPath implements ParentIdentifier
func (r *UploadFileRequest) GetParentPath() string { return r.ParentPath }

// GetTableName implements ParentIdentifier
func (r *UploadFileRequest) GetTableName() string { return r.TableName }

// GetName implements NamedRequest
func (r *UploadFileRequest) GetName() string { return r.Name }

// GetData implements DataRequest
func (r *UploadFileRequest) GetData() []byte { return r.Data }

// DeleteNodeRequest represents the request to delete a node
// You can specify either:
//   - ID: Direct node ID
//   - Path + TableName: Lookup by path in a specific table
//
// This struct implements NodeIdentifier.
type DeleteNodeRequest struct {
	ID        string `json:"id,omitempty"`
	Path      string `json:"path,omitempty"`
	TableName string `json:"table_name,omitempty"`
}

// GetID implements NodeIdentifier
func (r *DeleteNodeRequest) GetID() string { return r.ID }

// GetPath implements NodeIdentifier
func (r *DeleteNodeRequest) GetPath() string { return r.Path }

// GetTableName implements NodeIdentifier
func (r *DeleteNodeRequest) GetTableName() string { return r.TableName }

// UpdateTraversalStatusRequest represents the request to update a node's traversal status
// You can specify either:
//   - ID: Direct node ID
//   - Path + TableName: Lookup by path in a specific table
//
// Status is required ("pending", "successful", or "failed").
//
// This struct implements NodeIdentifier and StatusRequest.
type UpdateTraversalStatusRequest struct {
	ID        string `json:"id,omitempty"`
	Path      string `json:"path,omitempty"`
	TableName string `json:"table_name,omitempty"`
	Status    string `json:"status"`
}

// GetID implements NodeIdentifier
func (r *UpdateTraversalStatusRequest) GetID() string { return r.ID }

// GetPath implements NodeIdentifier
func (r *UpdateTraversalStatusRequest) GetPath() string { return r.Path }

// GetTableName implements NodeIdentifier
func (r *UpdateTraversalStatusRequest) GetTableName() string { return r.TableName }

// GetStatus implements StatusRequest
func (r *UpdateTraversalStatusRequest) GetStatus() string { return r.Status }
