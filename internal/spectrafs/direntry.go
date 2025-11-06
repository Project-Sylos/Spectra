package spectrafs

import (
	"io/fs"

	"github.com/Project-Sylos/Spectra/internal/types"
)

// nodeDirEntry wraps a types.Node to implement fs.DirEntry
type nodeDirEntry struct {
	node *types.Node
}

// NewDirEntry creates a new fs.DirEntry from a types.Node
func NewDirEntry(node *types.Node) fs.DirEntry {
	return &nodeDirEntry{node: node}
}

// Name returns the name of the file (or subdirectory) described by the entry
func (de *nodeDirEntry) Name() string {
	return de.node.Name
}

// IsDir reports whether the entry describes a directory
func (de *nodeDirEntry) IsDir() bool {
	return de.node.Type == types.NodeTypeFolder
}

// Type returns the type bits for the entry
func (de *nodeDirEntry) Type() fs.FileMode {
	if de.node.Type == types.NodeTypeFolder {
		return fs.ModeDir
	}
	return 0
}

// Info returns the FileInfo for the file or subdirectory described by the entry
func (de *nodeDirEntry) Info() (fs.FileInfo, error) {
	return NewFileInfo(de.node), nil
}
