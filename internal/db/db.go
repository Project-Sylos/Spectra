package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Project-Sylos/Spectra/internal/types"
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

	// Initialize schema
	if err := db.InitializeSchema(secondaryTables); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return db, nil
}

// InitializeSchema creates the unified nodes table with all traversal columns
// Note: This drops and recreates the table to ensure schema matches the provided secondary tables
func (db *DB) InitializeSchema(secondaryTables map[string]float64) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Drop existing table if it exists (to allow schema recreation)
	if _, err := db.conn.Exec("DROP TABLE IF EXISTS nodes"); err != nil {
		return fmt.Errorf("failed to drop existing nodes table: %w", err)
	}

	// Build and create unified nodes table with all traversal columns
	createTableSQL := BuildNodesTableSQL(secondaryTables)
	if _, err := db.conn.Exec(createTableSQL); err != nil {
		return fmt.Errorf("failed to create nodes table: %w", err)
	}

	// Build and create all indexes
	createIndexesSQL := BuildIndexesSQL(secondaryTables)
	if _, err := db.conn.Exec(createIndexesSQL); err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
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

// Close closes the database connection
func (db *DB) Close() error {
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
		return fmt.Errorf("failed to insert node %s: %w", node.ID, err)
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
			return nil, fmt.Errorf("node not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get node %s: %w", id, err)
	}

	// Handle checksum
	if checksumNull.Valid {
		node.Checksum = &checksumNull.String
	}

	// Deserialize existence map
	if existenceMapJSON != "" {
		if err := json.Unmarshal([]byte(existenceMapJSON), &node.ExistenceMap); err != nil {
			return nil, fmt.Errorf("failed to unmarshal existence map: %w", err)
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
		return nil, fmt.Errorf("failed to query children of %s in world %s: %w", parentID, world, err)
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
			return nil, fmt.Errorf("failed to scan child node: %w", err)
		}

		// Handle checksum
		if checksumNull.Valid {
			node.Checksum = &checksumNull.String
		}

		// Deserialize existence map
		if existenceMapJSON != "" {
			if err := json.Unmarshal([]byte(existenceMapJSON), &node.ExistenceMap); err != nil {
				return nil, fmt.Errorf("failed to unmarshal existence map: %w", err)
			}
		} else {
			node.ExistenceMap = make(map[string]bool)
		}

		children = append(children, node)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating children: %w", err)
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
		return nil, fmt.Errorf("failed to query parent and children: %w", err)
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
			return nil, fmt.Errorf("failed to scan node: %w", err)
		}

		// Handle checksum
		if checksumNull.Valid {
			node.Checksum = &checksumNull.String
		}

		// Deserialize existence map
		if existenceMapJSON != "" {
			if err := json.Unmarshal([]byte(existenceMapJSON), &node.ExistenceMap); err != nil {
				return nil, fmt.Errorf("failed to unmarshal existence map: %w", err)
			}
		} else {
			node.ExistenceMap = make(map[string]bool)
		}

		nodes = append(nodes, node)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating nodes: %w", err)
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
		return false, fmt.Errorf("failed to check children existence for %s: %w", parentID, err)
	}
	return count > 0, nil
}

// UpdateExistenceMap updates the existence map for a node
func (db *DB) UpdateExistenceMap(id string, existenceMap map[string]bool) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	existenceMapJSON, err := json.Marshal(existenceMap)
	if err != nil {
		return fmt.Errorf("failed to marshal existence map: %w", err)
	}

	query := `UPDATE nodes SET existence_map = ? WHERE id = ?`
	result, err := db.conn.Exec(query, string(existenceMapJSON), id)
	if err != nil {
		return fmt.Errorf("failed to update existence map for %s: %w", id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected for %s: %w", id, err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("node %s not found", id)
	}

	return nil
}

// DeleteAllNodes removes all nodes from the nodes table (for Reset)
func (db *DB) DeleteAllNodes() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if _, err := db.conn.Exec("DELETE FROM nodes"); err != nil {
		return fmt.Errorf("failed to delete from nodes table: %w", err)
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
		return 0, fmt.Errorf("failed to get node count for world %s: %w", world, err)
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
		return nil, fmt.Errorf("failed to get count for primary world: %w", err)
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
			return nil, fmt.Errorf("failed to get count for world %s: %w", worldName, err)
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
		return nil, fmt.Errorf("failed to get parent node: %w", err)
	}

	path := fmt.Sprintf("%s/%s", parentPath, name)

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
		return fmt.Errorf("failed to marshal existence map: %w", err)
	}

	// Build insert query with all traversal columns
	columns := []string{"id", "parent_id", "name", "path", "parent_path", "type", "depth_level", "size", "last_updated", "checksum", "existence_map"}
	placeholders := []string{"?", "?", "?", "?", "?", "?", "?", "?", "?", "?", "?"}
	values := []interface{}{
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

// DeleteNode deletes a node from the nodes table
func (db *DB) DeleteNode(id string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `DELETE FROM nodes WHERE id = ?`
	result, err := db.conn.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete node %s: %w", id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("node not found: %s", id)
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
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Build dynamic column list
	columns := []string{"id", "parent_id", "name", "path", "parent_path", "type", "depth_level", "size", "last_updated", "checksum", "existence_map"}
	placeholders := []string{"?", "?", "?", "?", "?", "?", "?", "?", "?", "?", "?"}

	query := fmt.Sprintf("INSERT INTO nodes (%s) VALUES (%s)",
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "))

	stmt, err := tx.Prepare(query)
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer stmt.Close()

	// Insert all nodes
	for _, node := range nodes {
		existenceMapJSON, err := json.Marshal(node.ExistenceMap)
		if err != nil {
			return fmt.Errorf("failed to marshal existence map: %w", err)
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
			return fmt.Errorf("failed to insert node %s: %w", node.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
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
			return nil, fmt.Errorf("node not found with path %s in world %s", path, world)
		}
		return nil, fmt.Errorf("failed to get node by path %s in world %s: %w", path, world, err)
	}

	// Handle checksum
	if checksumNull.Valid {
		node.Checksum = &checksumNull.String
	}

	// Deserialize existence map
	if existenceMapJSON != "" {
		if err := json.Unmarshal([]byte(existenceMapJSON), &node.ExistenceMap); err != nil {
			return nil, fmt.Errorf("failed to unmarshal existence map: %w", err)
		}
	} else {
		node.ExistenceMap = make(map[string]bool)
	}

	return node, nil
}
