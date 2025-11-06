package spectrafs

import (
	"io/fs"
	"time"

	"github.com/Project-Sylos/Spectra/internal/types"
)

// nodeFileInfo wraps a types.Node to implement fs.FileInfo
type nodeFileInfo struct {
	node *types.Node
}

// NewFileInfo creates a new fs.FileInfo from a types.Node
func NewFileInfo(node *types.Node) fs.FileInfo {
	return &nodeFileInfo{node: node}
}

// Name returns the base name of the file
func (fi *nodeFileInfo) Name() string {
	return fi.node.Name
}

// Size returns the length in bytes for regular files; 0 for directories
func (fi *nodeFileInfo) Size() int64 {
	return fi.node.Size
}

// Mode returns the file mode bits
func (fi *nodeFileInfo) Mode() fs.FileMode {
	if fi.node.Type == types.NodeTypeFolder {
		return fs.ModeDir | 0755
	}
	return 0644
}

// ModTime returns the modification time
func (fi *nodeFileInfo) ModTime() time.Time {
	return fi.node.LastUpdated
}

// IsDir reports whether the file describes a directory
func (fi *nodeFileInfo) IsDir() bool {
	return fi.node.Type == types.NodeTypeFolder
}

// Sys returns the underlying data source
func (fi *nodeFileInfo) Sys() any {
	return fi.node
}
