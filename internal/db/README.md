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
- `index_path`: Key format `{path}` → value `{nodeID}` for path-based lookups
- `index_parent_path`: Key format `{parentPath}|{nodeID}` for parent path queries

### Parent-Child Relationships
- Each node maintains a `ChildIDs []string` array of direct children
- O(1) lookup time for retrieving children (no bucket scanning required)

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
- `GetChildrenByParentID(parentID, world)` - Get children filtered by world using parent's ChildIDs array
- `GetParentAndChildren(parentID, world)` - Get parent + children in ONE operation (O(1) per child)
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
- **Value**: JSON-serialized `types.Node` struct (includes ChildIDs array)

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
    ChildIDs     []string        // Array of child node IDs (O(1) lookup)
}
```

## Performance Optimizations

### O(1) Parent-Child Lookups
The `GetParentAndChildren` method fetches both parent and all children using direct lookups:
1. Fetch parent node by ID from `nodes` bucket (O(1))
2. Read parent's `ChildIDs` array
3. Fetch each child by ID from `nodes` bucket (O(1) per child)
4. Filter by world in Go after deserialization
5. Sort results (parent first, then by type and name)

**Performance**: O(c) where c = number of children, vs. O(n) prefix scanning where n = total nodes

### ChildIDs Array Maintenance
Parent-child relationships are maintained in the parent node's `ChildIDs` array:
- Insert operations append to parent's ChildIDs
- Delete operations remove from parent's ChildIDs
- No separate index bucket required
- Atomic updates within node write transactions

### Bulk Operations
`BulkInsertNodes` performs all inserts in a single BoltDB transaction:
- All nodes inserted atomically
- All indexes updated in the same transaction
- Automatic rollback on any error

## Usage

The database layer is used internally by the SpectraFS implementation. It provides the foundation for all data persistence operations in the synthetic filesystem.
