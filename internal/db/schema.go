package db

import (
	"fmt"
	"strings"
)

// BuildNodesTableSQL generates the CREATE TABLE statement with all traversal columns
func BuildNodesTableSQL(secondaryTables map[string]float64) string {
	// Base columns
	columns := []string{
		"id VARCHAR PRIMARY KEY",
		"parent_id VARCHAR NOT NULL",
		"name VARCHAR NOT NULL",
		"path VARCHAR NOT NULL",
		"type VARCHAR NOT NULL CHECK (type IN ('folder', 'file'))",
		"depth_level INTEGER NOT NULL",
		"size BIGINT NOT NULL",
		"last_updated TIMESTAMP NOT NULL",
		"checksum VARCHAR",
		"existence_map JSON NOT NULL DEFAULT '{\"primary\": true}'",
		"traversal_primary VARCHAR NOT NULL DEFAULT 'pending' CHECK (traversal_primary IN ('pending', 'successful', 'failed'))",
	}

	// Add traversal column for each secondary world
	for tableName := range secondaryTables {
		col := fmt.Sprintf("traversal_%s VARCHAR NOT NULL DEFAULT 'pending' CHECK (traversal_%s IN ('pending', 'successful', 'failed'))", tableName, tableName)
		columns = append(columns, col)
	}

	// Add copy_status last
	columns = append(columns, "copy_status VARCHAR NOT NULL DEFAULT 'pending' CHECK (copy_status IN ('pending', 'in_progress', 'completed'))")

	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS nodes (\n    %s\n);", strings.Join(columns, ",\n    "))
}

// BuildIndexesSQL generates all index creation statements
func BuildIndexesSQL(secondaryTables map[string]float64) string {
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_parent_id ON nodes(parent_id);",
		"CREATE INDEX IF NOT EXISTS idx_type ON nodes(type);",
		"CREATE INDEX IF NOT EXISTS idx_depth_level ON nodes(depth_level);",
		"CREATE INDEX IF NOT EXISTS idx_path ON nodes(path);",
		"CREATE INDEX IF NOT EXISTS idx_traversal_primary ON nodes(traversal_primary);",
	}

	// Add index for each secondary traversal column
	for tableName := range secondaryTables {
		idx := fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_traversal_%s ON nodes(traversal_%s);", tableName, tableName)
		indexes = append(indexes, idx)
	}

	// Add copy_status index
	indexes = append(indexes, "CREATE INDEX IF NOT EXISTS idx_copy_status ON nodes(copy_status);")

	return strings.Join(indexes, "\n")
}
