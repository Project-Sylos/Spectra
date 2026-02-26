package utils

import (
	"encoding/hex"
	"hash/fnv"
	"strings"
)

// DeterministicNodeID returns a stable string ID for a node from its path and type.
// Same path (relative to root) and type always yield the same ID across all worlds.
// No randomness, no time, no ULIDs. Uses fnv128a and prefix "spc:".
func DeterministicNodeID(path string, nodeType string) string {
	// Root special case: backward compatibility with existing "root" checks
	if normalizePath(path) == "/" && nodeType == "folder" {
		return "root"
	}

	h := fnv.New128a()
	norm := normalizePath(path)
	_, _ = h.Write([]byte(norm))
	_, _ = h.Write([]byte("|"))
	_, _ = h.Write([]byte(nodeType))
	return "spc:" + hex.EncodeToString(h.Sum(nil))
}

// normalizePath returns a root-relative path with leading "/" and no trailing "/".
func normalizePath(path string) string {
	s := strings.TrimSpace(path)
	s = strings.TrimSuffix(s, "/")
	if s == "" || s == "." {
		return "/"
	}
	if !strings.HasPrefix(s, "/") {
		s = "/" + s
	}
	return s
}
