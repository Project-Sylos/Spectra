package mount

import (
	"errors"
	"fmt"
	"strings"

	"codeberg.org/Sylos/Spectra/internal/types"
	"codeberg.org/Sylos/Spectra/internal/utils"
)

// ErrNotExist is returned when a node does not exist in the bound world.
var ErrNotExist = errors.New("node does not exist")

// Backend abstracts Spectra filesystem operations for FUSE and other mount layers.
type Backend interface {
	World() string
	GetNode(path string) (*types.Node, error)
	ListChildren(parentPath string) (*types.ListResult, error)
	ReadFile(path string) ([]byte, error)
	CreateFolder(parentPath, name string) (*types.Node, error)
	CreateFile(parentPath, name string, size int64) (*types.Node, error)
	DeleteNode(path string) error
	Close() error
}

// NormalizePath converts FUSE-relative paths to Spectra root-relative paths.
func NormalizePath(name string) string {
	if name == "" || name == "." {
		return "/"
	}
	if name == "/" {
		return "/"
	}
	return utils.JoinPath(name)
}

// JoinChildPath joins a parent Spectra path with a child name.
func JoinChildPath(parentPath, name string) string {
	if parentPath == "/" {
		return utils.JoinPath(name)
	}
	return utils.JoinPath(parentPath, name)
}

// ParentPathDepth returns the depth level of a folder path (root "/" is 0).
func ParentPathDepth(path string) int {
	path = NormalizePath(path)
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return 0
	}
	return strings.Count(trimmed, "/") + 1
}

// NodeExistsInWorld reports whether node exists in the backend's world.
func NodeExistsInWorld(node *types.Node, world string) bool {
	if node == nil || node.ExistenceMap == nil {
		return false
	}
	return node.ExistenceMap[world]
}

// CheckNodeInWorld returns the node or ErrNotExist if missing in world.
func CheckNodeInWorld(node *types.Node, world string) (*types.Node, error) {
	if node == nil {
		return nil, ErrNotExist
	}
	if !NodeExistsInWorld(node, world) {
		return nil, fmt.Errorf("%w: not in world %s", ErrNotExist, world)
	}
	return node, nil
}
