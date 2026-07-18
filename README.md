# Spectra

**Spectra** is a synthetic filesystem simulator designed for testing traversal, migration, and data-mirroring pipelines. It generates and serves an artificial directory tree through a clean API interface, allowing deterministic or randomized environments for benchmarking and validation.

---

## Overview

Spectra behaves like a mock filesystem. Instead of relying on actual disk I/O, it **procedurally generates** folders and files from configuration (depth, fanout, seed). You can run it in **persistent** mode (BoltDB-backed, lazy generation) or **ephemeral** mode (no DB; children generated on the fly from path and depth). Both modes use deterministic node IDs and multi-world support for reproducible tests.

This design allows engineers to stress-test migration engines (such as Sylos) without interacting with real file systems or cloud APIs.

---

## Recent Changes & Migration

⚠️ **Version 2.0 introduces breaking changes to the configuration format and API signatures.**

### What Changed
1. **Fanout Configuration** (Breaking): Replaced uniform `min_folders`/`max_folders` with weighted distribution
   - Old: `"min_folders": 1, "max_folders": 3`
   - New: `"max_folders": 100, "folder_backoff_factor": 0.5, "folder_depth_decay_factor": 0.8`

2. **Node Identifiers**: Deterministic IDs (no UUID/ULID)
   - Root folder: `"root"`; all other nodes: `"spc:"` + 32-char hex (FNV128a of path and type)
   - Same path and type yield the same ID across all worlds; existing DBs with old IDs need regeneration

3. **Database API** (Breaking): `db.New()` and `db.Flush()` signatures changed
   - `db.New()` now requires `enableCache bool` parameter
   - `db.Flush()` accepts multiple node IDs and returns `bool`

4. **Node Structure**: Added `child_ids` field for O(1) parent-child lookups
   - Old databases won't have this field (regeneration required)

### Migration Guide
See [`INTEGRATION_GUIDE.md`](INTEGRATION_GUIDE.md) for detailed migration instructions, including:
- Configuration file updates
- API signature changes
- Performance tuning recommendations
- Cache configuration guidelines

**Quick Migration**: Update your config file to use the new fanout fields (see Configuration section below), delete existing `.db` files, and regenerate. The new distribution will produce more realistic filesystem structures.

---

## Key Features

* **Procedural Generation:** Randomly creates folder and file hierarchies using a seeded RNG for reproducibility.
* **Deterministic Mode:** When given a seed, the same folder structure is regenerated identically across runs.
* **Heavy-Tailed Distribution:** Realistic fanout using logarithmic buckets with exponential decay (mostly small directories, occasionally huge ones).
* **Unified Single-Bucket Architecture:** One bucket with world-based existence tracking for optimal performance.
* **RESTful API Interface:** Exposes a comprehensive HTTP API with folder/file CRUD operations.
* **Go fs.FS Interface:** Implements Go's standard library `fs.FS` interface for compatibility with tools like Rclone.
* **FUSE Mount:** Mount Spectra worlds as local directories (Linux/macOS) for Finder, Explorer, and shell access with lazy generation.
* **Optional Auth Tokens:** Per-world access/refresh tokens (opt-in) with sidecar persistence for migration testing.
* **BoltDB Persistence:** Each node is stored in a local BoltDB key-value database with metadata for path, type, size, timestamps, etc.
* **Configurable Complexity:** Control depth, fan-out distribution, file size ranges, and naming schemes through the config file or API.
* **Instant Cleanup:** Simple teardown between tests — delete the BoltDB database file and regenerate.
* **Deterministic Node IDs:** Stable IDs from path and type (`root` or `spc:` + hex); world-agnostic and reproducible.
* **Optimized Queries:** Vectorized queries reduce database round trips by 3-4x.
* **Write-Ahead Buffering:** Batched write operations with ordered insert/update queues for high throughput.
---

## Architecture

### Single-Bucket Design

Spectra uses an optimized single-bucket architecture for maximum performance:

- **Unified `nodes` Bucket**: All nodes stored in one bucket with string keys (`root` or `spc:` + hex, deterministic from path and type)
- **Existence Map**: JSON field tracking which "worlds" (primary, s1, s2, etc.) each node exists in
- **Direct Child References**: Each node stores `child_ids` array for O(1) children retrieval
- **Index Buckets**: Separate buckets for efficient lookups by path and parent_path
  - `index_path`: Maps path → node ID
  - `index_parent_path`: Maps parent_path → node IDs
  - ~~`index_parent_id`~~: Removed (replaced by `child_ids` for O(1) lookups)
- **World-Based Filtering**: Filtering done in Go after deserializing nodes, checking `existence_map` field

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

### Persistent vs Ephemeral Mode

Config **mode** selects the implementation:
- **Persistent** (default): BoltDB-backed; lazy generation; write-ahead buffer and optional cache; fs.FS support. ListChildren by parent ID or path+world; depth optional.
- **Ephemeral**: No database; children generated on the fly from path and depth. ListChildren **requires** parent_path and depth. Optional **diverging_tree_mode** seeds with world//path so each world gets a different tree shape. Ideal for traversal/copy tests without persistence.

### Internal Architecture & Performance

#### Write-Ahead Buffer (Output Buffer)

Spectra uses a sophisticated write-ahead buffer system to achieve high-throughput write performance:

- **Dual-Queue System**: Separate queues for insert and update operations
  - Insert operations execute first (establish nodes)
  - Update operations execute second (modify existing data)
  - Ensures referential integrity for parent-child relationships
  
- **Batched Parent Updates**: O(1) per parent instead of O(n²)
  - Collects all child additions/removals per parent during a batch
  - Applies changes to each parent's `ChildIDs` array once at flush time
  - Critical for bulk insert performance with large fanouts

- **Configurable Buffering**: Default 10,000 operations or 5-second interval
  - Automatic flush when batch size reached
  - Periodic flush via background goroutine
  - Graceful flush on shutdown

- **Thread-Safe with Backpressure**: Uses `sync.Cond` for efficient waiting
  - Operations block if flush in progress (no busy-waiting)
  - Single flush per batch (prevents thundering herd)
  - No channel creation/deletion overhead

#### Optional Sliding Window Cache

For large-scale BFS operations, Spectra offers an opt-in node cache:

- **3-Generation Window**: Maintains grandparent, parent, and child nodes
  - Tracks `maxDepth` across all cached nodes
  - Evicts nodes where `depth < maxDepth - 2`
  - Memory bounded to 3 levels regardless of tree size

- **Performance Impact**:
  - ~66% reduction in DB reads during BFS traversal
  - O(1) lookups for recently accessed nodes
  - Memory usage scales with fanout, not total tree size

- **Safety Mechanisms**:
  - Flush-before-evict: Never evicts nodes with pending buffered writes
  - Cache updated immediately when operations buffered
  - Thread-safe with `sync.RWMutex` (concurrent reads, exclusive writes)

- **Opt-In Design**: Set `enable_cache: true` in config
  - Defaults to `false` for backwards compatibility
  - No overhead when disabled

#### O(1) Parent-Child Lookups

Nodes store direct references to children for efficient traversal:

- **`ChildIDs` Array**: Each parent node maintains `[]string` of child node IDs
  - No more O(n) prefix scans through index buckets
  - Direct lookup of children via single parent node read
  - Updated atomically during batch flush operations

- **Index Optimization**: Reduced index bucket usage
  - `index_parent_id` removed (replaced by `ChildIDs`)
  - `index_path` and `index_parent_path` retained for path-based lookups
  - Smaller database footprint, faster queries

#### Heavy-Tailed Fanout Distribution

Realistic filesystem generation using weighted logarithmic buckets:

- **Logarithmic Bucketing**: [0-10), [10-100), [100-1K), [1K-10K), ...
  - Each bucket exponentially less likely than previous
  - Configurable via `backoff_factor` (default 0.5)
  - Produces "mostly small, occasionally huge" distributions

- **Depth-Based Decay**: Fanout reduces at deeper levels
  - `effectiveMax = max * (depth_decay ^ depth)`
  - Prevents BFS frontier explosion
  - Configurable per folders/files independently

- **Deterministic**: Uses seeded RNG for reproducibility
  - Same seed = same distribution every time
  - Predictable for testing and benchmarking

#### Deterministic Node IDs

Spectra uses deterministic string IDs (no UUIDs or ULIDs):

- **Root**: The root folder always has ID `"root"`.
- **All other nodes**: `"spc:"` + 32-character hex from FNV128a(path, type). Same path and type yield the same ID in every world and every run.
- **World-agnostic**: IDs do not encode world name, so the same logical path in primary and s1 has the same ID (enables overlap and copy semantics).

**Example IDs**: `root`, `spc:a1b2c3d4e5f6...`

---

## Tech Stack

| Component                                                        | Purpose                                                         |
| ---------------------------------------------------------------- | --------------------------------------------------------------- |
| **Go (Golang)**                                                  | Core implementation language                                    |
| **BoltDB**                                                       | Lightweight embedded key-value database for node persistence    |
| **Chi Router**                                                   | HTTP router for RESTful API endpoints                           |
| **Deterministic IDs**                                            | `root` or `spc:` + FNV128a hex (path + type)                    |
| **Go's `math/rand`**                                             | Deterministic random generation with seeding                    |
| **Go standard library (`os`, `path/filepath`, `time`, `io/fs`)** | Utility functions, path normalization, and filesystem interface |

---

## Project Structure

```
Spectra/
├── cmd/                       # Command-line applications
│   └── api/                   # API server (uses sdk.New; mode from config)
│       └── main.go
├── internal/                  # Internal implementation
│   ├── api/                  # HTTP API layer (handlers, middleware, router, server)
│   ├── config/               # Configuration load/validate/defaults (mode, seed, api)
│   ├── db/                   # BoltDB persistence, buffer, cache (persistent mode only)
│   ├── ephemeralfs/          # Stateless on-the-fly implementation (ephemeral mode)
│   ├── generator/            # Procedural generation (shared by both modes)
│   ├── spectrafs/            # Persistent filesystem implementation + fs.FS
│   ├── types/                # Config, Node, ListResult, etc.
│   └── utils/                # Path joining, deterministic node IDs
├── sdk/                      # Public SDK; New() picks spectrafs or ephemeralfs by config mode
├── main.go                   # SDK demo
└── go.mod
```

---

## Core Concepts

### Node Generation

Spectra represents all nodes as entries in a unified BoltDB key-value store:

| Column               | Type      | Description                                                |
| -------------------- | --------- | --------------------------------------------------------   |
| `id`                 | string    | `root` or `spc:` + hex (deterministic from path and type)   |
| `parent_id`          | string    | Parent node ID (`root` or `spc:...`)                       |
| `name`               | string    | Display name                                               |
| `path`               | string    | Relative path (root-relative, not absolute)                |
| `type`               | string    | `"folder"` or `"file"`                                     |
| `depth_level`        | int       | BFS-style depth index                                      |
| `size`               | int64     | File size (0 for folders)                                  |
| `last_updated`       | timestamp | Synthetic timestamp                                        |
| `checksum`           | string    | SHA256 checksum (for files only)                           |
| `existence_map`      | JSON      | Map tracking world existence: `{"primary":true,"s1":true}` |
| `child_ids`          | JSON      | Array of direct child node IDs: `["root","spc:...",...]`  |

### Example Behavior

Given a config:

```json
{
  "seed": {
    "max_depth": 4,
    "max_folders": 100,
    "folder_backoff_factor": 0.5,
    "folder_depth_decay_factor": 0.8,
    "max_files": 100,
    "file_backoff_factor": 0.5,
    "file_depth_decay_factor": 0.85,
    "seed": 42,
    "db_path": "./spectra.db",
    "enable_cache": false
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

Spectra will generate a reproducible tree up to 4 levels deep with realistic fanout distribution:
- **Folder fanout**: Logarithmic buckets [0-10), [10-100), [100-max] with 0.5 exponential decay
  - Most directories have 0-10 children
  - Some have 10-100 children  
  - Rare directories can have 100+ children
  - Deeper levels have smaller max fanout (80% reduction per level)
- **File fanout**: Similar distribution with slightly gentler decay (85% per level)
- **World probability**: Each node has 70% chance in s1, 30% in s2 (tracked in `existence_map`)
- **Cache**: Disabled by default (set `enable_cache: true` for ~66% read reduction in BFS)

---

## API Interface

### RESTful Endpoints

#### Folder Operations
- `POST /api/v1/folder/list` - List children with world detection
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
- `POST /api/v1/reset` - Reset all nodes
- `GET /api/v1/config` - Get current configuration
- `GET /api/v1/tables` - Get world information (API uses "tables" for compatibility)
- `GET /api/v1/tables/{tableName}/count` - Get node count for specific world

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
    GetTableInfo() ([]TableInfo, error)  // Returns world information
    GetNodeCount(tableName string) (int, error)  // Counts nodes in specific world
}
```

#### Request Types

All CRUD operations use simple request structs that support flexible lookup methods through a clean interface-based design:

**GetNodeRequest** - Retrieve a node by ID or Path+World
```go
// By ID
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
// By ParentID
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
    ID: "root",  // or spc:...
}
err := fs.DeleteNode(req)
```

**Design Note:** Each request struct implements the appropriate interfaces (`NodeIdentifier`, `ParentIdentifier`, etc.) for compile-time type safety and runtime validation. Users can pass any struct that implements these interfaces.

### fs.FS Interface

SpectraFS implements Go's standard library `fs.FS` interface, enabling compatibility with tools like Rclone and standard library functions:

```go
// Create SpectraFS instance
fs, _ := sdk.New("configs/default.json")

// Get fs.FS for a specific world
primaryFS := fs.AsFS("primary")
s1FS := fs.AsFS("s1")

// Use with standard library
import "io/fs"

data, _ := fs.ReadFile(primaryFS, "folder/file.txt")
entries, _ := fs.ReadDir(primaryFS, "folder")
```

**World Projection:** Each world is projected as a separate filesystem, allowing tools like Rclone to treat each world as an independent remote. This enables comparison and synchronization between different world projections.

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

#### FUSE Mount (Linux / macOS)

Mount Spectra worlds as real directories (similar to a cloud-drive mount). Requires FUSE on Linux (`fuse` package) or [macFUSE](https://osxfuse.github.io/) on macOS. Windows is not supported in v1.

**In-process (default)** — uses the SDK directly, shares the same BoltDB:

```bash
# Mount with CLI overrides (primary required; s1 auto-derived if omitted)
go run ./cmd/mount --mount primary:/tmp/spectra internal/config/default.json

# Explicit multi-world mounts
go run ./cmd/mount --mount primary:/tmp/spectra --mount s1:/tmp/spectra-s1 configs/custom.json

# Mount + HTTP API for other tools
go run ./cmd/mount --with-api internal/config/default.json
```

**Remote** — FUSE client talks to a running API server:

```bash
# Terminal 1: start API
go run cmd/api/main.go

# Terminal 2: mount via HTTP
go run ./cmd/mount --remote http://localhost:8086 \
  --mount primary:/tmp/spectra --mount s1:/tmp/spectra-s1
```

Configure mount paths in JSON (optional):

```json
"mount": {
  "enabled": true,
  "with_api": false,
  "paths": {
    "primary": "/mnt/spectra",
    "s1": "/mnt/spectra-s1"
  }
}
```

Secondary worlds omitted from `paths` auto-derive as `{primary_path}-{world}` (e.g. `/mnt/spectra-s1`).

**FUSE mount limitations:**

- File **content** is generated on read; writes update metadata (size/name) only, not byte payloads.
- Ephemeral mode: create/delete behave as in the SDK (non-persistent where applicable).
- Remote mode adds latency per lookup; prefer in-process for local testing.
- Do not execute binaries directly from the mount (go-fuse caveat); use a wrapper if needed.

Uninstall on Linux: `fusermount -u /path/to/mount`

#### Auth Tokens (opt-in)

When `auth.enabled` is true, FS/API calls require a per-world access token. Refresh tokens never expire; access tokens expire after `access_token_ttl_seconds` (`-1` = never; otherwise **minimum 10 seconds**). Concurrent `Refresh` calls within **5 seconds** reuse the current access token so FS workers do not stampede renewal on the same expiry.

```json
"auth": {
  "enabled": true,
  "access_token_ttl_seconds": 3600
}
```

```go
pair, err := fs.IssueTokens("primary")
// pair.AccessToken is also bound via SetAccessToken for SDK calls
fs.SetAccessToken("s1", other.AccessToken)
```

HTTP: `POST /api/v1/auth/token` and `POST /api/v1/auth/refresh`. Protected routes expect `Authorization: Bearer <access_token>` (401 when missing/invalid/expired).

Tokens persist next to the config as `.spectra-auth.json` (mode 0600; do not commit). Fallback: `.spectra-auth.env` in the working directory.

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

Spectra uses BoltDB, a pure Go embedded database, so setup is lightweight:

1. Install Go 1.24.2 or later.
2. Clone the repository and run `go mod tidy` to pull dependencies.
3. (Optional) Build local binaries:
   ```bash
   # Build SDK demo
   go build -o bin/spectra-demo main.go

   # Build API server
   go build -o bin/spectra-api cmd/api/main.go

   # Build FUSE mount helper (Linux/macOS)
   go build -o bin/spectra-mount cmd/mount/main.go
   ```

---

## Command-Line Applications

Spectra provides three main command-line applications:

### SDK Demo (`main.go`)
A demonstration application that showcases the Spectra SDK functionality:
- Loads configuration and initializes SpectraFS
- Demonstrates world information and node generation
- Shows multi-world operations and secondary world counts
- Performs a complete reset operation
- Perfect for testing and understanding the SDK

### API Server (`cmd/api/main.go`)
A production-ready HTTP server that exposes the Spectra filesystem via RESTful API:
- Starts HTTP server on configurable host and port
- Provides all CRUD operations via REST endpoints
- Includes graceful shutdown and timeout handling
- Supports CORS and proper error responses
- Ideal for integration testing and production use

### FUSE Mount (`cmd/mount/main.go`)
Mounts Spectra worlds as OS-visible directories via FUSE:
- In-process SDK backend (default) or remote HTTP backend (`--remote`)
- Multi-world mounts: `primary` plus each `secondary_tables` entry
- Optional co-located API server (`--with-api`)
- Metadata writes (mkdir/create/delete); file bytes remain generated on read
- Linux and macOS only (requires FUSE / macFUSE)

---

## Performance Characteristics

Spectra is designed for high-throughput synthetic filesystem operations:

### Write Performance
- **Buffered Writes**: 10,000 operations batched per flush (configurable)
- **Ordered Execution**: Inserts before updates (referential integrity)
- **Batched Parent Updates**: O(1) per parent instead of O(n²)
- **Typical Throughput**: 50K-100K+ nodes/second on consumer hardware (M1/M2, modern x64)

### Read Performance
- **O(1) Node Lookups**: Direct read by node ID
- **O(1) Children Retrieval**: Single parent read + `ChildIDs` array (no index scan)
- **Optional Cache**: ~66% reduction in DB reads during BFS (3-generation window)
- **Vectorized Queries**: Bulk operations reduce DB round trips by 3-4x

### Memory Profile
- **Base Overhead**: ~10-50 MB (BoltDB + server)
- **Buffer**: ~1 MB per 10K operations (configurable)
- **Cache (if enabled)**: ~100 KB - 10 MB depending on fanout and depth
- **Scalability**: Tested to 1M+ nodes with <500 MB total memory

### Deterministic Performance
- **Same Seed = Same Tree**: Identical structure every run
- **Reproducible Benchmarks**: Consistent performance characteristics
- **No I/O Variance**: No actual disk reads/writes for file content (deterministic generation)

### Bottlenecks & Limitations
- **BoltDB Write Locks**: Single writer at a time (but batched for efficiency)
- **Parent Fanout**: O(n) to update parent `ChildIDs` during flush (mitigated via batching)
- **Deep Trees**: Memory for cache scales with fanout × 3 generations
- **JSON Serialization**: ~30% of CPU time during bulk operations (acceptable tradeoff)

---

## Use Cases

* **Migration Engine Testing:** Validate traversal and BFS logic with reproducible data.
* **Performance Benchmarks:** Measure traversal throughput without real I/O.
* **Integration Testing:** Simulate different storage backends through the same API shape.
* **Rclone Integration:** Use SpectraFS as a backend for Rclone, enabling comparison and synchronization between different world projections.
* **Chaos Simulation:** Test rate limiting, throttling, or transient "missing node" scenarios.
* **Multi-Source Testing:** Test migration scenarios with multiple data sources and probability-based data distribution.
* **Standard Library Compatibility:** Use with any tool or library that works with Go's `fs.FS` interface.

---

## Configuration

The configuration file supports three main sections:

- **`seed`**: Controls procedural generation parameters
- **`api`**: Configures HTTP server settings
- **`secondary_tables`**: Defines secondary world probabilities (config key name kept for compatibility)

### Seed Configuration Reference

| Field | Type | Default | Range | Description |
|-------|------|---------|-------|-------------|
| `max_depth` | int | 4 | ≥ 1 | Maximum tree depth (BFS levels) |
| `max_folders` | int | 100 | ≥ 0 | Upper bound for folder fanout |
| `folder_backoff_factor` | float64 | 0.5 | (0.0, 1.0] | Exponential decay per bucket (lower = more aggressive) |
| `folder_depth_decay_factor` | float64 | 0.8 | (0.0, 1.0] | Max reduction per depth level (lower = stronger decay) |
| `max_files` | int | 100 | ≥ 0 | Upper bound for file fanout |
| `file_backoff_factor` | float64 | 0.5 | (0.0, 1.0] | Exponential decay for file distribution |
| `file_depth_decay_factor` | float64 | 0.85 | (0.0, 1.0] | File count reduction at deeper levels |
| `seed` | int64 | 42 | any | RNG seed for reproducibility |
| `db_path` | string | "./spectra.db" | any path | BoltDB file location |
| `file_binary_seed` | int64 | 0 | any | Seed for deterministic file content generation |
| `enable_cache` | bool | false | true/false | Enable sliding window node cache (opt-in) |

### Distribution Tuning Guide

**Conservative (default)**: Safe for all use cases, prevents memory explosion
```json
{
  "max_folders": 100,
  "folder_backoff_factor": 0.5,
  "folder_depth_decay_factor": 0.8
}
```
Result: Most dirs 0-10 children, occasional 10-100, rare 100+

**Aggressive**: For stress testing and large-scale benchmarks
```json
{
  "max_folders": 10000,
  "folder_backoff_factor": 0.2,
  "folder_depth_decay_factor": 0.6
}
```
Result: More "monster directories" (1K-10K children), but still mostly small

**Uniform-like**: Simulate old min/max behavior (less realistic)
```json
{
  "max_folders": 10,
  "folder_backoff_factor": 0.99,
  "folder_depth_decay_factor": 1.0
}
```
Result: Mostly uniform 0-10 range, no depth decay

### Cache Configuration

**When to enable cache** (`enable_cache: true`):
- ✅ Large-scale BFS traversal operations
- ✅ Deep trees (depth > 5) with moderate fanout
- ✅ Memory available (~MB per 1000 nodes in window)
- ✅ Read-heavy workloads (ListChildren calls)

**When to disable cache** (`enable_cache: false`, default):
- ✅ Small trees (< 10K nodes total)
- ✅ Memory-constrained environments
- ✅ Write-heavy workloads
- ✅ Random access patterns (non-BFS)

See `internal/config/default.json` for a complete example.

---

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

---

## License

This project is part of the Sylos ecosystem and follows the same overall copyright laws and protections, however this specific library uses an MIT license to permit the community to utilize it and update it as they see fit. <3 
