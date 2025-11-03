package types

import (
	"time"
)

// Config represents the complete configuration for Spectra
type Config struct {
	Seed            SeedConfig         `json:"seed"`
	API             APIConfig          `json:"api"`
	SecondaryTables map[string]float64 `json:"secondary_tables"`
}

// SeedConfig represents the filesystem generation configuration
type SeedConfig struct {
	MaxDepth   int    `json:"max_depth"`
	MinFolders int    `json:"min_folders"`
	MaxFolders int    `json:"max_folders"`
	MinFiles   int    `json:"min_files"`
	MaxFiles   int    `json:"max_files"`
	Seed       int64  `json:"seed"`
	DBPath     string `json:"db_path"`
}

// APIConfig represents the HTTP API configuration
type APIConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// Node represents a filesystem node (file or folder) in the DuckDB table
// Unified single-table design with existence tracking across worlds
type Node struct {
	ID           string          `json:"id" db:"id"`                       // UUID identifier
	ParentID     string          `json:"parent_id" db:"parent_id"`         // UUID parent reference
	Name         string          `json:"name" db:"name"`                   // Display name
	Path         string          `json:"path" db:"path"`                   // Relative path
	ParentPath   string          `json:"parent_path" db:"parent_path"`     // Parent path
	Type         string          `json:"type" db:"type"`                   // "folder" or "file"
	DepthLevel   int             `json:"depth_level" db:"depth_level"`     // BFS-style depth index
	Size         int64           `json:"size" db:"size"`                   // File size (0 for folders)
	LastUpdated  time.Time       `json:"last_updated" db:"last_updated"`   // Synthetic timestamp
	Checksum     *string         `json:"checksum" db:"checksum"`           // SHA256 checksum (NULL for folders)
	ExistenceMap map[string]bool `json:"existence_map" db:"existence_map"` // JSON: {"primary": true, "s1": true, "s2": false}
}

// Folder represents a folder node
type Folder struct {
	Node
}

// File represents a file node
type File struct {
	Node
}

// ListResult represents the result of ListChildren operation
// Enhanced with success/failure response
type ListResult struct {
	Success bool     `json:"success"`
	Message string   `json:"message,omitempty"`
	Folders []Folder `json:"folders"`
	Files   []File   `json:"files"`
}

// APIResponse represents a generic API response
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// TableInfo represents information about a database table
type TableInfo struct {
	Name      string `json:"name"`
	RowCount  int    `json:"row_count"`
	TableType string `json:"table_type"` // "primary" or "secondary"
}

// NodeType constants
const (
	NodeTypeFolder = "folder"
	NodeTypeFile   = "file"
)

// TraversalStatus constants
const (
	StatusPending    = "pending"
	StatusSuccessful = "successful"
	StatusFailed     = "failed"
)

// CopyStatus constants
const (
	CopyStatusPending    = "pending"
	CopyStatusInProgress = "in_progress"
	CopyStatusCompleted  = "completed"
)

// GetTableName returns the full table name (always "nodes" now)
func GetTableName(world string) string {
	return "nodes"
}
