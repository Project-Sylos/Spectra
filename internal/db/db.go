package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Project-Sylos/Spectra/internal/types"
	"github.com/google/uuid"
	_ "github.com/marcboeker/go-duckdb"
)

// DB wraps DuckDB connection and provides multi-table CRUD operations
type DB struct {
	conn            *sql.DB
	secondaryTables map[string]bool
}

// New creates a new database connection and initializes the schema
func New(dbPath string, secondaryTables map[string]float64) (*DB, error) {
	// Open DuckDB connection
	conn, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open DuckDB connection: %w", err)
	}

	// Register DuckDB driver
	conn.Driver()

	// Create secondary tables map
	secondaryMap := make(map[string]bool)
	for tableName := range secondaryTables {
		secondaryMap[tableName] = true
	}

	db := &DB{
		conn:            conn,
		secondaryTables: secondaryMap,
	}

	// Initialize schema
	if err := db.InitializeSchema(secondaryTables); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return db, nil
}

// InitializeSchema creates the primary and secondary tables with indexes
func (db *DB) InitializeSchema(secondaryTables map[string]float64) error {
	// Create primary table
	if _, err := db.conn.Exec(CreatePrimaryTableSQL); err != nil {
		return fmt.Errorf("failed to create primary table: %w", err)
	}

	// Create primary table indexes
	if _, err := db.conn.Exec(CreatePrimaryIndexesSQL); err != nil {
		return fmt.Errorf("failed to create primary indexes: %w", err)
	}

	// Create secondary tables
	for tableName := range secondaryTables {
		tableSQL := fmt.Sprintf(CreateSecondaryTableSQL, "nodes_"+tableName)
		if _, err := db.conn.Exec(tableSQL); err != nil {
			return fmt.Errorf("failed to create secondary table %s: %w", tableName, err)
		}

		// Create secondary table indexes
		indexSQL := fmt.Sprintf(CreateSecondaryIndexesSQL, tableName, "nodes_"+tableName, tableName, "nodes_"+tableName, tableName, "nodes_"+tableName, tableName, "nodes_"+tableName)
		if _, err := db.conn.Exec(indexSQL); err != nil {
			return fmt.Errorf("failed to create indexes for secondary table %s: %w", tableName, err)
		}
	}

	return nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// InsertPrimaryNode inserts a new node into the primary table
func (db *DB) InsertPrimaryNode(node *types.Node) error {
	// Serialize secondary existence map to JSON
	existenceMapJSON, err := json.Marshal(node.SecondaryExistenceMap)
	if err != nil {
		return fmt.Errorf("failed to marshal secondary existence map: %w", err)
	}

	query := `
INSERT INTO nodes_primary (id, parent_id, name, path, type, depth_level, size, last_updated, traversal_status, secondary_existence_map)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = db.conn.Exec(query,
		node.ID,
		node.ParentID,
		node.Name,
		node.Path,
		node.Type,
		node.DepthLevel,
		node.Size,
		node.LastUpdated,
		node.TraversalStatus,
		string(existenceMapJSON),
	)
	if err != nil {
		return fmt.Errorf("failed to insert primary node %s: %w", node.ID, err)
	}
	return nil
}

// InsertSecondaryNode inserts a new node into a secondary table
func (db *DB) InsertSecondaryNode(tableName string, node *types.Node) error {
	query := fmt.Sprintf(`
INSERT INTO nodes_%s (id, parent_id, name, path, type, depth_level, size, last_updated, traversal_status)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, tableName)

	_, err := db.conn.Exec(query,
		node.ID,
		node.ParentID,
		node.Name,
		node.Path,
		node.Type,
		node.DepthLevel,
		node.Size,
		node.LastUpdated,
		node.TraversalStatus,
	)
	if err != nil {
		return fmt.Errorf("failed to insert secondary node %s into %s: %w", node.ID, tableName, err)
	}
	return nil
}

// GetNodeByID retrieves a node by its ID from the appropriate table
func (db *DB) GetNodeByID(id string) (*types.Node, error) {
	tableName := types.GetTableFromID(id)
	if tableName == "" {
		return nil, fmt.Errorf("invalid node ID format: %s", id)
	}

	query := fmt.Sprintf(`
SELECT id, parent_id, name, path, type, depth_level, size, last_updated, traversal_status, secondary_existence_map
FROM %s
WHERE id = ?`, tableName)

	row := db.conn.QueryRow(query, id)

	node := &types.Node{}
	var existenceMapJSON string

	err := row.Scan(
		&node.ID,
		&node.ParentID,
		&node.Name,
		&node.Path,
		&node.Type,
		&node.DepthLevel,
		&node.Size,
		&node.LastUpdated,
		&node.TraversalStatus,
		&existenceMapJSON,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("node not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get node %s: %w", id, err)
	}

	// Deserialize secondary existence map (only for primary table)
	if tableName == "nodes_primary" && existenceMapJSON != "" {
		if err := json.Unmarshal([]byte(existenceMapJSON), &node.SecondaryExistenceMap); err != nil {
			return nil, fmt.Errorf("failed to unmarshal secondary existence map: %w", err)
		}
	} else {
		node.SecondaryExistenceMap = make(map[string]bool)
	}

	return node, nil
}

// GetChildrenByParentID retrieves all children of a parent node from the appropriate table
func (db *DB) GetChildrenByParentID(parentID string) ([]*types.Node, error) {
	tableName := types.GetTableFromID(parentID)
	if tableName == "" {
		return nil, fmt.Errorf("invalid parent ID format: %s", parentID)
	}

	query := fmt.Sprintf(`
SELECT id, parent_id, name, path, type, depth_level, size, last_updated, traversal_status, secondary_existence_map
FROM %s
WHERE parent_id = ?
ORDER BY type, name`, tableName)

	rows, err := db.conn.Query(query, parentID)
	if err != nil {
		return nil, fmt.Errorf("failed to query children of %s: %w", parentID, err)
	}
	defer rows.Close()

	var children []*types.Node
	for rows.Next() {
		node := &types.Node{}
		var existenceMapJSON string

		err := rows.Scan(
			&node.ID,
			&node.ParentID,
			&node.Name,
			&node.Path,
			&node.Type,
			&node.DepthLevel,
			&node.Size,
			&node.LastUpdated,
			&node.TraversalStatus,
			&existenceMapJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan child node: %w", err)
		}

		// Deserialize secondary existence map (only for primary table)
		if tableName == "nodes_primary" && existenceMapJSON != "" {
			if err := json.Unmarshal([]byte(existenceMapJSON), &node.SecondaryExistenceMap); err != nil {
				return nil, fmt.Errorf("failed to unmarshal secondary existence map: %w", err)
			}
		} else {
			node.SecondaryExistenceMap = make(map[string]bool)
		}

		children = append(children, node)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating children: %w", err)
	}

	return children, nil
}

// CheckChildrenExist checks if a parent has any children in the appropriate table
func (db *DB) CheckChildrenExist(parentID string) (bool, error) {
	tableName := types.GetTableFromID(parentID)
	if tableName == "" {
		return false, fmt.Errorf("invalid parent ID format: %s", parentID)
	}

	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE parent_id = ?`, tableName)
	var count int
	err := db.conn.QueryRow(query, parentID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check children existence for %s: %w", parentID, err)
	}
	return count > 0, nil
}

// UpdateTraversalStatus updates the traversal status of a node in the appropriate table
func (db *DB) UpdateTraversalStatus(id, status string) error {
	tableName := types.GetTableFromID(id)
	if tableName == "" {
		return fmt.Errorf("invalid node ID format: %s", id)
	}

	query := fmt.Sprintf(`UPDATE %s SET traversal_status = ? WHERE id = ?`, tableName)
	_, err := db.conn.Exec(query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update traversal status for %s: %w", id, err)
	}
	return nil
}

// UpdateSecondaryExistenceMap updates the secondary existence map in the primary table
func (db *DB) UpdateSecondaryExistenceMap(id string, existenceMap map[string]bool) error {
	existenceMapJSON, err := json.Marshal(existenceMap)
	if err != nil {
		return fmt.Errorf("failed to marshal secondary existence map: %w", err)
	}

	query := `UPDATE nodes_primary SET secondary_existence_map = ? WHERE id = ?`
	_, err = db.conn.Exec(query, string(existenceMapJSON), id)
	if err != nil {
		return fmt.Errorf("failed to update secondary existence map for %s: %w", id, err)
	}
	return nil
}

// DeleteAllNodes removes all nodes from all tables (for Reset)
func (db *DB) DeleteAllNodes() error {
	// Delete from primary table
	if _, err := db.conn.Exec("DELETE FROM nodes_primary"); err != nil {
		return fmt.Errorf("failed to delete from primary table: %w", err)
	}

	// Delete from secondary tables
	for tableName := range db.secondaryTables {
		query := fmt.Sprintf("DELETE FROM nodes_%s", tableName)
		if _, err := db.conn.Exec(query); err != nil {
			return fmt.Errorf("failed to delete from secondary table %s: %w", tableName, err)
		}
	}

	return nil
}

// GetNodeCount returns the total number of nodes in a specific table
func (db *DB) GetNodeCount(tableName string) (int, error) {
	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s`, tableName)
	var count int
	err := db.conn.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get node count for table %s: %w", tableName, err)
	}
	return count, nil
}

// GetTableInfo returns information about all tables
func (db *DB) GetTableInfo() ([]types.TableInfo, error) {
	query := `
SELECT table_name 
FROM information_schema.tables 
WHERE table_schema = 'main' 
AND table_name LIKE 'nodes_%'
ORDER BY table_name`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get table list: %w", err)
	}
	defer rows.Close()

	var tables []types.TableInfo
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %w", err)
		}

		// Get row count
		count, err := db.GetNodeCount(tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get count for table %s: %w", tableName, err)
		}

		// Determine table type
		tableType := "secondary"
		if tableName == "nodes_primary" {
			tableType = "primary"
		}

		tables = append(tables, types.TableInfo{
			Name:      tableName,
			RowCount:  count,
			TableType: tableType,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tables: %w", err)
	}

	return tables, nil
}

// CreateFolder creates a new folder node
func (db *DB) CreateFolder(parentID, name string, depth int) (*types.Node, error) {
	// Generate UUID for the new folder
	nodeUUID := uuid.New().String()
	primaryID := types.PrimaryPrefix + nodeUUID

	// Get parent node to determine path
	parent, err := db.GetNodeByID(parentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent node: %w", err)
	}

	path := fmt.Sprintf("%s/%s", parent.Path, name)

	folderNode := &types.Node{
		ID:                    primaryID,
		ParentID:              parentID,
		Name:                  name,
		Path:                  path,
		Type:                  types.NodeTypeFolder,
		DepthLevel:            depth,
		Size:                  0, // Folders have size 0
		LastUpdated:           time.Now(),
		TraversalStatus:       types.StatusPending,
		SecondaryExistenceMap: make(map[string]bool),
	}

	return folderNode, nil
}

// CreateRootNode creates the root node of the filesystem in the primary table
func (db *DB) CreateRootNode() error {
	rootNode := &types.Node{
		ID:                    "p-root",
		ParentID:              "",
		Name:                  "root",
		Path:                  "/",
		Type:                  types.NodeTypeFolder,
		DepthLevel:            0,
		Size:                  0,
		LastUpdated:           time.Now(),
		TraversalStatus:       types.StatusPending,
		SecondaryExistenceMap: make(map[string]bool),
	}

	return db.InsertPrimaryNode(rootNode)
}

// DeleteNode deletes a node from the appropriate table
func (db *DB) DeleteNode(id string) error {
	tableName := types.GetTableFromID(id)
	if tableName == "" {
		return fmt.Errorf("invalid node ID format: %s", id)
	}

	query := fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, tableName)
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

// GetSecondaryTables returns the list of secondary table names
func (db *DB) GetSecondaryTables() []string {
	var tables []string
	for tableName := range db.secondaryTables {
		tables = append(tables, tableName)
	}
	return tables
}
