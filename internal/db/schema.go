package db

import (
	"fmt"
	"strings"
)

// BuildNodesTableSQL generates the CREATE TABLE statement with all traversal columns
func BuildNodesTableSQL(secondaryTables map[string]float64) string {
	// Base columns
	columns := []string{
		"id VARCHAR NOT NULL UNIQUE",
		"parent_id VARCHAR NOT NULL",
		"name VARCHAR NOT NULL",
		"path VARCHAR NOT NULL",
		"parent_path VARCHAR NOT NULL",
		"type VARCHAR NOT NULL CHECK (type IN ('folder', 'file'))",
		"depth_level INTEGER NOT NULL",
		"size BIGINT NOT NULL",
		"last_updated TIMESTAMP NOT NULL",
		"checksum VARCHAR",
		"existence_map TEXT NOT NULL DEFAULT '{\"primary\": true}'",
	}

	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS nodes (\n    %s\n);", strings.Join(columns, ",\n    "))
}

// BuildIndexesSQL generates all index creation statements
func BuildIndexesSQL(secondaryTables map[string]float64) string {
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_parent_id ON nodes(parent_id);",
		"CREATE INDEX IF NOT EXISTS idx_type ON nodes(type);",
		"CREATE INDEX IF NOT EXISTS idx_depth_level ON nodes(depth_level);",
		"CREATE INDEX IF NOT EXISTS idx_path ON nodes(path);",
		"CREATE INDEX IF NOT EXISTS idx_parent_path ON nodes(parent_path);",
	}

	return strings.Join(indexes, "\n")
}
