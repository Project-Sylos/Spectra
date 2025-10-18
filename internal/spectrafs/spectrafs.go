package spectrafs

import (
	"fmt"
	"time"

	"github.com/Project-Sylos/Spectra/internal/config"
	"github.com/Project-Sylos/Spectra/internal/db"
	"github.com/Project-Sylos/Spectra/internal/generator"
	"github.com/Project-Sylos/Spectra/internal/types"
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
	database, err := db.New(cfg.Seed.DBPath, cfg.SecondaryTables)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Create root node if it doesn't exist
	rootExists, err := database.CheckChildrenExist("p-root")
	if err != nil {
		database.Close()
		return nil, fmt.Errorf("failed to check root existence: %w", err)
	}

	if !rootExists {
		if err := database.CreateRootNode(); err != nil {
			database.Close()
			return nil, fmt.Errorf("failed to create root node: %w", err)
		}
	}

	// Initialize seeded random number generator
	rng := generator.NewRNG(cfg.Seed.Seed)

	return &SpectraFS{
		root: "p-root",
		db:   database,
		cfg:  cfg,
		rng:  rng,
	}, nil
}

// ListChildren implements the enhanced API with multi-table support
// Returns success/failure response with proper error handling
func (s *SpectraFS) ListChildren(parentID string) (*types.ListResult, error) {
	// First, verify the parent node exists
	parent, err := s.db.GetNodeByID(parentID)
	if err != nil {
		return &types.ListResult{
			Success: false,
			Message: fmt.Sprintf("Parent node not found: %s", parentID),
		}, nil
	}

	// Check if children already exist in the appropriate table
	childrenExist, err := s.db.CheckChildrenExist(parentID)
	if err != nil {
		return &types.ListResult{
			Success: false,
			Message: fmt.Sprintf("Failed to check children existence: %v", err),
		}, nil
	}

	// If children don't exist, generate them
	if !childrenExist {
		// Generate children in primary table
		children, err := generator.GenerateChildren(parent, parent.DepthLevel, s.rng, s.cfg)
		if err != nil {
			return &types.ListResult{
				Success: false,
				Message: fmt.Sprintf("Failed to generate children: %v", err),
			}, nil
		}

		// Process each child for secondary table generation
		for _, child := range children {
			// Insert into primary table
			if err := s.db.InsertPrimaryNode(child); err != nil {
				return &types.ListResult{
					Success: false,
					Message: fmt.Sprintf("Failed to insert primary child node: %v", err),
				}, nil
			}

			// Generate secondary nodes based on probability
			secondaryNodes, err := generator.GenerateSecondaryNodes(child, s.cfg, s.rng)
			if err != nil {
				return &types.ListResult{
					Success: false,
					Message: fmt.Sprintf("Failed to generate secondary nodes: %v", err),
				}, nil
			}

			// Update secondary existence map in primary node
			existenceMap := make(map[string]bool)
			for tableName, secondaryNode := range secondaryNodes {
				existenceMap[tableName] = true

				// Insert into secondary table
				if err := s.db.InsertSecondaryNode(tableName, secondaryNode); err != nil {
					return &types.ListResult{
						Success: false,
						Message: fmt.Sprintf("Failed to insert secondary node into %s: %v", tableName, err),
					}, nil
				}
			}

			// Update primary node with secondary existence map
			if err := s.db.UpdateSecondaryExistenceMap(child.ID, existenceMap); err != nil {
				return &types.ListResult{
					Success: false,
					Message: fmt.Sprintf("Failed to update secondary existence map: %v", err),
				}, nil
			}
		}
	}

	// Retrieve children from the appropriate table
	children, err := s.db.GetChildrenByParentID(parentID)
	if err != nil {
		return &types.ListResult{
			Success: false,
			Message: fmt.Sprintf("Failed to retrieve children: %v", err),
		}, nil
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

// GetNode retrieves a node by ID from the appropriate table
func (s *SpectraFS) GetNode(id string) (*types.Node, error) {
	return s.db.GetNodeByID(id)
}

// GetFileData generates 1KB random data and checksum for a file (not persisted)
func (s *SpectraFS) GetFileData(id string) ([]byte, string, error) {
	// Verify the node exists and is a file
	node, err := s.db.GetNodeByID(id)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get file node: %w", err)
	}

	if node.Type != types.NodeTypeFile {
		return nil, "", fmt.Errorf("node %s is not a file", id)
	}

	// Generate random data and checksum
	data, checksum, err := generator.GenerateFileData(s.rng)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate file data: %w", err)
	}
	return data, checksum, nil
}

// CreateFolder creates a new folder node
func (s *SpectraFS) CreateFolder(parentID, name string) (*types.Node, error) {
	// Verify parent exists and is a folder
	parent, err := s.db.GetNodeByID(parentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent node: %w", err)
	}

	if parent.Type != types.NodeTypeFolder {
		return nil, fmt.Errorf("parent %s is not a folder", parentID)
	}

	// Create folder node
	folderNode, err := s.db.CreateFolder(parentID, name, parent.DepthLevel+1)
	if err != nil {
		return nil, fmt.Errorf("failed to create folder node: %w", err)
	}

	// Insert primary node
	if err := s.db.InsertPrimaryNode(folderNode); err != nil {
		return nil, fmt.Errorf("failed to insert primary folder node: %w", err)
	}

	// Generate secondary nodes based on probabilities
	secondaryNodes, err := generator.GenerateSecondaryNodes(folderNode, s.cfg, s.rng)
	if err != nil {
		return nil, fmt.Errorf("failed to generate secondary nodes: %w", err)
	}

	// Insert secondary nodes
	for tableName, secondaryNode := range secondaryNodes {
		if err := s.db.InsertSecondaryNode(tableName, secondaryNode); err != nil {
			return nil, fmt.Errorf("failed to insert secondary node into %s: %w", tableName, err)
		}
	}

	// Update secondary existence map in primary node
	existenceMap := make(map[string]bool)
	for tableName := range secondaryNodes {
		existenceMap[tableName] = true
	}
	folderNode.SecondaryExistenceMap = existenceMap

	if err := s.db.UpdateSecondaryExistenceMap(folderNode.ID, existenceMap); err != nil {
		return nil, fmt.Errorf("failed to update secondary existence map: %w", err)
	}

	return folderNode, nil
}

// UploadFile handles file uploads with multi-table support
func (s *SpectraFS) UploadFile(parentID, name string, data []byte) (*types.Node, error) {
	// Verify parent exists and is a folder
	parent, err := s.db.GetNodeByID(parentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent node: %w", err)
	}

	if parent.Type != types.NodeTypeFolder {
		return nil, fmt.Errorf("parent %s is not a folder", parentID)
	}

	// Generate UUID for the new file
	fileUUID := uuid.New().String()
	primaryID := types.PrimaryPrefix + fileUUID

	// Create file node
	path := fmt.Sprintf("%s/%s", parent.Path, name)
	fileNode := &types.Node{
		ID:                    primaryID,
		ParentID:              parentID,
		Name:                  name,
		Path:                  path,
		Type:                  types.NodeTypeFile,
		DepthLevel:            parent.DepthLevel + 1,
		Size:                  1024, // 1KB as specified
		LastUpdated:           time.Now(),
		TraversalStatus:       types.StatusPending,
		SecondaryExistenceMap: make(map[string]bool),
	}

	// Insert into primary table
	if err := s.db.InsertPrimaryNode(fileNode); err != nil {
		return nil, fmt.Errorf("failed to insert uploaded file node: %w", err)
	}

	// Generate secondary nodes based on probability
	secondaryNodes, err := generator.GenerateSecondaryNodes(fileNode, s.cfg, s.rng)
	if err != nil {
		return nil, fmt.Errorf("failed to generate secondary nodes: %w", err)
	}

	// Update secondary existence map
	existenceMap := make(map[string]bool)
	for tableName, secondaryNode := range secondaryNodes {
		existenceMap[tableName] = true

		// Insert into secondary table
		if err := s.db.InsertSecondaryNode(tableName, secondaryNode); err != nil {
			return nil, fmt.Errorf("failed to insert secondary node into %s: %w", tableName, err)
		}
	}

	// Update primary node with secondary existence map
	if err := s.db.UpdateSecondaryExistenceMap(fileNode.ID, existenceMap); err != nil {
		return nil, fmt.Errorf("failed to update secondary existence map: %w", err)
	}

	// Note: checksum is computed but not stored in DB per current design
	_, _, err = generator.GenerateFileDataForUpload(data, s.rng)
	if err != nil {
		return nil, fmt.Errorf("failed to generate file data for upload: %w", err)
	}

	return fileNode, nil
}

// Reset clears all nodes and recreates the root
func (s *SpectraFS) Reset() error {
	// Delete all nodes from all tables
	if err := s.db.DeleteAllNodes(); err != nil {
		return fmt.Errorf("failed to delete all nodes: %w", err)
	}

	// Recreate root node
	if err := s.db.CreateRootNode(); err != nil {
		return fmt.Errorf("failed to recreate root node: %w", err)
	}

	// Reset random number generator with same seed for reproducibility
	s.rng = generator.NewRNG(s.cfg.Seed.Seed)

	return nil
}

// Close closes the database connection
func (s *SpectraFS) Close() error {
	return s.db.Close()
}

// GetConfig returns the current configuration
func (s *SpectraFS) GetConfig() *types.Config {
	return s.cfg
}

// GetNodeCount returns the total number of nodes in a specific table
func (s *SpectraFS) GetNodeCount(tableName string) (int, error) {
	return s.db.GetNodeCount(tableName)
}

// GetTableInfo returns information about all tables
func (s *SpectraFS) GetTableInfo() ([]types.TableInfo, error) {
	return s.db.GetTableInfo()
}

// UpdateTraversalStatus updates the traversal status of a node
func (s *SpectraFS) UpdateTraversalStatus(id, status string) error {
	// Validate status
	if status != types.StatusPending && status != types.StatusSuccessful && status != types.StatusFailed {
		return fmt.Errorf("invalid traversal status: %s", status)
	}
	return s.db.UpdateTraversalStatus(id, status)
}

// DeleteNode deletes a node by its ID
func (s *SpectraFS) DeleteNode(id string) error {
	// Prevent deletion of root node
	if id == "p-root" {
		return fmt.Errorf("cannot delete root node")
	}
	return s.db.DeleteNode(id)
}

// GetSecondaryTables returns the list of secondary table names
func (s *SpectraFS) GetSecondaryTables() []string {
	return s.db.GetSecondaryTables()
}
