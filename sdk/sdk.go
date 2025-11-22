package sdk

import (
	"fmt"
	"io/fs"

	"github.com/Project-Sylos/Spectra/internal/spectrafs"
	"github.com/Project-Sylos/Spectra/internal/spectrafs/models"
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
func (s *SpectraFS) ListChildren(req *models.ListChildrenRequest) (*types.ListResult, error) {
	return s.impl.ListChildren(req)
}

// GetNode retrieves a node using either ID or Path+TableName
func (s *SpectraFS) GetNode(req *models.GetNodeRequest) (*types.Node, error) {
	return s.impl.GetNode(req)
}

// GetFileData generates and returns file data with checksum for a given file ID
// The data is generated on-the-fly and not persisted
func (s *SpectraFS) GetFileData(id string) ([]byte, string, error) {
	return s.impl.GetFileData(id)
}

// CreateFolder creates a new folder node
func (s *SpectraFS) CreateFolder(req *models.CreateFolderRequest) (*types.Node, error) {
	return s.impl.CreateFolder(req)
}

// UploadFile handles file uploads - processes the data and creates a file node
// The actual file data is not persisted, only metadata
func (s *SpectraFS) UploadFile(req *models.UploadFileRequest) (*types.Node, error) {
	return s.impl.UploadFile(req)
}

// Reset clears all nodes and recreates the root
func (s *SpectraFS) Reset() error {
	return s.impl.Reset()
}

// Close closes the database connection after performing a WAL checkpoint to ensure data persistence.
// This ensures all changes are fully saved before the process finishes.
// Always call this method during graceful shutdown to guarantee data integrity.
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

// DeleteNode deletes a node using either ID or Path+World
func (s *SpectraFS) DeleteNode(req *models.DeleteNodeRequest) error {
	return s.impl.DeleteNode(req)
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

// Re-export request models
type (
	GetNodeRequest      = models.GetNodeRequest
	ListChildrenRequest = models.ListChildrenRequest
	CreateFolderRequest = models.CreateFolderRequest
	UploadFileRequest   = models.UploadFileRequest
	DeleteNodeRequest   = models.DeleteNodeRequest
)

// Re-export constants
const (
	NodeTypeFolder = types.NodeTypeFolder
	NodeTypeFile   = types.NodeTypeFile

	StatusPending    = types.StatusPending
	StatusSuccessful = types.StatusSuccessful
	StatusFailed     = types.StatusFailed
)

// AsFS returns an fs.FS instance bound to a specific world
// This allows SpectraFS to be used with tools like Rclone
// Each world is projected as its own separate filesystem
func (s *SpectraFS) AsFS(world string) fs.FS {
	return spectrafs.NewSpectraFSWrapper(s.impl, world)
}

// AsFSWithDefaults returns an fs.FS instance using the "primary" world
// This is a convenience method for the most common use case
func (s *SpectraFS) AsFSWithDefaults() fs.FS {
	return s.AsFS("primary")
}
