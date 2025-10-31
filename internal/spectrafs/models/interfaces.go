package models

import "fmt"

// NodeIdentifier interface for identifying a node by ID or Path+TableName
// This allows any struct to be used as long as it provides these fields
type NodeIdentifier interface {
	GetID() string
	GetPath() string
	GetTableName() string
}

// ParentIdentifier interface for identifying a parent node
// Either ParentID must be set, OR (ParentPath + TableName) must both be set
type ParentIdentifier interface {
	GetParentID() string
	GetParentPath() string
	GetTableName() string
}

// NamedRequest interface for requests that require a name field
type NamedRequest interface {
	GetName() string
}

// DataRequest interface for requests that include data
type DataRequest interface {
	GetData() []byte
}

// StatusRequest interface for requests that include a status
type StatusRequest interface {
	GetStatus() string
}

// BaseRequest is the base struct containing common fields for all requests
// Users can embed this and add their own fields
type BaseRequest struct {
	ID        string `json:"id,omitempty"`
	Path      string `json:"path,omitempty"`
	TableName string `json:"table_name,omitempty"`

	ParentID   string `json:"parent_id,omitempty"`
	ParentPath string `json:"parent_path,omitempty"`

	Name   string `json:"name,omitempty"`
	Data   []byte `json:"data,omitempty"`
	Status string `json:"status,omitempty"`
}

// GetID implements NodeIdentifier
func (b *BaseRequest) GetID() string {
	return b.ID
}

// GetPath implements NodeIdentifier
func (b *BaseRequest) GetPath() string {
	return b.Path
}

// GetTableName implements NodeIdentifier and ParentIdentifier
func (b *BaseRequest) GetTableName() string {
	return b.TableName
}

// GetParentID implements ParentIdentifier
func (b *BaseRequest) GetParentID() string {
	return b.ParentID
}

// GetParentPath implements ParentIdentifier
func (b *BaseRequest) GetParentPath() string {
	return b.ParentPath
}

// GetName implements NamedRequest
func (b *BaseRequest) GetName() string {
	return b.Name
}

// GetData implements DataRequest
func (b *BaseRequest) GetData() []byte {
	return b.Data
}

// GetStatus implements StatusRequest
func (b *BaseRequest) GetStatus() string {
	return b.Status
}

// ValidateNodeIdentifier validates that either ID is provided OR (Path + TableName) are both provided
func ValidateNodeIdentifier(req NodeIdentifier) error {
	id := req.GetID()
	path := req.GetPath()
	tableName := req.GetTableName()

	if id != "" {
		// ID-based lookup - valid
		return nil
	}

	if path != "" && tableName != "" {
		// Path + TableName lookup - valid
		return nil
	}

	return fmt.Errorf("either ID or (Path + TableName) must be provided")
}

// ValidateParentIdentifier validates that either ParentID is provided OR (ParentPath + TableName) are both provided
func ValidateParentIdentifier(req ParentIdentifier) error {
	parentID := req.GetParentID()
	parentPath := req.GetParentPath()
	tableName := req.GetTableName()

	if parentID != "" {
		// ParentID-based lookup - valid
		return nil
	}

	if parentPath != "" && tableName != "" {
		// ParentPath + TableName lookup - valid
		return nil
	}

	return fmt.Errorf("either ParentID or (ParentPath + TableName) must be provided")
}
