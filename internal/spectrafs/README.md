# SpectraFS Package

The SpectraFS package contains the core filesystem simulator logic. It orchestrates the database layer, generator, and configuration to provide the main filesystem operations.

## Structure

```
spectrafs/
└── spectrafs.go  # Core filesystem simulator implementation
```

## Core Responsibilities

- **Orchestration**: Coordinates between database, generator, and configuration layers
- **Multi-Table Management**: Handles primary and secondary table operations
- **Lazy Generation**: Generates children on-demand when requested
- **State Management**: Manages filesystem state and traversal status
- **API Integration**: Provides the interface used by the SDK

## Key Features

### Multi-Table Operations
- Automatic table detection based on node ID prefixes
- Primary and secondary table coordination
- Secondary existence map management
- Probability-based secondary node generation

### Lazy Generation
- Children generated only when requested via `ListChildren()`
- Deterministic generation based on configuration and seed
- Efficient storage of generated structures

### State Management
- Traversal status tracking
- Node metadata management
- Path and depth level maintenance

## Core Operations

### Node Operations
- `GetNode(id)` - Retrieve node from appropriate table
- `CreateFolder(parentID, name)` - Create new folder with multi-table support
- `UploadFile(parentID, name, data)` - Create file node with data processing
- `DeleteNode(id)` - Delete node from appropriate table

### Children Operations
- `ListChildren(parentID)` - List children with lazy generation
- `CheckChildrenExist(parentID)` - Check if children exist

### System Operations
- `Reset()` - Clear all tables and recreate root
- `GetConfig()` - Get current configuration
- `GetTableInfo()` - Get table metadata
- `GetNodeCount(tableName)` - Count nodes in specific table

## ListChildren Logic

The core `ListChildren` operation implements the sophisticated multi-table logic:

1. **Table Detection**: Determine which table contains the parent node
2. **Children Check**: Query the parent's table for existing children
3. **Lazy Generation**: If no children exist, generate them using the generator
4. **Multi-Table Insertion**: Insert primary nodes and probability-based secondary nodes
5. **Existence Map Update**: Update primary parent's secondary existence map
6. **Result Retrieval**: Return children from the appropriate table

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

## Usage

SpectraFS is the core implementation used by the SDK. It provides the main filesystem operations that are exposed through the public API.

## Example

```go
// Initialize SpectraFS
fs, err := spectrafs.NewSpectraFS(config)

// List children (with lazy generation)
result, err := fs.ListChildren("p-root")

// Create folder
folder, err := fs.CreateFolder("p-root", "new-folder")

// Upload file
file, err := fs.UploadFile("p-root", "test.txt", []byte("data"))
```
