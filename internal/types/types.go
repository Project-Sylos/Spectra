package types

import (
	"time"
)

// Config represents the complete configuration for Spectra
type Config struct {
	Mode            string             `json:"mode"`             // "persistent" or "ephemeral"
	Seed            SeedConfig         `json:"seed"`
	API             APIConfig          `json:"api"`
	SecondaryTables map[string]float64 `json:"secondary_tables"`
}

// SeedConfig represents the filesystem generation configuration
type SeedConfig struct {
	MaxDepth               int     `json:"max_depth"`
	MaxFolders             int     `json:"max_folders"`
	FolderBackoffFactor    float64 `json:"folder_backoff_factor"`
	FolderDepthDecayFactor float64 `json:"folder_depth_decay_factor"`
	MaxFiles               int     `json:"max_files"`
	FileBackoffFactor      float64 `json:"file_backoff_factor"`
	FileDepthDecayFactor   float64 `json:"file_depth_decay_factor"`
	Seed                   int64   `json:"seed"`
	DBPath                 string  `json:"db_path"`
	FileBinarySeed         int64   `json:"file_binary_seed,omitempty"`
	EnableCache            bool    `json:"enable_cache"`
	// DivergingTreeMode (ephemeral only): seed child generation with worldName//path
	// instead of path alone, so each world gets a different tree shape (e.g. for copy tests).
	// Default false: same path in any world yields same children (identical trees).
	DivergingTreeMode bool `json:"diverging_tree_mode,omitempty"`
}

// APIConfig represents the HTTP API configuration
type APIConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// Node represents a filesystem node (file or folder) in the BoltDB database
// Unified single-bucket design with existence tracking across worlds
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
	ChildIDs     []string        `json:"child_ids" db:"child_ids"`         // Array of child node IDs (O(1) lookup)
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
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

// TableInfo represents information about a database table
type TableInfo struct {
	Name      string `json:"name"`
	RowCount  int    `json:"row_count"`
	TableType string `json:"table_type"` // "primary" or "secondary"
}

// Stats represents filesystem statistics
type Stats struct {
	FileCount      int64            `json:"file_count"`      // Total number of files
	FolderCount    int64            `json:"folder_count"`    // Total number of folders
	TotalFileSize  int64            `json:"total_file_size"` // Total size of all files combined
	SecondaryNodes map[string]int64 `json:"secondary_nodes"` // Node counts broken down by world (excluding primary)
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
