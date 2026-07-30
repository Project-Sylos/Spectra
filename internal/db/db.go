package db

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"codeberg.org/Sylos/Spectra/internal/types"
	"codeberg.org/Sylos/Spectra/internal/utils"
	"go.etcd.io/bbolt"
)

// statsUpdate represents a single stats update operation
type statsUpdate struct {
	node      *types.Node
	increment bool
}

// StatsBuffer handles asynchronous batched stats updates
type StatsBuffer struct {
	updates       chan statsUpdate
	forceFlush    chan struct{}
	done          chan struct{}
	buffer        []statsUpdate
	bufferSize    int
	flushInterval time.Duration
	mu            sync.Mutex
	db            *DB
}

// DB wraps BoltDB connection and provides key-value CRUD operations
type DB struct {
	db              *bbolt.DB
	secondaryTables []string      // List of secondary world names (e.g., ["s1", "s2"])
	statsBuffer     *StatsBuffer  // Async stats buffer
	outputBuffer    *OutputBuffer // Buffered write operations
	nodeCache       *NodeCache    // Optional sliding window cache
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

	// Initialize stats buffer
	db.statsBuffer = newStatsBuffer(db, 100, 500*time.Millisecond)
	db.statsBuffer.start()

	// Initialize output buffer (batch size 10K, flush every 5 seconds)
	db.outputBuffer = NewOutputBuffer(db, 10000, 5*time.Second)

	// Node cache disabled (removed from config)
	db.nodeCache = NewNodeCache(false)

	return db, nil
}

// VerifyAndInitialize performs comprehensive database verification and initialization
// It checks each stage and creates what's missing:
// A) Database file exists (checked before connection)
// B) Buckets exist
// C) Root node exists
// D) Stats are initialized
// This is a glue function - it does not lock, but calls functions that handle their own locking
func (db *DB) VerifyAndInitialize(dbFileExists bool, secondaryTables map[string]float64) error {
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

	// C) Check if root node exists (rootNodeExists handles its own locking)
	rootExists, err := db.rootNodeExists()
	if err != nil {
		return fmt.Errorf("failed to check if root node exists: %w", err)
	}

	if !rootExists {
		// Create root node (createRootNodeInternal handles its own locking)
		if err := db.createRootNodeInternal(); err != nil {
			return fmt.Errorf("failed to create root node: %w", err)
		}
	}

	// D) Initialize stats if needed (initializeStats calls getStats/setStats which handle locking)
	if err := db.initializeStats(); err != nil {
		return fmt.Errorf("failed to initialize stats: %w", err)
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

// createRootNodeInternal creates the root node
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
		ChildIDs:     []string{}, // Initialize empty ChildIDs array
	}

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
	// Force flush output buffer before shutdown to ensure all operations are persisted
	if db.outputBuffer != nil {
		db.outputBuffer.Flush() // Synchronous flush
		db.outputBuffer.Stop()  // Then stop the background goroutine
	}
	// Shutdown stats buffer and force flush
	if db.statsBuffer != nil {
		db.statsBuffer.shutdown()
	}
	return db.db.Close()
}

// InsertNode inserts a new node into the nodes bucket and updates all indexes
func (db *DB) InsertNode(node *types.Node) error {
	// Add to output buffer
	op := &InsertNodeOperation{Node: node}
	db.outputBuffer.Add(op)

	// Queue stats update (async, non-blocking)
	if db.statsBuffer != nil {
		db.statsBuffer.queueUpdate(node, true)
	}

	return nil
}

// GetNodeByID retrieves a node by its ID from the nodes bucket
func (db *DB) GetNodeByID(id string) (*types.Node, error) {
	// Check cache first (if enabled)
	if node, ok := db.nodeCache.Get(id); ok {
		return node, nil
	}

	// Flush buffer if this node has pending writes
	db.Flush(id)

	// BoltDB handles its own read locking
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

	// Don't add to cache on miss (per requirement)
	return node, nil
}

// GetChildrenByParentID retrieves all children of a parent node filtered by world
func (db *DB) GetChildrenByParentID(parentID, world string) ([]*types.Node, error) {
	// Flush buffer if this node has pending writes
	db.Flush(parentID)

	// BoltDB handles its own read locking
	var children []*types.Node
	err := db.db.View(func(tx *bbolt.Tx) error {
		nodesBucket := tx.Bucket([]byte(bucketNodes))
		if nodesBucket == nil {
			return fmt.Errorf("[SpectraFS] nodes bucket does not exist")
		}

		// Get parent node to retrieve ChildIDs
		parentData := nodesBucket.Get([]byte(parentID))
		if parentData == nil {
			return fmt.Errorf("[SpectraFS] parent node not found: %s", parentID)
		}

		var parent types.Node
		if err := json.Unmarshal(parentData, &parent); err != nil {
			return fmt.Errorf("[SpectraFS] failed to unmarshal parent node %s: %w", parentID, err)
		}

		// Use parent's ChildIDs for O(1) lookup
		if parent.ChildIDs == nil {
			return nil // No children
		}

		// Get each child node by ID
		for _, childID := range parent.ChildIDs {
			nodeData := nodesBucket.Get([]byte(childID))
			if nodeData == nil {
				continue // Skip if node not found (shouldn't happen)
			}

			var node types.Node
			if err := json.Unmarshal(nodeData, &node); err != nil {
				return fmt.Errorf("[SpectraFS] failed to unmarshal node %s: %w", childID, err)
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
	// Flush buffer if this node has pending writes
	db.Flush(parentID)

	// BoltDB handles its own read locking
	var nodes []*types.Node
	err := db.db.View(func(tx *bbolt.Tx) error {
		nodesBucket := tx.Bucket([]byte(bucketNodes))
		if nodesBucket == nil {
			return fmt.Errorf("[SpectraFS] nodes bucket does not exist")
		}

		// Get parent node
		parentData := nodesBucket.Get([]byte(parentID))
		if parentData == nil {
			return fmt.Errorf("[SpectraFS] parent node not found: %s", parentID)
		}

		var parent types.Node
		if err := json.Unmarshal(parentData, &parent); err != nil {
			return fmt.Errorf("[SpectraFS] failed to unmarshal parent node %s: %w", parentID, err)
		}

		// Add parent if it exists in the world
		if parent.ExistenceMap[world] {
			nodes = append(nodes, &parent)
		}

		// Get children using parent's ChildIDs array (O(1) per child)
		if parent.ChildIDs != nil {
			for _, childID := range parent.ChildIDs {
				nodeData := nodesBucket.Get([]byte(childID))
				if nodeData == nil {
					continue // Skip if node not found (shouldn't happen)
				}

				var node types.Node
				if err := json.Unmarshal(nodeData, &node); err != nil {
					return fmt.Errorf("[SpectraFS] failed to unmarshal node %s: %w", childID, err)
				}

				// Filter by world - check existence map
				if node.ExistenceMap[world] {
					nodes = append(nodes, &node)
				}
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
	// BoltDB handles its own read locking
	var hasChildren bool
	err := db.db.View(func(tx *bbolt.Tx) error {
		nodesBucket := tx.Bucket([]byte(bucketNodes))
		if nodesBucket == nil {
			return fmt.Errorf("[SpectraFS] nodes bucket does not exist")
		}

		// Get parent node to check ChildIDs
		parentData := nodesBucket.Get([]byte(parentID))
		if parentData == nil {
			return fmt.Errorf("[SpectraFS] parent node not found: %s", parentID)
		}

		var parent types.Node
		if err := json.Unmarshal(parentData, &parent); err != nil {
			return fmt.Errorf("[SpectraFS] failed to unmarshal parent node %s: %w", parentID, err)
		}

		// Check if any child exists in the specified world
		if parent.ChildIDs != nil {
			for _, childID := range parent.ChildIDs {
				nodeData := nodesBucket.Get([]byte(childID))
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
	// Add to output buffer
	op := &UpdateExistenceMapOperation{
		NodeID:       id,
		ExistenceMap: existenceMap,
	}
	db.outputBuffer.Add(op)
	return nil
}

// DeleteAllNodes removes all nodes from the nodes bucket and all indexes (for Reset)
func (db *DB) DeleteAllNodes() error {
	// BoltDB handles its own write locking
	err := db.db.Update(func(tx *bbolt.Tx) error {
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
		indexBuckets := []string{bucketIndexPath, bucketIndexParentPath}
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

	// Reset stats after successful deletion
	if err == nil {
		if err := db.resetStats(); err != nil {
			// Log error but don't fail the reset
			// Stats update failure shouldn't prevent reset
			_ = err
		}
	}

	return err
}

// GetNodeCount returns the total number of nodes in a specific world
func (db *DB) GetNodeCount(world string) (int, error) {
	// BoltDB handles its own read locking
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
	// BoltDB handles its own read locking
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
	// Get parent node to determine path (BoltDB handles its own read locking)
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
	nodeID := utils.DeterministicNodeID(path, types.NodeTypeFolder)

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
		ChildIDs:     []string{}, // Initialize empty ChildIDs array
	}

	return folderNode, nil
}

// CreateRootNode creates a single root node with existence in all worlds
// This function is idempotent - it will skip creating the node if it already exists
func (db *DB) CreateRootNode() error {
	// BoltDB handles its own write locking
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
			ChildIDs:     []string{}, // Initialize empty ChildIDs array
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
	// Get node first for stats update (before deletion)
	// BoltDB handles its own read locking
	var node *types.Node
	err := db.db.View(func(tx *bbolt.Tx) error {
		nodesBucket := tx.Bucket([]byte(bucketNodes))
		if nodesBucket == nil {
			return fmt.Errorf("nodes bucket does not exist")
		}
		nodeData := nodesBucket.Get([]byte(id))
		if nodeData == nil {
			return fmt.Errorf("node not found: %s", id)
		}
		node = &types.Node{}
		return json.Unmarshal(nodeData, node)
	})

	// Add to output buffer (buffer handles its own locking)
	op := &DeleteNodeOperation{
		NodeID:   id,
		ParentID: node.ParentID, // Include parent ID for ChildIDs update tracking
	}
	db.outputBuffer.Add(op)

	// Queue stats update (async, non-blocking)
	if err == nil && db.statsBuffer != nil && node != nil {
		db.statsBuffer.queueUpdate(node, false)
	}

	return err
}

// GetSecondaryTables returns the list of secondary world names
func (db *DB) GetSecondaryTables() []string {
	return db.secondaryTables
}

// newStatsBuffer creates a new stats buffer
func newStatsBuffer(db *DB, bufferSize int, flushInterval time.Duration) *StatsBuffer {
	return &StatsBuffer{
		updates:       make(chan statsUpdate, 1000), // Buffered channel for non-blocking sends
		forceFlush:    make(chan struct{}, 1),
		done:          make(chan struct{}),
		buffer:        make([]statsUpdate, 0, bufferSize),
		bufferSize:    bufferSize,
		flushInterval: flushInterval,
		db:            db,
	}
}

// start begins the async buffer processing
func (sb *StatsBuffer) start() {
	go sb.processLoop()
}

// processLoop is the main loop that processes stats updates
func (sb *StatsBuffer) processLoop() {
	ticker := time.NewTicker(sb.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case update := <-sb.updates:
			// Add to buffer
			sb.mu.Lock()
			sb.buffer = append(sb.buffer, update)
			shouldFlush := len(sb.buffer) >= sb.bufferSize
			sb.mu.Unlock()

			// Flush if buffer is full
			if shouldFlush {
				sb.flush()
			}

		case <-ticker.C:
			// Time-based flush
			sb.flush()

		case <-sb.forceFlush:
			// Manual flush requested
			sb.flush()

		case <-sb.done:
			// Shutdown - flush remaining updates and exit
			sb.flush()
			return
		}
	}
}

// flush processes all buffered updates
func (sb *StatsBuffer) flush() {
	sb.mu.Lock()
	if len(sb.buffer) == 0 {
		sb.mu.Unlock()
		return
	}

	// Take snapshot and clear buffer
	snapshot := make([]statsUpdate, len(sb.buffer))
	copy(snapshot, sb.buffer)
	sb.buffer = sb.buffer[:0] // Clear buffer but keep capacity
	sb.mu.Unlock()

	// Process snapshot asynchronously (don't block incoming updates)
	go sb.processSnapshot(snapshot)
}

// processSnapshot applies batched stats updates
func (sb *StatsBuffer) processSnapshot(updates []statsUpdate) {
	if len(updates) == 0 {
		return
	}

	// Separate into increment and decrement nodes
	incrementNodes := make([]*types.Node, 0, len(updates))
	decrementNodes := make([]*types.Node, 0, len(updates))

	for _, update := range updates {
		if update.increment {
			incrementNodes = append(incrementNodes, update.node)
		} else {
			decrementNodes = append(decrementNodes, update.node)
		}
	}

	// Apply increments
	if len(incrementNodes) > 0 {
		if err := sb.db.updateStatsForNodes(incrementNodes, true); err != nil {
			// Log error but don't fail - stats are non-critical
			_ = err
		}
	}

	// Apply decrements
	if len(decrementNodes) > 0 {
		if err := sb.db.updateStatsForNodes(decrementNodes, false); err != nil {
			// Log error but don't fail - stats are non-critical
			_ = err
		}
	}
}

// queueUpdate adds a stats update to the buffer (non-blocking)
func (sb *StatsBuffer) queueUpdate(node *types.Node, increment bool) {
	select {
	case sb.updates <- statsUpdate{node: node, increment: increment}:
		// Successfully queued
	default:
		// Channel full - drop update (stats are non-critical)
	}
}

// Flush forces an immediate flush of the buffer
func (sb *StatsBuffer) Flush() {
	select {
	case sb.forceFlush <- struct{}{}:
	default:
		// Flush already in progress
	}
}

// shutdown stops the buffer and flushes remaining updates
func (sb *StatsBuffer) shutdown() {
	close(sb.done)
	// Give it a moment to flush
	time.Sleep(100 * time.Millisecond)
}

// GetStats retrieves stats from the database
func (db *DB) GetStats() (*types.Stats, error) {
	// BoltDB handles its own read locking
	var stats *types.Stats
	err := db.db.View(func(tx *bbolt.Tx) error {
		statsBucket := tx.Bucket([]byte(bucketStats))
		if statsBucket == nil {
			return fmt.Errorf("[SpectraFS] stats bucket does not exist")
		}

		statsData := statsBucket.Get([]byte("global"))
		if statsData == nil {
			// Stats not initialized, return zero stats
			stats = &types.Stats{
				FileCount:      0,
				FolderCount:    0,
				TotalFileSize:  0,
				SecondaryNodes: make(map[string]int64),
			}
			// Initialize secondary nodes map
			for _, worldName := range db.secondaryTables {
				stats.SecondaryNodes[worldName] = 0
			}
			return nil
		}

		stats = &types.Stats{}
		if err := json.Unmarshal(statsData, stats); err != nil {
			return fmt.Errorf("[SpectraFS] failed to unmarshal stats: %w", err)
		}

		// Ensure SecondaryNodes map is initialized
		if stats.SecondaryNodes == nil {
			stats.SecondaryNodes = make(map[string]int64)
		}

		// Ensure all secondary worlds are in the map
		for _, worldName := range db.secondaryTables {
			if _, exists := stats.SecondaryNodes[worldName]; !exists {
				stats.SecondaryNodes[worldName] = 0
			}
		}

		return nil
	})

	return stats, err
}

// setStats saves stats to the database
func (db *DB) setStats(stats *types.Stats) error {
	// BoltDB handles its own write locking
	return db.db.Update(func(tx *bbolt.Tx) error {
		statsBucket := tx.Bucket([]byte(bucketStats))
		if statsBucket == nil {
			return fmt.Errorf("[SpectraFS] stats bucket does not exist")
		}

		statsJSON, err := json.Marshal(stats)
		if err != nil {
			return fmt.Errorf("[SpectraFS] failed to marshal stats: %w", err)
		}

		if err := statsBucket.Put([]byte("global"), statsJSON); err != nil {
			return fmt.Errorf("[SpectraFS] failed to save stats: %w", err)
		}

		return nil
	})
}

// initializeStats initializes the stats bucket with zero values
func (db *DB) initializeStats() error {
	// Check if stats already exist
	existingStats, err := db.GetStats()
	if err != nil {
		return err
	}

	// If stats already exist (not all zeros), don't reinitialize
	if existingStats.FileCount != 0 || existingStats.FolderCount != 0 || existingStats.TotalFileSize != 0 {
		return nil
	}

	// Initialize with zero values
	stats := &types.Stats{
		FileCount:      0,
		FolderCount:    0,
		TotalFileSize:  0,
		SecondaryNodes: make(map[string]int64),
	}

	// Initialize secondary nodes map for each secondary world
	for _, worldName := range db.secondaryTables {
		stats.SecondaryNodes[worldName] = 0
	}

	return db.setStats(stats)
}

// updateStatsForNodes batch updates stats for multiple nodes (optimized for bulk operations)
func (db *DB) updateStatsForNodes(nodes []*types.Node, increment bool) error {
	if len(nodes) == 0 {
		return nil
	}

	// Get current stats once (getStats handles locking)
	stats, err := db.GetStats()
	if err != nil {
		return err
	}

	delta := int64(1)
	if !increment {
		delta = -1
	}

	// Update stats for all nodes
	for _, node := range nodes {
		switch node.Type {
		case types.NodeTypeFile:
			stats.FileCount += delta
			if increment {
				stats.TotalFileSize += node.Size
			} else {
				stats.TotalFileSize -= node.Size
			}
		case types.NodeTypeFolder:
			stats.FolderCount += delta
		}

		// Update secondary node counts for each world
		for worldName := range stats.SecondaryNodes {
			if node.ExistenceMap[worldName] {
				stats.SecondaryNodes[worldName] += delta
			}
		}
	}

	// Ensure no negative values
	if stats.FileCount < 0 {
		stats.FileCount = 0
	}
	if stats.FolderCount < 0 {
		stats.FolderCount = 0
	}
	if stats.TotalFileSize < 0 {
		stats.TotalFileSize = 0
	}
	for worldName := range stats.SecondaryNodes {
		if stats.SecondaryNodes[worldName] < 0 {
			stats.SecondaryNodes[worldName] = 0
		}
	}

	// Ensure all secondary worlds are in the map
	for _, worldName := range db.secondaryTables {
		if _, exists := stats.SecondaryNodes[worldName]; !exists {
			stats.SecondaryNodes[worldName] = 0
		}
	}

	// Save updated stats once (setStats handles locking)
	return db.setStats(stats)
}

// FlushStats forces an immediate flush of the stats buffer
func (db *DB) FlushStats() {
	if db.statsBuffer != nil {
		db.statsBuffer.Flush()
		// Wait a moment for flush to complete
		time.Sleep(50 * time.Millisecond)
	}
}

// Flush forces an immediate flush of the output buffer.
// If nodeID is provided, only flushes if that node has pending writes.
// If nodeID is empty, always flushes.
func (db *DB) Flush(nodeIDs ...string) bool {
	if db.outputBuffer == nil {
		return false
	}

	// If nodeIDs provided, check if ANY have pending writes
	if len(nodeIDs) > 0 {
		hasPending := false
		for _, nodeID := range nodeIDs {
			if nodeID != "" && db.outputBuffer.HasPendingWrites(nodeID) {
				hasPending = true
				break // Short-circuit on first match
			}
		}
		if !hasPending {
			return false // No flush needed
		}
	}

	db.outputBuffer.Flush()
	return true // Flush occurred
}

// AddToCache adds a node to the cache (if cache is enabled)
func (db *DB) AddToCache(node *types.Node) {
	if db.nodeCache != nil {
		db.nodeCache.Set(node)
	}
}

// EvictOldGenerations performs safe eviction of old nodes from the cache
// Flushes buffer for any nodes that need to be evicted to ensure data durability
func (db *DB) EvictOldGenerations() {
	if db.nodeCache == nil {
		return
	}

	// Get nodes that should be evicted
	toEvict := db.nodeCache.GetNodesForEviction()
	if len(toEvict) == 0 {
		return
	}

	// Flush buffer if any of these nodes have pending writes
	db.Flush(toEvict...)

	// Now safe to evict from cache
	db.nodeCache.EvictNodes(toEvict)
}

// resetStats resets all stats to zero
func (db *DB) resetStats() error {
	// Reset to zero values
	stats := &types.Stats{
		FileCount:      0,
		FolderCount:    0,
		TotalFileSize:  0,
		SecondaryNodes: make(map[string]int64),
	}

	// Initialize secondary nodes map for each secondary world
	for _, worldName := range db.secondaryTables {
		stats.SecondaryNodes[worldName] = 0
	}

	return db.setStats(stats)
}

// Note: ParentInfo and GetParentInfo removed - replaced by GetParentAndChildren for better performance

// BulkInsertNodes inserts multiple nodes in a single BoltDB transaction
func (db *DB) BulkInsertNodes(nodes []*types.Node) error {
	if len(nodes) == 0 {
		return nil
	}

	// Add to output buffer
	op := &BulkInsertNodeOperation{Nodes: nodes}
	db.outputBuffer.Add(op)

	// Queue stats updates (async, non-blocking)
	if db.statsBuffer != nil {
		for _, node := range nodes {
			db.statsBuffer.queueUpdate(node, true)
		}
	}

	return nil
}

// GetNodeByPath retrieves a node by its path, optionally filtering by world
func (db *DB) GetNodeByPath(path, world string) (*types.Node, error) {
	// First, get the nodeID from the path index so we can flush pending operations
	// BoltDB handles its own read locking
	var nodeID string
	err := db.db.View(func(tx *bbolt.Tx) error {
		indexPath := tx.Bucket([]byte(bucketIndexPath))
		if indexPath == nil {
			return fmt.Errorf("[SpectraFS] index_path bucket does not exist")
		}
		nodeIDBytes := indexPath.Get([]byte(path))
		if nodeIDBytes == nil {
			return fmt.Errorf("[SpectraFS] node not found with path %s", path)
		}
		nodeID = string(nodeIDBytes)
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Flush buffer if this node has pending writes
	db.Flush(nodeID)

	// Now do the full read (BoltDB handles its own read locking)
	var node *types.Node
	err = db.db.View(func(tx *bbolt.Tx) error {

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
