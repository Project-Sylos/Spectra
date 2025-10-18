# Database Package

The database package provides the data persistence layer for Spectra using DuckDB. It implements a multi-table architecture with primary and secondary tables for probability-based data distribution.

## Structure

```
db/
├── db.go      # Main database operations and multi-table CRUD
└── schema.go  # Table schemas and index definitions
```

## Multi-Table Architecture

### Primary Table (`nodes_primary`)
- Contains all nodes with `p-{UUID}` IDs
- Includes `secondary_existence_map` JSON column
- Stores complete node metadata

### Secondary Tables (`nodes_s1`, `nodes_s2`, etc.)
- Contains probability-based subsets with `s1-{UUID}`, `s2-{UUID}` IDs
- Same schema as primary table (minus existence map)
- Used for testing multi-source migration scenarios

## Key Features

- **Table Detection**: Automatic table selection based on node ID prefixes
- **Multi-Table CRUD**: Operations work across primary and secondary tables
- **Probability-Based Generation**: Secondary nodes created based on configurable probabilities
- **Existence Tracking**: Primary table tracks which secondary tables contain each node
- **Indexed Queries**: Optimized indexes for common query patterns

## Core Operations

### Node Management
- `InsertPrimaryNode()` - Insert node into primary table
- `InsertSecondaryNode()` - Insert node into secondary table
- `GetNodeByID()` - Retrieve node from appropriate table
- `DeleteNode()` - Delete node from appropriate table

### Children Operations
- `GetChildrenByParentID()` - Get all children of a parent
- `CheckChildrenExist()` - Check if parent has children

### System Operations
- `InitializeSchema()` - Create tables and indexes
- `DeleteAllNodes()` - Clear all tables
- `GetTableInfo()` - Get table metadata
- `GetNodeCount()` - Count nodes in specific table

## Table Schemas

### Primary Table Schema
```sql
CREATE TABLE nodes_primary (
    id VARCHAR PRIMARY KEY,
    parent_id VARCHAR NOT NULL,
    name VARCHAR NOT NULL,
    path VARCHAR NOT NULL,
    type VARCHAR NOT NULL CHECK (type IN ('folder', 'file')),
    depth_level INTEGER NOT NULL,
    size BIGINT NOT NULL,
    last_updated TIMESTAMP NOT NULL,
    traversal_status VARCHAR NOT NULL DEFAULT 'pending',
    secondary_existence_map JSON NOT NULL DEFAULT '{}'
);
```

### Secondary Table Schema
```sql
CREATE TABLE nodes_s1 (
    id VARCHAR PRIMARY KEY,
    parent_id VARCHAR NOT NULL,
    name VARCHAR NOT NULL,
    path VARCHAR NOT NULL,
    type VARCHAR NOT NULL CHECK (type IN ('folder', 'file')),
    depth_level INTEGER NOT NULL,
    size BIGINT NOT NULL,
    last_updated TIMESTAMP NOT NULL,
    traversal_status VARCHAR NOT NULL DEFAULT 'pending'
);
```

## Indexes

Both primary and secondary tables have indexes on:
- `parent_id` - For efficient children queries
- `type` - For filtering by node type
- `depth_level` - For depth-based queries
- `traversal_status` - For status-based queries

## Usage

The database layer is used internally by the SpectraFS implementation. It provides the foundation for all data persistence operations in the synthetic filesystem.
