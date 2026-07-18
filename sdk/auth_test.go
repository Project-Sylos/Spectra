package sdk_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"codeberg.org/Sylos/Spectra/sdk"
)

func writeAuthConfig(t *testing.T, authEnabled bool, ttl int64) string {
	t.Helper()
	dir := t.TempDir()
	cfg := map[string]any{
		"mode": "ephemeral",
		"seed": map[string]any{
			"max_depth":                2,
			"max_folders":              5,
			"folder_backoff_factor":    0.5,
			"folder_depth_decay_factor": 0.8,
			"max_files":                5,
			"file_backoff_factor":       0.5,
			"file_depth_decay_factor":   0.85,
			"seed":                     42,
		},
		"api":               map[string]any{"host": "localhost", "port": 8086},
		"secondary_tables":  map[string]any{},
		"auth":              map[string]any{"enabled": authEnabled, "access_token_ttl_seconds": ttl},
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAuthRejectsWithoutToken(t *testing.T) {
	path := writeAuthConfig(t, true, 3600)
	fs, err := sdk.New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer fs.Close()

	_, err = fs.ListChildren(&sdk.ListChildrenRequest{ParentPath: "/", TableName: "primary"})
	if err == nil {
		t.Fatal("expected unauthorized")
	}
	if _, ok := sdk.IsUnauthorized(err); !ok {
		t.Fatalf("expected UnauthorizedError, got %v", err)
	}
}

func TestAuthAllowsAfterIssue(t *testing.T) {
	path := writeAuthConfig(t, true, -1)
	fs, err := sdk.New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer fs.Close()

	if _, err := fs.IssueTokens("primary"); err != nil {
		t.Fatalf("IssueTokens: %v", err)
	}
	depth := 1
	result, err := fs.ListChildren(&sdk.ListChildrenRequest{
		ParentPath: "/",
		TableName:  "primary",
		Depth:      &depth,
	})
	if err != nil {
		t.Fatalf("ListChildren: %v", err)
	}
	if result == nil || !result.Success {
		t.Fatalf("list failed: %+v", result)
	}
}

func TestAuthDisabledAllows(t *testing.T) {
	path := writeAuthConfig(t, false, 3600)
	fs, err := sdk.New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer fs.Close()

	depth := 1
	_, err = fs.ListChildren(&sdk.ListChildrenRequest{
		ParentPath: "/",
		TableName:  "primary",
		Depth:      &depth,
	})
	if err != nil {
		t.Fatalf("ListChildren: %v", err)
	}
}
