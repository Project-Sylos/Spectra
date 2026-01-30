package generator

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"codeberg.org/Sylos/Spectra/internal/types"
	"codeberg.org/Sylos/Spectra/internal/utils"
)

// RNG wraps math/rand.Rand for seeded random generation with thread-safety
type RNG struct {
	mu   sync.Mutex
	rand *rand.Rand
}

// NewRNG creates a new seeded random number generator
func NewRNG(seed int64) *RNG {
	return &RNG{
		rand: rand.New(rand.NewSource(seed)),
	}
}

// Intn returns a random integer in [0, n) with thread-safety
func (r *RNG) Intn(n int) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rand.Intn(n)
}

// Float64 returns a random float64 in [0.0, 1.0) with thread-safety
func (r *RNG) Float64() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rand.Float64()
}

// Read fills the slice with random bytes with thread-safety
func (r *RNG) Read(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rand.Read(p)
}

// sampleWeightedRange samples from [0, max] using logarithmic buckets with exponential decay.
// Returns a random value where smaller values are exponentially more likely.
// Uses buckets: [0,10), [10,100), [100,1000), [1000,max] with weights decaying exponentially.
func sampleWeightedRange(max int, backoffFactor float64, rng *RNG) int {
	if max <= 0 {
		return 0
	}

	// Define logarithmic bucket boundaries
	// We use powers of 10: [0,10), [10,100), [100,1000), [1000,max]
	type bucket struct {
		min    int
		max    int
		weight float64
	}

	var buckets []bucket
	var totalWeight float64

	// Create buckets dynamically based on max value
	currentMin := 0
	bucketIndex := 0
	for currentMin < max {
		var currentMax int
		if bucketIndex == 0 {
			currentMax = min(10, max)
		} else {
			// Powers of 10: 10, 100, 1000, etc.
			currentMax = min(int(math.Pow(10, float64(bucketIndex+1))), max)
		}

		weight := math.Pow(backoffFactor, float64(bucketIndex))
		buckets = append(buckets, bucket{
			min:    currentMin,
			max:    currentMax,
			weight: weight,
		})
		totalWeight += weight

		currentMin = currentMax
		bucketIndex++

		// Safety check to prevent infinite loop
		if currentMin >= max {
			break
		}
	}

	// Sample a bucket using weighted random selection
	target := rng.Float64() * totalWeight
	cumulative := 0.0
	selectedBucket := buckets[0]

	for _, b := range buckets {
		cumulative += b.weight
		if target <= cumulative {
			selectedBucket = b
			break
		}
	}

	// Uniform sample within the selected bucket
	if selectedBucket.max == selectedBucket.min {
		return selectedBucket.min
	}
	return selectedBucket.min + rng.Intn(selectedBucket.max-selectedBucket.min)
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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

	// Apply depth decay to effective max values
	effectiveFolderMax := int(float64(cfg.Seed.MaxFolders) * math.Pow(cfg.Seed.FolderDepthDecayFactor, float64(depth)))
	effectiveFileMax := int(float64(cfg.Seed.MaxFiles) * math.Pow(cfg.Seed.FileDepthDecayFactor, float64(depth)))

	// Sample counts using weighted distribution
	folderCount := sampleWeightedRange(effectiveFolderMax, cfg.Seed.FolderBackoffFactor, rng)
	fileCount := sampleWeightedRange(effectiveFileMax, cfg.Seed.FileBackoffFactor, rng)

	// Generate folders
	for i := 0; i < folderCount; i++ {
		folder, err := generateFolder(parent, i+1, depth+1, cfg, rng)
		if err != nil {
			return nil, fmt.Errorf("failed to generate folder %d: %w", i+1, err)
		}
		children = append(children, folder)
	}

	// Generate files
	for i := 0; i < fileCount; i++ {
		file, err := generateFile(parent, i+1, depth+1, cfg, rng)
		if err != nil {
			return nil, fmt.Errorf("failed to generate file %d: %w", i+1, err)
		}
		children = append(children, file)
	}

	return children, nil
}

// generateFolder creates a new folder node with deterministic ID and ExistenceMap
func generateFolder(parent *types.Node, index int, depth int, cfg *types.Config, rng *RNG) (*types.Node, error) {
	name := fmt.Sprintf("folder_%d", index)
	pathStr := utils.JoinPath(parent.Path, name)
	nodeID := utils.DeterministicNodeID(pathStr, types.NodeTypeFolder)

	// Create existence map - ensure all worlds have keys
	existenceMap := make(map[string]bool)

	// Primary is always true
	existenceMap["primary"] = true

	// For each secondary world, check parent existence first
	for worldName, probability := range cfg.SecondaryTables {
		// If parent doesn't exist in this world, child cannot exist
		if !parent.ExistenceMap[worldName] {
			existenceMap[worldName] = false
		} else {
			// Parent exists, so roll dice: roll [0.0, 1.0) must be <= probability
			roll := rng.Float64()
			existenceMap[worldName] = (roll <= probability)
		}
	}

	return &types.Node{
		ID:           nodeID,
		ParentID:     parent.ID,
		Name:         name,
		Path:         pathStr,
		ParentPath:   parent.Path,
		Type:         types.NodeTypeFolder,
		DepthLevel:   depth,
		Size:         0, // Folders have size 0
		LastUpdated:  time.Now(),
		Checksum:     nil, // Folders don't have checksums
		ExistenceMap: existenceMap,
	}, nil
}

// generateFile creates a new file node with deterministic ID and ExistenceMap
func generateFile(parent *types.Node, index int, depth int, cfg *types.Config, rng *RNG) (*types.Node, error) {
	name := fmt.Sprintf("file_%d.txt", index)
	pathStr := utils.JoinPath(parent.Path, name)
	nodeID := utils.DeterministicNodeID(pathStr, types.NodeTypeFile)

	// Generate file data and checksum deterministically so repeated reads always
	// return identical content, regardless of node identity
	data, checksum, err := GenerateDeterministicFileData(cfg.Seed.FileBinarySeed)
	if err != nil {
		return nil, fmt.Errorf("failed to generate file data: %w", err)
	}

	// Create existence map - ensure all worlds have keys
	existenceMap := make(map[string]bool)

	// Primary is always true
	existenceMap["primary"] = true

	// For each secondary world, check parent existence first
	for worldName, probability := range cfg.SecondaryTables {
		// If parent doesn't exist in this world, child cannot exist
		if !parent.ExistenceMap[worldName] {
			existenceMap[worldName] = false
		} else {
			// Parent exists, so roll dice: roll [0.0, 1.0) must be <= probability
			roll := rng.Float64()
			existenceMap[worldName] = (roll <= probability)
		}
	}

	return &types.Node{
		ID:           nodeID,
		ParentID:     parent.ID,
		Name:         name,
		Path:         pathStr,
		ParentPath:   parent.Path,
		Type:         types.NodeTypeFile,
		DepthLevel:   depth,
		Size:         int64(len(data)),
		LastUpdated:  time.Now(),
		Checksum:     &checksum, // Store the computed checksum
		ExistenceMap: existenceMap,
	}, nil
}
