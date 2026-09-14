package ephemeralfs

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"path"
	"strings"
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

	// Seed key: only used for RNG seeding so tree shape can differ per world. Never used for Node.Path.
	// In diverging-tree mode we use worldName//path as the seed key; all returned nodes still have
	// normal paths (e.g. /foo/bar), so overlap checks across worlds see the same path space.
	seedKey := pathStr
	if e.cfg.Seed.DivergingTreeMode {
		if world := req.GetTableName(); world != "" {
			seedKey = world + "//" + pathStr
		}
	}
	seed := deterministicSeed(e.cfg.Seed.Seed, seedKey, depth)
	rng := generator.NewRNG(seed)

	// Generate children using shared generator (no shared mutable RNG). parent.Path is unchanged.
	generated, err := generator.GenerateChildren(parent, depth, rng, e.cfg)
	if err != nil {
		return &types.ListResult{
			Success: false,
			Message: fmt.Sprintf("Failed to generate children: %v", err),
		}, nil
	}

	// Separate folders and files. In ephemeral mode, the list is for a specific world (table);
	// ensure every returned child is marked as existing in that world so callers that filter
	// by ExistenceMap[world] don't end up with 0 children due to RNG rolls.
	requestWorld := req.GetTableName()
	result := &types.ListResult{
		Success: true,
		Message: "Children generated successfully",
		Folders: make([]types.Folder, 0),
		Files:   make([]types.File, 0),
	}

	for _, child := range generated {
		if requestWorld != "" && child.ExistenceMap != nil {
			child.ExistenceMap[requestWorld] = true
		}
		switch child.Type {
		case types.NodeTypeFolder:
			result.Folders = append(result.Folders, types.Folder{Node: *child})
		case types.NodeTypeFile:
			result.Files = append(result.Files, types.File{Node: *child})
		}
	}

	return result, nil
}

// GetNode returns a synthetic node. Path+world looks up a generated child of the parent;
// ID-only lookups return a generic node (root when id is empty or "root").
func (e *EphemeralFS) GetNode(req models.NodeIdentifier) (*types.Node, error) {
	if err := models.ValidateNodeIdentifier(req); err != nil {
		return nil, err
	}

	pathStr := req.GetPath()
	if pathStr != "" && pathStr != "/" {
		return e.nodeAtPath(pathStr, req.GetTableName())
	}

	id := req.GetID()
	if id == "" || id == "root" || pathStr == "/" {
		return e.rootNode(), nil
	}

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

func (e *EphemeralFS) rootNode() *types.Node {
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
	}
}

func (e *EphemeralFS) nodeAtPath(pathStr, world string) (*types.Node, error) {
	parentPath := path.Dir(pathStr)
	if parentPath == "." {
		parentPath = "/"
	}
	name := path.Base(pathStr)
	depth := utils.PathDepth(parentPath)
	result, err := e.ListChildren(&models.ListChildrenRequest{
		ParentPath: parentPath,
		TableName:  world,
		Depth:      &depth,
	})
	if err != nil {
		return nil, err
	}
	if result == nil || !result.Success {
		msg := "node not found"
		if result != nil && result.Message != "" {
			msg = result.Message
		}
		return nil, fmt.Errorf("%s", msg)
	}
	for i := range result.Folders {
		if result.Folders[i].Name == name {
			node := result.Folders[i].Node
			return &node, nil
		}
	}
	for i := range result.Files {
		if result.Files[i].Name == name {
			node := result.Files[i].Node
			return &node, nil
		}
	}
	return nil, fmt.Errorf("node not found")
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

// RenameNode returns a renamed node metadata object (not persisted) in ephemeral mode.
func (e *EphemeralFS) RenameNode(req *models.RenameNodeRequest) (*types.Node, error) {
	if req == nil {
		return nil, fmt.Errorf("rename request is required")
	}
	newName := strings.TrimSpace(req.GetNewName())
	if newName == "" {
		return nil, fmt.Errorf("new name is required")
	}
	oldPath := strings.TrimSpace(req.GetPath())
	if oldPath == "" {
		return nil, fmt.Errorf("ephemeral rename requires path")
	}
	parentPath := "/"
	if i := strings.LastIndex(strings.TrimSuffix(oldPath, "/"), "/"); i >= 0 {
		parentPath = oldPath[:i]
		if parentPath == "" {
			parentPath = "/"
		}
	}
	newPath := utils.JoinPath(parentPath, newName)
	nodeType := types.NodeTypeFolder
	if strings.Contains(newName, ".") {
		nodeType = types.NodeTypeFile
	}
	return &types.Node{
		ID:           utils.DeterministicNodeID(newPath, nodeType),
		ParentID:     req.GetID(),
		Name:         newName,
		Path:         newPath,
		ParentPath:   parentPath,
		Type:         nodeType,
		LastUpdated:  time.Now(),
		ExistenceMap: buildFullExistenceMap(e.cfg),
	}, nil
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
