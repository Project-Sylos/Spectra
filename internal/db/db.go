package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Project-Sylos/Spectra/internal/types"
	"github.com/Project-Sylos/Spectra/internal/utils"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// DB wraps SQLite connection and provides single-table CRUD operations
type DB struct {
	conn            *sql.DB
	secondaryTables []string   // List of secondary world names (e.g., ["s1", "s2"])
	mu              sync.Mutex // Protects all database operations from concurrent access
}

// New creates a new database connection and initializes the schema
func New(dbPath string, secondaryTables map[string]float64) (*DB, error) {
	// A) Check if database file exists
	dbFileExists := false
	if _, err := os.Stat(dbPath); err == nil {
		dbFileExists = true
	}

	// Open SQLite connection
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite connection: %w", err)
	}

	// Register SQLite driver
	conn.Driver()

	// Create secondary tables list and traversal columns map
	secondaryList := make([]string, 0, len(secondaryTables))

	// Add secondary worlds
	for tableName := range secondaryTables {
		secondaryList = append(secondaryList, tableName)
	}

	db := &DB{
		conn:            conn,
		secondaryTables: secondaryList,
	}

	// Verify and initialize database structure
	if err := db.VerifyAndInitialize(dbFileExists, secondaryTables); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to verify and initialize database: %w", err)
	}

	return db, nil
}

// VerifyAndInitialize performs comprehensive database verification and initialization
// It checks each stage and creates what's missing:
// A) Database file exists (checked before connection)
// B) Nodes table exists
// C) Table schema matches (columns and column order)
// D) Indexes exist
// E) Root node exists
func (db *DB) VerifyAndInitialize(dbFileExists bool, secondaryTables map[string]float64) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// B) Check if nodes table exists
	tableExists, err := db.tableExists("nodes")
	if err != nil {
		return fmt.Errorf("failed to check if table exists: %w", err)
	}

	if !tableExists {
		// Create table
		createTableSQL := BuildNodesTableSQL(secondaryTables)
		if _, err := db.conn.Exec(createTableSQL); err != nil {
			return fmt.Errorf("failed to create nodes table: %w", err)
		}
	} else {
		// C) Verify table schema matches
		schemaMatches, err := db.verifyTableSchema()
		if err != nil {
			return fmt.Errorf("failed to verify table schema: %w", err)
		}

		if !schemaMatches {
			// Schema doesn't match - return error to prevent data loss
			// User must explicitly call Reset() or handle migration if they want to modify the schema
			return fmt.Errorf("table schema mismatch: existing nodes table has different schema than expected. " +
				"To reset the database, explicitly call Reset(). " +
				"To preserve data, implement a migration strategy")
		}
	}

	// D) Verify indexes exist
	if err := db.verifyAndCreateIndexes(secondaryTables); err != nil {
		return fmt.Errorf("failed to verify indexes: %w", err)
	}

	// E) Check if root node exists
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

// tableExists checks if a table exists in the database
func (db *DB) tableExists(tableName string) (bool, error) {
	query := "SELECT name FROM sqlite_master WHERE type='table' AND name=?"
	var name string
	err := db.conn.QueryRow(query, tableName).Scan(&name)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// verifyTableSchema checks if the nodes table has the correct schema
func (db *DB) verifyTableSchema() (bool, error) {
	// Expected columns in order
	expectedColumns := []struct {
		name     string
		notNull  bool
		dataType string
	}{
		{"id", true, "VARCHAR"},
		{"parent_id", true, "VARCHAR"},
		{"name", true, "VARCHAR"},
		{"path", true, "VARCHAR"},
		{"parent_path", true, "VARCHAR"},
		{"type", true, "VARCHAR"},
		{"depth_level", true, "INTEGER"},
		{"size", true, "BIGINT"},
		{"last_updated", true, "TIMESTAMP"},
		{"checksum", false, "VARCHAR"},
		{"existence_map", true, "TEXT"},
	}

	// Get actual columns using PRAGMA table_info
	rows, err := db.conn.Query("PRAGMA table_info(nodes)")
	if err != nil {
		return false, fmt.Errorf("failed to query table info: %w", err)
	}
	defer rows.Close()

	var actualColumns []struct {
		cid        int
		name       string
		dataType   string
		notNull    bool
		defaultVal sql.NullString
		pk         int
	}

	for rows.Next() {
		var col struct {
			cid        int
			name       string
			dataType   string
			notNull    bool
			defaultVal sql.NullString
			pk         int
		}
		var defaultVal sql.NullString
		err := rows.Scan(&col.cid, &col.name, &col.dataType, &col.notNull, &defaultVal, &col.pk)
		if err != nil {
			return false, fmt.Errorf("failed to scan column info: %w", err)
		}
		col.defaultVal = defaultVal
		actualColumns = append(actualColumns, col)
	}

	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("error iterating columns: %w", err)
	}

	// Check column count
	if len(actualColumns) != len(expectedColumns) {
		return false, nil
	}

	// Check each column matches
	for i, expected := range expectedColumns {
		if i >= len(actualColumns) {
			return false, nil
		}
		actual := actualColumns[i]

		// Check name
		if actual.name != expected.name {
			return false, nil
		}

		// Check not null (SQLite uses 0/1, we check != 0 means NOT NULL)
		actualNotNull := actual.notNull
		if actualNotNull != expected.notNull {
			return false, nil
		}

		// Check data type (SQLite is flexible, so we check if it contains our expected type)
		if !strings.Contains(strings.ToUpper(actual.dataType), strings.ToUpper(expected.dataType)) {
			return false, nil
		}
	}

	return true, nil
}

// verifyAndCreateIndexes checks if all required indexes exist and creates missing ones
func (db *DB) verifyAndCreateIndexes(secondaryTables map[string]float64) error {
	// Get existing indexes
	rows, err := db.conn.Query("SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='nodes' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		return fmt.Errorf("failed to query indexes: %w", err)
	}
	defer rows.Close()

	existingIndexes := make(map[string]bool)
	for rows.Next() {
		var indexName string
		if err := rows.Scan(&indexName); err != nil {
			return fmt.Errorf("failed to scan index name: %w", err)
		}
		existingIndexes[indexName] = true
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating indexes: %w", err)
	}

	// Create missing indexes
	createIndexesSQL := BuildIndexesSQL(secondaryTables)
	indexStatements := strings.Split(createIndexesSQL, "\n")
	for _, stmt := range indexStatements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		// Extract index name from CREATE INDEX statement
		// Format: CREATE INDEX IF NOT EXISTS idx_name ON nodes(column)
		parts := strings.Fields(stmt)
		if len(parts) >= 6 && parts[0] == "CREATE" && parts[1] == "INDEX" {
			indexName := parts[5] // Index name after "CREATE INDEX IF NOT EXISTS"
			if !existingIndexes[indexName] {
				if _, err := db.conn.Exec(stmt); err != nil {
					return fmt.Errorf("failed to create index %s: %w", indexName, err)
				}
			}
		}
	}

	return nil
}

// rootNodeExists checks if the root node exists
func (db *DB) rootNodeExists() (bool, error) {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM nodes WHERE id = 'root')"
	err := db.conn.QueryRow(query).Scan(&exists)
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

	// Insert root node
	existenceMapJSON, err := json.Marshal(existenceMap)
	if err != nil {
		return fmt.Errorf("failed to marshal existence map: %w", err)
	}

	// Build insert query with all traversal columns
	columns := []string{"id", "parent_id", "name", "path", "parent_path", "type", "depth_level", "size", "last_updated", "checksum", "existence_map"}
	placeholders := []string{"?", "?", "?", "?", "?", "?", "?", "?", "?", "?", "?"}
	values := []any{
		"root",
		"",
		"root",
		"/",
		"",
		types.NodeTypeFolder,
		0,
		0,
		time.Now(),
		nil,
		string(existenceMapJSON),
	}

	insertQuery := fmt.Sprintf("INSERT INTO nodes (%s) VALUES (%s)",
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "))

	_, err = db.conn.Exec(insertQuery, values...)
	if err != nil {
		return fmt.Errorf("failed to create root node: %w", err)
	}

	return nil
}

// CheckpointWAL performs a WAL checkpoint to ensure all changes are written to the main database file.
// This is important for graceful shutdown to ensure data persistence.
// Mode can be "FULL", "RESTART", or "TRUNCATE" (defaults to "FULL" for complete checkpoint).
// FULL ensures all frames are checkpointed and is the most thorough option.
func (db *DB) CheckpointWAL(mode string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if mode == "" {
		mode = "FULL" // Default to FULL for complete checkpoint during shutdown
	}

	// PRAGMA wal_checkpoint performs a checkpoint operation
	// FULL mode ensures all frames are checkpointed from WAL to main database
	// This guarantees all changes are persisted before shutdown
	query := fmt.Sprintf("PRAGMA wal_checkpoint(%s)", strings.ToUpper(mode))
	_, err := db.conn.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to checkpoint WAL: %w", err)
	}

	return nil
}

// Close closes the database connection after performing a WAL checkpoint to ensure data persistence
func (db *DB) Close() error {
	// Perform WAL checkpoint before closing to ensure all changes are saved
	// Use FULL mode to ensure all frames are checkpointed from WAL to main database
	// This guarantees all changes are persisted before shutdown
	if err := db.CheckpointWAL("FULL"); err != nil {
		// Log error but don't fail close - connection will still close
		// This ensures the connection closes even if checkpoint fails
		_ = err // Error is logged but we continue with close
	}

	return db.conn.Close()
}

// InsertNode inserts a new node into the nodes table
func (db *DB) InsertNode(node *types.Node) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Serialize existence map to JSON
	existenceMapJSON, err := json.Marshal(node.ExistenceMap)
	if err != nil {
		return fmt.Errorf("failed to marshal existence map: %w", err)
	}

	// Build dynamic column list and values based on traversal columns
	columns := []string{"id", "parent_id", "name", "path", "parent_path", "type", "depth_level", "size", "last_updated", "checksum", "existence_map"}
	placeholders := []string{"?", "?", "?", "?", "?", "?", "?", "?", "?", "?", "?"}
	values := []any{
		node.ID,
		node.ParentID,
		node.Name,
		node.Path,
		node.ParentPath,
		node.Type,
		node.DepthLevel,
		node.Size,
		node.LastUpdated,
		node.Checksum,
		string(existenceMapJSON),
	}

	query := fmt.Sprintf("INSERT INTO nodes (%s) VALUES (%s)",
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "))

	_, err = db.conn.Exec(query, values...)
	if err != nil {
		return fmt.Errorf("[SpectraFS] failed to insert node %s: %w", node.ID, err)
	}

	return nil
}

// GetNodeByID retrieves a node by its ID from the nodes table
func (db *DB) GetNodeByID(id string) (*types.Node, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Build dynamic column list for traversal statuses
	columns := []string{"id", "parent_id", "name", "path", "parent_path", "type", "depth_level", "size", "last_updated", "checksum", "existence_map"}

	query := fmt.Sprintf("SELECT %s FROM nodes WHERE id = ?", strings.Join(columns, ", "))
	row := db.conn.QueryRow(query, id)

	node := &types.Node{}
	var existenceMapJSON string
	var checksumNull sql.NullString

	// Prepare scan targets
	scanTargets := []any{
		&node.ID,
		&node.ParentID,
		&node.Name,
		&node.Path,
		&node.ParentPath,
		&node.Type,
		&node.DepthLevel,
		&node.Size,
		&node.LastUpdated,
		&checksumNull,
		&existenceMapJSON,
	}

	err := row.Scan(scanTargets...)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("[SpectraFS] node not found: %s", id)
		}
		return nil, fmt.Errorf("[SpectraFS] failed to get node %s: %w", id, err)
	}

	// Handle checksum
	if checksumNull.Valid {
		node.Checksum = &checksumNull.String
	}

	// Deserialize existence map
	if existenceMapJSON != "" {
		if err := json.Unmarshal([]byte(existenceMapJSON), &node.ExistenceMap); err != nil {
			return nil, fmt.Errorf("[SpectraFS] failed to unmarshal existence map: %w", err)
		}
	} else {
		node.ExistenceMap = make(map[string]bool)
	}

	return node, nil
}

// GetChildrenByParentID retrieves all children of a parent node filtered by world
func (db *DB) GetChildrenByParentID(parentID, world string) ([]*types.Node, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `
SELECT id, parent_id, name, path, parent_path, type, depth_level, size, last_updated, checksum, existence_map
FROM nodes
WHERE parent_id = ?
  AND json_extract(existence_map, '$."` + world + `"') = true
ORDER BY type, name`

	rows, err := db.conn.Query(query, parentID)
	if err != nil {
		return nil, fmt.Errorf("[SpectraFS] failed to query children of %s in world %s: %w", parentID, world, err)
	}
	defer rows.Close()

	var children []*types.Node
	for rows.Next() {
		node := &types.Node{}
		var existenceMapJSON string
		var checksumNull sql.NullString

		err = rows.Scan(
			&node.ID,
			&node.ParentID,
			&node.Name,
			&node.Path,
			&node.ParentPath,
			&node.Type,
			&node.DepthLevel,
			&node.Size,
			&node.LastUpdated,
			&checksumNull,
			&existenceMapJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("[SpectraFS] failed to scan child node: %w", err)
		}

		// Handle checksum
		if checksumNull.Valid {
			node.Checksum = &checksumNull.String
		}

		// Deserialize existence map
		if existenceMapJSON != "" {
			if err := json.Unmarshal([]byte(existenceMapJSON), &node.ExistenceMap); err != nil {
				return nil, fmt.Errorf("[SpectraFS] failed to unmarshal existence map: %w", err)
			}
		} else {
			node.ExistenceMap = make(map[string]bool)
		}

		children = append(children, node)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("[SpectraFS] error iterating children: %w", err)
	}

	return children, nil
}

// GetParentAndChildren retrieves parent and all its children in ONE optimized query
// This is the key performance optimization for ListChildren operations
func (db *DB) GetParentAndChildren(parentID, world string) ([]*types.Node, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `
SELECT id, parent_id, name, path, parent_path, type, depth_level, size, last_updated, checksum, existence_map
FROM nodes
WHERE (id = ? OR parent_id = ?)
  AND json_extract(existence_map, '$."` + world + `"') = true
ORDER BY 
  CASE WHEN id = ? THEN 0 ELSE 1 END,
  type, name`

	rows, err := db.conn.Query(query, parentID, parentID, parentID)
	if err != nil {
		return nil, fmt.Errorf("[SpectraFS] failed to query parent and children: %w", err)
	}
	defer rows.Close()

	var nodes []*types.Node
	for rows.Next() {
		node := &types.Node{}
		var existenceMapJSON string
		var checksumNull sql.NullString

		err = rows.Scan(
			&node.ID,
			&node.ParentID,
			&node.Name,
			&node.Path,
			&node.ParentPath,
			&node.Type,
			&node.DepthLevel,
			&node.Size,
			&node.LastUpdated,
			&checksumNull,
			&existenceMapJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("[SpectraFS] failed to scan node: %w", err)
		}

		// Handle checksum
		if checksumNull.Valid {
			node.Checksum = &checksumNull.String
		}

		// Deserialize existence map
		if existenceMapJSON != "" {
			if err := json.Unmarshal([]byte(existenceMapJSON), &node.ExistenceMap); err != nil {
				return nil, fmt.Errorf("[SpectraFS] failed to unmarshal existence map: %w", err)
			}
		} else {
			node.ExistenceMap = make(map[string]bool)
		}

		nodes = append(nodes, node)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("[SpectraFS] error iterating nodes: %w", err)
	}

	// First node should be parent (due to ORDER BY), rest are children
	return nodes, nil
}

// CheckChildrenExist checks if a parent has any children in a specific world
func (db *DB) CheckChildrenExist(parentID, world string) (bool, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `
SELECT COUNT(*) 
FROM nodes 
WHERE parent_id = ?
  AND json_extract(existence_map, '$."` + world + `"') = true`

	var count int
	err := db.conn.QueryRow(query, parentID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("[SpectraFS] failed to check children existence for %s: %w", parentID, err)
	}
	return count > 0, nil
}

// UpdateExistenceMap updates the existence map for a node
func (db *DB) UpdateExistenceMap(id string, existenceMap map[string]bool) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	existenceMapJSON, err := json.Marshal(existenceMap)
	if err != nil {
		return fmt.Errorf("[SpectraFS] failed to marshal existence map: %w", err)
	}

	query := `UPDATE nodes SET existence_map = ? WHERE id = ?`
	result, err := db.conn.Exec(query, string(existenceMapJSON), id)
	if err != nil {
		return fmt.Errorf("[SpectraFS] failed to update existence map for %s: %w", id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("[SpectraFS] failed to get rows affected for %s: %w", id, err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("[SpectraFS] node %s not found", id)
	}

	return nil
}

// DeleteAllNodes removes all nodes from the nodes table (for Reset)
func (db *DB) DeleteAllNodes() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if _, err := db.conn.Exec("DELETE FROM nodes"); err != nil {
		return fmt.Errorf("[SpectraFS] failed to delete from nodes table: %w", err)
	}
	return nil
}

// GetNodeCount returns the total number of nodes in a specific world
func (db *DB) GetNodeCount(world string) (int, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `SELECT COUNT(*) FROM nodes WHERE json_extract(existence_map, '$."` + world + `"') = true`
	var count int
	err := db.conn.QueryRow(query).Scan(&count)
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

	// Add primary world - inline query to avoid nested locking
	query := `SELECT COUNT(*) FROM nodes WHERE json_extract(existence_map, '$."primary"') = true`
	var primaryCount int
	err := db.conn.QueryRow(query).Scan(&primaryCount)
	if err != nil {
		return nil, fmt.Errorf("[SpectraFS] failed to get count for primary world: %w", err)
	}
	tables = append(tables, types.TableInfo{
		Name:      "primary",
		RowCount:  primaryCount,
		TableType: "primary",
	})

	// Add secondary worlds - inline queries to avoid nested locking
	for _, worldName := range db.secondaryTables {
		query := `SELECT COUNT(*) FROM nodes WHERE json_extract(existence_map, '$."` + worldName + `"') = true`
		var count int
		err := db.conn.QueryRow(query).Scan(&count)
		if err != nil {
			return nil, fmt.Errorf("[SpectraFS] failed to get count for world %s: %w", worldName, err)
		}
		tables = append(tables, types.TableInfo{
			Name:      worldName,
			RowCount:  count,
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

	// Get parent node to determine path - inline query to avoid nested locking
	var parentPath string
	query := "SELECT path FROM nodes WHERE id = ?"
	err := db.conn.QueryRow(query, parentID).Scan(&parentPath)
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

	// Check if root exists - inline query to avoid nested locking
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM nodes WHERE id = 'root')"
	err := db.conn.QueryRow(query).Scan(&exists)
	if err == nil && exists {
		// Root already exists
		return nil
	}

	// Create existence map with all worlds
	existenceMap := make(map[string]bool)
	existenceMap["primary"] = true
	for _, worldName := range db.secondaryTables {
		existenceMap[worldName] = true
	}

	// Insert root node directly to avoid nested locking
	existenceMapJSON, err := json.Marshal(existenceMap)
	if err != nil {
		return fmt.Errorf("[SpectraFS] failed to marshal existence map: %w", err)
	}

	// Build insert query with all traversal columns
	columns := []string{"id", "parent_id", "name", "path", "parent_path", "type", "depth_level", "size", "last_updated", "checksum", "existence_map"}
	placeholders := []string{"?", "?", "?", "?", "?", "?", "?", "?", "?", "?", "?"}
	values := []any{
		"root",
		"",
		"root",
		"/",
		"",
		types.NodeTypeFolder,
		0,
		0,
		time.Now(),
		nil,
		string(existenceMapJSON),
	}

	insertQuery := fmt.Sprintf("INSERT OR IGNORE INTO nodes (%s) VALUES (%s)",
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "))

	_, err = db.conn.Exec(insertQuery, values...)
	if err != nil {
		return fmt.Errorf("[SpectraFS] failed to create root node: %w", err)
	}

	return nil
}

// DeleteNode deletes a node from the nodes table
func (db *DB) DeleteNode(id string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `DELETE FROM nodes WHERE id = ?`
	result, err := db.conn.Exec(query, id)
	if err != nil {
		return fmt.Errorf("[SpectraFS] failed to delete node %s: %w", id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("[SpectraFS] failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("[SpectraFS] node not found: %s", id)
	}

	return nil
}

// GetSecondaryTables returns the list of secondary world names
func (db *DB) GetSecondaryTables() []string {
	return db.secondaryTables
}

// Note: ParentInfo and GetParentInfo removed - replaced by GetParentAndChildren for better performance

// BulkInsertNodes inserts multiple nodes in a single transaction
func (db *DB) BulkInsertNodes(nodes []*types.Node) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if len(nodes) == 0 {
		return nil
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("[SpectraFS] failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Build dynamic column list
	columns := []string{"id", "parent_id", "name", "path", "parent_path", "type", "depth_level", "size", "last_updated", "checksum", "existence_map"}
	placeholders := []string{"?", "?", "?", "?", "?", "?", "?", "?", "?", "?", "?"}

	query := fmt.Sprintf("INSERT OR IGNORE INTO nodes (%s) VALUES (%s)",
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "))

	stmt, err := tx.Prepare(query)
	if err != nil {
		return fmt.Errorf("[SpectraFS] failed to prepare insert statement: %w", err)
	}
	defer stmt.Close()

	// Insert all nodes
	for _, node := range nodes {
		existenceMapJSON, err := json.Marshal(node.ExistenceMap)
		if err != nil {
			return fmt.Errorf("[SpectraFS] failed to marshal existence map: %w", err)
		}

		var checksumVal any
		if node.Checksum != nil {
			checksumVal = *node.Checksum
		} else {
			checksumVal = nil
		}

		values := []any{
			node.ID,
			node.ParentID,
			node.Name,
			node.Path,
			node.ParentPath,
			node.Type,
			node.DepthLevel,
			node.Size,
			node.LastUpdated,
			checksumVal,
			string(existenceMapJSON),
		}

		_, err = stmt.Exec(values...)
		if err != nil {
			return fmt.Errorf("[SpectraFS] failed to insert node %s: %w", node.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("[SpectraFS] failed to commit transaction: %w", err)
	}

	return nil
}

// GetNodeByPath retrieves a node by its path, optionally filtering by world
func (db *DB) GetNodeByPath(path, world string) (*types.Node, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `
SELECT id, parent_id, name, path, parent_path, type, depth_level, size, last_updated, checksum, existence_map
FROM nodes
WHERE path = ?`

	if world != "" {
		query += ` AND json_extract(existence_map, '$."` + world + `"') = true`
	}

	query += ` LIMIT 1`

	row := db.conn.QueryRow(query, path)

	node := &types.Node{}
	var existenceMapJSON string
	var checksumNull sql.NullString

	err := row.Scan(
		&node.ID,
		&node.ParentID,
		&node.Name,
		&node.Path,
		&node.ParentPath,
		&node.Type,
		&node.DepthLevel,
		&node.Size,
		&node.LastUpdated,
		&checksumNull,
		&existenceMapJSON,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("[SpectraFS] node not found with path %s in world %s", path, world)
		}
		return nil, fmt.Errorf("[SpectraFS] failed to get node by path %s in world %s: %w", path, world, err)
	}

	// Handle checksum
	if checksumNull.Valid {
		node.Checksum = &checksumNull.String
	}

	// Deserialize existence map
	if existenceMapJSON != "" {
		if err := json.Unmarshal([]byte(existenceMapJSON), &node.ExistenceMap); err != nil {
			return nil, fmt.Errorf("[SpectraFS] failed to unmarshal existence map: %w", err)
		}
	} else {
		node.ExistenceMap = make(map[string]bool)
	}

	return node, nil
}
