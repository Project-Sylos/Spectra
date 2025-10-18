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
// Enhanced with UUID-based IDs and secondary existence mapping
type Node struct {
	ID                    string          `json:"id" db:"id"`                                           // UUID-based: p-{uuid}, s1-{uuid}, etc.
	ParentID              string          `json:"parent_id" db:"parent_id"`                             // UUID-based parent reference
	Name                  string          `json:"name" db:"name"`                                       // Display name
	Path                  string          `json:"path" db:"path"`                                       // Relative path
	Type                  string          `json:"type" db:"type"`                                       // "folder" or "file"
	DepthLevel            int             `json:"depth_level" db:"depth_level"`                         // BFS-style depth index
	Size                  int64           `json:"size" db:"size"`                                       // File size (0 for folders)
	LastUpdated           time.Time       `json:"last_updated" db:"last_updated"`                       // Synthetic timestamp
	TraversalStatus       string          `json:"traversal_status" db:"traversal_status"`               // "pending", "successful", "failed"
	SecondaryExistenceMap map[string]bool `json:"secondary_existence_map" db:"secondary_existence_map"` // JSON: {"s1": true, "s2": false}
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

// Table prefix constants
const (
	PrimaryPrefix   = "p-"
	SecondaryPrefix = "s"
)

// Helper functions for ID management
func IsPrimaryID(id string) bool {
	return len(id) > 2 && id[:2] == PrimaryPrefix
}

func IsSecondaryID(id string) bool {
	return len(id) > 2 && id[:1] == SecondaryPrefix && id[1:2] != "-"
}

func GetTableFromID(id string) string {
	if IsPrimaryID(id) {
		return "nodes_primary"
	}
	if IsSecondaryID(id) {
		// Extract table name like "s1" from "s1-uuid"
		for i, char := range id {
			if char == '-' {
				return "nodes_" + id[:i]
			}
		}
	}
	return ""
}

func GetUUIDFromID(id string) string {
	// Extract UUID portion from "p-uuid" or "s1-uuid"
	for i, char := range id {
		if char == '-' {
			return id[i+1:]
		}
	}
	return id
}
