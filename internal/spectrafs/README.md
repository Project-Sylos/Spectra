# SpectraFS Package

The SpectraFS package contains the core filesystem simulator logic. It orchestrates the database layer, generator, and configuration to provide the main filesystem operations.

## Structure

```
spectrafs/
├── spectrafs.go  # Core filesystem simulator implementation
├── file.go       # fs.File and fs.ReadDirFile implementations
├── fileinfo.go   # fs.FileInfo implementation
└── direntry.go   # fs.DirEntry implementation
```

## Core Responsibilities

- **Orchestration**: Coordinates between database, generator, and configuration layers
- **World-Aware Operations**: Filters nodes by world (primary, s1, s2, etc.)
- **Lazy Generation**: Generates children on-demand when requested
- **State Management**: Manages filesystem state and per-world traversal status
- **API Integration**: Provides the interface used by the SDK

## Key Features

### Optimized Single-Table Operations
- Vectorized queries fetch parent + children in one query
- World-based filtering via `ExistenceMap`
- Conditional traversal updates (skip already-successful nodes)
- Bulk inserts in single transactions

### Lazy Generation
- Children generated only when requested via `ListChildren()`
- Deterministic generation based on configuration and seed
- Efficient storage of generated structures with embedded existence information

### State Management
- Per-world traversal status tracking
- Node metadata management
- Path and depth level maintenance
- Migration status tracking via `CopyStatus`

## Core Operations

All operations use interface-based request structs for flexible lookup (by ID or by Path+TableName).

### Node Operations
- `GetNode(req)` - Retrieve node by ID or Path+World using NodeIdentifier
- `CreateFolder(req)` - Create new folder with ExistenceMap using ParentIdentifier
- `UploadFile(req)` - Create file node with data processing using ParentIdentifier
- `DeleteNode(req)` - Delete node by ID using NodeIdentifier
- `UpdateTraversalStatus(req)` - Update per-world traversal status using NodeIdentifier

### Children Operations
- `ListChildren(req)` - List children with lazy generation using ParentIdentifier
- World-aware filtering based on request context (defaults to "primary")

### System Operations
- `Reset()` - Clear nodes table and recreate single root
- `GetConfig()` - Get current configuration
- `GetTableInfo()` - Get world metadata
- `GetNodeCount(world)` - Count nodes in specific world
- `GetFileData(id)` - Generate and return file data with checksum
- `GetSecondaryTables()` - Get list of configured secondary worlds

### fs.FS Interface Support
- `NewSpectraFSWrapper(fs *SpectraFS, world string) *SpectraFSWrapper` - Creates an `fs.FS` wrapper bound to a specific world
- `SpectraFSWrapper` implements `fs.FS`, `fs.ReadFileFS`, `fs.ReadDirFS`, `fs.StatFS`, and `fs.GlobFS`
- Each world can be projected as a separate filesystem for compatibility with Go standard library and tools like Rclone

## ListChildren Logic (Optimized)

The core `ListChildren` operation is dramatically simplified with the single-table architecture:

1. **Single Vectorized Query**: Fetch parent + children in ONE query using `GetParentAndChildren(parentID, world)`
2. **Existence Check**: Verify parent exists in requested world
3. **Lazy Generation**: If no children exist, generate them in-memory
4. **Bulk Insert**: Insert all generated nodes in ONE transaction
5. **World Filtering**: Filter children by `ExistenceMap` for requested world
6. **Conditional Update**: Update traversal status (skips if already successful)
7. **Result Formatting**: Separate folders and files, return result

**Performance:** Reduced from 4-6 queries to 1-2 queries per operation.

## Configuration Integration

SpectraFS uses configuration for:
- Database connection settings
- Generation parameters (depth, counts, seed)
- Secondary table probabilities
- File data generation settings

## Error Handling

Comprehensive error handling for:
- Invalid node IDs
- Missing parent nodes
- Database operation failures
- Generation errors
- Configuration issues

## Request Interface System

SpectraFS operations use an interface-based request system (see `models/` subdirectory) that supports:

- **ID-based lookup**: Direct node/parent identification
- **Path-based lookup**: Lookup by path + table name
- **Type safety**: Interface validation ensures correct usage
- **Flexibility**: Users can pass any struct implementing the required interfaces

See `internal/spectrafs/models/README.md` for detailed documentation on the request system.

## Usage

SpectraFS is the core implementation used by the SDK. It provides the main filesystem operations that are exposed through the public API.

## Example

```go
// Initialize SpectraFS
fs, err := spectrafs.NewSpectraFS(config)

// List children (with lazy generation) - by ID
result, err := fs.ListChildren(&models.ListChildrenRequest{
    ParentID: "root",  // Plain ID, no prefixes
})

// List children - by path with world specification
result, err := fs.ListChildren(&models.ListChildrenRequest{
    ParentPath: "/",
    TableName:  "s1",  // TableName used to specify world
})

// Create folder (will get ExistenceMap based on probabilities)
folder, err := fs.CreateFolder(&models.CreateFolderRequest{
    ParentID: "root",
    Name:     "new-folder",
})
// folder.ExistenceMap might be: {"primary": true, "s1": true, "s2": false}

// Upload file
file, err := fs.UploadFile(&models.UploadFileRequest{
    ParentID: "root",
    Name:     "test.txt",
    Data:     []byte("data"),
})

// Get node by ID (plain UUID)
node, err := fs.GetNode(&models.GetNodeRequest{
    ID: "abc123-...",  // Plain UUID
})

// Get node by path in specific world
node, err := fs.GetNode(&models.GetNodeRequest{
    Path:      "/folder/file.txt",
    TableName: "s1",  // Retrieve from s1 world
})

// Delete node
err = fs.DeleteNode(&models.DeleteNodeRequest{
    ID: "abc123-...",
})

// Update traversal status for specific world
err = fs.UpdateTraversalStatus(&models.UpdateTraversalStatusRequest{
    ID:     "abc123-...",
    Status: "successful",
})

// Create fs.FS wrapper for a specific world
fsWrapper := spectrafs.NewSpectraFSWrapper(fs, "primary")

// Use with standard library functions
import "io/fs"

data, err := fs.ReadFile(fsWrapper, "folder/file.txt")
entries, err := fs.ReadDir(fsWrapper, "folder")
info, err := fs.Stat(fsWrapper, "folder/file.txt")
```

## fs.FS Interface Implementation

SpectraFS provides a complete implementation of Go's standard library `fs.FS` interface, enabling compatibility with tools like Rclone and standard library functions.

### Key Features

- **World-Aware Projection**: Each world (primary, s1, s2, etc.) can be projected as a separate filesystem
- **Deterministic File Data**: File data is generated deterministically based on node ID, ensuring consistency
- **Extended Interfaces**: Implements `fs.ReadFileFS`, `fs.ReadDirFS`, `fs.StatFS`, and `fs.GlobFS` for optimized operations
- **Path Handling**: Properly handles root path "/" and normalizes paths according to `fs.FS` conventions
- **Error Handling**: Uses `fs.PathError` for proper error reporting

### Implementation Details

The `SpectraFSWrapper` struct wraps a `SpectraFS` instance and binds it to a specific world. When operations are performed through the wrapper, they automatically filter by the bound world's `existence_map`.

- **File Data Generation**: File data is generated on-demand using a deterministic seed derived from the node ID
- **Directory Listings**: Directories trigger lazy generation if children don't exist, then filter by world
- **Path Validation**: Uses `fs.ValidPath` for path validation, with special handling for root path "/"
