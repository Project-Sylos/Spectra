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
- `GetNode(id)` - Retrieve node by ID
- `CreateFolder(parentID, name)` - Create new folder
- `UploadFile(parentID, name, data)` - Upload file with data processing
- `DeleteNode(id)` - Delete node by ID

#### Children Operations
- `ListChildren(parentID)` - List children with lazy generation
- `CheckChildrenExist(parentID)` - Check if children exist

#### System Operations
- `Reset()` - Clear all tables and recreate root
- `GetConfig()` - Get current configuration
- `GetTableInfo()` - Get table metadata
- `GetNodeCount(tableName)` - Count nodes in specific table

#### File Data Operations
- `GetFileData(id)` - Get file data and checksum

#### Status Operations
- `UpdateTraversalStatus(id, status)` - Update node traversal status

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
```

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
```go
// List children (with lazy generation)
result, err := fs.ListChildren("p-root")
if err != nil {
    log.Fatal(err)
}

// Create folder
folder, err := fs.CreateFolder("p-root", "new-folder")
if err != nil {
    log.Fatal(err)
}

// Upload file
file, err := fs.UploadFile("p-root", "test.txt", []byte("data"))
if err != nil {
    log.Fatal(err)
}

// Get node
node, err := fs.GetNode("p-root")
if err != nil {
    log.Fatal(err)
}
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
