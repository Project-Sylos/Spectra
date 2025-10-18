package db

// Table schema constants
const (
	// CreatePrimaryTableSQL creates the primary nodes table with enhanced schema
	CreatePrimaryTableSQL = `
CREATE TABLE IF NOT EXISTS nodes_primary (
    id VARCHAR PRIMARY KEY,
    parent_id VARCHAR NOT NULL,
    name VARCHAR NOT NULL,
    path VARCHAR NOT NULL,
    type VARCHAR NOT NULL CHECK (type IN ('folder', 'file')),
    depth_level INTEGER NOT NULL,
    size BIGINT NOT NULL,
    last_updated TIMESTAMP NOT NULL,
    traversal_status VARCHAR NOT NULL DEFAULT 'pending' CHECK (traversal_status IN ('pending', 'successful', 'failed')),
    secondary_existence_map JSON NOT NULL DEFAULT '{}'
);`

	// CreateSecondaryTableSQL creates a secondary table with the same schema as primary
	CreateSecondaryTableSQL = `
CREATE TABLE IF NOT EXISTS %s (
    id VARCHAR PRIMARY KEY,
    parent_id VARCHAR NOT NULL,
    name VARCHAR NOT NULL,
    path VARCHAR NOT NULL,
    type VARCHAR NOT NULL CHECK (type IN ('folder', 'file')),
    depth_level INTEGER NOT NULL,
    size BIGINT NOT NULL,
    last_updated TIMESTAMP NOT NULL,
    traversal_status VARCHAR NOT NULL DEFAULT 'pending' CHECK (traversal_status IN ('pending', 'successful', 'failed'))
);`

	// CreatePrimaryIndexesSQL creates indexes for the primary table
	CreatePrimaryIndexesSQL = `
CREATE INDEX IF NOT EXISTS idx_primary_parent_id ON nodes_primary(parent_id);
CREATE INDEX IF NOT EXISTS idx_primary_type ON nodes_primary(type);
CREATE INDEX IF NOT EXISTS idx_primary_depth_level ON nodes_primary(depth_level);
CREATE INDEX IF NOT EXISTS idx_primary_traversal_status ON nodes_primary(traversal_status);`

	// CreateSecondaryIndexesSQL creates indexes for a secondary table
	CreateSecondaryIndexesSQL = `
CREATE INDEX IF NOT EXISTS idx_%s_parent_id ON %s(parent_id);
CREATE INDEX IF NOT EXISTS idx_%s_type ON %s(type);
CREATE INDEX IF NOT EXISTS idx_%s_depth_level ON %s(depth_level);
CREATE INDEX IF NOT EXISTS idx_%s_traversal_status ON %s(traversal_status);`
)
