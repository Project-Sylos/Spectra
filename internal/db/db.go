package db

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/Project-Sylos/Spectra/internal/types"
	"github.com/Project-Sylos/Spectra/internal/utils"
	"github.com/google/uuid"
	"go.etcd.io/bbolt"
)

// DB wraps BoltDB connection and provides key-value CRUD operations
type DB struct {
	db              *bbolt.DB
	secondaryTables []string   // List of secondary world names (e.g., ["s1", "s2"])
	mu              sync.Mutex // Protects all database operations from concurrent access
}

// New creates a new database connection and initializes the schema
func New(dbPath string, secondaryTables map[string]float64) (*DB, error) {
	// Check if database file exists
	dbFileExists := false
	if _, err := os.Stat(dbPath); err == nil {
		dbFileExists = true
	}

	// Open BoltDB connection
	boltDB, err := bbolt.Open(dbPath, 0600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open BoltDB connection: %w", err)
	}

	// Create secondary tables list
	secondaryList := make([]string, 0, len(secondaryTables))
	for tableName := range secondaryTables {
		secondaryList = append(secondaryList, tableName)
	}

	db := &DB{
		db:              boltDB,
		secondaryTables: secondaryList,
	}

	// Verify and initialize database structure
	if err := db.VerifyAndInitialize(dbFileExists, secondaryTables); err != nil {
		boltDB.Close()
		return nil, fmt.Errorf("failed to verify and initialize database: %w", err)
	}

	return db, nil
}

// VerifyAndInitialize performs comprehensive database verification and initialization
// It checks each stage and creates what's missing:
// A) Database file exists (checked before connection)
// B) Buckets exist
// C) Root node exists
func (db *DB) VerifyAndInitialize(dbFileExists bool, secondaryTables map[string]float64) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// B) Initialize or verify buckets exist
	if !dbFileExists {
		// New database - create all buckets
		if err := InitializeBuckets(db.db); err != nil {
			return fmt.Errorf("failed to initialize buckets: %w", err)
		}
	} else {
		// Existing database - verify buckets exist
		if err := VerifyBucketsExist(db.db); err != nil {
			return fmt.Errorf("failed to verify buckets: %w", err)
		}
	}

	// C) Check if root node exists
	rootExists, err := db.rootNodeExists()
	if err != nil {
		return fmt.Errorf("failed to check if root node exists: %w", err)
	}

	if !rootExists {
		// Create root node
		if err := db.createRootNodeInternal(); err != nil {
			return fmt.Errorf("failed to create root node: %w", err)
		}
	}

	return nil
}

// rootNodeExists checks if the root node exists
func (db *DB) rootNodeExists() (bool, error) {
	var exists bool
	err := db.db.View(func(tx *bbolt.Tx) error {
		nodesBucket := tx.Bucket([]byte(bucketNodes))
		if nodesBucket == nil {
			return fmt.Errorf("nodes bucket does not exist")
		}

		rootData := nodesBucket.Get([]byte("root"))
		exists = rootData != nil
		return nil
	})

	if err != nil {
		return false, err
	}
	return exists, nil
}

// createRootNodeInternal creates the root node (internal, assumes lock is held)
func (db *DB) createRootNodeInternal() error {
	// Create existence map with all worlds
	existenceMap := make(map[string]bool)
	existenceMap["primary"] = true
	for _, worldName := range db.secondaryTables {
		existenceMap[worldName] = true
	}

	// Create root node
	rootNode := &types.Node{
		ID:           "root",
		ParentID:     "",
		Name:         "root",
		Path:         "/",
		ParentPath:   "",
		Type:         types.NodeTypeFolder,
		DepthLevel:   0,
		Size:         0,
		LastUpdated:  time.Now(),
		Checksum:     nil,
		ExistenceMap: existenceMap,
	}

	// Use InsertNode logic but within the existing transaction context
	// Since lock is already held, we can't call InsertNode directly
	return db.db.Update(func(tx *bbolt.Tx) error {
		// Serialize node to JSON
		nodeJSON, err := json.Marshal(rootNode)
		if err != nil {
			return fmt.Errorf("failed to marshal root node: %w", err)
		}

		// Store node in nodes bucket
		nodesBucket := tx.Bucket([]byte(bucketNodes))
		if nodesBucket == nil {
			return fmt.Errorf("nodes bucket does not exist")
		}

		if err := nodesBucket.Put([]byte("root"), nodeJSON); err != nil {
			return fmt.Errorf("failed to create root node: %w", err)
		}

		// Update index_parent_id: key format "{parentID}|{nodeID}"
		indexParentID := tx.Bucket([]byte(bucketIndexParentID))
		if indexParentID != nil {
			parentIDKey := fmt.Sprintf("%s|%s", rootNode.ParentID, rootNode.ID)
			if err := indexParentID.Put([]byte(parentIDKey), []byte{}); err != nil {
				return fmt.Errorf("failed to update parent_id index: %w", err)
			}
		}

		// Update index_path: key format "{path}" -> value "{nodeID}"
		indexPath := tx.Bucket([]byte(bucketIndexPath))
		if indexPath != nil {
			if err := indexPath.Put([]byte(rootNode.Path), []byte(rootNode.ID)); err != nil {
				return fmt.Errorf("failed to update path index: %w", err)
			}
		}

		// Update index_parent_path: key format "{parentPath}|{nodeID}"
		indexParentPath := tx.Bucket([]byte(bucketIndexParentPath))
		if indexParentPath != nil {
			parentPathKey := fmt.Sprintf("%s|%s", rootNode.ParentPath, rootNode.ID)
			if err := indexParentPath.Put([]byte(parentPathKey), []byte{}); err != nil {
				return fmt.Errorf("failed to update parent_path index: %w", err)
			}
		}

		return nil
	})
}

// Close closes the database connection
// BoltDB is ACID compliant and automatically persists all changes
func (db *DB) Close() error {
	return db.db.Close()
}

// InsertNode inserts a new node into the nodes bucket and updates all indexes
func (db *DB) InsertNode(node *types.Node) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	return db.db.Update(func(tx *bbolt.Tx) error {
		// Serialize node to JSON
		nodeJSON, err := json.Marshal(node)
		if err != nil {
			return fmt.Errorf("[SpectraFS] failed to marshal node %s: %w", node.ID, err)
		}

		// Store node in nodes bucket
		nodesBucket := tx.Bucket([]byte(bucketNodes))
		if nodesBucket == nil {
			return fmt.Errorf("[SpectraFS] nodes bucket does not exist")
		}

		if err := nodesBucket.Put([]byte(node.ID), nodeJSON); err != nil {
			return fmt.Errorf("[SpectraFS] failed to insert node %s: %w", node.ID, err)
		}

		// Update index_parent_id: key format "{parentID}|{nodeID}"
		indexParentID := tx.Bucket([]byte(bucketIndexParentID))
		if indexParentID == nil {
			return fmt.Errorf("[SpectraFS] index_parent_id bucket does not exist")
		}
		parentIDKey := fmt.Sprintf("%s|%s", node.ParentID, node.ID)
		if err := indexParentID.Put([]byte(parentIDKey), []byte{}); err != nil {
			return fmt.Errorf("[SpectraFS] failed to update parent_id index for node %s: %w", node.ID, err)
		}

		// Update index_path: key format "{path}" -> value "{nodeID}"
		indexPath := tx.Bucket([]byte(bucketIndexPath))
		if indexPath == nil {
			return fmt.Errorf("[SpectraFS] index_path bucket does not exist")
		}
		if err := indexPath.Put([]byte(node.Path), []byte(node.ID)); err != nil {
			return fmt.Errorf("[SpectraFS] failed to update path index for node %s: %w", node.ID, err)
		}

		// Update index_parent_path: key format "{parentPath}|{nodeID}"
		indexParentPath := tx.Bucket([]byte(bucketIndexParentPath))
		if indexParentPath == nil {
			return fmt.Errorf("[SpectraFS] index_parent_path bucket does not exist")
		}
		parentPathKey := fmt.Sprintf("%s|%s", node.ParentPath, node.ID)
		if err := indexParentPath.Put([]byte(parentPathKey), []byte{}); err != nil {
			return fmt.Errorf("[SpectraFS] failed to update parent_path index for node %s: %w", node.ID, err)
		}

		return nil
	})
}

// GetNodeByID retrieves a node by its ID from the nodes bucket
func (db *DB) GetNodeByID(id string) (*types.Node, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	var node *types.Node
	err := db.db.View(func(tx *bbolt.Tx) error {
		nodesBucket := tx.Bucket([]byte(bucketNodes))
		if nodesBucket == nil {
			return fmt.Errorf("[SpectraFS] nodes bucket does not exist")
		}

		nodeData := nodesBucket.Get([]byte(id))
		if nodeData == nil {
			return fmt.Errorf("[SpectraFS] node not found: %s", id)
		}

		node = &types.Node{}
		if err := json.Unmarshal(nodeData, node); err != nil {
			return fmt.Errorf("[SpectraFS] failed to unmarshal node %s: %w", id, err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return node, nil
}

// GetChildrenByParentID retrieves all children of a parent node filtered by world
func (db *DB) GetChildrenByParentID(parentID, world string) ([]*types.Node, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	var children []*types.Node
	err := db.db.View(func(tx *bbolt.Tx) error {
		// Use index_parent_id bucket to find all children
		indexParentID := tx.Bucket([]byte(bucketIndexParentID))
		if indexParentID == nil {
			return fmt.Errorf("[SpectraFS] index_parent_id bucket does not exist")
		}

		nodesBucket := tx.Bucket([]byte(bucketNodes))
		if nodesBucket == nil {
			return fmt.Errorf("[SpectraFS] nodes bucket does not exist")
		}

		// Prefix to search for: "{parentID}|"
		prefix := []byte(parentID + "|")
		cursor := indexParentID.Cursor()

		// Iterate over all keys with the parentID prefix
		for key, _ := cursor.Seek(prefix); key != nil && len(key) > len(prefix) && string(key[:len(prefix)]) == string(prefix); key, _ = cursor.Next() {
			// Extract nodeID from key: "{parentID}|{nodeID}"
			nodeID := string(key[len(prefix):])

			// Get node from nodes bucket
			nodeData := nodesBucket.Get([]byte(nodeID))
			if nodeData == nil {
				continue // Skip if node not found
			}

			var node types.Node
			if err := json.Unmarshal(nodeData, &node); err != nil {
				return fmt.Errorf("[SpectraFS] failed to unmarshal node %s: %w", nodeID, err)
			}

			// Filter by world - check existence map
			if node.ExistenceMap[world] {
				children = append(children, &node)
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("[SpectraFS] failed to query children of %s in world %s: %w", parentID, world, err)
	}

	// Sort by type, then name (folders first, then files)
	sort.Slice(children, func(i, j int) bool {
		if children[i].Type != children[j].Type {
			return children[i].Type < children[j].Type // "folder" < "file"
		}
		return children[i].Name < children[j].Name
	})

	return children, nil
}

// GetParentAndChildren retrieves parent and all its children in ONE optimized query
// This is the key performance optimization for ListChildren operations
func (db *DB) GetParentAndChildren(parentID, world string) ([]*types.Node, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	var nodes []*types.Node
	err := db.db.View(func(tx *bbolt.Tx) error {
		nodesBucket := tx.Bucket([]byte(bucketNodes))
		if nodesBucket == nil {
			return fmt.Errorf("[SpectraFS] nodes bucket does not exist")
		}

		// Get parent node
		parentData := nodesBucket.Get([]byte(parentID))
		if parentData != nil {
			var parent types.Node
			if err := json.Unmarshal(parentData, &parent); err != nil {
				return fmt.Errorf("[SpectraFS] failed to unmarshal parent node %s: %w", parentID, err)
			}
			// Filter by world
			if parent.ExistenceMap[world] {
				nodes = append(nodes, &parent)
			}
		}

		// Get children using index_parent_id
		indexParentID := tx.Bucket([]byte(bucketIndexParentID))
		if indexParentID == nil {
			return fmt.Errorf("[SpectraFS] index_parent_id bucket does not exist")
		}

		// Prefix to search for: "{parentID}|"
		prefix := []byte(parentID + "|")
		cursor := indexParentID.Cursor()

		// Iterate over all keys with the parentID prefix
		for key, _ := cursor.Seek(prefix); key != nil && len(key) > len(prefix) && string(key[:len(prefix)]) == string(prefix); key, _ = cursor.Next() {
			// Extract nodeID from key: "{parentID}|{nodeID}"
			nodeID := string(key[len(prefix):])

			// Get node from nodes bucket
			nodeData := nodesBucket.Get([]byte(nodeID))
			if nodeData == nil {
				continue // Skip if node not found
			}

			var node types.Node
			if err := json.Unmarshal(nodeData, &node); err != nil {
				return fmt.Errorf("[SpectraFS] failed to unmarshal node %s: %w", nodeID, err)
			}

			// Filter by world - check existence map
			if node.ExistenceMap[world] {
				nodes = append(nodes, &node)
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("[SpectraFS] failed to query parent and children: %w", err)
	}

	// Sort: parent first (if exists), then children by type, name
	sort.Slice(nodes, func(i, j int) bool {
		// Parent should be first (id == parentID)
		if nodes[i].ID == parentID && nodes[j].ID != parentID {
			return true
		}
		if nodes[i].ID != parentID && nodes[j].ID == parentID {
			return false
		}
		// Both are children or both are parent - sort by type, then name
		if nodes[i].Type != nodes[j].Type {
			return nodes[i].Type < nodes[j].Type // "folder" < "file"
		}
		return nodes[i].Name < nodes[j].Name
	})

	return nodes, nil
}

// CheckChildrenExist checks if a parent has any children in a specific world
func (db *DB) CheckChildrenExist(parentID, world string) (bool, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	var hasChildren bool
	err := db.db.View(func(tx *bbolt.Tx) error {
		// Use index_parent_id bucket to find children
		indexParentID := tx.Bucket([]byte(bucketIndexParentID))
		if indexParentID == nil {
			return fmt.Errorf("[SpectraFS] index_parent_id bucket does not exist")
		}

		nodesBucket := tx.Bucket([]byte(bucketNodes))
		if nodesBucket == nil {
			return fmt.Errorf("[SpectraFS] nodes bucket does not exist")
		}

		// Prefix to search for: "{parentID}|"
		prefix := []byte(parentID + "|")
		cursor := indexParentID.Cursor()

		// Check if any child exists in the specified world
		for key, _ := cursor.Seek(prefix); key != nil && len(key) > len(prefix) && string(key[:len(prefix)]) == string(prefix); key, _ = cursor.Next() {
			// Extract nodeID from key: "{parentID}|{nodeID}"
			nodeID := string(key[len(prefix):])

			// Get node from nodes bucket
			nodeData := nodesBucket.Get([]byte(nodeID))
			if nodeData == nil {
				continue // Skip if node not found
			}

			var node types.Node
			if err := json.Unmarshal(nodeData, &node); err != nil {
				continue // Skip on error
			}

			// Check if node exists in the specified world
			if node.ExistenceMap[world] {
				hasChildren = true
				return nil // Found one, we can return early
			}
		}

		return nil
	})

	if err != nil {
		return false, fmt.Errorf("[SpectraFS] failed to check children existence for %s: %w", parentID, err)
	}

	return hasChildren, nil
}

// UpdateExistenceMap updates the existence map for a node
func (db *DB) UpdateExistenceMap(id string, existenceMap map[string]bool) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	return db.db.Update(func(tx *bbolt.Tx) error {
		nodesBucket := tx.Bucket([]byte(bucketNodes))
		if nodesBucket == nil {
			return fmt.Errorf("[SpectraFS] nodes bucket does not exist")
		}

		// Get existing node
		nodeData := nodesBucket.Get([]byte(id))
		if nodeData == nil {
			return fmt.Errorf("[SpectraFS] node %s not found", id)
		}

		var node types.Node
		if err := json.Unmarshal(nodeData, &node); err != nil {
			return fmt.Errorf("[SpectraFS] failed to unmarshal node %s: %w", id, err)
		}

		// Update existence map
		node.ExistenceMap = existenceMap

		// Serialize updated node
		updatedNodeData, err := json.Marshal(node)
		if err != nil {
			return fmt.Errorf("[SpectraFS] failed to marshal node %s: %w", id, err)
		}

		// Store updated node
		if err := nodesBucket.Put([]byte(id), updatedNodeData); err != nil {
			return fmt.Errorf("[SpectraFS] failed to update existence map for %s: %w", id, err)
		}

		return nil
	})
}

// DeleteAllNodes removes all nodes from the nodes bucket and all indexes (for Reset)
func (db *DB) DeleteAllNodes() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	return db.db.Update(func(tx *bbolt.Tx) error {
		// Delete all nodes from nodes bucket
		nodesBucket := tx.Bucket([]byte(bucketNodes))
		if nodesBucket != nil {
			cursor := nodesBucket.Cursor()
			for key, _ := cursor.First(); key != nil; key, _ = cursor.Next() {
				if err := nodesBucket.Delete(key); err != nil {
					return fmt.Errorf("[SpectraFS] failed to delete node: %w", err)
				}
			}
		}

		// Delete all entries from index buckets
		indexBuckets := []string{bucketIndexParentID, bucketIndexPath, bucketIndexParentPath}
		for _, bucketName := range indexBuckets {
			bucket := tx.Bucket([]byte(bucketName))
			if bucket != nil {
				cursor := bucket.Cursor()
				for key, _ := cursor.First(); key != nil; key, _ = cursor.Next() {
					if err := bucket.Delete(key); err != nil {
						return fmt.Errorf("[SpectraFS] failed to delete from index %s: %w", bucketName, err)
					}
				}
			}
		}

		return nil
	})
}

// GetNodeCount returns the total number of nodes in a specific world
func (db *DB) GetNodeCount(world string) (int, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	var count int
	err := db.db.View(func(tx *bbolt.Tx) error {
		nodesBucket := tx.Bucket([]byte(bucketNodes))
		if nodesBucket == nil {
			return fmt.Errorf("[SpectraFS] nodes bucket does not exist")
		}

		cursor := nodesBucket.Cursor()
		for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
			var node types.Node
			if err := json.Unmarshal(value, &node); err != nil {
				continue // Skip on error
			}

			// Count if node exists in the specified world
			if node.ExistenceMap[world] {
				count++
			}
		}

		return nil
	})

	if err != nil {
		return 0, fmt.Errorf("[SpectraFS] failed to get node count for world %s: %w", world, err)
	}

	return count, nil
}

// GetTableInfo returns information about all worlds
func (db *DB) GetTableInfo() ([]types.TableInfo, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	var tables []types.TableInfo

	// Get counts for all worlds in a single pass
	worldCounts := make(map[string]int)
	worldCounts["primary"] = 0
	for _, worldName := range db.secondaryTables {
		worldCounts[worldName] = 0
	}

	err := db.db.View(func(tx *bbolt.Tx) error {
		nodesBucket := tx.Bucket([]byte(bucketNodes))
		if nodesBucket == nil {
			return fmt.Errorf("[SpectraFS] nodes bucket does not exist")
		}

		cursor := nodesBucket.Cursor()
		for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
			var node types.Node
			if err := json.Unmarshal(value, &node); err != nil {
				continue // Skip on error
			}

			// Count node in each world it exists in
			for world := range worldCounts {
				if node.ExistenceMap[world] {
					worldCounts[world]++
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("[SpectraFS] failed to get table info: %w", err)
	}

	// Add primary world
	tables = append(tables, types.TableInfo{
		Name:      "primary",
		RowCount:  worldCounts["primary"],
		TableType: "primary",
	})

	// Add secondary worlds
	for _, worldName := range db.secondaryTables {
		tables = append(tables, types.TableInfo{
			Name:      worldName,
			RowCount:  worldCounts[worldName],
			TableType: "secondary",
		})
	}

	return tables, nil
}

// CreateFolder creates a new folder node
func (db *DB) CreateFolder(parentID, name string, depth int) (*types.Node, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Generate UUID for the new folder
	nodeID := uuid.New().String()

	// Get parent node to determine path
	var parentPath string
	err := db.db.View(func(tx *bbolt.Tx) error {
		nodesBucket := tx.Bucket([]byte(bucketNodes))
		if nodesBucket == nil {
			return fmt.Errorf("[SpectraFS] nodes bucket does not exist")
		}

		parentData := nodesBucket.Get([]byte(parentID))
		if parentData == nil {
			return fmt.Errorf("[SpectraFS] parent node not found: %s", parentID)
		}

		var parent types.Node
		if err := json.Unmarshal(parentData, &parent); err != nil {
			return fmt.Errorf("[SpectraFS] failed to unmarshal parent node: %w", err)
		}

		parentPath = parent.Path
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("[SpectraFS] failed to get parent node: %w", err)
	}

	path := utils.JoinPath(parentPath, name)

	folderNode := &types.Node{
		ID:           nodeID,
		ParentID:     parentID,
		Name:         name,
		Path:         path,
		Type:         types.NodeTypeFolder,
		DepthLevel:   depth,
		Size:         0, // Folders have size 0
		LastUpdated:  time.Now(),
		Checksum:     nil, // Folders don't have checksums
		ExistenceMap: make(map[string]bool),
	}

	return folderNode, nil
}

// CreateRootNode creates a single root node with existence in all worlds
// This function is idempotent - it will skip creating the node if it already exists
func (db *DB) CreateRootNode() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	return db.db.Update(func(tx *bbolt.Tx) error {
		nodesBucket := tx.Bucket([]byte(bucketNodes))
		if nodesBucket == nil {
			return fmt.Errorf("[SpectraFS] nodes bucket does not exist")
		}

		// Check if root already exists
		rootData := nodesBucket.Get([]byte("root"))
		if rootData != nil {
			// Root already exists
			return nil
		}

		// Create existence map with all worlds
		existenceMap := make(map[string]bool)
		existenceMap["primary"] = true
		for _, worldName := range db.secondaryTables {
			existenceMap[worldName] = true
		}

		// Create root node
		rootNode := &types.Node{
			ID:           "root",
			ParentID:     "",
			Name:         "root",
			Path:         "/",
			ParentPath:   "",
			Type:         types.NodeTypeFolder,
			DepthLevel:   0,
			Size:         0,
			LastUpdated:  time.Now(),
			Checksum:     nil,
			ExistenceMap: existenceMap,
		}

		// Serialize node to JSON
		nodeJSON, err := json.Marshal(rootNode)
		if err != nil {
			return fmt.Errorf("[SpectraFS] failed to marshal root node: %w", err)
		}

		// Store node in nodes bucket
		if err := nodesBucket.Put([]byte("root"), nodeJSON); err != nil {
			return fmt.Errorf("[SpectraFS] failed to create root node: %w", err)
		}

		// Update index_parent_id: key format "{parentID}|{nodeID}"
		indexParentID := tx.Bucket([]byte(bucketIndexParentID))
		if indexParentID != nil {
			parentIDKey := fmt.Sprintf("%s|%s", rootNode.ParentID, rootNode.ID)
			if err := indexParentID.Put([]byte(parentIDKey), []byte{}); err != nil {
				return fmt.Errorf("[SpectraFS] failed to update parent_id index: %w", err)
			}
		}

		// Update index_path: key format "{path}" -> value "{nodeID}"
		indexPath := tx.Bucket([]byte(bucketIndexPath))
		if indexPath != nil {
			if err := indexPath.Put([]byte(rootNode.Path), []byte(rootNode.ID)); err != nil {
				return fmt.Errorf("[SpectraFS] failed to update path index: %w", err)
			}
		}

		// Update index_parent_path: key format "{parentPath}|{nodeID}"
		indexParentPath := tx.Bucket([]byte(bucketIndexParentPath))
		if indexParentPath != nil {
			parentPathKey := fmt.Sprintf("%s|%s", rootNode.ParentPath, rootNode.ID)
			if err := indexParentPath.Put([]byte(parentPathKey), []byte{}); err != nil {
				return fmt.Errorf("[SpectraFS] failed to update parent_path index: %w", err)
			}
		}

		return nil
	})
}

// DeleteNode deletes a node from the nodes bucket and all indexes
func (db *DB) DeleteNode(id string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	return db.db.Update(func(tx *bbolt.Tx) error {
		// First, get the node to retrieve its path and parent info for index cleanup
		nodesBucket := tx.Bucket([]byte(bucketNodes))
		if nodesBucket == nil {
			return fmt.Errorf("[SpectraFS] nodes bucket does not exist")
		}

		nodeData := nodesBucket.Get([]byte(id))
		if nodeData == nil {
			return fmt.Errorf("[SpectraFS] node not found: %s", id)
		}

		var node types.Node
		if err := json.Unmarshal(nodeData, &node); err != nil {
			return fmt.Errorf("[SpectraFS] failed to unmarshal node %s: %w", id, err)
		}

		// Delete from nodes bucket
		if err := nodesBucket.Delete([]byte(id)); err != nil {
			return fmt.Errorf("[SpectraFS] failed to delete node %s: %w", id, err)
		}

		// Delete from index_parent_id
		indexParentID := tx.Bucket([]byte(bucketIndexParentID))
		if indexParentID != nil {
			parentIDKey := fmt.Sprintf("%s|%s", node.ParentID, node.ID)
			if err := indexParentID.Delete([]byte(parentIDKey)); err != nil {
				return fmt.Errorf("[SpectraFS] failed to delete from parent_id index: %w", err)
			}
		}

		// Delete from index_path
		indexPath := tx.Bucket([]byte(bucketIndexPath))
		if indexPath != nil {
			if err := indexPath.Delete([]byte(node.Path)); err != nil {
				return fmt.Errorf("[SpectraFS] failed to delete from path index: %w", err)
			}
		}

		// Delete from index_parent_path
		indexParentPath := tx.Bucket([]byte(bucketIndexParentPath))
		if indexParentPath != nil {
			parentPathKey := fmt.Sprintf("%s|%s", node.ParentPath, node.ID)
			if err := indexParentPath.Delete([]byte(parentPathKey)); err != nil {
				return fmt.Errorf("[SpectraFS] failed to delete from parent_path index: %w", err)
			}
		}

		return nil
	})
}

// GetSecondaryTables returns the list of secondary world names
func (db *DB) GetSecondaryTables() []string {
	return db.secondaryTables
}

// Note: ParentInfo and GetParentInfo removed - replaced by GetParentAndChildren for better performance

// BulkInsertNodes inserts multiple nodes in a single BoltDB transaction
func (db *DB) BulkInsertNodes(nodes []*types.Node) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if len(nodes) == 0 {
		return nil
	}

	return db.db.Update(func(tx *bbolt.Tx) error {
		nodesBucket := tx.Bucket([]byte(bucketNodes))
		if nodesBucket == nil {
			return fmt.Errorf("[SpectraFS] nodes bucket does not exist")
		}

		indexParentID := tx.Bucket([]byte(bucketIndexParentID))
		if indexParentID == nil {
			return fmt.Errorf("[SpectraFS] index_parent_id bucket does not exist")
		}

		indexPath := tx.Bucket([]byte(bucketIndexPath))
		if indexPath == nil {
			return fmt.Errorf("[SpectraFS] index_path bucket does not exist")
		}

		indexParentPath := tx.Bucket([]byte(bucketIndexParentPath))
		if indexParentPath == nil {
			return fmt.Errorf("[SpectraFS] index_parent_path bucket does not exist")
		}

		// Insert all nodes
		for _, node := range nodes {
			// Check if node already exists (INSERT OR IGNORE behavior)
			existingData := nodesBucket.Get([]byte(node.ID))
			if existingData != nil {
				continue // Skip if node already exists
			}

			// Serialize node to JSON
			nodeJSON, err := json.Marshal(node)
			if err != nil {
				return fmt.Errorf("[SpectraFS] failed to marshal node %s: %w", node.ID, err)
			}

			// Store node in nodes bucket
			if err := nodesBucket.Put([]byte(node.ID), nodeJSON); err != nil {
				return fmt.Errorf("[SpectraFS] failed to insert node %s: %w", node.ID, err)
			}

			// Update index_parent_id: key format "{parentID}|{nodeID}"
			parentIDKey := fmt.Sprintf("%s|%s", node.ParentID, node.ID)
			if err := indexParentID.Put([]byte(parentIDKey), []byte{}); err != nil {
				return fmt.Errorf("[SpectraFS] failed to update parent_id index for node %s: %w", node.ID, err)
			}

			// Update index_path: key format "{path}" -> value "{nodeID}"
			if err := indexPath.Put([]byte(node.Path), []byte(node.ID)); err != nil {
				return fmt.Errorf("[SpectraFS] failed to update path index for node %s: %w", node.ID, err)
			}

			// Update index_parent_path: key format "{parentPath}|{nodeID}"
			parentPathKey := fmt.Sprintf("%s|%s", node.ParentPath, node.ID)
			if err := indexParentPath.Put([]byte(parentPathKey), []byte{}); err != nil {
				return fmt.Errorf("[SpectraFS] failed to update parent_path index for node %s: %w", node.ID, err)
			}
		}

		return nil
	})
}

// GetNodeByPath retrieves a node by its path, optionally filtering by world
func (db *DB) GetNodeByPath(path, world string) (*types.Node, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	var node *types.Node
	err := db.db.View(func(tx *bbolt.Tx) error {
		// Use index_path bucket to get nodeID from path
		indexPath := tx.Bucket([]byte(bucketIndexPath))
		if indexPath == nil {
			return fmt.Errorf("[SpectraFS] index_path bucket does not exist")
		}

		nodeIDBytes := indexPath.Get([]byte(path))
		if nodeIDBytes == nil {
			return fmt.Errorf("[SpectraFS] node not found with path %s", path)
		}

		nodeID := string(nodeIDBytes)

		// Get node from nodes bucket
		nodesBucket := tx.Bucket([]byte(bucketNodes))
		if nodesBucket == nil {
			return fmt.Errorf("[SpectraFS] nodes bucket does not exist")
		}

		nodeData := nodesBucket.Get([]byte(nodeID))
		if nodeData == nil {
			return fmt.Errorf("[SpectraFS] node not found with path %s", path)
		}

		node = &types.Node{}
		if err := json.Unmarshal(nodeData, node); err != nil {
			return fmt.Errorf("[SpectraFS] failed to unmarshal node %s: %w", nodeID, err)
		}

		// Filter by world if specified
		if world != "" && !node.ExistenceMap[world] {
			return fmt.Errorf("[SpectraFS] node not found with path %s in world %s", path, world)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return node, nil
}
