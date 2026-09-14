package utils

import (
	"strings"
)

// JoinPath joins path parts using forward slashes regardless of host OS.
// It strips leading/trailing slashes from each component, then prefixes the result with "/".
// Pattern:
//   - Root path = "/"
//   - Child of root = "/{child}"
//   - Children of that = "/{child}/{grandchild}" etc.
func JoinPath(parts ...string) string {
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		part = strings.Trim(part, "/")
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}

	if len(cleaned) == 0 {
		return "/"
	}

	return "/" + strings.Join(cleaned, "/")
}

// PathDepth returns the folder depth of a Spectra path. Root "/" is 0.
func PathDepth(p string) int {
	p = strings.TrimSpace(p)
	if p == "" || p == "." || p == "/" {
		return 0
	}
	trimmed := strings.Trim(p, "/")
	if trimmed == "" {
		return 0
	}
	return strings.Count(trimmed, "/") + 1
}
