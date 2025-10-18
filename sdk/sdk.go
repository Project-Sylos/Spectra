package sdk

import (
	"fmt"

	"github.com/Project-Sylos/Spectra/internal/spectrafs"
	"github.com/Project-Sylos/Spectra/internal/types"
)

// SpectraFS is the public SDK interface for the synthetic filesystem
// This wraps the internal implementation to provide a clean public API
type SpectraFS struct {
	impl *spectrafs.SpectraFS
}

// New creates a new SpectraFS instance using the specified config file
func New(configPath string) (*SpectraFS, error) {
	impl, err := spectrafs.NewSpectraFS(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize SpectraFS: %w", err)
	}

	return &SpectraFS{
		impl: impl,
	}, nil
}

// NewWithDefaults creates a new SpectraFS instance using default configuration
func NewWithDefaults() (*SpectraFS, error) {
	return New("configs/default.json")
}

// ListChildren returns the children of a given parent node
// This implements the API interface from readme lines 91-94
func (s *SpectraFS) ListChildren(parentID string) (*types.ListResult, error) {
	return s.impl.ListChildren(parentID)
}

// GetNode retrieves a node by its ID
func (s *SpectraFS) GetNode(id string) (*types.Node, error) {
	return s.impl.GetNode(id)
}

// GetFileData generates and returns file data with checksum for a given file ID
// The data is generated on-the-fly and not persisted
func (s *SpectraFS) GetFileData(id string) ([]byte, string, error) {
	return s.impl.GetFileData(id)
}

// CreateFolder creates a new folder node
func (s *SpectraFS) CreateFolder(parentID, name string) (*types.Node, error) {
	return s.impl.CreateFolder(parentID, name)
}

// UploadFile handles file uploads - processes the data and creates a file node
// The actual file data is not persisted, only metadata
func (s *SpectraFS) UploadFile(parentID, name string, data []byte) (*types.Node, error) {
	return s.impl.UploadFile(parentID, name, data)
}

// Reset clears all nodes and recreates the root
func (s *SpectraFS) Reset() error {
	return s.impl.Reset()
}

// Close closes the database connection
func (s *SpectraFS) Close() error {
	return s.impl.Close()
}

// GetConfig returns the current configuration
func (s *SpectraFS) GetConfig() *types.Config {
	return s.impl.GetConfig()
}

// GetNodeCount returns the total number of nodes in a specific table
func (s *SpectraFS) GetNodeCount(tableName string) (int, error) {
	return s.impl.GetNodeCount(tableName)
}

// GetTableInfo returns information about all tables
func (s *SpectraFS) GetTableInfo() ([]types.TableInfo, error) {
	return s.impl.GetTableInfo()
}

// GetSecondaryTables returns the list of secondary table names
func (s *SpectraFS) GetSecondaryTables() []string {
	return s.impl.GetSecondaryTables()
}

// DeleteNode deletes a node by its ID
func (s *SpectraFS) DeleteNode(id string) error {
	return s.impl.DeleteNode(id)
}

// UpdateTraversalStatus updates the traversal status of a node
func (s *SpectraFS) UpdateTraversalStatus(id, status string) error {
	return s.impl.UpdateTraversalStatus(id, status)
}

// Re-export types for convenience
type (
	Config      = types.Config
	Node        = types.Node
	Folder      = types.Folder
	File        = types.File
	ListResult  = types.ListResult
	TableInfo   = types.TableInfo
	APIResponse = types.APIResponse
)

// Re-export constants
const (
	NodeTypeFolder = types.NodeTypeFolder
	NodeTypeFile   = types.NodeTypeFile

	StatusPending    = types.StatusPending
	StatusSuccessful = types.StatusSuccessful
	StatusFailed     = types.StatusFailed
)
