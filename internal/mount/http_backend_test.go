package mount

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/Sylos/Spectra/internal/api"
	"codeberg.org/Sylos/Spectra/internal/types"
	"codeberg.org/Sylos/Spectra/sdk"
)

func startTestAPIServer(t *testing.T) (*httptest.Server, *sdk.SpectraFS) {
	t.Helper()
	cfgPath := writePersistentTestConfig(t)
	fs, err := sdk.New(cfgPath)
	if err != nil {
		t.Fatalf("sdk.New: %v", err)
	}
	cfg := fs.GetConfig()
	server := api.NewServer(fs, &cfg.API)
	ts := httptest.NewServer(server.GetRouter())
	t.Cleanup(func() {
		ts.Close()
		fs.Close()
	})
	return ts, fs
}

func TestHTTPBackend_GetNode_Root(t *testing.T) {
	ts, _ := startTestAPIServer(t)
	backend := NewHTTPBackend(ts.URL, "primary")

	node, err := backend.GetNode("/")
	if err != nil {
		t.Fatalf("GetNode(/): %v", err)
	}
	if node.ID != "root" {
		t.Errorf("root ID = %q, want root", node.ID)
	}
}

func TestHTTPBackend_ListChildren(t *testing.T) {
	ts, _ := startTestAPIServer(t)
	backend := NewHTTPBackend(ts.URL, "primary")

	result, err := backend.ListChildren("/")
	if err != nil {
		t.Fatalf("ListChildren: %v", err)
	}
	if len(result.Folders)+len(result.Files) == 0 {
		t.Error("expected generated children at root")
	}
}

func TestHTTPBackend_ReadFile(t *testing.T) {
	ts, _ := startTestAPIServer(t)
	backend := NewHTTPBackend(ts.URL, "primary")

	result, err := backend.ListChildren("/")
	if err != nil {
		t.Fatalf("ListChildren: %v", err)
	}
	if len(result.Files) == 0 {
		t.Skip("no files generated at root")
	}

	filePath := result.Files[0].Path
	data, err := backend.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) == 0 && result.Files[0].Size > 0 {
		t.Error("expected non-empty file data")
	}
}

func TestHTTPBackend_GetNode_NotFound(t *testing.T) {
	ts, _ := startTestAPIServer(t)
	backend := NewHTTPBackend(ts.URL, "primary")

	_, err := backend.GetNode("/missing-node")
	if err == nil {
		t.Fatal("expected error for missing node")
	}
}

func TestDecodeNode(t *testing.T) {
	raw := map[string]any{
		"id":   "root",
		"path": "/",
		"type": types.NodeTypeFolder,
		"existence_map": map[string]any{
			"primary": true,
		},
	}
	node, err := decodeNode(raw)
	if err != nil {
		t.Fatalf("decodeNode: %v", err)
	}
	if node.ID != "root" {
		t.Errorf("id = %q", node.ID)
	}

	b, _ := json.Marshal(node)
	if len(b) == 0 {
		t.Error("expected marshaled node")
	}
	_ = httptest.NewRecorder()
	_ = http.StatusOK
}
