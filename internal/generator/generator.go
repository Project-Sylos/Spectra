package generator

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"time"

	"github.com/Project-Sylos/Spectra/internal/types"
	"github.com/google/uuid"
)

// RNG wraps math/rand.Rand for seeded random generation
type RNG struct {
	*rand.Rand
}

// NewRNG creates a new seeded random number generator
func NewRNG(seed int64) *RNG {
	return &RNG{
		Rand: rand.New(rand.NewSource(seed)),
	}
}

// GenerateChildren generates children nodes for a given parent based on configuration
// Returns a single list of nodes with ExistenceMap populated for each
func GenerateChildren(parent *types.Node, depth int, rng *RNG, cfg *types.Config) ([]*types.Node, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}

	var children []*types.Node

	// Don't generate children if we've reached max depth
	if depth >= cfg.Seed.MaxDepth {
		return children, nil
	}

	// Generate folders
	folderCount := rng.Intn(cfg.Seed.MaxFolders-cfg.Seed.MinFolders+1) + cfg.Seed.MinFolders
	for i := 0; i < folderCount; i++ {
		folder, err := generateFolder(parent, i+1, depth+1, cfg, rng)
		if err != nil {
			return nil, fmt.Errorf("failed to generate folder %d: %w", i+1, err)
		}
		children = append(children, folder)
	}

	// Generate files
	fileCount := rng.Intn(cfg.Seed.MaxFiles-cfg.Seed.MinFiles+1) + cfg.Seed.MinFiles
	for i := 0; i < fileCount; i++ {
		file, err := generateFile(parent, i+1, depth+1, cfg, rng)
		if err != nil {
			return nil, fmt.Errorf("failed to generate file %d: %w", i+1, err)
		}
		children = append(children, file)
	}

	return children, nil
}

// generateFolder creates a new folder node with UUID and ExistenceMap
func generateFolder(parent *types.Node, index int, depth int, cfg *types.Config, rng *RNG) (*types.Node, error) {
	name := fmt.Sprintf("folder_%d", index)
	path := filepath.Join(parent.Path, name)

	// Generate UUID for the node
	nodeID := uuid.New().String()

	// Create existence map - start with primary
	existenceMap := make(map[string]bool)
	existenceMap["primary"] = true

	// Roll dice for each secondary world
	for worldName, probability := range cfg.SecondaryTables {
		roll := rng.Float64()
		if roll <= probability {
			existenceMap[worldName] = true
		}
	}

	return &types.Node{
		ID:           nodeID,
		ParentID:     parent.ID,
		Name:         name,
		Path:         path,
		Type:         types.NodeTypeFolder,
		DepthLevel:   depth,
		Size:         0, // Folders have size 0
		LastUpdated:  time.Now(),
		Checksum:     nil, // Folders don't have checksums
		ExistenceMap: existenceMap,
		CopyStatus:   types.CopyStatusPending,
	}, nil
}

// generateFile creates a new file node with UUID and ExistenceMap
func generateFile(parent *types.Node, index int, depth int, cfg *types.Config, rng *RNG) (*types.Node, error) {
	name := fmt.Sprintf("file_%d.txt", index)
	path := filepath.Join(parent.Path, name)

	// Generate UUID for the node
	nodeID := uuid.New().String()

	// Generate file data and checksum
	_, checksum, err := GenerateFileData(rng)
	if err != nil {
		return nil, fmt.Errorf("failed to generate file data: %w", err)
	}

	// Create existence map - start with primary
	existenceMap := make(map[string]bool)
	existenceMap["primary"] = true

	// Roll dice for each secondary world
	for worldName, probability := range cfg.SecondaryTables {
		roll := rng.Float64()
		if roll <= probability {
			existenceMap[worldName] = true
		}
	}

	return &types.Node{
		ID:           nodeID,
		ParentID:     parent.ID,
		Name:         name,
		Path:         path,
		Type:         types.NodeTypeFile,
		DepthLevel:   depth,
		Size:         1024, // 1KB files as specified
		LastUpdated:  time.Now(),
		Checksum:     &checksum, // Store the computed checksum
		ExistenceMap: existenceMap,
		CopyStatus:   types.CopyStatusPending,
	}, nil
}

// GenerateFileDataForUpload generates random data and checksum for uploaded files
// This is used when someone uploads file data - we compute checksum but don't persist the data
func GenerateFileDataForUpload(uploadedData []byte, rng *RNG) ([]byte, string, error) {
	// Generate 1KB of random data (we ignore the uploaded data as per requirements)
	data, checksum, err := GenerateFileData(rng)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate file data: %w", err)
	}
	return data, checksum, nil
}

// ValidateConfig validates the generator configuration
func ValidateConfig(cfg *types.Config) error {
	if cfg.Seed.MaxDepth < 1 {
		return fmt.Errorf("max_depth must be at least 1")
	}
	if cfg.Seed.MinFolders < 0 || cfg.Seed.MaxFolders < cfg.Seed.MinFolders {
		return fmt.Errorf("invalid folder count range: min=%d, max=%d", cfg.Seed.MinFolders, cfg.Seed.MaxFolders)
	}
	if cfg.Seed.MinFiles < 0 || cfg.Seed.MaxFiles < cfg.Seed.MinFiles {
		return fmt.Errorf("invalid file count range: min=%d, max=%d", cfg.Seed.MinFiles, cfg.Seed.MaxFiles)
	}
	return nil
}

// Note: GenerateSecondaryNodes removed - existence is now determined inline during node generation
