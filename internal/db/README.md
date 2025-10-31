# Database Package

The database package provides the data persistence layer for Spectra using DuckDB. It implements a unified single-table architecture with per-world existence tracking for maximum performance.

## Structure

```
db/
├── db.go      # Main database operations and CRUD
└── schema.go  # Table schema and index definitions
```

## Single-Table Architecture

### Unified `nodes` Table
- All nodes stored in a single table with plain UUID IDs
- `existence_map` JSON column tracks which "worlds" (primary, s1, s2, etc.) each node exists in
- Dynamic `traversal_*` columns per world for fast batch verification
- Optimized for minimal database round trips

### World-Based Filtering
- Nodes are filtered by world using JSON queries on `existence_map`
- Each node can exist in multiple worlds simultaneously
- Traversal status tracked per-world in dedicated columns

## Key Features

- **Single Vectorized Queries**: Fetch parent + children in one query
- **World-Aware Filtering**: Efficient JSON-based world filtering
- **Conditional Updates**: Skip already-successful traversal updates
- **Bulk Operations**: Transaction-based bulk inserts for performance
- **Dynamic Schema**: Traversal columns added based on configuration
- **Indexed Queries**: Optimized indexes for common query patterns

## Core Operations

### Node Management
- `InsertNode(node)` - Insert node into nodes table
- `GetNodeByID(id)` - Retrieve node by ID
- `GetNodeByPath(path, world)` - Retrieve node by path in specific world
- `DeleteNode(id)` - Delete node by ID
- `BulkInsertNodes(nodes)` - Insert multiple nodes in one transaction

### Children Operations
- `GetChildrenByParentID(parentID, world)` - Get children filtered by world
- `GetParentAndChildren(parentID, world)` - Get parent + children in ONE query (optimized)
- `CheckChildrenExist(parentID, world)` - Check if parent has children in world

### Traversal Operations
- `UpdateTraversalStatus(id, world, status)` - Update traversal status for specific world (conditional)
- `UpdateExistenceMap(id, existenceMap)` - Update node's existence map

### System Operations
- `InitializeSchema()` - Create nodes table with dynamic traversal columns
- `CreateRootNode()` - Create single root node with existence in all worlds
- `DeleteAllNodes()` - Clear nodes table
- `GetTableInfo()` - Get world metadata
- `GetNodeCount(world)` - Count nodes in specific world

## Table Schema

### Unified `nodes` Table
```sql
CREATE TABLE nodes (
    id VARCHAR PRIMARY KEY,
    parent_id VARCHAR NOT NULL,
    name VARCHAR NOT NULL,
    path VARCHAR NOT NULL,
    type VARCHAR NOT NULL CHECK (type IN ('folder', 'file')),
    depth_level INTEGER NOT NULL,
    size BIGINT NOT NULL,
    last_updated TIMESTAMP NOT NULL,
    checksum VARCHAR,
    existence_map JSON NOT NULL DEFAULT '{"primary": true}',
    traversal_primary VARCHAR NOT NULL DEFAULT 'pending' CHECK (traversal_primary IN ('pending', 'successful', 'failed')),
    copy_status VARCHAR NOT NULL DEFAULT 'pending' CHECK (copy_status IN ('pending', 'in_progress', 'completed'))
    -- Dynamic columns: traversal_s1, traversal_s2, etc. added per configuration
);
```

**Key Fields:**
- `id` - Plain UUID identifier (no prefixes)
- `existence_map` - JSON map: `{"primary": true, "s1": true, "s2": false}`
- `traversal_primary`, `traversal_s1`, etc. - Per-world traversal status columns
- `copy_status` - Migration status tracking

## Indexes

The nodes table has indexes on:
- `parent_id` - For efficient children queries
- `type` - For filtering by node type
- `depth_level` - For depth-based queries
- `path` - For path-based lookups
- `traversal_primary`, `traversal_s1`, etc. - For per-world status queries
- `copy_status` - For migration status queries

## Performance Optimizations

### Vectorized Queries
The `GetParentAndChildren` method fetches both parent and all children in a single query:
```sql
SELECT * FROM nodes
WHERE (id = ? OR parent_id = ?)
  AND json_extract_string(existence_map, 'primary') = 'true'
ORDER BY CASE WHEN id = ? THEN 0 ELSE 1 END, type, name
```

### Conditional Updates
Traversal status updates skip already-successful nodes:
```sql
UPDATE nodes 
SET traversal_primary = ?
WHERE id = ? AND traversal_primary <> 'successful'
```

This reduces unnecessary writes and improves performance for large-scale traversals.

## Usage

The database layer is used internally by the SpectraFS implementation. It provides the foundation for all data persistence operations in the synthetic filesystem.
