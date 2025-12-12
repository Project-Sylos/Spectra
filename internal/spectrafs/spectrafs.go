package spectrafs

import (
	"fmt"
	"io"
	"io/fs"
	"strings"
	"time"

	"github.com/Project-Sylos/Spectra/internal/config"
	"github.com/Project-Sylos/Spectra/internal/db"
	"github.com/Project-Sylos/Spectra/internal/generator"
	"github.com/Project-Sylos/Spectra/internal/spectrafs/models"
	"github.com/Project-Sylos/Spectra/internal/types"
	"github.com/Project-Sylos/Spectra/internal/utils"
	"github.com/google/uuid"
)

// SpectraFS represents the main filesystem simulator with multi-table support
type SpectraFS struct {
	root string
	db   *db.DB
	cfg  *types.Config
	rng  *generator.RNG
}

// NewSpectraFS creates a new SpectraFS instance with multi-table support
func NewSpectraFS(configPath string) (*SpectraFS, error) {
	// Load configuration
	cfg, err := config.LoadFromFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize database with secondary tables
	// Note: InitializeSchema() already creates root nodes automatically
	database, err := db.New(cfg.Seed.DBPath, cfg.SecondaryTables)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize seeded random number generator
	rng := generator.NewRNG(cfg.Seed.Seed)

	return &SpectraFS{
		root: "root",
		db:   database,
		cfg:  cfg,
		rng:  rng,
	}, nil
}

// ListChildren retrieves children for a parent node in a specific world
// This is the OPTIMIZED single-table version with minimal DB queries
// Accepts any struct that implements the ParentIdentifier interface
func (s *SpectraFS) ListChildren(req models.ParentIdentifier) (*types.ListResult, error) {
	// Validate request
	if err := models.ValidateParentIdentifier(req); err != nil {
		return &types.ListResult{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// Resolve the parent node and extract world from request
	parent, world, err := s.resolveNodeAndWorld(req)
	if err != nil {
		return &types.ListResult{
			Success: false,
			Message: fmt.Sprintf("Parent node not found: %v", err),
		}, nil
	}

	// Check if parent exists in the requested world
	if !parent.ExistenceMap[world] {
		return &types.ListResult{
			Success: true,
			Message: fmt.Sprintf("Node does not exist in world %s", world),
			Folders: make([]types.Folder, 0),
			Files:   make([]types.File, 0),
		}, nil
	}

	// OPTIMIZATION: Get parent + children in ONE query
	nodes, err := s.db.GetParentAndChildren(parent.ID, world)
	if err != nil {
		return &types.ListResult{
			Success: false,
			Message: fmt.Sprintf("Failed to get parent and children: %v", err),
		}, nil
	}

	// First node is parent (if found), rest are children
	var children []*types.Node
	if len(nodes) > 0 {
		// nodes[0] is the parent, nodes[1:] are children
		children = nodes[1:]
	}

	// If no children exist, generate them
	if len(children) == 0 {
		generated, err := generator.GenerateChildren(parent, parent.DepthLevel, s.rng, s.cfg)
		if err != nil {
			return &types.ListResult{
				Success: false,
				Message: fmt.Sprintf("Failed to generate children: %v", err),
			}, nil
		}

		// OPTIMIZATION: Bulk insert all nodes in ONE transaction
		if err := s.db.BulkInsertNodes(generated); err != nil {
			return &types.ListResult{
				Success: false,
				Message: fmt.Sprintf("Failed to bulk insert nodes: %v", err),
			}, nil
		}

		// Filter children by requested world
		for _, node := range generated {
			if node.ExistenceMap[world] {
				children = append(children, node)
			}
		}
	}

	// Separate folders and files
	result := &types.ListResult{
		Success: true,
		Message: "Children retrieved successfully",
		Folders: make([]types.Folder, 0),
		Files:   make([]types.File, 0),
	}

	for _, child := range children {
		switch child.Type {
		case types.NodeTypeFolder:
			result.Folders = append(result.Folders, types.Folder{Node: *child})
		case types.NodeTypeFile:
			result.Files = append(result.Files, types.File{Node: *child})
		}
	}

	return result, nil
}

// GetNode retrieves a node using either ID or Path+World
// Accepts any struct that implements the NodeIdentifier interface
func (s *SpectraFS) GetNode(req models.NodeIdentifier) (*types.Node, error) {
	if err := models.ValidateNodeIdentifier(req); err != nil {
		return nil, err
	}

	id := req.GetID()
	path := req.GetPath()
	tableName := req.GetTableName()

	var (
		node *types.Node
		err  error
	)

	if id != "" {
		node, err = s.db.GetNodeByID(id)
	} else if path != "" {
		if tableName == "" {
			tableName = "primary" // Default to primary world
		}
		node, err = s.db.GetNodeByPath(path, tableName)
	} else {
		return nil, fmt.Errorf("either id or path must be specified")
	}

	if err != nil {
		return nil, err
	}

	if node == nil {
		return nil, fmt.Errorf("node not found")
	}

	return node, nil
}

// GetFileData generates deterministic file data and checksum for a file (not persisted)
func (s *SpectraFS) GetFileData(id string) ([]byte, string, error) {
	// Verify the node exists and is a file
	node, err := s.db.GetNodeByID(id)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get file node: %w", err)
	}

	if node.Type != types.NodeTypeFile {
		return nil, "", fmt.Errorf("node %s is not a file", id)
	}

	// Generate deterministic data and checksum
	data, checksum, err := s.getFileDataDeterministic(node.ID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate file data: %w", err)
	}

	return data, checksum, nil
}

// CreateFolder creates a new folder node
// Accepts any struct that implements ParentIdentifier and NamedRequest interfaces
func (s *SpectraFS) CreateFolder(req interface {
	models.ParentIdentifier
	models.NamedRequest
}) (*types.Node, error) {
	if err := models.ValidateParentIdentifier(req); err != nil {
		return nil, err
	}
	if req.GetName() == "" {
		return nil, fmt.Errorf("name is required")
	}

	// Resolve parent node
	parent, _, err := s.resolveNodeAndWorld(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent node: %w", err)
	}

	if parent.Type != types.NodeTypeFolder {
		return nil, fmt.Errorf("parent %s is not a folder", parent.ID)
	}

	// Create folder node with UUID
	nodeID := uuid.New().String()
	path := utils.JoinPath(parent.Path, req.GetName())

	// Roll dice for existence in each world - ensure all worlds have keys
	existenceMap := make(map[string]bool)

	// Primary is always true
	existenceMap["primary"] = true

	// For each secondary world, check parent existence first
	for worldName, probability := range s.cfg.SecondaryTables {
		// If parent doesn't exist in this world, child cannot exist
		if !parent.ExistenceMap[worldName] {
			existenceMap[worldName] = false
		} else {
			// Parent exists, so roll dice: roll [0.0, 1.0) must be <= probability
			roll := s.rng.Float64()
			existenceMap[worldName] = (roll <= probability)
		}
	}

	folderNode := &types.Node{
		ID:           nodeID,
		ParentID:     parent.ID,
		Name:         req.GetName(),
		Path:         path,
		ParentPath:   parent.Path,
		Type:         types.NodeTypeFolder,
		DepthLevel:   parent.DepthLevel + 1,
		Size:         0,
		LastUpdated:  time.Now(),
		Checksum:     nil,
		ExistenceMap: existenceMap,
	}

	// Insert node
	if err := s.db.InsertNode(folderNode); err != nil {
		return nil, fmt.Errorf("failed to insert folder node: %w", err)
	}

	return folderNode, nil
}

// UploadFile handles file uploads with single-table support
// Accepts any struct that implements ParentIdentifier, NamedRequest, and DataRequest interfaces
func (s *SpectraFS) UploadFile(req interface {
	models.ParentIdentifier
	models.NamedRequest
	models.DataRequest
}) (*types.Node, error) {
	if err := models.ValidateParentIdentifier(req); err != nil {
		return nil, err
	}
	if req.GetName() == "" {
		return nil, fmt.Errorf("name is required")
	}
	if len(req.GetData()) == 0 {
		return nil, fmt.Errorf("data is required")
	}

	// Resolve parent node
	parent, _, err := s.resolveNodeAndWorld(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent node: %w", err)
	}

	if parent.Type != types.NodeTypeFolder {
		return nil, fmt.Errorf("parent %s is not a folder", parent.ID)
	}

	// Generate UUID for the new file
	nodeID := uuid.New().String()
	path := utils.JoinPath(parent.Path, req.GetName())

	// Generate deterministic file data metadata (data itself is not persisted)
	data, checksum, err := generator.GenerateDeterministicFileData(s.cfg.Seed.FileBinarySeed)
	if err != nil {
		return nil, fmt.Errorf("failed to generate file data: %w", err)
	}

	// Roll dice for existence in each world - ensure all worlds have keys
	existenceMap := make(map[string]bool)

	// Primary is always true
	existenceMap["primary"] = true

	// For each secondary world, check parent existence first
	for worldName, probability := range s.cfg.SecondaryTables {
		// If parent doesn't exist in this world, child cannot exist
		if !parent.ExistenceMap[worldName] {
			existenceMap[worldName] = false
		} else {
			// Parent exists, so roll dice: roll [0.0, 1.0) must be <= probability
			roll := s.rng.Float64()
			existenceMap[worldName] = (roll <= probability)
		}
	}

	fileNode := &types.Node{
		ID:           nodeID,
		ParentID:     parent.ID,
		Name:         req.GetName(),
		Path:         path,
		ParentPath:   parent.Path,
		Type:         types.NodeTypeFile,
		DepthLevel:   parent.DepthLevel + 1,
		Size:         int64(len(data)),
		LastUpdated:  time.Now(),
		Checksum:     &checksum,
		ExistenceMap: existenceMap,
	}

	// Insert node
	if err := s.db.InsertNode(fileNode); err != nil {
		return nil, fmt.Errorf("failed to insert uploaded file node: %w", err)
	}

	return fileNode, nil
}

// Reset clears all nodes and recreates the root
func (s *SpectraFS) Reset() error {
	// Delete all nodes
	if err := s.db.DeleteAllNodes(); err != nil {
		return fmt.Errorf("failed to delete all nodes: %w", err)
	}

	// Recreate single root node with all worlds
	if err := s.db.CreateRootNode(); err != nil {
		return fmt.Errorf("failed to recreate root node: %w", err)
	}

	// Reset random number generator with same seed for reproducibility
	s.rng = generator.NewRNG(s.cfg.Seed.Seed)

	return nil
}

// Close closes the database connection after performing a WAL checkpoint to ensure data persistence.
// This ensures all changes are fully saved before the process finishes.
func (s *SpectraFS) Close() error {
	return s.db.Close()
}

// GetConfig returns the current configuration
func (s *SpectraFS) GetConfig() *types.Config {
	return s.cfg
}

// GetNodeCount returns the total number of nodes in a specific world
func (s *SpectraFS) GetNodeCount(world string) (int, error) {
	return s.db.GetNodeCount(world)
}

// GetTableInfo returns information about all tables
func (s *SpectraFS) GetTableInfo() ([]types.TableInfo, error) {
	return s.db.GetTableInfo()
}

// DeleteNode deletes a node using either ID or Path+World
// Accepts any struct that implements the NodeIdentifier interface
func (s *SpectraFS) DeleteNode(req models.NodeIdentifier) error {
	if err := models.ValidateNodeIdentifier(req); err != nil {
		return err
	}

	// Resolve node to get its ID
	node, _, err := s.resolveNodeAndWorld(req)
	if err != nil {
		return fmt.Errorf("failed to resolve node: %w", err)
	}

	// Prevent deletion of root node
	if node.ID == "root" {
		return fmt.Errorf("cannot delete root node")
	}
	return s.db.DeleteNode(node.ID)
}

// GetSecondaryTables returns the list of secondary table names
func (s *SpectraFS) GetSecondaryTables() []string {
	return s.db.GetSecondaryTables()
}

// GetStats retrieves the current filesystem statistics
func (s *SpectraFS) GetStats() (*types.Stats, error) {
	return s.db.GetStats()
}

// resolveNodeAndWorld resolves a node and world from a request using interfaces
// Supports both NodeIdentifier (for ID or Path+World) and ParentIdentifier (for ParentID or ParentPath+World)
// Returns the node and the world name (defaults to "primary" if not specified)
func (s *SpectraFS) resolveNodeAndWorld(req any) (*types.Node, string, error) {
	var node *types.Node
	var world string
	var err error

	// Try NodeIdentifier first (for GetNode, DeleteNode)
	if nodeID, ok := req.(models.NodeIdentifier); ok {
		id := nodeID.GetID()
		path := nodeID.GetPath()
		world = nodeID.GetTableName() // TableName is used for world name

		if world == "" {
			world = "primary" // Default to primary world
		}

		if id != "" {
			node, err = s.db.GetNodeByID(id)
		} else if path != "" {
			node, err = s.db.GetNodeByPath(path, world)
		} else {
			return nil, "", fmt.Errorf("either id or path must be specified")
		}
		return node, world, err
	}

	// Try ParentIdentifier (for ListChildren, CreateFolder, UploadFile)
	if parentID, ok := req.(models.ParentIdentifier); ok {
		parentIDStr := parentID.GetParentID()
		parentPath := parentID.GetParentPath()
		world = parentID.GetTableName() // TableName is used for world name

		if world == "" {
			world = "primary" // Default to primary world
		}

		if parentIDStr != "" {
			node, err = s.db.GetNodeByID(parentIDStr)
		} else if parentPath != "" {
			node, err = s.db.GetNodeByPath(parentPath, world)
		} else {
			return nil, "", fmt.Errorf("either parent_id or parent_path must be specified")
		}
		return node, world, err
	}

	return nil, "", fmt.Errorf("unsupported request type - must implement NodeIdentifier or ParentIdentifier")
}

// getFileDataDeterministic generates deterministic file data using the configured binary seed
// This ensures every retrieval returns the same data, satisfying tools that rely on stable content
func (s *SpectraFS) getFileDataDeterministic(nodeID string) ([]byte, string, error) {
	return generator.GenerateDeterministicFileData(s.cfg.Seed.FileBinarySeed)
}

// SpectraFSWrapper wraps SpectraFS to implement fs.FS interface for a specific world
type SpectraFSWrapper struct {
	fs    *SpectraFS
	world string
}

// NewSpectraFSWrapper creates a new fs.FS wrapper for a specific world
func NewSpectraFSWrapper(fs *SpectraFS, world string) *SpectraFSWrapper {
	if world == "" {
		world = "primary"
	}
	return &SpectraFSWrapper{
		fs:    fs,
		world: world,
	}
}

// Open opens the named file or directory
func (w *SpectraFSWrapper) Open(name string) (fs.File, error) {
	// Normalize path - handle root specially
	path := name
	if path == "." || path == "" {
		path = "/"
	} else if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Validate path (skip validation for root "/")
	if path != "/" && !fs.ValidPath(strings.TrimPrefix(path, "/")) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	// Get node by path in the bound world
	req := &models.GetNodeRequest{
		Path:      path,
		TableName: w.world,
	}

	node, err := w.fs.GetNode(req)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	// Check if node exists in the requested world
	if !node.ExistenceMap[w.world] {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	// Return appropriate file handle
	if node.Type == types.NodeTypeFolder {
		// For directories, we need to list children
		listReq := &models.ListChildrenRequest{
			ParentPath: path,
			TableName:  w.world,
		}

		result, err := w.fs.ListChildren(listReq)
		if err != nil {
			return nil, &fs.PathError{Op: "open", Path: name, Err: err}
		}

		// Convert to DirEntry slice
		entries := make([]fs.DirEntry, 0, len(result.Folders)+len(result.Files))
		for _, folder := range result.Folders {
			entries = append(entries, NewDirEntry(&folder.Node))
		}
		for _, file := range result.Files {
			entries = append(entries, NewDirEntry(&file.Node))
		}

		return &spectraDir{
			node:    node,
			entries: entries,
		}, nil
	}

	// For files, generate deterministic data
	data, _, err := w.fs.getFileDataDeterministic(node.ID)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}

	return &spectraFile{
		node:   node,
		data:   data,
		offset: 0,
	}, nil
}

// ReadFile reads the named file and returns its contents
func (w *SpectraFSWrapper) ReadFile(name string) ([]byte, error) {
	file, err := w.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		return nil, &fs.PathError{Op: "readfile", Path: name, Err: fmt.Errorf("is a directory")}
	}

	// Read all data - we know the exact size, so use ReadFull
	data := make([]byte, info.Size())
	_, err = io.ReadFull(file, data)
	if err != nil {
		// ReadFull may return EOF or ErrUnexpectedEOF if not all bytes read
		// This shouldn't happen with our implementation, but handle gracefully
		return nil, &fs.PathError{Op: "readfile", Path: name, Err: err}
	}

	return data, nil
}

// ReadDir reads the named directory and returns a list of directory entries
func (w *SpectraFSWrapper) ReadDir(name string) ([]fs.DirEntry, error) {
	file, err := w.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	dir, ok := file.(fs.ReadDirFile)
	if !ok {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fmt.Errorf("not a directory")}
	}

	entries, err := dir.ReadDir(-1)
	if err != nil && err != fs.ErrClosed {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: err}
	}

	return entries, nil
}

// Stat returns a FileInfo describing the named file
func (w *SpectraFSWrapper) Stat(name string) (fs.FileInfo, error) {
	// Normalize path
	path := name
	if path == "." || path == "" {
		path = "/"
	} else if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Validate path (skip validation for root "/")
	if path != "/" && !fs.ValidPath(strings.TrimPrefix(path, "/")) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}

	// Get node by path in the bound world
	req := &models.GetNodeRequest{
		Path:      path,
		TableName: w.world,
	}

	node, err := w.fs.GetNode(req)
	if err != nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
	}

	// Check if node exists in the requested world
	if !node.ExistenceMap[w.world] {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
	}

	return NewFileInfo(node), nil
}

// Glob returns the names of all files matching pattern
func (w *SpectraFSWrapper) Glob(pattern string) ([]string, error) {
	return fs.Glob(w, pattern)
}
