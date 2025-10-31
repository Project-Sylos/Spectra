# Spectra

**Spectra** is a synthetic filesystem simulator designed for testing traversal, migration, and data-mirroring pipelines. It generates and serves an artificial directory tree through a clean API interface, allowing deterministic or randomized environments for benchmarking and validation.

---

## Overview

Spectra behaves like a mock filesystem. Instead of relying on actual disk I/O, it **procedurally generates** folders and files based on configuration parameters (min/max depth, file counts, folder counts, etc.). Each generated node is **persisted to a DuckDB database** with multi-table support, enabling reproducible state across test runs.

This design allows engineers to stress-test migration engines (such as Sylos) without interacting with real file systems or cloud APIs.

---

## Key Features

* **Procedural Generation:** Randomly creates folder and file hierarchies using a seeded RNG for reproducibility.
* **Deterministic Mode:** When given a seed, the same folder structure is regenerated identically across runs.
* **Unified Single-Table Architecture:** One table with world-based existence tracking for optimal performance.
* **RESTful API Interface:** Exposes a comprehensive HTTP API with folder/file CRUD operations.
* **DuckDB Persistence:** Each node is stored in a local DuckDB database with metadata for path, type, size, timestamps, etc.
* **Configurable Complexity:** Control depth, fan-out, file size ranges, and naming schemes through the config file or API.
* **Instant Cleanup:** Simple teardown between tests — delete the DuckDB file and regenerate.
* **Plain UUID IDs:** Simple unique identifiers without prefixes.
* **Optimized Queries:** Vectorized queries reduce database round trips by 3-4x.

---

## Architecture

### Single-Table Design

Spectra uses an optimized single-table architecture for maximum performance:

- **Unified `nodes` Table**: All nodes stored in one table with plain UUID IDs
- **Existence Map**: JSON column tracking which "worlds" (primary, s1, s2, etc.) each node exists in
- **Dynamic Traversal Columns**: Per-world traversal status columns (`traversal_primary`, `traversal_s1`, etc.)
- **World-Based Filtering**: Queries filter nodes by world using JSON operations on `existence_map`

### Probability-Based Generation

When generating children:
1. Generate nodes based on configuration rules
2. For each node, roll dice against world probabilities
3. Populate `existence_map` with results: `{"primary": true, "s1": true, "s2": false}`
4. Insert all nodes in a single bulk operation

### World-Aware Operations

The system filters nodes by "world" context:
- Default world is "primary"
- Operations can specify target world (s1, s2, etc.)
- Nodes can exist in multiple worlds simultaneously
- Traversal status tracked independently per world

---

## Tech Stack

| Component                                               | Purpose                                                |
| ------------------------------------------------------- | ------------------------------------------------------ |
| **Go (Golang)**                                         | Core implementation language                           |
| **DuckDB**                                              | Lightweight embedded SQL database for node persistence |
| **Chi Router**                                          | HTTP router for RESTful API endpoints                 |
| **Google UUID**                                         | UUID generation for consistent node identification     |
| **Go's `math/rand`**                                    | Deterministic random generation with seeding           |
| **Go standard library (`os`, `path/filepath`, `time`)** | Utility functions and path normalization               |

---

## Project Structure

```
Spectra/
├── cmd/                       # Command-line applications
│   └── api/                   # API server application
│       └── main.go           # HTTP API server entry point
├── configs/                   # Configuration files
│   └── default.json          # Default configuration
├── internal/                  # Internal implementation
│   ├── api/                  # HTTP API layer
│   │   ├── handlers/         # Endpoint handlers
│   │   ├── middleware/       # HTTP middleware
│   │   ├── models/           # Request/response models
│   │   ├── router.go         # Route configuration
│   │   └── server.go         # HTTP server
│   ├── config/               # Configuration management
│   ├── db/                   # Database layer
│   ├── generator/            # Procedural generation
│   ├── spectrafs/            # Core filesystem logic
│   └── types/                # Type definitions
├── sdk/                      # Public SDK interface
├── dev_setup_scripts/        # Development setup scripts
├── main.go                   # SDK demo application
└── go.mod                    # Go module definition
```

---

## Core Concepts

### Node Generation

Spectra represents all nodes as entries in a unified DuckDB table:

| Column               | Type      | Description                                              |
| -------------------- | --------- | -------------------------------------------------------- |
| `id`                 | string    | Plain UUID identifier                                    |
| `parent_id`          | string    | UUID of parent folder                                    |
| `name`               | string    | Display name                                             |
| `path`               | string    | Relative path (root-relative, not absolute)              |
| `type`               | string    | `"folder"` or `"file"`                                   |
| `depth_level`        | int       | BFS-style depth index                                    |
| `size`               | int64     | File size (0 for folders)                                |
| `last_updated`       | timestamp | Synthetic timestamp                                      |
| `checksum`           | string    | SHA256 checksum (for files only)                         |
| `existence_map`      | JSON      | Map tracking world existence: `{"primary":true,"s1":true}` |
| `traversal_primary`  | string    | Primary world traversal status                           |
| `traversal_s1`, etc. | string    | Per-world traversal status (dynamic columns)             |
| `copy_status`        | string    | Migration status: `"pending"`, `"in_progress"`, `"completed"` |

### Example Behavior

Given a config:

```json
{
  "seed": {
    "max_depth": 4,
    "min_folders": 1,
    "max_folders": 3,
    "min_files": 2,
    "max_files": 5,
    "seed": 42,
    "db_path": "./spectra.db"
  },
  "api": {
    "host": "localhost",
    "port": 8086
  },
  "secondary_tables": {
    "s1": 0.7,
    "s2": 0.3
  }
}
```

Spectra will generate a reproducible tree up to 4 levels deep, where each folder contains between 1–3 subfolders and 2–5 files. Each node will have a 70% chance of existing in the s1 world and a 30% chance of existing in the s2 world, tracked in its `existence_map`.

---

## API Interface

### RESTful Endpoints

#### Folder Operations
- `POST /api/v1/folder/list` - List children with table detection
- `POST /api/v1/folder/create` - Create new folder
- `GET /api/v1/folder/{id}` - Get folder metadata

#### File Operations
- `POST /api/v1/file/upload` - Upload file with data processing
- `GET /api/v1/file/{id}` - Get file metadata
- `GET /api/v1/file/{id}/data` - Get file data + checksum

#### Node Operations
- `GET /api/v1/node/{id}` - Get any node metadata
- `DELETE /api/v1/node/{id}` - Delete node

#### System Operations
- `POST /api/v1/reset` - Reset all tables
- `GET /api/v1/config` - Get current configuration
- `GET /api/v1/tables` - Get table information
- `GET /api/v1/tables/{tableName}/count` - Get table row count

### SDK Interface

```go
type SpectraFS struct {
    // Core operations
    ListChildren(req *ListChildrenRequest) (*ListResult, error)
    GetNode(req *GetNodeRequest) (*Node, error)
    CreateFolder(req *CreateFolderRequest) (*Node, error)
    UploadFile(req *UploadFileRequest) (*Node, error)
    DeleteNode(req *DeleteNodeRequest) error
    
    // System operations
    Reset() error
    GetConfig() *Config
    GetTableInfo() ([]TableInfo, error)
    GetNodeCount(tableName string) (int, error)
}
```

#### Request Types

All CRUD operations use simple request structs that support flexible lookup methods through a clean interface-based design:

**GetNodeRequest** - Retrieve a node by ID or Path+World
```go
// By ID (plain UUID)
req := &sdk.GetNodeRequest{
    ID: "root",
}
// OR by Path in specific world
req := &sdk.GetNodeRequest{
    Path:      "/",
    TableName: "s1",  // TableName specifies the world
}
node, err := fs.GetNode(req)
```

**ListChildrenRequest** - List children of a parent node
```go
// By ParentID (plain UUID)
req := &sdk.ListChildrenRequest{
    ParentID: "root",
}
// OR by ParentPath in specific world
req := &sdk.ListChildrenRequest{
    ParentPath: "/",
    TableName:  "s1",  // Defaults to "primary" if not specified
}
result, err := fs.ListChildren(req)
```

**CreateFolderRequest** - Create a new folder
```go
req := &sdk.CreateFolderRequest{
    ParentID: "root",  // OR ParentPath + TableName
    Name:     "new-folder",
}
folder, err := fs.CreateFolder(req)
// folder.ExistenceMap will contain world existence based on probabilities
```

**UploadFileRequest** - Upload a file
```go
req := &sdk.UploadFileRequest{
    ParentID: "root",  // OR ParentPath + TableName
    Name:     "test.txt",
    Data:     []byte("file content"),
}
file, err := fs.UploadFile(req)
```

**DeleteNodeRequest** - Delete a node
```go
req := &sdk.DeleteNodeRequest{
    ID: "abc123-...",  // Plain UUID
}
err := fs.DeleteNode(req)
```

**UpdateTraversalStatusRequest** - Update node traversal status for a world
```go
req := &sdk.UpdateTraversalStatusRequest{
    ID:     "abc123-...",  // Plain UUID
    Status: "successful",  // "pending", "successful", or "failed"
}
err := fs.UpdateTraversalStatus(req)
```

**Design Note:** Each request struct implements the appropriate interfaces (`NodeIdentifier`, `ParentIdentifier`, etc.) for compile-time type safety and runtime validation. Users can pass any struct that implements these interfaces.

---

## Usage

### Running the Applications

#### SDK Demo Application
```bash
# Run the SDK demonstration
go run main.go

# Use custom configuration
go run main.go -config configs/custom.json
```

#### API Server
```bash
# Start the HTTP API server with default configuration
go run cmd/api/main.go

# Start with custom configuration
go run cmd/api/main.go configs/custom.json
```

### Example API Calls

#### List Children
```bash
curl -X POST http://localhost:8086/api/v1/folder/list \
  -H "Content-Type: application/json" \
  -d '{"parent_id": "root"}'
```

#### Create Folder
```bash
curl -X POST http://localhost:8086/api/v1/folder/create \
  -H "Content-Type: application/json" \
  -d '{"parent_id": "root", "name": "new-folder"}'
```

#### Upload File
```bash
curl -X POST http://localhost:8086/api/v1/file/upload \
  -H "Content-Type: application/json" \
  -d '{"parent_id": "root", "name": "test.txt", "data": "SGVsbG8gV29ybGQ="}'
```

---

## Development Setup

### Windows Setup
Run the provided PowerShell script to set up the development environment:

```powershell
.\dev_setup_scripts\windows.ps1
```

This script will:
- Install MSYS2 and GCC for CGO support
- Set up DuckDB binaries
- Configure environment variables
- Install Go dependencies

**Note**: Spectra uses `go-duckdb v1.7.0` and `apache/arrow/go/v14 v14.0.2` for Windows compatibility. These versions are known to work well with the Windows CGO setup.

### Manual Setup
1. Install Go 1.24.2 or later
2. Install DuckDB with CGO support
3. Run `go mod tidy` to install dependencies
4. Build applications:
   ```bash
   # Build SDK demo
   go build -o bin/spectra-demo main.go
   
   # Build API server
   go build -o bin/spectra-api cmd/api/main.go
   ```

---

## Command-Line Applications

Spectra provides two main command-line applications:

### SDK Demo (`main.go`)
A demonstration application that showcases the Spectra SDK functionality:
- Loads configuration and initializes SpectraFS
- Demonstrates table information and node generation
- Shows multi-table operations and secondary table counts
- Performs a complete reset operation
- Perfect for testing and understanding the SDK

### API Server (`cmd/api/main.go`)
A production-ready HTTP server that exposes the Spectra filesystem via RESTful API:
- Starts HTTP server on configurable host and port
- Provides all CRUD operations via REST endpoints
- Includes graceful shutdown and timeout handling
- Supports CORS and proper error responses
- Ideal for integration testing and production use

---

## Use Cases

* **Migration Engine Testing:** Validate traversal and BFS logic with reproducible data.
* **Performance Benchmarks:** Measure traversal throughput without real I/O.
* **Integration Testing:** Simulate different storage backends through the same API shape.
* **Chaos Simulation:** Test rate limiting, throttling, or transient "missing node" scenarios.
* **Multi-Source Testing:** Test migration scenarios with multiple data sources and probability-based data distribution.

---

## Configuration

The configuration file supports three main sections:

- **`seed`**: Controls procedural generation parameters
- **`api`**: Configures HTTP server settings
- **`secondary_tables`**: Defines secondary table probabilities

See `configs/default.json` for a complete example.

---

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

---

## License

This project is part of the Sylos ecosystem and follows the same licensing terms.