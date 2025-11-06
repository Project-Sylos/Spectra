# SDK Package

The SDK package provides the public interface for Spectra. It wraps the internal implementation and exposes a clean, stable API for external use.

## Structure

```
sdk/
└── sdk.go  # Public SDK interface and type re-exports
```

## Design Principles

- **Stable API**: Public interface that won't change without version bumps
- **Clean Abstraction**: Hides internal implementation details
- **Type Safety**: Strongly typed interface with proper error handling
- **Comprehensive Coverage**: Exposes all necessary functionality

## Core Interface

### SpectraFS
The main SDK interface that provides all filesystem operations:

```go
type SpectraFS struct {
    impl *spectrafs.SpectraFS
}
```

### Core Operations

#### Node Operations
- `GetNode(req *GetNodeRequest)` - Retrieve node by ID or Path+TableName
- `CreateFolder(req *CreateFolderRequest)` - Create new folder
- `UploadFile(req *UploadFileRequest)` - Upload file with data processing
- `DeleteNode(req *DeleteNodeRequest)` - Delete node by ID or Path+TableName

#### Children Operations
- `ListChildren(req *ListChildrenRequest)` - List children with lazy generation (supports ID or Path+TableName lookup)
- `CheckChildrenExist(parentID)` - Check if children exist

#### System Operations
- `Reset()` - Clear all tables and recreate root
- `GetConfig()` - Get current configuration
- `GetTableInfo()` - Get table metadata
- `GetNodeCount(tableName)` - Count nodes in specific table

#### File Data Operations
- `GetFileData(id)` - Get file data and checksum

#### Status Operations
- `UpdateTraversalStatus(req *UpdateTraversalStatusRequest)` - Update node traversal status (supports ID or Path+TableName lookup)

#### fs.FS Interface Operations
- `AsFS(world string) fs.FS` - Returns an `fs.FS` instance bound to a specific world for compatibility with Go standard library and tools like Rclone
- `AsFSWithDefaults() fs.FS` - Returns an `fs.FS` instance using the "primary" world (convenience method)

## Type Re-exports

The SDK re-exports commonly used types for convenience:

```go
type (
    Config      = types.Config
    Node        = types.Node
    ListResult  = types.ListResult
    APIResponse = types.APIResponse
    TableInfo   = types.TableInfo
)

// Request types
type (
    GetNodeRequest              = models.GetNodeRequest
    ListChildrenRequest         = models.ListChildrenRequest
    CreateFolderRequest         = models.CreateFolderRequest
    UploadFileRequest           = models.UploadFileRequest
    DeleteNodeRequest           = models.DeleteNodeRequest
    UpdateTraversalStatusRequest = models.UpdateTraversalStatusRequest
)
```

## Request Types

All request types support flexible lookup methods through a clean interface-based design:

- **ID-based lookup**: Provide the `ID` field (or `ParentID` for parent operations)
- **Path-based lookup**: Provide `Path` (or `ParentPath`) + `TableName` together

**Validation Rules:**
- For operations requiring a parent: Either `ParentID` OR (`ParentPath` + `TableName`) must be provided
- For node operations: Either `ID` OR (`Path` + `TableName`) must be provided
- When using path-based lookup, `TableName` is required and must be one of: `"primary"`, `"s1"`, `"s2"`, etc.

**Design Philosophy:**
The request structs use a simple struct literal syntax - no embedding or complex initialization required. Each request struct implements the appropriate interfaces (`NodeIdentifier`, `ParentIdentifier`, `NamedRequest`, etc.) for type safety and validation.

## Usage

### Initialization
```go
// Load configuration
config, err := config.LoadFromFile("configs/default.json")
if err != nil {
    log.Fatal(err)
}

// Create SpectraFS instance
fs, err := sdk.NewSpectraFS(config)
if err != nil {
    log.Fatal(err)
}
```

### Basic Operations

#### ID-Based Operations
```go
// List children by parent ID (with lazy generation)
result, err := fs.ListChildren(&sdk.ListChildrenRequest{
    ParentID: "root",
})
if err != nil {
    log.Fatal(err)
}

// Create folder
folder, err := fs.CreateFolder(&sdk.CreateFolderRequest{
    ParentID: "root",
    Name:     "new-folder",
})
if err != nil {
    log.Fatal(err)
}

// Upload file
file, err := fs.UploadFile(&sdk.UploadFileRequest{
    ParentID: "root",
    Name:     "test.txt",
    Data:     []byte("data"),
})
if err != nil {
    log.Fatal(err)
}

// Get node by ID
node, err := fs.GetNode(&sdk.GetNodeRequest{
    ID: "root",
})
if err != nil {
    log.Fatal(err)
}

// Delete node
err = fs.DeleteNode(&sdk.DeleteNodeRequest{
    ID: folder.ID,
})
if err != nil {
    log.Fatal(err)
}

// Update traversal status
err = fs.UpdateTraversalStatus(&sdk.UpdateTraversalStatusRequest{
    ID:     node.ID,
    Status: sdk.StatusSuccessful,
})
```

#### Path-Based Operations
```go
// List children by path and table name
result, err := fs.ListChildren(&sdk.ListChildrenRequest{
    ParentPath: "/",
    TableName:  "primary",
})

// Get node by path
node, err := fs.GetNode(&sdk.GetNodeRequest{
    Path:      "/folder1",
    TableName: "s1",
})

// Create folder using path lookup
folder, err := fs.CreateFolder(&sdk.CreateFolderRequest{
    ParentPath: "/",
    TableName:  "primary",
    Name:       "new-folder",
})

// Upload file using path lookup
file, err := fs.UploadFile(&sdk.UploadFileRequest{
    ParentPath: "/folder1",
    TableName:  "primary",
    Name:       "data.txt",
    Data:       []byte("content"),
})

// Delete node by path
err = fs.DeleteNode(&sdk.DeleteNodeRequest{
    Path:      "/folder1/subfolder",
    TableName: "primary",
})
```

### System Operations
```go
// Get configuration
config := fs.GetConfig()

// Get table information
tables, err := fs.GetTableInfo()
if err != nil {
    log.Fatal(err)
}

// Reset filesystem
err = fs.Reset()
if err != nil {
    log.Fatal(err)
}
```

### fs.FS Interface Usage

SpectraFS implements Go's standard library `fs.FS` interface, enabling compatibility with tools like Rclone and standard library functions. Each world can be projected as a separate filesystem:

```go
// Create SpectraFS instance
fs, _ := sdk.New("configs/default.json")

// Get fs.FS for primary world
primaryFS := fs.AsFS("primary")

// Get fs.FS for secondary world
s1FS := fs.AsFS("s1")

// Use with standard library functions
import "io/fs"

// Read a file
data, err := fs.ReadFile(primaryFS, "folder/file.txt")

// Read directory entries
entries, err := fs.ReadDir(primaryFS, "folder")

// Stat a file
info, err := fs.Stat(primaryFS, "folder/file.txt")

// Walk directory tree
fs.WalkDir(primaryFS, ".", func(path string, d fs.DirEntry, err error) error {
    // Process each entry
    return nil
})

// Use with glob patterns
matches, err := fs.Glob(primaryFS, "**/*.txt")
```

**World Projection:**
Each world (primary, s1, s2, etc.) is projected as its own separate filesystem. This allows tools like Rclone to treat each world as an independent remote, enabling comparison and synchronization between different world projections. All wrappers share the same underlying SpectraFS instance, ensuring consistency while allowing world-specific filtering.

## Error Handling

All SDK methods return proper Go errors that should be handled by the caller. The SDK provides clear error messages for common failure scenarios.

## Thread Safety

The SDK is designed to be thread-safe and can be used concurrently from multiple goroutines.

## Command-Line Applications

The SDK is used by two main command-line applications:

- **SDK Demo** (`main.go`): Demonstrates SDK functionality and testing
- **API Server** (`cmd/api/main.go`): Production HTTP server exposing SDK via REST API

See the main project README for usage instructions.

## Versioning

The SDK follows semantic versioning. Breaking changes will result in a major version bump, while new features and bug fixes will result in minor and patch version bumps respectively.
