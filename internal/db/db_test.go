package db

import (
	"os"
	"testing"
	"time"

	"github.com/Project-Sylos/Spectra/internal/types"
)

// TestNewDB tests the NewDB function
func TestNewDB(t *testing.T) {
	tests := []struct {
		name        string
		dbPath      string
		expectError bool
		setup       func() string
		cleanup     func(string)
	}{
		{
			name:        "temporary file database",
			dbPath:      "",
			expectError: false,
			setup: func() string {
				tmpFile, err := os.CreateTemp("", "test-*.db")
				if err != nil {
					t.Fatal(err)
				}
				tmpFile.Close()
				os.Remove(tmpFile.Name()) // Remove the empty file
				return tmpFile.Name()
			},
			cleanup: func(path string) {
				os.Remove(path)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dbPath string
			if tt.setup != nil {
				dbPath = tt.setup()
				if tt.cleanup != nil {
					defer tt.cleanup(dbPath)
				}
			} else {
				dbPath = tt.dbPath
			}

			db, err := New(dbPath, map[string]float64{})
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				if db != nil {
					t.Errorf("Expected nil DB but got %v", db)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if db == nil {
					t.Errorf("Expected DB but got nil")
					return
				}
				if db.conn == nil {
					t.Errorf("Expected database connection but got nil")
					return
				}
			}
		})
	}
}

// TestDBMethods tests the core database methods
func TestDBMethods(t *testing.T) {
	// Create a temporary database
	tmpFile, err := os.CreateTemp("", "test-db-methods-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	os.Remove(tmpFile.Name()) // Remove the empty file
	defer os.Remove(tmpFile.Name())

	db, err := New(tmpFile.Name(), map[string]float64{})
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Test InitializeSchema
	t.Run("InitializeSchema", func(t *testing.T) {
		secondaryTables := map[string]float64{"s1": 0.7, "s2": 0.3}
		err := db.InitializeSchema(secondaryTables)
		if err != nil {
			t.Errorf("Unexpected error initializing schema: %v", err)
		}
	})

	// Test InsertPrimaryNode
	t.Run("InsertPrimaryNode", func(t *testing.T) {
		node := &types.Node{
			ID:                    "p-test123",
			ParentID:              "p-root",
			Name:                  "test-node",
			Path:                  "/test-node",
			Type:                  types.NodeTypeFolder,
			DepthLevel:            1,
			Size:                  0,
			LastUpdated:           time.Now(),
			TraversalStatus:       types.StatusPending,
			SecondaryExistenceMap: map[string]bool{"s1": true, "s2": false},
		}

		err := db.InsertPrimaryNode(node)
		if err != nil {
			t.Errorf("Unexpected error inserting primary node: %v", err)
		}
	})

	// Test GetNodeByID
	t.Run("GetNodeByID", func(t *testing.T) {
		node, err := db.GetNodeByID("p-test123")
		if err != nil {
			t.Errorf("Unexpected error getting node by ID: %v", err)
		}
		if node == nil {
			t.Errorf("Expected node but got nil")
			return
		}
		if node.ID != "p-test123" {
			t.Errorf("Expected node ID 'p-test123', got '%s'", node.ID)
		}
		if node.Name != "test-node" {
			t.Errorf("Expected node name 'test-node', got '%s'", node.Name)
		}
		if node.Type != types.NodeTypeFolder {
			t.Errorf("Expected node type 'folder', got '%s'", node.Type)
		}
	})

	// Test GetChildren
	t.Run("GetChildren", func(t *testing.T) {
		// Insert a child node
		childNode := &types.Node{
			ID:                    "p-child123",
			ParentID:              "p-test123",
			Name:                  "child-node",
			Path:                  "/test-node/child-node",
			Type:                  types.NodeTypeFile,
			DepthLevel:            2,
			Size:                  1024,
			LastUpdated:           time.Now(),
			TraversalStatus:       types.StatusPending,
			SecondaryExistenceMap: map[string]bool{},
		}

		err := db.InsertPrimaryNode(childNode)
		if err != nil {
			t.Fatalf("Failed to insert child node: %v", err)
		}

		children, err := db.GetChildrenByParentID("p-test123")
		if err != nil {
			t.Errorf("Unexpected error getting children: %v", err)
		}
		if len(children) != 1 {
			t.Errorf("Expected 1 child, got %d", len(children))
		}
		if children[0].ID != "p-child123" {
			t.Errorf("Expected child ID 'p-child123', got '%s'", children[0].ID)
		}
	})

	// Test InsertSecondaryNode
	t.Run("InsertSecondaryNode", func(t *testing.T) {
		secondaryNode := &types.Node{
			ID:              "s1-test123",
			ParentID:        "p-root",
			Name:            "test-node",
			Path:            "/test-node",
			Type:            types.NodeTypeFolder,
			DepthLevel:      1,
			Size:            0,
			LastUpdated:     time.Now(),
			TraversalStatus: types.StatusPending,
		}

		err := db.InsertSecondaryNode("s1", secondaryNode)
		if err != nil {
			t.Errorf("Unexpected error inserting secondary node: %v", err)
		}
	})

	// Test GetNodeByID
	t.Run("GetNodeByID", func(t *testing.T) {
		node, err := db.GetNodeByID("s1-test123")
		if err != nil {
			t.Errorf("Unexpected error getting secondary node: %v", err)
		}
		if node == nil {
			t.Errorf("Expected secondary node but got nil")
			return
		}
		if node.ID != "s1-test123" {
			t.Errorf("Expected secondary node ID 's1-test123', got '%s'", node.ID)
		}
	})

	// Test UpdateTraversalStatus
	t.Run("UpdateTraversalStatus", func(t *testing.T) {
		err := db.UpdateTraversalStatus("p-test123", types.StatusSuccessful)
		if err != nil {
			t.Errorf("Unexpected error updating traversal status: %v", err)
		}

		// Verify the update
		node, err := db.GetNodeByID("p-test123")
		if err != nil {
			t.Fatalf("Failed to get updated node: %v", err)
		}
		if node.TraversalStatus != types.StatusSuccessful {
			t.Errorf("Expected traversal status 'successful', got '%s'", node.TraversalStatus)
		}
	})

	// Test UpdateSecondaryExistenceMap
	t.Run("UpdateSecondaryExistenceMap", func(t *testing.T) {
		newMap := map[string]bool{"s1": true, "s2": true, "s3": false}
		err := db.UpdateSecondaryExistenceMap("p-test123", newMap)
		if err != nil {
			t.Errorf("Unexpected error updating secondary existence map: %v", err)
		}

		// Verify the update
		node, err := db.GetNodeByID("p-test123")
		if err != nil {
			t.Fatalf("Failed to get updated node: %v", err)
		}
		if len(node.SecondaryExistenceMap) != 3 {
			t.Errorf("Expected 3 entries in secondary existence map, got %d", len(node.SecondaryExistenceMap))
		}
		if !node.SecondaryExistenceMap["s1"] {
			t.Errorf("Expected s1 to be true")
		}
		if !node.SecondaryExistenceMap["s2"] {
			t.Errorf("Expected s2 to be true")
		}
		if node.SecondaryExistenceMap["s3"] {
			t.Errorf("Expected s3 to be false")
		}
	})

	// Test DeleteNode
	t.Run("DeleteNode", func(t *testing.T) {
		err := db.DeleteNode("p-child123")
		if err != nil {
			t.Errorf("Unexpected error deleting node: %v", err)
		}

		// Verify deletion
		_, err = db.GetNodeByID("p-child123")
		if err == nil {
			t.Errorf("Expected error getting deleted node")
		}
	})

	// Test GetTableInfo
	t.Run("GetTableInfo", func(t *testing.T) {
		tableInfo, err := db.GetTableInfo()
		if err != nil {
			t.Errorf("Unexpected error getting table info: %v", err)
		}
		if tableInfo == nil {
			t.Errorf("Expected table info but got nil")
		}
		if len(tableInfo) < 2 { // Should have at least primary and one secondary table
			t.Errorf("Expected at least 2 tables, got %d", len(tableInfo))
		}
	})

	// Test GetNodeCount
	t.Run("GetNodeCount", func(t *testing.T) {
		count, err := db.GetNodeCount("nodes_primary")
		if err != nil {
			t.Errorf("Unexpected error getting node count: %v", err)
		}
		if count < 1 {
			t.Errorf("Expected at least 1 node in primary table, got %d", count)
		}
	})

	// Test Reset
	t.Run("Reset", func(t *testing.T) {
		err := db.DeleteAllNodes()
		if err != nil {
			t.Errorf("Unexpected error resetting database: %v", err)
		}

		// Verify reset
		count, err := db.GetNodeCount("nodes_primary")
		if err != nil {
			t.Errorf("Unexpected error getting node count after reset: %v", err)
		}
		if count != 0 {
			t.Errorf("Expected 0 nodes after reset, got %d", count)
		}
	})
}

// TestDBErrorHandling tests error handling scenarios
func TestDBErrorHandling(t *testing.T) {
	// Create a temporary database
	tmpFile, err := os.CreateTemp("", "test-db-errors-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	os.Remove(tmpFile.Name()) // Remove the empty file
	defer os.Remove(tmpFile.Name())

	db, err := New(tmpFile.Name(), map[string]float64{})
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create tables
	err = db.InitializeSchema(map[string]float64{"s1": 1.0})
	if err != nil {
		t.Fatalf("Failed to create tables: %v", err)
	}

	tests := []struct {
		name        string
		testFunc    func() error
		expectError bool
	}{
		{
			name: "GetNodeByID with invalid ID",
			testFunc: func() error {
				_, err := db.GetNodeByID("invalid-id")
				return err
			},
			expectError: true,
		},
		{
			name: "GetNodeByID with invalid table",
			testFunc: func() error {
				_, err := db.GetNodeByID("s1-test")
				return err
			},
			expectError: true,
		},
		{
			name: "GetNodeByID with invalid ID",
			testFunc: func() error {
				_, err := db.GetNodeByID("invalid-id")
				return err
			},
			expectError: true,
		},
		{
			name: "UpdateTraversalStatus with invalid ID",
			testFunc: func() error {
				err := db.UpdateTraversalStatus("invalid-id", types.StatusSuccessful)
				return err
			},
			expectError: true,
		},
		{
			name: "UpdateSecondaryExistenceMap with invalid ID",
			testFunc: func() error {
				err := db.UpdateSecondaryExistenceMap("invalid-id", map[string]bool{})
				return err
			},
			expectError: true,
		},
		{
			name: "DeleteNode with invalid ID",
			testFunc: func() error {
				err := db.DeleteNode("invalid-id")
				return err
			},
			expectError: true,
		},
		{
			name: "GetNodeCount with invalid table",
			testFunc: func() error {
				_, err := db.GetNodeCount("invalid-table")
				return err
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.testFunc()
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// TestDBConcurrency tests concurrent database operations
func TestDBConcurrency(t *testing.T) {
	// Create a temporary database
	tmpFile, err := os.CreateTemp("", "test-db-concurrency-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	os.Remove(tmpFile.Name()) // Remove the empty file
	defer os.Remove(tmpFile.Name())

	db, err := New(tmpFile.Name(), map[string]float64{})
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create tables
	err = db.InitializeSchema(map[string]float64{"s1": 0.7, "s2": 0.3})
	if err != nil {
		t.Fatalf("Failed to create tables: %v", err)
	}

	// Test concurrent operations
	done := make(chan bool, 10)

	// Run multiple goroutines with different operations
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			// Create a primary node
			node := &types.Node{
				ID:                    "p-concurrent-" + string(rune(id)),
				ParentID:              "p-root",
				Name:                  "concurrent-node-" + string(rune(id)),
				Path:                  "/concurrent-node-" + string(rune(id)),
				Type:                  types.NodeTypeFolder,
				DepthLevel:            1,
				Size:                  0,
				LastUpdated:           time.Now(),
				TraversalStatus:       types.StatusPending,
				SecondaryExistenceMap: map[string]bool{"s1": true, "s2": false},
			}

			err := db.InsertPrimaryNode(node)
			if err != nil {
				t.Errorf("Failed to insert primary node in goroutine %d: %v", id, err)
				return
			}

			// Create a secondary node
			secondaryNode := &types.Node{
				ID:              "s1-concurrent-" + string(rune(id)),
				ParentID:        "p-root",
				Name:            "concurrent-node-" + string(rune(id)),
				Path:            "/concurrent-node-" + string(rune(id)),
				Type:            types.NodeTypeFolder,
				DepthLevel:      1,
				Size:            0,
				LastUpdated:     time.Now(),
				TraversalStatus: types.StatusPending,
			}

			err = db.InsertSecondaryNode("s1", secondaryNode)
			if err != nil {
				t.Errorf("Failed to insert secondary node in goroutine %d: %v", id, err)
				return
			}

			// Get the node
			_, err = db.GetNodeByID(node.ID)
			if err != nil {
				t.Errorf("Failed to get node in goroutine %d: %v", id, err)
				return
			}

			// Update traversal status
			err = db.UpdateTraversalStatus(node.ID, types.StatusSuccessful)
			if err != nil {
				t.Errorf("Failed to update traversal status in goroutine %d: %v", id, err)
				return
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify final state
	count, err := db.GetNodeCount("nodes_primary")
	if err != nil {
		t.Errorf("Failed to get final node count: %v", err)
	}
	if count < 10 {
		t.Errorf("Expected at least 10 nodes, got %d", count)
	}
}

// TestDBCreateFolder tests the CreateFolder method
func TestDBCreateFolder(t *testing.T) {
	// Create a temporary database
	tmpFile, err := os.CreateTemp("", "test-db-create-folder-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	os.Remove(tmpFile.Name()) // Remove the empty file
	defer os.Remove(tmpFile.Name())

	db, err := New(tmpFile.Name(), map[string]float64{})
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create tables
	err = db.InitializeSchema(map[string]float64{"s1": 1.0})
	if err != nil {
		t.Fatalf("Failed to create tables: %v", err)
	}

	// Create a parent node first
	parentNode := &types.Node{
		ID:                    "p-parent",
		ParentID:              "p-root",
		Name:                  "parent",
		Path:                  "/parent",
		Type:                  types.NodeTypeFolder,
		DepthLevel:            1,
		Size:                  0,
		LastUpdated:           time.Now(),
		TraversalStatus:       types.StatusPending,
		SecondaryExistenceMap: map[string]bool{},
	}

	err = db.InsertPrimaryNode(parentNode)
	if err != nil {
		t.Fatalf("Failed to insert parent node: %v", err)
	}

	// Test CreateFolder
	folderNode, err := db.CreateFolder("p-parent", "test-folder", 2)
	if err != nil {
		t.Errorf("Unexpected error creating folder: %v", err)
	}
	if folderNode == nil {
		t.Errorf("Expected folder node but got nil")
		return
	}
	if folderNode.ParentID != "p-parent" {
		t.Errorf("Expected parent ID 'p-parent', got '%s'", folderNode.ParentID)
	}
	if folderNode.Name != "test-folder" {
		t.Errorf("Expected name 'test-folder', got '%s'", folderNode.Name)
	}
	if folderNode.Type != types.NodeTypeFolder {
		t.Errorf("Expected type 'folder', got '%s'", folderNode.Type)
	}
	if folderNode.DepthLevel != 2 {
		t.Errorf("Expected depth level 2, got %d", folderNode.DepthLevel)
	}
	if folderNode.Path != "/parent/test-folder" {
		t.Errorf("Expected path '/parent/test-folder', got '%s'", folderNode.Path)
	}
	if !types.IsPrimaryID(folderNode.ID) {
		t.Errorf("Expected primary ID format, got '%s'", folderNode.ID)
	}
}

// TestDBIntegration tests complex database integration scenarios
func TestDBIntegration(t *testing.T) {
	// Create a temporary database
	tmpFile, err := os.CreateTemp("", "test-db-integration-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	os.Remove(tmpFile.Name()) // Remove the empty file
	defer os.Remove(tmpFile.Name())

	db, err := New(tmpFile.Name(), map[string]float64{})
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create tables with multiple secondary tables
	secondaryTables := map[string]float64{"s1": 0.7, "s2": 0.3, "s3": 0.1}
	err = db.InitializeSchema(secondaryTables)
	if err != nil {
		t.Fatalf("Failed to create tables: %v", err)
	}

	// Test complex workflow
	t.Run("ComplexWorkflow", func(t *testing.T) {
		// 1. Create a primary node
		primaryNode := &types.Node{
			ID:                    "p-integration-test",
			ParentID:              "p-root",
			Name:                  "integration-test",
			Path:                  "/integration-test",
			Type:                  types.NodeTypeFolder,
			DepthLevel:            1,
			Size:                  0,
			LastUpdated:           time.Now(),
			TraversalStatus:       types.StatusPending,
			SecondaryExistenceMap: map[string]bool{},
		}

		err := db.InsertPrimaryNode(primaryNode)
		if err != nil {
			t.Fatalf("Failed to insert primary node: %v", err)
		}

		// 2. Create secondary nodes
		for table := range secondaryTables {
			secondaryNode := &types.Node{
				ID:              table + "-integration-test",
				ParentID:        "p-root",
				Name:            "integration-test",
				Path:            "/integration-test",
				Type:            types.NodeTypeFolder,
				DepthLevel:      1,
				Size:            0,
				LastUpdated:     time.Now(),
				TraversalStatus: types.StatusPending,
			}

			err := db.InsertSecondaryNode(table, secondaryNode)
			if err != nil {
				t.Fatalf("Failed to insert secondary node into %s: %v", table, err)
			}

			// Verify secondary node exists
			retrievedNode, err := db.GetNodeByID(secondaryNode.ID)
			if err != nil {
				t.Fatalf("Failed to get secondary node from %s: %v", table, err)
			}
			if retrievedNode.ID != secondaryNode.ID {
				t.Errorf("Secondary node ID mismatch in %s: expected %s, got %s", table, secondaryNode.ID, retrievedNode.ID)
			}
		}

		// 3. Update secondary existence map
		existenceMap := map[string]bool{
			"s1": true,
			"s2": true,
			"s3": false,
		}
		err = db.UpdateSecondaryExistenceMap(primaryNode.ID, existenceMap)
		if err != nil {
			t.Fatalf("Failed to update secondary existence map: %v", err)
		}

		// 4. Verify the update
		updatedNode, err := db.GetNodeByID(primaryNode.ID)
		if err != nil {
			t.Fatalf("Failed to get updated node: %v", err)
		}
		if len(updatedNode.SecondaryExistenceMap) != 3 {
			t.Errorf("Expected 3 entries in secondary existence map, got %d", len(updatedNode.SecondaryExistenceMap))
		}

		// 5. Create child nodes
		childNode := &types.Node{
			ID:                    "p-child-integration",
			ParentID:              primaryNode.ID,
			Name:                  "child",
			Path:                  "/integration-test/child",
			Type:                  types.NodeTypeFile,
			DepthLevel:            2,
			Size:                  1024,
			LastUpdated:           time.Now(),
			TraversalStatus:       types.StatusPending,
			SecondaryExistenceMap: map[string]bool{},
		}

		err = db.InsertPrimaryNode(childNode)
		if err != nil {
			t.Fatalf("Failed to insert child node: %v", err)
		}

		// 6. Verify child relationship
		children, err := db.GetChildrenByParentID(primaryNode.ID)
		if err != nil {
			t.Fatalf("Failed to get children: %v", err)
		}
		if len(children) != 1 {
			t.Errorf("Expected 1 child, got %d", len(children))
		}
		if children[0].ID != childNode.ID {
			t.Errorf("Expected child ID %s, got %s", childNode.ID, children[0].ID)
		}

		// 7. Update traversal status
		err = db.UpdateTraversalStatus(primaryNode.ID, types.StatusSuccessful)
		if err != nil {
			t.Fatalf("Failed to update traversal status: %v", err)
		}

		// 8. Verify final state
		finalNode, err := db.GetNodeByID(primaryNode.ID)
		if err != nil {
			t.Fatalf("Failed to get final node: %v", err)
		}
		if finalNode.TraversalStatus != types.StatusSuccessful {
			t.Errorf("Expected traversal status 'successful', got '%s'", finalNode.TraversalStatus)
		}

		// 9. Test table info
		tableInfo, err := db.GetTableInfo()
		if err != nil {
			t.Fatalf("Failed to get table info: %v", err)
		}
		if len(tableInfo) < 4 { // primary + 3 secondary tables
			t.Errorf("Expected at least 4 tables, got %d", len(tableInfo))
		}

		// 10. Test node counts
		tableNames := []string{"primary"}
		for table := range secondaryTables {
			tableNames = append(tableNames, table)
		}
		for _, table := range tableNames {
			count, err := db.GetNodeCount("nodes_" + table)
			if err != nil {
				t.Errorf("Failed to get node count for %s: %v", table, err)
			}
			if count < 1 {
				t.Errorf("Expected at least 1 node in %s, got %d", table, count)
			}
		}
	})
}

// Benchmark tests for database operations
func BenchmarkDBInsertPrimaryNode(b *testing.B) {
	// Create a temporary database
	tmpFile, err := os.CreateTemp("", "bench-db-insert-*.db")
	if err != nil {
		b.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	db, err := New(tmpFile.Name(), map[string]float64{})
	if err != nil {
		b.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create tables
	err = db.InitializeSchema(map[string]float64{"s1": 1.0})
	if err != nil {
		b.Fatalf("Failed to create tables: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		node := &types.Node{
			ID:                    "p-bench-" + string(rune(i)),
			ParentID:              "p-root",
			Name:                  "bench-node-" + string(rune(i)),
			Path:                  "/bench-node-" + string(rune(i)),
			Type:                  types.NodeTypeFolder,
			DepthLevel:            1,
			Size:                  0,
			LastUpdated:           time.Now(),
			TraversalStatus:       types.StatusPending,
			SecondaryExistenceMap: map[string]bool{},
		}

		err := db.InsertPrimaryNode(node)
		if err != nil {
			b.Fatalf("Failed to insert node: %v", err)
		}
	}
}

func BenchmarkDBGetNodeByID(b *testing.B) {
	// Create a temporary database
	tmpFile, err := os.CreateTemp("", "bench-db-get-*.db")
	if err != nil {
		b.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	db, err := New(tmpFile.Name(), map[string]float64{})
	if err != nil {
		b.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create tables
	err = db.InitializeSchema(map[string]float64{"s1": 1.0})
	if err != nil {
		b.Fatalf("Failed to create tables: %v", err)
	}

	// Insert a test node
	node := &types.Node{
		ID:                    "p-bench-get",
		ParentID:              "p-root",
		Name:                  "bench-get-node",
		Path:                  "/bench-get-node",
		Type:                  types.NodeTypeFolder,
		DepthLevel:            1,
		Size:                  0,
		LastUpdated:           time.Now(),
		TraversalStatus:       types.StatusPending,
		SecondaryExistenceMap: map[string]bool{},
	}

	err = db.InsertPrimaryNode(node)
	if err != nil {
		b.Fatalf("Failed to insert test node: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := db.GetNodeByID("p-bench-get")
		if err != nil {
			b.Fatalf("Failed to get node: %v", err)
		}
	}
}
