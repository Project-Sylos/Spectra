package mount

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"codeberg.org/Sylos/Spectra/internal/types"
	"codeberg.org/Sylos/Spectra/sdk"
)

func writeTestConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cfg := types.Config{
		Mode: "ephemeral",
		Seed: types.SeedConfig{
			MaxDepth:               4,
			MaxFolders:             10,
			FolderBackoffFactor:    0.5,
			FolderDepthDecayFactor: 0.8,
			MaxFiles:               10,
			FileBackoffFactor:      0.5,
			FileDepthDecayFactor:   0.85,
			Seed:                   42,
		},
		API:             types.APIConfig{Host: "localhost", Port: 8086},
		SecondaryTables: map[string]float64{},
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", "/"},
		{".", "/"},
		{"/", "/"},
		{"foo", "/foo"},
		{"/foo/bar", "/foo/bar"},
	}
	for _, tc := range tests {
		if got := NormalizePath(tc.in); got != tc.want {
			t.Errorf("NormalizePath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestJoinChildPath(t *testing.T) {
	if got := JoinChildPath("/", "foo"); got != "/foo" {
		t.Errorf("JoinChildPath(/, foo) = %q", got)
	}
	if got := JoinChildPath("/foo", "bar"); got != "/foo/bar" {
		t.Errorf("JoinChildPath(/foo, bar) = %q", got)
	}
}

func TestSDKBackend_GetNode_Root(t *testing.T) {
	cfgPath := writeTestConfig(t)
	fs, err := sdk.New(cfgPath)
	if err != nil {
		t.Fatalf("sdk.New: %v", err)
	}
	defer fs.Close()

	backend := NewSDKBackend(fs, "primary")
	node, err := backend.GetNode("/")
	if err != nil {
		t.Fatalf("GetNode(/): %v", err)
	}
	if node.ID != "root" {
		t.Errorf("root ID = %q, want root", node.ID)
	}
}

func writePersistentTestConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cfg := types.Config{
		Mode: "persistent",
		Seed: types.SeedConfig{
			MaxDepth:               2,
			MaxFolders:             5,
			FolderBackoffFactor:    0.5,
			FolderDepthDecayFactor: 0.8,
			MaxFiles:               5,
			FileBackoffFactor:      0.5,
			FileDepthDecayFactor:   0.85,
			Seed:                   42,
			DBPath:                 filepath.Join(dir, "test.db"),
		},
		API:             types.APIConfig{Host: "localhost", Port: 8086},
		SecondaryTables: map[string]float64{},
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestSDKBackend_GetNode_NotExist(t *testing.T) {
	cfgPath := writePersistentTestConfig(t)
	fs, err := sdk.New(cfgPath)
	if err != nil {
		t.Fatalf("sdk.New: %v", err)
	}
	defer fs.Close()

	backend := NewSDKBackend(fs, "primary")
	_, err = backend.GetNode("/does-not-exist")
	if !errors.Is(err, ErrNotExist) {
		t.Errorf("GetNode missing path: got %v, want ErrNotExist", err)
	}
}

func TestSDKBackend_ListChildren_Ephemeral(t *testing.T) {
	cfgPath := writeTestConfig(t)
	fs, err := sdk.New(cfgPath)
	if err != nil {
		t.Fatalf("sdk.New: %v", err)
	}
	defer fs.Close()

	backend := NewSDKBackend(fs, "primary")
	result, err := backend.ListChildren("/")
	if err != nil {
		t.Fatalf("ListChildren: %v", err)
	}
	if len(result.Folders)+len(result.Files) == 0 {
		t.Error("expected generated children at root in ephemeral mode")
	}
}

func TestSDKBackend_ListChildren(t *testing.T) {
	cfgPath := writePersistentTestConfig(t)
	fs, err := sdk.New(cfgPath)
	if err != nil {
		t.Fatalf("sdk.New: %v", err)
	}
	defer fs.Close()

	backend := NewSDKBackend(fs, "primary")
	result, err := backend.ListChildren("/")
	if err != nil {
		t.Fatalf("ListChildren: %v", err)
	}
	if len(result.Folders)+len(result.Files) == 0 {
		t.Error("expected generated children at root")
	}
}
