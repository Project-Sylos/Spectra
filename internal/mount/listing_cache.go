//go:build linux || darwin

package mount

import (
	"path"
	"sync"
	"time"

	"codeberg.org/Sylos/Spectra/internal/types"
)

// listingCacheTTL covers ReadDir, readdirplus lookups, and the immediate Lstat
// burst from clients. Entries expire automatically; only one map per recently
// listed directory is retained (not the whole tree).
const listingCacheTTL = time.Second

type dirListingEntry struct {
	expires time.Time
	byName  map[string]*types.Node
}

var dirListings sync.Map // normalized dir path -> *dirListingEntry

func publishListingCache(dirPath string, result *types.ListResult) {
	if result == nil {
		return
	}
	byName := make(map[string]*types.Node, len(result.Folders)+len(result.Files))
	for i := range result.Folders {
		node := result.Folders[i].Node
		byName[node.Name] = &node
	}
	for i := range result.Files {
		node := result.Files[i].Node
		byName[node.Name] = &node
	}
	dirListings.Store(NormalizePath(dirPath), &dirListingEntry{
		expires: time.Now().Add(listingCacheTTL),
		byName:  byName,
	})
}

func invalidateListingCache(dirPath string) {
	dirListings.Delete(NormalizePath(dirPath))
}

func listingChild(parentPath, name string) (*types.Node, bool) {
	key := NormalizePath(parentPath)
	v, ok := dirListings.Load(key)
	if !ok {
		return nil, false
	}
	entry := v.(*dirListingEntry)
	if time.Now().After(entry.expires) {
		dirListings.Delete(key)
		return nil, false
	}
	node, ok := entry.byName[name]
	return node, ok
}

func listingChildByPath(childPath string) (*types.Node, bool) {
	parentPath, name, ok := parentNameFromPath(childPath)
	if !ok {
		return nil, false
	}
	return listingChild(parentPath, name)
}

func parentNameFromPath(p string) (parentPath, name string, ok bool) {
	p = NormalizePath(p)
	if p == "/" {
		return "", "", false
	}
	dir, base := path.Split(p)
	if base == "" {
		return "", "", false
	}
	parentPath = NormalizePath(dir)
	if parentPath == "" {
		parentPath = "/"
	}
	return parentPath, base, true
}
