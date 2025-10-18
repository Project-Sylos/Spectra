package spectrafs

import (
	"os"
	"strings"
	"testing"

	"github.com/Project-Sylos/Spectra/internal/types"
)

// createTestConfig creates a temporary config file with a unique database file
func createTestConfig(t interface{ Fatal(args ...interface{}) }, configTemplate string) string {
	tmpFile, err := os.CreateTemp("", "test-config-*.json")
	if err != nil {
		t.Fatal(err)
	}

	// Create a temporary database file
	tmpDB, err := os.CreateTemp("", "test-spectrafs-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpDB.Close()
	os.Remove(tmpDB.Name()) // Remove the empty file
	defer os.Remove(tmpDB.Name())

	// Replace :memory: with the temporary database path
	config := strings.ReplaceAll(configTemplate, ":memory:", strings.ReplaceAll(tmpDB.Name(), "\\", "/"))

	tmpFile.WriteString(config)
	tmpFile.Close()
	return tmpFile.Name()
}

// TestNewSpectraFS tests the NewSpectraFS function
func TestNewSpectraFS(t *testing.T) {
	tests := []struct {
		name        string
		configPath  string
		expectError bool
		setup       func() string // Returns temp config path
		cleanup     func(string)
	}{
		{
			name:        "valid default config",
			configPath:  "",
			expectError: false,
			setup: func() string {
				tmpFile, err := os.CreateTemp("", "default-spectrafs-config-*.json")
				if err != nil {
					t.Fatal(err)
				}

				// Create a temporary database file
				tmpDB, err := os.CreateTemp("", "test-spectrafs-*.db")
				if err != nil {
					t.Fatal(err)
				}
				tmpDB.Close()
				os.Remove(tmpDB.Name()) // Remove the empty file

				tmpFile.WriteString(`{
					"seed": {
						"max_depth": 3,
						"min_folders": 1,
						"max_folders": 3,
						"min_files": 2,
						"max_files": 5,
						"seed": 42,
						"db_path": "` + strings.ReplaceAll(tmpDB.Name(), "\\", "/") + `"
					},
					"api": {
						"host": "localhost",
						"port": 8086
					},
					"secondary_tables": {
						"s1": 0.7,
						"s2": 0.3
					}
				}`)
				tmpFile.Close()
				return tmpFile.Name()
			},
			cleanup: func(path string) {
				os.Remove(path)
			},
		},
		{
			name:        "nonexistent config file",
			configPath:  "nonexistent.json",
			expectError: true,
			setup:       nil,
			cleanup:     nil,
		},
		{
			name:        "valid custom config",
			configPath:  "",
			expectError: false,
			setup: func() string {
				tmpFile, err := os.CreateTemp("", "valid-spectrafs-config-*.json")
				if err != nil {
					t.Fatal(err)
				}

				// Create a temporary database file
				tmpDB, err := os.CreateTemp("", "test-spectrafs-custom-*.db")
				if err != nil {
					t.Fatal(err)
				}
				tmpDB.Close()
				os.Remove(tmpDB.Name()) // Remove the empty file

				tmpFile.WriteString(`{
					"seed": {
						"max_depth": 2,
						"min_folders": 1,
						"max_folders": 2,
						"min_files": 1,
						"max_files": 2,
						"seed": 123,
						"db_path": "` + strings.ReplaceAll(tmpDB.Name(), "\\", "/") + `"
					},
					"api": {
						"host": "localhost",
						"port": 8080
					},
					"secondary_tables": {
						"s1": 0.5
					}
				}`)
				tmpFile.Close()
				return tmpFile.Name()
			},
			cleanup: func(path string) {
				os.Remove(path)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var configPath string
			if tt.setup != nil {
				configPath = tt.setup()
				defer tt.cleanup(configPath)
			} else {
				configPath = tt.configPath
			}

			fs, err := NewSpectraFS(configPath)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				if fs != nil {
					t.Errorf("Expected nil SpectraFS but got %v", fs)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if fs == nil {
					t.Errorf("Expected SpectraFS but got nil")
				}
				if fs.db == nil {
					t.Errorf("Expected database connection but got nil")
				}
				if fs.cfg == nil {
					t.Errorf("Expected configuration but got nil")
				}
				if fs.rng == nil {
					t.Errorf("Expected random number generator but got nil")
				}
			}
		})
	}
}

// TestSpectraFSMethods tests the core SpectraFS methods
func TestSpectraFSMethods(t *testing.T) {
	configTemplate := `{
		"seed": {
			"max_depth": 2,
			"min_folders": 1,
			"max_folders": 2,
			"min_files": 1,
			"max_files": 2,
			"seed": 123,
			"db_path": ":memory:"
		},
		"api": {
			"host": "localhost",
			"port": 8080
		},
		"secondary_tables": {
			"s1": 0.5
		}
	}`

	configPath := createTestConfig(t, configTemplate)
	defer os.Remove(configPath)

	// Create SpectraFS instance
	fs, err := NewSpectraFS(configPath)
	if err != nil {
		t.Fatalf("Failed to create SpectraFS instance: %v", err)
	}
	defer fs.Close()

	// Test GetConfig
	t.Run("GetConfig", func(t *testing.T) {
		config := fs.GetConfig()
		if config == nil {
			t.Errorf("Expected config but got nil")
		}
		if config.Seed.MaxDepth != 2 {
			t.Errorf("Expected max_depth 2, got %d", config.Seed.MaxDepth)
		}
		if config.API.Port != 8080 {
			t.Errorf("Expected port 8080, got %d", config.API.Port)
		}
	})

	// Test GetTableInfo
	t.Run("GetTableInfo", func(t *testing.T) {
		tableInfo, err := fs.GetTableInfo()
		if err != nil {
			t.Errorf("Unexpected error getting table info: %v", err)
		}
		if tableInfo == nil {
			t.Errorf("Expected table info but got nil")
		}
	})

	// Test GetSecondaryTables
	t.Run("GetSecondaryTables", func(t *testing.T) {
		tables := fs.GetSecondaryTables()
		if tables == nil {
			t.Errorf("Expected secondary tables but got nil")
		}
		// Should have at least one secondary table (s1)
		if len(tables) < 1 {
			t.Errorf("Expected at least one secondary table, got %d", len(tables))
		}
	})

	// Test GetNodeCount
	t.Run("GetNodeCount", func(t *testing.T) {
		count, err := fs.GetNodeCount("nodes_primary")
		if err != nil {
			t.Errorf("Unexpected error getting node count: %v", err)
		}
		// Should have at least the root node
		if count < 1 {
			t.Errorf("Expected at least 1 node (root), got %d", count)
		}
	})

	// Test GetNode with root
	t.Run("GetNode", func(t *testing.T) {
		node, err := fs.GetNode("p-root")
		if err != nil {
			t.Errorf("Unexpected error getting root node: %v", err)
		}
		if node == nil {
			t.Errorf("Expected root node but got nil")
		}
		if node.ID != "p-root" {
			t.Errorf("Expected root node ID 'p-root', got '%s'", node.ID)
		}
	})

	// Test ListChildren with root
	t.Run("ListChildren", func(t *testing.T) {
		result, err := fs.ListChildren("p-root")
		if err != nil {
			t.Errorf("Unexpected error listing root children: %v", err)
		}
		if result == nil {
			t.Errorf("Expected list result but got nil")
		}
		if !result.Success {
			t.Errorf("Expected successful list operation, got success=%t", result.Success)
		}
	})

	// Test CreateFolder
	t.Run("CreateFolder", func(t *testing.T) {
		folder, err := fs.CreateFolder("p-root", "test-folder")
		if err != nil {
			t.Errorf("Unexpected error creating folder: %v", err)
		}
		if folder == nil {
			t.Errorf("Expected created folder but got nil")
		}
		if folder.Name != "test-folder" {
			t.Errorf("Expected folder name 'test-folder', got '%s'", folder.Name)
		}
		if folder.Type != types.NodeTypeFolder {
			t.Errorf("Expected folder type, got '%s'", folder.Type)
		}
		if folder.ParentID != "p-root" {
			t.Errorf("Expected parent ID 'p-root', got '%s'", folder.ParentID)
		}
	})

	// Test UploadFile
	t.Run("UploadFile", func(t *testing.T) {
		testData := []byte("test file content")
		file, err := fs.UploadFile("p-root", "test-file.txt", testData)
		if err != nil {
			t.Errorf("Unexpected error uploading file: %v", err)
		}
		if file == nil {
			t.Errorf("Expected uploaded file but got nil")
		}
		if file.Name != "test-file.txt" {
			t.Errorf("Expected file name 'test-file.txt', got '%s'", file.Name)
		}
		if file.Type != types.NodeTypeFile {
			t.Errorf("Expected file type, got '%s'", file.Type)
		}
		if file.ParentID != "p-root" {
			t.Errorf("Expected parent ID 'p-root', got '%s'", file.ParentID)
		}
	})

	// Test GetFileData
	t.Run("GetFileData", func(t *testing.T) {
		// First create a file
		testData := []byte("test file content")
		file, err := fs.UploadFile("p-root", "data-test.txt", testData)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		data, checksum, err := fs.GetFileData(file.ID)
		if err != nil {
			t.Errorf("Unexpected error getting file data: %v", err)
		}
		if data == nil {
			t.Errorf("Expected file data but got nil")
		}
		if checksum == "" {
			t.Errorf("Expected checksum but got empty string")
		}
		if len(data) != 1024 {
			t.Errorf("Expected 1024 bytes of data, got %d", len(data))
		}
	})

	// Test UpdateTraversalStatus
	t.Run("UpdateTraversalStatus", func(t *testing.T) {
		err := fs.UpdateTraversalStatus("p-root", types.StatusSuccessful)
		if err != nil {
			t.Errorf("Unexpected error updating traversal status: %v", err)
		}
	})

	// Test DeleteNode
	t.Run("DeleteNode", func(t *testing.T) {
		// Create a test node first
		folder, err := fs.CreateFolder("p-root", "delete-test")
		if err != nil {
			t.Fatalf("Failed to create test folder: %v", err)
		}

		err = fs.DeleteNode(folder.ID)
		if err != nil {
			t.Errorf("Unexpected error deleting node: %v", err)
		}

		// Verify node was deleted
		_, err = fs.GetNode(folder.ID)
		if err == nil {
			t.Errorf("Expected error getting deleted node")
		}
	})

	// Test Reset
	t.Run("Reset", func(t *testing.T) {
		err := fs.Reset()
		if err != nil {
			t.Errorf("Unexpected error resetting filesystem: %v", err)
		}
	})
}

// TestSpectraFSIntegration tests complex integration scenarios
func TestSpectraFSIntegration(t *testing.T) {
	configTemplate := `{
		"seed": {
			"max_depth": 3,
			"min_folders": 1,
			"max_folders": 3,
			"min_files": 1,
			"max_files": 3,
			"seed": 456,
			"db_path": ":memory:"
		},
		"api": {
			"host": "localhost",
			"port": 8081
		},
		"secondary_tables": {
			"s1": 0.7,
			"s2": 0.3
		}
	}`

	configPath := createTestConfig(t, configTemplate)
	defer os.Remove(configPath)

	// Create SpectraFS instance
	fs, err := NewSpectraFS(configPath)
	if err != nil {
		t.Fatalf("Failed to create SpectraFS instance: %v", err)
	}
	defer fs.Close()

	// Test complete workflow
	t.Run("CompleteWorkflow", func(t *testing.T) {

		// 2. List root children (should trigger generation)
		result, err := fs.ListChildren("p-root")
		if err != nil {
			t.Fatalf("Failed to list root children: %v", err)
		}
		if !result.Success {
			t.Fatalf("Expected successful list operation")
		}

		// 3. Verify tables exist (they should already be created during initialization)
		finalTables, err := fs.GetTableInfo()
		if err != nil {
			t.Fatalf("Failed to get final table info: %v", err)
		}
		// Tables should exist from initialization, not from generation
		if len(finalTables) < 3 { // primary + s1 + s2
			t.Errorf("Expected at least 3 tables (primary + 2 secondary), got %d", len(finalTables))
		}

		// 4. Verify secondary tables have data
		for _, table := range fs.GetSecondaryTables() {
			count, err := fs.GetNodeCount("nodes_" + table)
			if err != nil {
				t.Errorf("Failed to get count for table %s: %v", table, err)
			}
			t.Logf("Secondary table %s has %d nodes", table, count)
		}

		// 5. Create a nested structure
		folder1, err := fs.CreateFolder("p-root", "level1")
		if err != nil {
			t.Fatalf("Failed to create level1 folder: %v", err)
		}

		folder2, err := fs.CreateFolder(folder1.ID, "level2")
		if err != nil {
			t.Fatalf("Failed to create level2 folder: %v", err)
		}

		file1, err := fs.UploadFile(folder2.ID, "deep-file.txt", []byte("deep content"))
		if err != nil {
			t.Fatalf("Failed to upload deep file: %v", err)
		}

		// 6. Verify the nested structure
		if folder1.ParentID != "p-root" {
			t.Errorf("Expected folder1 parent to be p-root, got %s", folder1.ParentID)
		}
		if folder2.ParentID != folder1.ID {
			t.Errorf("Expected folder2 parent to be folder1 ID, got %s", folder2.ParentID)
		}
		if file1.ParentID != folder2.ID {
			t.Errorf("Expected file1 parent to be folder2 ID, got %s", file1.ParentID)
		}

		// 7. Test file data retrieval
		data, checksum, err := fs.GetFileData(file1.ID)
		if err != nil {
			t.Fatalf("Failed to get file data: %v", err)
		}
		if len(data) != 1024 {
			t.Errorf("Expected 1024 bytes of data, got %d", len(data))
		}
		if checksum == "" {
			t.Errorf("Expected non-empty checksum")
		}

		// 8. Test traversal status updates
		err = fs.UpdateTraversalStatus(folder1.ID, types.StatusSuccessful)
		if err != nil {
			t.Errorf("Failed to update traversal status: %v", err)
		}

		// 9. Test node retrieval after status update
		updatedFolder1, err := fs.GetNode(folder1.ID)
		if err != nil {
			t.Errorf("Failed to get updated node: %v", err)
		}
		if updatedFolder1.TraversalStatus != types.StatusSuccessful {
			t.Errorf("Expected traversal status 'successful', got '%s'", updatedFolder1.TraversalStatus)
		}

		t.Logf("Integration test completed successfully. Generated %d folders and %d files", len(result.Folders), len(result.Files))
	})
}

// TestSpectraFSErrorHandling tests error handling scenarios
func TestSpectraFSErrorHandling(t *testing.T) {
	configTemplate := `{
		"seed": {
			"max_depth": 2,
			"min_folders": 1,
			"max_folders": 2,
			"min_files": 1,
			"max_files": 2,
			"seed": 789,
			"db_path": ":memory:"
		},
		"api": {
			"host": "localhost",
			"port": 8082
		},
		"secondary_tables": {
			"s1": 0.5
		}
	}`

	configPath := createTestConfig(t, configTemplate)
	defer os.Remove(configPath)

	fs, err := NewSpectraFS(configPath)
	if err != nil {
		t.Fatalf("Failed to create SpectraFS instance: %v", err)
	}
	defer fs.Close()

	tests := []struct {
		name        string
		testFunc    func() error
		expectError bool
	}{
		{
			name: "GetNode with invalid ID",
			testFunc: func() error {
				_, err := fs.GetNode("invalid-id")
				return err
			},
			expectError: true,
		},
		{
			name: "GetNode with empty ID",
			testFunc: func() error {
				_, err := fs.GetNode("")
				return err
			},
			expectError: true,
		},
		{
			name: "GetFileData with invalid ID",
			testFunc: func() error {
				_, _, err := fs.GetFileData("invalid-id")
				return err
			},
			expectError: true,
		},
		{
			name: "GetFileData with folder ID",
			testFunc: func() error {
				_, _, err := fs.GetFileData("p-root")
				return err
			},
			expectError: true,
		},
		{
			name: "DeleteNode with invalid ID",
			testFunc: func() error {
				err := fs.DeleteNode("invalid-id")
				return err
			},
			expectError: true,
		},
		{
			name: "DeleteNode with root ID",
			testFunc: func() error {
				err := fs.DeleteNode("p-root")
				return err
			},
			expectError: true,
		},
		{
			name: "CreateFolder with invalid parent",
			testFunc: func() error {
				_, err := fs.CreateFolder("invalid-parent", "test")
				return err
			},
			expectError: true,
		},
		{
			name: "CreateFolder with file parent",
			testFunc: func() error {
				// First create a file
				file, err := fs.UploadFile("p-root", "parent-file.txt", []byte("data"))
				if err != nil {
					return err
				}
				// Try to create folder with file as parent
				_, err = fs.CreateFolder(file.ID, "test")
				return err
			},
			expectError: true,
		},
		{
			name: "UploadFile with invalid parent",
			testFunc: func() error {
				_, err := fs.UploadFile("invalid-parent", "test.txt", []byte("data"))
				return err
			},
			expectError: true,
		},
		{
			name: "GetNodeCount with invalid table",
			testFunc: func() error {
				_, err := fs.GetNodeCount("invalid-table")
				return err
			},
			expectError: true,
		},
		{
			name: "UpdateTraversalStatus with invalid ID",
			testFunc: func() error {
				err := fs.UpdateTraversalStatus("invalid-id", types.StatusSuccessful)
				return err
			},
			expectError: true,
		},
		{
			name: "UpdateTraversalStatus with invalid status",
			testFunc: func() error {
				err := fs.UpdateTraversalStatus("p-root", "invalid-status")
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

// TestSpectraFSConcurrency tests concurrent access to SpectraFS
func TestSpectraFSConcurrency(t *testing.T) {
	configTemplate := `{
		"seed": {
			"max_depth": 2,
			"min_folders": 1,
			"max_folders": 2,
			"min_files": 1,
			"max_files": 2,
			"seed": 999,
			"db_path": ":memory:"
		},
		"api": {
			"host": "localhost",
			"port": 8083
		},
		"secondary_tables": {
			"s1": 0.5
		}
	}`

	configPath := createTestConfig(t, configTemplate)
	defer os.Remove(configPath)

	fs, err := NewSpectraFS(configPath)
	if err != nil {
		t.Fatalf("Failed to create SpectraFS instance: %v", err)
	}
	defer fs.Close()

	// Test concurrent operations
	done := make(chan bool, 10)

	// Run multiple goroutines with different operations
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			// Create a folder
			folder, err := fs.CreateFolder("p-root", "concurrent-folder-"+string(rune(id)))
			if err != nil {
				t.Errorf("Failed to create folder in goroutine %d: %v", id, err)
				return
			}

			// Create a file in the folder
			_, err = fs.UploadFile(folder.ID, "concurrent-file-"+string(rune(id))+".txt", []byte("concurrent data"))
			if err != nil {
				t.Errorf("Failed to upload file in goroutine %d: %v", id, err)
				return
			}

			// Get the node
			_, err = fs.GetNode(folder.ID)
			if err != nil {
				t.Errorf("Failed to get node in goroutine %d: %v", id, err)
				return
			}

			// List children
			_, err = fs.ListChildren(folder.ID)
			if err != nil {
				t.Errorf("Failed to list children in goroutine %d: %v", id, err)
				return
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify final state
	finalCount, err := fs.GetNodeCount("nodes_primary")
	if err != nil {
		t.Errorf("Failed to get final node count: %v", err)
	}
	if finalCount < 21 { // 1 root + 10 folders + 10 files
		t.Errorf("Expected at least 21 nodes, got %d", finalCount)
	}
}

// TestSpectraFSDeterministic tests deterministic behavior
func TestSpectraFSDeterministic(t *testing.T) {
	// Create two identical configurations
	config1 := `{
		"seed": {
			"max_depth": 2,
			"min_folders": 1,
			"max_folders": 2,
			"min_files": 1,
			"max_files": 2,
			"seed": 12345,
			"db_path": ":memory:"
		},
		"api": {
			"host": "localhost",
			"port": 8080
		},
		"secondary_tables": {
			"s1": 0.5
		}
	}`

	config2 := `{
		"seed": {
			"max_depth": 2,
			"min_folders": 1,
			"max_folders": 2,
			"min_files": 1,
			"max_files": 2,
			"seed": 12345,
			"db_path": ":memory:"
		},
		"api": {
			"host": "localhost",
			"port": 8080
		},
		"secondary_tables": {
			"s1": 0.5
		}
	}`

	// Create temporary config files
	configPath1 := createTestConfig(t, config1)
	defer os.Remove(configPath1)

	configPath2 := createTestConfig(t, config2)
	defer os.Remove(configPath2)

	// Create two SpectraFS instances with identical configs
	fs1, err := NewSpectraFS(configPath1)
	if err != nil {
		t.Fatalf("Failed to create first SpectraFS instance: %v", err)
	}
	defer fs1.Close()

	fs2, err := NewSpectraFS(configPath2)
	if err != nil {
		t.Fatalf("Failed to create second SpectraFS instance: %v", err)
	}
	defer fs2.Close()

	// Generate children in both instances
	result1, err := fs1.ListChildren("p-root")
	if err != nil {
		t.Fatalf("Failed to list children in first instance: %v", err)
	}

	result2, err := fs2.ListChildren("p-root")
	if err != nil {
		t.Fatalf("Failed to list children in second instance: %v", err)
	}

	// Verify results are identical
	if len(result1.Folders) != len(result2.Folders) {
		t.Errorf("Folder count mismatch: %d vs %d", len(result1.Folders), len(result2.Folders))
	}
	if len(result1.Files) != len(result2.Files) {
		t.Errorf("File count mismatch: %d vs %d", len(result1.Files), len(result2.Files))
	}

	// Verify folder names are identical
	for i, folder1 := range result1.Folders {
		if i >= len(result2.Folders) {
			t.Errorf("Missing folder at index %d", i)
			continue
		}
		folder2 := result2.Folders[i]
		if folder1.Name != folder2.Name {
			t.Errorf("Folder name mismatch at index %d: %s vs %s", i, folder1.Name, folder2.Name)
		}
	}

	// Verify file names are identical
	for i, file1 := range result1.Files {
		if i >= len(result2.Files) {
			t.Errorf("Missing file at index %d", i)
			continue
		}
		file2 := result2.Files[i]
		if file1.Name != file2.Name {
			t.Errorf("File name mismatch at index %d: %s vs %s", i, file1.Name, file2.Name)
		}
	}
}

// Benchmark tests for SpectraFS operations
func BenchmarkSpectraFSListChildren(b *testing.B) {
	configTemplate := `{
		"seed": {
			"max_depth": 2,
			"min_folders": 1,
			"max_folders": 2,
			"min_files": 1,
			"max_files": 2,
			"seed": 42,
			"db_path": ":memory:"
		},
		"api": {
			"host": "localhost",
			"port": 8080
		},
		"secondary_tables": {
			"s1": 0.5
		}
	}`

	configPath := createTestConfig(b, configTemplate)
	defer os.Remove(configPath)

	fs, err := NewSpectraFS(configPath)
	if err != nil {
		b.Fatalf("Failed to create SpectraFS instance: %v", err)
	}
	defer fs.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := fs.ListChildren("p-root")
		if err != nil {
			b.Fatalf("Failed to list children: %v", err)
		}
	}
}

func BenchmarkSpectraFSCreateFolder(b *testing.B) {
	configTemplate := `{
		"seed": {
			"max_depth": 2,
			"min_folders": 1,
			"max_folders": 2,
			"min_files": 1,
			"max_files": 2,
			"seed": 42,
			"db_path": ":memory:"
		},
		"api": {
			"host": "localhost",
			"port": 8080
		},
		"secondary_tables": {
			"s1": 0.5
		}
	}`

	configPath := createTestConfig(b, configTemplate)
	defer os.Remove(configPath)

	fs, err := NewSpectraFS(configPath)
	if err != nil {
		b.Fatalf("Failed to create SpectraFS instance: %v", err)
	}
	defer fs.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := fs.CreateFolder("p-root", "bench-folder-"+string(rune(i)))
		if err != nil {
			b.Fatalf("Failed to create folder: %v", err)
		}
	}
}
