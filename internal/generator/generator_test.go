package generator

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/Project-Sylos/Spectra/internal/types"
)

// TestRNG tests the random number generator functionality
func TestRNG(t *testing.T) {
	// Test with same seed produces same sequence
	rng1 := NewRNG(42)
	rng2 := NewRNG(42)

	for i := 0; i < 100; i++ {
		val1 := rng1.Intn(1000)
		val2 := rng2.Intn(1000)
		if val1 != val2 {
			t.Errorf("Same seed should produce same sequence. Iteration %d: got %d and %d", i, val1, val2)
		}
	}

	// Test with different seeds produces different sequences
	rng3 := NewRNG(123)
	rng4 := NewRNG(456)

	allSame := true
	for i := 0; i < 100; i++ {
		val3 := rng3.Intn(1000)
		val4 := rng4.Intn(1000)
		if val3 != val4 {
			allSame = false
			break
		}
	}
	if allSame {
		t.Errorf("Different seeds should produce different sequences")
	}

	// Test Float64 returns values in range [0, 1)
	for i := 0; i < 1000; i++ {
		val := rng1.Float64()
		if val < 0 || val >= 1 {
			t.Errorf("Float64 should return value in range [0, 1), got %f", val)
		}
	}

	// Test Intn returns values in range [0, n)
	n := 10
	for i := 0; i < 1000; i++ {
		val := rng1.Intn(n)
		if val < 0 || val >= n {
			t.Errorf("Intn(%d) should return value in range [0, %d), got %d", n, n, val)
		}
	}
}

// TestGenerateChildren tests the children generation functionality
func TestGenerateChildren(t *testing.T) {
	// Create test configuration
	config := &types.Config{
		Seed: types.SeedConfig{
			MaxDepth:   3,
			MinFolders: 1,
			MaxFolders: 3,
			MinFiles:   1,
			MaxFiles:   3,
			Seed:       42,
		},
		API: types.APIConfig{
			Host: "localhost",
			Port: 8080,
		},
		SecondaryTables: map[string]float64{
			"s1": 0.7,
			"s2": 0.3,
		},
	}

	// Create parent node
	parent := &types.Node{
		ID:                    "p-parent",
		ParentID:              "p-root",
		Name:                  "parent",
		Path:                  "/parent",
		Type:                  types.NodeTypeFolder,
		DepthLevel:            1,
		Size:                  0,
		SecondaryExistenceMap: make(map[string]bool),
	}

	// Test generation with deterministic seed
	rng := NewRNG(42)
	children, err := GenerateChildren(parent, parent.DepthLevel, rng, config)
	if err != nil {
		t.Fatalf("Unexpected error generating children: %v", err)
	}

	if children == nil {
		t.Errorf("Expected children but got nil")
	}

	// Verify we have both folders and files
	hasFolders := false
	hasFiles := false
	for _, child := range children {
		switch child.Type {
		case types.NodeTypeFolder:
			hasFolders = true
		case types.NodeTypeFile:
			hasFiles = true
		}

		// Verify child properties
		if child.ParentID != parent.ID {
			t.Errorf("Expected child parent ID %s, got %s", parent.ID, child.ParentID)
		}
		if child.DepthLevel != parent.DepthLevel+1 {
			t.Errorf("Expected child depth %d, got %d", parent.DepthLevel+1, child.DepthLevel)
		}
		expectedPath := filepath.Join(parent.Path, child.Name)
		if child.Path != expectedPath {
			t.Errorf("Expected child path %s, got %s", expectedPath, child.Path)
		}
		if !types.IsPrimaryID(child.ID) {
			t.Errorf("Expected primary ID format, got %s", child.ID)
		}
	}

	if !hasFolders {
		t.Errorf("Expected at least one folder in generated children")
	}
	if !hasFiles {
		t.Errorf("Expected at least one file in generated children")
	}

	// Test deterministic generation - same seed should produce same results
	rng2 := NewRNG(42)
	children2, err := GenerateChildren(parent, 0, rng2, config)
	if err != nil {
		t.Fatalf("Unexpected error generating children (second time): %v", err)
	}

	if len(children) != len(children2) {
		t.Errorf("Expected same number of children with same seed, got %d and %d", len(children), len(children2))
	}

	// Verify children are the same (names should match)
	for i, child1 := range children {
		if i >= len(children2) {
			t.Errorf("Missing child at index %d", i)
			continue
		}
		child2 := children2[i]
		if child1.Name != child2.Name || child1.Type != child2.Type {
			t.Errorf("Children at index %d differ: %s/%s vs %s/%s", i, child1.Name, child1.Type, child2.Name, child2.Type)
		}
	}
}

// TestGenerateSecondaryNodes tests the secondary node generation functionality
func TestGenerateSecondaryNodes(t *testing.T) {
	// Create test configuration
	config := &types.Config{
		SecondaryTables: map[string]float64{
			"s1": 0.7, // 70% chance
			"s2": 0.3, // 30% chance
			"s3": 0.0, // 0% chance (should never be created)
		},
	}

	// Test multiple generations to check probability behavior
	rng := NewRNG(42)
	totalRuns := 1000
	s1Count := 0
	s2Count := 0
	s3Count := 0

	for i := 0; i < totalRuns; i++ {
		// Create a new primary node for each run
		testNode := &types.Node{
			ID:         fmt.Sprintf("p-test%d", i),
			ParentID:   "p-root",
			Name:       "test-node",
			Path:       "/test-node",
			Type:       types.NodeTypeFolder,
			DepthLevel: 1,
			Size:       0,
		}

		secondaryNodes, err := GenerateSecondaryNodes(testNode, config, rng)
		if err != nil {
			t.Fatalf("Unexpected error generating secondary nodes: %v", err)
		}

		// Count occurrences
		if _, exists := secondaryNodes["s1"]; exists {
			s1Count++
		}
		if _, exists := secondaryNodes["s2"]; exists {
			s2Count++
		}
		if _, exists := secondaryNodes["s3"]; exists {
			s3Count++
		}

		// Verify secondary node properties
		for tableName, secondaryNode := range secondaryNodes {
			if secondaryNode.ParentID != testNode.ParentID {
				t.Errorf("Expected secondary node parent %s, got %s", testNode.ParentID, secondaryNode.ParentID)
			}
			if secondaryNode.Name != testNode.Name {
				t.Errorf("Expected secondary node name %s, got %s", testNode.Name, secondaryNode.Name)
			}
			if secondaryNode.Type != testNode.Type {
				t.Errorf("Expected secondary node type %s, got %s", testNode.Type, secondaryNode.Type)
			}
			if !types.IsSecondaryID(secondaryNode.ID) {
				t.Errorf("Expected secondary ID format, got %s", secondaryNode.ID)
			}
			expectedID := tableName + "-" + types.GetUUIDFromID(testNode.ID)
			if secondaryNode.ID != expectedID {
				t.Errorf("Expected secondary ID %s, got %s", expectedID, secondaryNode.ID)
			}
		}
	}

	// Check probability behavior (with some tolerance)
	s1Rate := float64(s1Count) / float64(totalRuns)
	s2Rate := float64(s2Count) / float64(totalRuns)
	s3Rate := float64(s3Count) / float64(totalRuns)

	if s1Rate < 0.6 || s1Rate > 0.8 {
		t.Errorf("S1 probability out of expected range: expected ~0.7, got %f (count: %d)", s1Rate, s1Count)
	}
	if s2Rate < 0.2 || s2Rate > 0.4 {
		t.Errorf("S2 probability out of expected range: expected ~0.3, got %f (count: %d)", s2Rate, s2Count)
	}
	if s3Rate != 0.0 {
		t.Errorf("S3 probability should be 0.0, got %f (count: %d)", s3Rate, s3Count)
	}

	t.Logf("Probability test results: s1=%f (%d), s2=%f (%d), s3=%f (%d)", s1Rate, s1Count, s2Rate, s2Count, s3Rate, s3Count)
}

// TestCompoundProbabilityBehavior tests the real-world compound probability effect
// This tests what happens when you have a tree structure where each level has probability
func TestCompoundProbabilityBehavior(t *testing.T) {
	// Create configuration with high secondary probability to make effects visible
	config := &types.Config{
		SecondaryTables: map[string]float64{
			"s1": 0.8, // 80% chance - high enough to see compound effects
		},
	}

	// Test the compound probability effect across multiple levels
	t.Run("CompoundProbabilityAcrossLevels", func(t *testing.T) {
		rng := NewRNG(12345) // Fixed seed for reproducible results
		totalSimulations := 1000

		// Track how many nodes appear at each level
		level1Count := 0 // Direct children of root
		level2Count := 0 // Grandchildren (children of level1)
		level3Count := 0 // Great-grandchildren (children of level2)

		for sim := 0; sim < totalSimulations; sim++ {
			// Simulate level 1: Direct children of root
			level1Node := &types.Node{
				ID:         fmt.Sprintf("p-level1-%d", sim),
				ParentID:   "p-root",
				Name:       "level1-node",
				Type:       types.NodeTypeFolder,
				DepthLevel: 1,
			}

			level1Secondary, err := GenerateSecondaryNodes(level1Node, config, rng)
			if err != nil {
				t.Fatalf("Failed to generate level1 secondary nodes: %v", err)
			}

			if _, exists := level1Secondary["s1"]; exists {
				level1Count++

				// Simulate level 2: Children of level1 nodes
				level2Node := &types.Node{
					ID:         fmt.Sprintf("p-level2-%d", sim),
					ParentID:   level1Node.ID,
					Name:       "level2-node",
					Type:       types.NodeTypeFolder,
					DepthLevel: 2,
				}

				level2Secondary, err := GenerateSecondaryNodes(level2Node, config, rng)
				if err != nil {
					t.Fatalf("Failed to generate level2 secondary nodes: %v", err)
				}

				if _, exists := level2Secondary["s1"]; exists {
					level2Count++

					// Simulate level 3: Children of level2 nodes
					level3Node := &types.Node{
						ID:         fmt.Sprintf("p-level3-%d", sim),
						ParentID:   level2Node.ID,
						Name:       "level3-node",
						Type:       types.NodeTypeFolder,
						DepthLevel: 3,
					}

					level3Secondary, err := GenerateSecondaryNodes(level3Node, config, rng)
					if err != nil {
						t.Fatalf("Failed to generate level3 secondary nodes: %v", err)
					}

					if _, exists := level3Secondary["s1"]; exists {
						level3Count++
					}
				}
			}
		}

		// Calculate actual rates
		level1Rate := float64(level1Count) / float64(totalSimulations)
		level2Rate := float64(level2Count) / float64(totalSimulations)
		level3Rate := float64(level3Count) / float64(totalSimulations)

		// Expected compound probabilities
		expectedLevel1Rate := 0.8
		expectedLevel2Rate := 0.8 * 0.8       // 0.64 (compound: 80% of 80%)
		expectedLevel3Rate := 0.8 * 0.8 * 0.8 // 0.512 (compound: 80% of 80% of 80%)

		t.Logf("Compound probability results:")
		t.Logf("Level 1: %f (expected ~%f)", level1Rate, expectedLevel1Rate)
		t.Logf("Level 2: %f (expected ~%f)", level2Rate, expectedLevel2Rate)
		t.Logf("Level 3: %f (expected ~%f)", level3Rate, expectedLevel3Rate)

		// Verify with tolerance (allowing for statistical variance)
		tolerance := 0.1
		if abs(level1Rate-expectedLevel1Rate) > tolerance {
			t.Errorf("Level 1 rate %f is too far from expected %f (tolerance: %f)", level1Rate, expectedLevel1Rate, tolerance)
		}
		if abs(level2Rate-expectedLevel2Rate) > tolerance {
			t.Errorf("Level 2 rate %f is too far from expected %f (tolerance: %f)", level2Rate, expectedLevel2Rate, tolerance)
		}
		if abs(level3Rate-expectedLevel3Rate) > tolerance {
			t.Errorf("Level 3 rate %f is too far from expected %f (tolerance: %f)", level3Rate, expectedLevel3Rate, tolerance)
		}

		// Verify the compound effect: each level should have fewer nodes than the previous
		if level2Rate >= level1Rate {
			t.Errorf("Compound effect not working: level2 rate %f should be less than level1 rate %f", level2Rate, level1Rate)
		}
		if level3Rate >= level2Rate {
			t.Errorf("Compound effect not working: level3 rate %f should be less than level2 rate %f", level3Rate, level2Rate)
		}
	})
}

// Helper function for absolute value
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// TestGenerateFileData tests the file data generation functionality
func TestGenerateFileData(t *testing.T) {
	rng := NewRNG(42)

	// Test multiple generations
	for i := 0; i < 10; i++ {
		data, checksum, err := GenerateFileData(rng)
		if err != nil {
			t.Fatalf("Unexpected error generating file data: %v", err)
		}

		// Verify data properties
		if len(data) != 1024 {
			t.Errorf("Expected 1024 bytes of data, got %d", len(data))
		}

		// Verify checksum properties
		if checksum == "" {
			t.Errorf("Expected non-empty checksum")
		}
		if len(checksum) != 64 { // SHA256 produces 64-character hex string
			t.Errorf("Expected 64-character checksum, got %d", len(checksum))
		}

		// Verify checksum matches data
		expectedChecksum := ComputeChecksum(data)
		if checksum != expectedChecksum {
			t.Errorf("Checksum mismatch: expected %s, got %s", expectedChecksum, checksum)
		}
	}

	// Test deterministic generation with same seed
	rng1 := NewRNG(42)
	rng2 := NewRNG(42)

	_, checksum1, err1 := GenerateFileData(rng1)
	_, checksum2, err2 := GenerateFileData(rng2)

	if err1 != nil || err2 != nil {
		t.Fatalf("Unexpected errors: %v, %v", err1, err2)
	}

	if checksum1 != checksum2 {
		t.Errorf("Same seed should produce same checksum: %s vs %s", checksum1, checksum2)
	}
}

// TestGenerateFileDataForUpload tests the upload file data generation functionality
func TestGenerateFileDataForUpload(t *testing.T) {
	rng := NewRNG(42)

	// Test with empty data
	data, checksum, err := GenerateFileDataForUpload([]byte{}, rng)
	if err != nil {
		t.Fatalf("Unexpected error with empty data: %v", err)
	}
	if len(data) != 1024 {
		t.Errorf("Expected 1024 bytes of data, got %d", len(data))
	}
	if checksum == "" {
		t.Errorf("Expected non-empty checksum")
	}

	// Test with provided data
	uploadData := []byte("test upload content")
	data, checksum, err = GenerateFileDataForUpload(uploadData, rng)
	if err != nil {
		t.Fatalf("Unexpected error with provided data: %v", err)
	}
	if len(data) != 1024 {
		t.Errorf("Expected 1024 bytes of data, got %d", len(data))
	}
	if checksum == "" {
		t.Errorf("Expected non-empty checksum")
	}

	// Verify checksum matches generated data
	expectedChecksum := ComputeChecksum(data)
	if checksum != expectedChecksum {
		t.Errorf("Checksum mismatch: expected %s, got %s", expectedChecksum, checksum)
	}
}

// TestGeneratorEdgeCases tests edge cases and error conditions
func TestGeneratorEdgeCases(t *testing.T) {
	// Test with nil configuration
	parent := &types.Node{
		ID:                    "p-test",
		ParentID:              "p-root",
		Name:                  "test",
		Path:                  "/test",
		Type:                  types.NodeTypeFolder,
		DepthLevel:            1,
		Size:                  0,
		SecondaryExistenceMap: make(map[string]bool),
	}

	rng := NewRNG(42)
	_, err := GenerateChildren(parent, 0, rng, nil)
	if err == nil {
		t.Errorf("Expected error with nil configuration")
	}

	// Test with empty secondary tables configuration
	config := &types.Config{
		SecondaryTables: map[string]float64{},
	}

	secondaryNodes, err := GenerateSecondaryNodes(parent, config, rng)
	if err != nil {
		t.Fatalf("Unexpected error with empty secondary tables: %v", err)
	}
	if len(secondaryNodes) != 0 {
		t.Errorf("Expected no secondary nodes with empty configuration, got %d", len(secondaryNodes))
	}

	// Test with invalid probability values (should still work but may behave unexpectedly)
	configInvalid := &types.Config{
		SecondaryTables: map[string]float64{
			"s1": 1.5,  // > 1.0
			"s2": -0.1, // < 0.0
		},
	}

	// This should still work, but probabilities outside [0,1] may behave unexpectedly
	_, err = GenerateSecondaryNodes(parent, configInvalid, rng)
	if err != nil {
		t.Errorf("Generator should handle invalid probabilities gracefully: %v", err)
	}
}

// Benchmark tests for performance
func BenchmarkGenerateChildren(b *testing.B) {
	config := &types.Config{
		Seed: types.SeedConfig{
			MaxDepth:   3,
			MinFolders: 2,
			MaxFolders: 5,
			MinFiles:   2,
			MaxFiles:   5,
			Seed:       42,
		},
		SecondaryTables: map[string]float64{
			"s1": 0.7,
			"s2": 0.3,
		},
	}

	parent := &types.Node{
		ID:                    "p-bench",
		ParentID:              "p-root",
		Name:                  "bench",
		Path:                  "/bench",
		Type:                  types.NodeTypeFolder,
		DepthLevel:            1,
		Size:                  0,
		SecondaryExistenceMap: make(map[string]bool),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rng := NewRNG(int64(i))
		_, err := GenerateChildren(parent, 0, rng, config)
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}

func BenchmarkGenerateFileData(b *testing.B) {
	rng := NewRNG(42)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := GenerateFileData(rng)
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}
