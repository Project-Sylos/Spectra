package ephemeralfs

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"path"
	"time"

	"codeberg.org/Sylos/Spectra/internal/generator"
	"codeberg.org/Sylos/Spectra/internal/spectrafs/models"
	"codeberg.org/Sylos/Spectra/internal/types"
	"codeberg.org/Sylos/Spectra/internal/utils"
)

// EphemeralFS is a stateless, deterministic filesystem emulator.
// Callers must supply path (parent_path) and depth for ListChildren.
// Child generation is deterministic: same (path, depth) always yields the same children.
// Node identity is derived, not persisted.
type EphemeralFS struct {
	cfg *types.Config
}

// NewEphemeralFS creates a new EphemeralFS instance
func NewEphemeralFS(cfg *types.Config) (*EphemeralFS, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}

	return &EphemeralFS{
		cfg: cfg,
	}, nil
}

// deterministicSeed returns a stable int64 seed from config seed, path, and depth.
func deterministicSeed(globalSeed int64, pathStr string, depth int) int64 {
	h := fnv.New64a()
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(globalSeed))
	_, _ = h.Write(buf[:])
	_, _ = h.Write([]byte(pathStr))
	binary.LittleEndian.PutUint64(buf[:], uint64(depth))
	_, _ = h.Write(buf[:])
	return int64(h.Sum64())
}

// buildFullExistenceMap creates an existence map with all worlds set to true
// This is used for generator compatibility
func buildFullExistenceMap(cfg *types.Config) map[string]bool {
	em := map[string]bool{"primary": true}
	for world := range cfg.SecondaryTables {
		em[world] = true
	}
	return em
}

// ListChildren retrieves children for a parent node, generating them on-the-fly.
// Path (parent_path) and depth are required in ephemeral mode.
// Same (path, depth) always yields the same children (deterministic).
func (e *EphemeralFS) ListChildren(req models.ParentIdentifier) (*types.ListResult, error) {
	// EphemeralFS contract: require path and depth (do not infer or fabricate)
	pathStr := req.GetParentPath()
	if pathStr == "" {
		return &types.ListResult{
			Success: false,
			Message: "path is required in ephemeral mode (use parent_path)",
		}, nil
	}

	var depth int
	listReq, ok := req.(*models.ListChildrenRequest)
	if !ok || listReq.Depth == nil {
		return &types.ListResult{
			Success: false,
			Message: "depth is required in ephemeral mode",
		}, nil
	}
	depth = *listReq.Depth

	// Validate request for other fields (e.g. ParentID or TableName if needed by caller)
	if err := models.ValidateParentIdentifier(req); err != nil {
		return &types.ListResult{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// Check depth constraint
	if depth >= e.cfg.Seed.MaxDepth {
		return &types.ListResult{
			Success: true,
			Message: "Maximum depth reached",
			Folders: make([]types.Folder, 0),
			Files:   make([]types.File, 0),
		}, nil
	}

	// Build synthetic parent from real path (do not fabricate path)
	parentPath := path.Dir(pathStr)
	parentID := req.GetParentID()
	if parentID == "" {
		parentID = utils.DeterministicNodeID(pathStr, types.NodeTypeFolder)
	}
	parent := &types.Node{
		ID:           parentID,
		ParentID:     "",
		Name:         "",
		Path:         pathStr,
		ParentPath:   parentPath,
		Type:         types.NodeTypeFolder,
		DepthLevel:   depth,
		Size:         0,
		LastUpdated:  time.Now(),
		Checksum:     nil,
		ExistenceMap: buildFullExistenceMap(e.cfg),
		ChildIDs:     nil,
	}

	// Derive deterministic RNG seed from (global seed, effectivePath, depth); fresh RNG per call.
	// In diverging-tree mode, use worldName//path so each world gets a different tree shape.
	effectivePath := pathStr
	if e.cfg.Seed.DivergingTreeMode {
		if world := req.GetTableName(); world != "" {
			effectivePath = world + "//" + pathStr
		}
	}
	seed := deterministicSeed(e.cfg.Seed.Seed, effectivePath, depth)
	rng := generator.NewRNG(seed)

	// Generate children using shared generator (no shared mutable RNG)
	generated, err := generator.GenerateChildren(parent, depth, rng, e.cfg)
	if err != nil {
		return &types.ListResult{
			Success: false,
			Message: fmt.Sprintf("Failed to generate children: %v", err),
		}, nil
	}

	// Separate folders and files
	result := &types.ListResult{
		Success: true,
		Message: "Children generated successfully",
		Folders: make([]types.Folder, 0),
		Files:   make([]types.File, 0),
	}

	for _, child := range generated {
		switch child.Type {
		case types.NodeTypeFolder:
			result.Folders = append(result.Folders, types.Folder{Node: *child})
		case types.NodeTypeFile:
			result.Files = append(result.Files, types.File{Node: *child})
		}
	}

	return result, nil
}

// GetNode generates a synthetic node from ID
func (e *EphemeralFS) GetNode(req models.NodeIdentifier) (*types.Node, error) {
	if err := models.ValidateNodeIdentifier(req); err != nil {
		return nil, err
	}

	id := req.GetID()
	if id == "" {
		id = "root"
	}

	// Generate synthetic node
	// For root, create a folder node
	if id == "root" {
		return &types.Node{
			ID:           "root",
			ParentID:     "",
			Name:         "",
			Path:         "/",
			ParentPath:   "",
			Type:         types.NodeTypeFolder,
			DepthLevel:   0,
			Size:         0,
			LastUpdated:  time.Now(),
			Checksum:     nil,
			ExistenceMap: buildFullExistenceMap(e.cfg),
			ChildIDs:     nil,
		}, nil
	}

	// For other IDs, generate a generic node
	// In ephemeral mode, we don't track what type it is, so default to folder
	return &types.Node{
		ID:           id,
		ParentID:     "",
		Name:         "",
		Path:         "/",
		ParentPath:   "",
		Type:         types.NodeTypeFolder,
		DepthLevel:   0,
		Size:         0,
		LastUpdated:  time.Now(),
		Checksum:     nil,
		ExistenceMap: buildFullExistenceMap(e.cfg),
		ChildIDs:     nil,
	}, nil
}

// GetFileData generates deterministic file data and checksum
func (e *EphemeralFS) GetFileData(id string) ([]byte, string, error) {
	// Generate deterministic data and checksum
	data, checksum, err := generator.GenerateDeterministicFileData(e.cfg.Seed.FileBinarySeed)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate file data: %w", err)
	}

	return data, checksum, nil
}

// CreateFolder creates a new folder node (not persisted)
func (e *EphemeralFS) CreateFolder(req interface {
	models.ParentIdentifier
	models.NamedRequest
}) (*types.Node, error) {
	if err := models.ValidateParentIdentifier(req); err != nil {
		return nil, err
	}
	if req.GetName() == "" {
		return nil, fmt.Errorf("name is required")
	}

	parentPath := req.GetParentPath()
	if parentPath == "" {
		parentPath = "/"
	}
	pathStr := utils.JoinPath(parentPath, req.GetName())
	nodeID := utils.DeterministicNodeID(pathStr, types.NodeTypeFolder)

	folderNode := &types.Node{
		ID:           nodeID,
		ParentID:     req.GetParentID(),
		Name:         req.GetName(),
		Path:         pathStr,
		ParentPath:   parentPath,
		Type:         types.NodeTypeFolder,
		DepthLevel:   0,
		Size:         0,
		LastUpdated:  time.Now(),
		Checksum:     nil,
		ExistenceMap: buildFullExistenceMap(e.cfg),
	}

	return folderNode, nil
}

// UploadFile handles file uploads (not persisted)
func (e *EphemeralFS) UploadFile(req interface {
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

	parentPath := req.GetParentPath()
	if parentPath == "" {
		parentPath = "/"
	}
	pathStr := utils.JoinPath(parentPath, req.GetName())
	nodeID := utils.DeterministicNodeID(pathStr, types.NodeTypeFile)

	// Generate deterministic file data metadata
	data, checksum, err := generator.GenerateDeterministicFileData(e.cfg.Seed.FileBinarySeed)
	if err != nil {
		return nil, fmt.Errorf("failed to generate file data: %w", err)
	}

	fileNode := &types.Node{
		ID:           nodeID,
		ParentID:     req.GetParentID(),
		Name:         req.GetName(),
		Path:         pathStr,
		ParentPath:   parentPath,
		Type:         types.NodeTypeFile,
		DepthLevel:   0,
		Size:         int64(len(data)),
		LastUpdated:  time.Now(),
		Checksum:     &checksum,
		ExistenceMap: buildFullExistenceMap(e.cfg),
	}

	return fileNode, nil
}

// DeleteNode is a no-op in ephemeral mode
func (e *EphemeralFS) DeleteNode(req models.NodeIdentifier) error {
	// No-op - nothing to delete
	return nil
}

// Reset is a no-op in ephemeral mode (no state to reset)
func (e *EphemeralFS) Reset() error {
	return nil
}

// Close is a no-op in ephemeral mode
func (e *EphemeralFS) Close() error {
	// No-op - no DB to close
	return nil
}

// GetConfig returns the current configuration
func (e *EphemeralFS) GetConfig() *types.Config {
	return e.cfg
}

// GetNodeCount returns 0 (no nodes tracked)
func (e *EphemeralFS) GetNodeCount(world string) (int, error) {
	return 0, nil
}

// GetTableInfo returns empty list
func (e *EphemeralFS) GetTableInfo() ([]types.TableInfo, error) {
	return []types.TableInfo{}, nil
}

// GetSecondaryTables returns the list of secondary table names
func (e *EphemeralFS) GetSecondaryTables() []string {
	tables := make([]string, 0, len(e.cfg.SecondaryTables))
	for name := range e.cfg.SecondaryTables {
		tables = append(tables, name)
	}
	return tables
}

// GetStats returns zero stats
func (e *EphemeralFS) GetStats() (*types.Stats, error) {
	return &types.Stats{
		FileCount:      0,
		FolderCount:    0,
		TotalFileSize:  0,
		SecondaryNodes: make(map[string]int64),
	}, nil
}

// FlushStats is a no-op in ephemeral mode
func (e *EphemeralFS) FlushStats() {
	// No-op - no stats to flush
}
