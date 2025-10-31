# Types Package

The types package defines all data structures and type definitions used throughout Spectra. It provides the core data models for nodes, configuration, and API responses.

## Structure

```
types/
└── types.go  # All type definitions and helper functions
```

## Core Types

### Node
The fundamental data structure representing filesystem nodes:

```go
type Node struct {
    ID                string            `json:"id" db:"id"`
    ParentID          string            `json:"parent_id" db:"parent_id"`
    Name              string            `json:"name" db:"name"`
    Path              string            `json:"path" db:"path"`
    Type              string            `json:"type" db:"type"`
    DepthLevel        int               `json:"depth_level" db:"depth_level"`
    Size              int64             `json:"size" db:"size"`
    LastUpdated       time.Time         `json:"last_updated" db:"last_updated"`
    Checksum          *string           `json:"checksum" db:"checksum"`
    ExistenceMap      map[string]bool   `json:"existence_map" db:"existence_map"`
    TraversalStatuses map[string]string `json:"traversal_statuses,omitempty" db:"-"`
    CopyStatus        string            `json:"copy_status" db:"copy_status"`
}
```

**Key Changes:**
- `ID` is now a plain UUID (no prefixes like `p-` or `s1-`)
- `ExistenceMap` tracks which worlds the node exists in (e.g., `{"primary": true, "s1": true}`)
- `TraversalStatuses` is an in-memory map for per-world traversal states
- `CopyStatus` tracks migration status ("pending", "in_progress", "completed")

### Configuration
Multi-section configuration structure:

```go
type Config struct {
    SeedConfig
    API            APIConfig
    SecondaryTables map[string]float64
}

type SeedConfig struct {
    MaxDepth    int    `json:"max_depth"`
    MinFolders  int    `json:"min_folders"`
    MaxFolders  int    `json:"max_folders"`
    MinFiles    int    `json:"min_files"`
    MaxFiles    int    `json:"max_files"`
    Seed        int64  `json:"seed"`
    DBPath      string `json:"db_path"`
}

type APIConfig struct {
    Host string `json:"host"`
    Port int    `json:"port"`
}
```

### API Responses
Standardized API response structures:

```go
type APIResponse struct {
    Success bool        `json:"success"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}

type ListResult struct {
    Success bool     `json:"success"`
    Message string   `json:"message"`
    Folders []*Node  `json:"folders"`
    Files   []*Node  `json:"files"`
}
```

## Constants

### Node Types
- `NodeTypeFolder` - "folder"
- `NodeTypeFile` - "file"

### Traversal Status
- `StatusPending` - "pending"
- `StatusSuccessful` - "successful"
- `StatusFailed` - "failed"

### Copy Status
- `CopyStatusPending` - "pending"
- `CopyStatusInProgress` - "in_progress"
- `CopyStatusCompleted` - "completed"

## Helper Functions

### Table Management
- `GetTableName(world)` - Always returns "nodes" (single table architecture)

## Usage

The types package is used throughout the application to ensure type safety and consistency. All data structures are defined here and used across the database, API, and SDK layers.

## JSON Tags

All types include JSON tags for API serialization and database tags for DuckDB operations. This ensures consistent data representation across all layers.

## Validation

Types include validation logic where appropriate, particularly for configuration validation and ID format checking.
