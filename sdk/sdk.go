package sdk

import (
	"fmt"
	"io/fs"

	"codeberg.org/Sylos/Spectra/internal/chaos"
	"codeberg.org/Sylos/Spectra/internal/config"
	"codeberg.org/Sylos/Spectra/internal/ephemeralfs"
	"codeberg.org/Sylos/Spectra/internal/spectrafs"
	"codeberg.org/Sylos/Spectra/internal/spectrafs/models"
	"codeberg.org/Sylos/Spectra/internal/types"
)

// fsInterface defines the common interface for both persistent and ephemeral implementations
type fsInterface interface {
	ListChildren(req models.ParentIdentifier) (*types.ListResult, error)
	GetNode(req models.NodeIdentifier) (*types.Node, error)
	GetFileData(id string) ([]byte, string, error)
	CreateFolder(req interface {
		models.ParentIdentifier
		models.NamedRequest
	}) (*types.Node, error)
	UploadFile(req interface {
		models.ParentIdentifier
		models.NamedRequest
		models.DataRequest
	}) (*types.Node, error)
	DeleteNode(req models.NodeIdentifier) error
	Reset() error
	Close() error
	GetConfig() *types.Config
	GetNodeCount(world string) (int, error)
	GetTableInfo() ([]types.TableInfo, error)
	GetSecondaryTables() []string
	GetStats() (*types.Stats, error)
	FlushStats()
}

// SpectraFS is the public SDK interface for the synthetic filesystem
// This wraps the internal implementation to provide a clean public API
type SpectraFS struct {
	impl  fsInterface
	chaos *chaos.Engine
}

// New creates a new SpectraFS instance using the specified config file
// The implementation (persistent or ephemeral) is selected based on the config mode
func New(configPath string) (*SpectraFS, error) {
	// Load configuration to determine mode
	cfg, err := config.LoadFromFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	var impl fsInterface

	// Select implementation based on mode
	if cfg.Mode == "ephemeral" {
		impl, err = ephemeralfs.NewEphemeralFS(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize EphemeralFS: %w", err)
		}
	} else {
		// Default to persistent mode
		impl, err = spectrafs.NewSpectraFS(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize SpectraFS: %w", err)
		}
	}

	return &SpectraFS{
		impl:  impl,
		chaos: chaos.NewEngine(cfg.Chaos, cfg.Seed.Seed),
	}, nil
}

// NewWithDefaults creates a new SpectraFS instance using default configuration
func NewWithDefaults() (*SpectraFS, error) {
	return New("configs/default.json")
}

// ListChildren returns the children of a given parent node
func (s *SpectraFS) ListChildren(req *models.ListChildrenRequest) (*types.ListResult, error) {
	if err := s.beforeOp(chaos.EndpointListChildren, 0); err != nil {
		return nil, err
	}
	return s.impl.ListChildren(req)
}

// GetNode retrieves a node using either ID or Path+TableName
func (s *SpectraFS) GetNode(req *models.GetNodeRequest) (*types.Node, error) {
	if err := s.beforeOp(chaos.EndpointGetNode, 0); err != nil {
		return nil, err
	}
	return s.impl.GetNode(req)
}

// GetFileData generates and returns file data with checksum for a given file ID
// The data is generated on-the-fly and not persisted
func (s *SpectraFS) GetFileData(id string) ([]byte, string, error) {
	if err := s.beforeOp(chaos.EndpointGetFileData, 0); err != nil {
		return nil, "", err
	}
	data, sum, err := s.impl.GetFileData(id)
	if err != nil {
		return nil, "", err
	}
	if err := s.afterOp(chaos.EndpointGetFileData, int64(len(data))); err != nil {
		return nil, "", err
	}
	return data, sum, nil
}

// CreateFolder creates a new folder node
func (s *SpectraFS) CreateFolder(req *models.CreateFolderRequest) (*types.Node, error) {
	if err := s.beforeOp(chaos.EndpointCreateFolder, 0); err != nil {
		return nil, err
	}
	return s.impl.CreateFolder(req)
}

// UploadFile handles file uploads - processes the data and creates a file node
// The actual file data is not persisted, only metadata
func (s *SpectraFS) UploadFile(req *models.UploadFileRequest) (*types.Node, error) {
	var n int64
	if req != nil {
		n = int64(len(req.Data))
	}
	if err := s.beforeOp(chaos.EndpointUploadFile, n); err != nil {
		return nil, err
	}
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

// GetStats retrieves the current filesystem statistics
func (s *SpectraFS) GetStats() (*Stats, error) {
	return s.impl.GetStats()
}

// FlushStats forces an immediate flush of the stats buffer
func (s *SpectraFS) FlushStats() {
	s.impl.FlushStats()
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
	Stats       = types.Stats
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
// Note: Currently only supported for persistent mode
func (s *SpectraFS) AsFS(world string) fs.FS {
	// Type assert to check if it's persistent mode
	if persistentImpl, ok := s.impl.(*spectrafs.SpectraFS); ok {
		return spectrafs.NewSpectraFSWrapper(persistentImpl, world)
	}
	// For ephemeral mode, return nil or a no-op wrapper
	// TODO: Implement ephemeral fs.FS wrapper if needed
	return nil
}

// AsFSWithDefaults returns an fs.FS instance using the "primary" world
// This is a convenience method for the most common use case
func (s *SpectraFS) AsFSWithDefaults() fs.FS {
	return s.AsFS("primary")
}

func (s *SpectraFS) beforeOp(endpoint string, bytes int64) error {
	if s == nil || s.chaos == nil {
		return nil
	}
	return s.chaos.BeforeOperation(endpoint, bytes)
}

func (s *SpectraFS) afterOp(endpoint string, bytes int64) error {
	if s == nil || s.chaos == nil {
		return nil
	}
	return s.chaos.AfterOperation(endpoint, bytes)
}

// ChaosEngine returns the chaos engine for HTTP middleware wiring.
func (s *SpectraFS) ChaosEngine() *chaos.Engine {
	if s == nil {
		return nil
	}
	return s.chaos
}
