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
