# Database Package

The database package provides the data persistence layer for Spectra using BoltDB. It implements a unified single-bucket architecture with per-world existence tracking for maximum performance.

## Structure

```
db/
├── db.go      # Main database operations and CRUD
└── schema.go  # Bucket initialization and verification
```

## Single-Bucket Architecture

### Unified `nodes` Bucket
- All nodes stored in a single bucket with plain UUID IDs as keys
- Each node stored as JSON-serialized `types.Node` struct
- `existence_map` JSON field tracks which "worlds" (primary, s1, s2, etc.) each node exists in
- Optimized for minimal database round trips

### Index Buckets
- `index_parent_id`: Key format `{parentID}|{nodeID}` for efficient parent-child lookups
- `index_path`: Key format `{path}` → value `{nodeID}` for path-based lookups
- `index_parent_path`: Key format `{parentPath}|{nodeID}` for parent path queries

### World-Based Filtering
- Nodes are filtered by world in Go code after deserialization
- Each node can exist in multiple worlds simultaneously
- Filtering checks `existence_map[world]` boolean value

## Key Features

- **Single Vectorized Queries**: Fetch parent + children in one operation
- **World-Aware Filtering**: Efficient Go-based world filtering
- **Bulk Operations**: Transaction-based bulk inserts for performance
- **Indexed Queries**: Optimized index buckets for common query patterns
- **ACID Compliance**: BoltDB provides ACID transactions automatically

## Core Operations

### Node Management
- `InsertNode(node)` - Insert node into nodes bucket and update all indexes
- `GetNodeByID(id)` - Retrieve node by ID from nodes bucket
- `GetNodeByPath(path, world)` - Retrieve node by path using index_path bucket
- `DeleteNode(id)` - Delete node from nodes bucket and all indexes
- `BulkInsertNodes(nodes)` - Insert multiple nodes in one transaction

### Children Operations
- `GetChildrenByParentID(parentID, world)` - Get children filtered by world using index_parent_id
- `GetParentAndChildren(parentID, world)` - Get parent + children in ONE operation (optimized)
- `CheckChildrenExist(parentID, world)` - Check if parent has children in world

### System Operations
- `InitializeBuckets()` - Create all required buckets
- `CreateRootNode()` - Create single root node with existence in all worlds
- `DeleteAllNodes()` - Clear nodes bucket and all index buckets
- `GetTableInfo()` - Get world metadata
- `GetNodeCount(world)` - Count nodes in specific world

## Bucket Structure

### `nodes` Bucket
- **Key**: Node ID (UUID string)
- **Value**: JSON-serialized `types.Node` struct

### `index_parent_id` Bucket
- **Key**: `{parentID}|{nodeID}` (e.g., `"root|abc-123"`)
- **Value**: Empty (key contains all information)

### `index_path` Bucket
- **Key**: Node path (e.g., `"/folder/file.txt"`)
- **Value**: Node ID (UUID string)

### `index_parent_path` Bucket
- **Key**: `{parentPath}|{nodeID}` (e.g., `"/folder|abc-123"`)
- **Value**: Empty (key contains all information)

## Node Structure

Each node is stored as a JSON-serialized `types.Node`:

```go
type Node struct {
    ID           string          // UUID identifier
    ParentID     string          // UUID parent reference
    Name         string          // Display name
    Path         string          // Relative path
    ParentPath   string          // Parent path
    Type         string          // "folder" or "file"
    DepthLevel   int             // BFS-style depth index
    Size         int64           // File size (0 for folders)
    LastUpdated  time.Time       // Synthetic timestamp
    Checksum     *string         // SHA256 checksum (NULL for folders)
    ExistenceMap map[string]bool // JSON: {"primary": true, "s1": true, "s2": false}
}
```

## Performance Optimizations

### Vectorized Queries
The `GetParentAndChildren` method fetches both parent and all children efficiently:
1. Fetch parent node by ID from `nodes` bucket
2. Use `index_parent_id` bucket with prefix scan to find all children
3. Filter by world in Go after deserialization
4. Sort results (parent first, then by type and name)

### Index Prefix Scans
Parent-child queries use efficient prefix scans on `index_parent_id`:
- Prefix: `{parentID}|`
- All keys starting with this prefix represent children of that parent
- BoltDB's ordered key structure makes this very efficient

### Bulk Operations
`BulkInsertNodes` performs all inserts in a single BoltDB transaction:
- All nodes inserted atomically
- All indexes updated in the same transaction
- Automatic rollback on any error

## Usage

The database layer is used internally by the SpectraFS implementation. It provides the foundation for all data persistence operations in the synthetic filesystem.
