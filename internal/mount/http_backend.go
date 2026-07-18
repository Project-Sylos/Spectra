package mount

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"codeberg.org/Sylos/Spectra/internal/types"
	"codeberg.org/Sylos/Spectra/sdk"
)

// HTTPBackend implements Backend over the Spectra REST API.
type HTTPBackend struct {
	baseURL     string
	world       string
	accessToken string
	client      *http.Client
}

// NewHTTPBackend creates a remote backend for the given base URL and world.
func NewHTTPBackend(baseURL, world string) *HTTPBackend {
	if world == "" {
		world = "primary"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &HTTPBackend{
		baseURL: baseURL,
		world:   world,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// SetAccessToken sets the Bearer token used for authenticated API calls.
func (b *HTTPBackend) SetAccessToken(token string) {
	b.accessToken = token
}

func (b *HTTPBackend) World() string {
	return b.world
}

func (b *HTTPBackend) GetNode(path string) (*types.Node, error) {
	body := map[string]string{
		"path":       NormalizePath(path),
		"table_name": b.world,
	}
	var resp types.APIResponse
	if err := b.postJSON("/api/v1/items/get", body, &resp); err != nil {
		return nil, err
	}
	if !resp.Success {
		if strings.Contains(strings.ToLower(resp.Message), "not found") {
			return nil, ErrNotExist
		}
		return nil, fmt.Errorf("%s", resp.Message)
	}
	node, err := decodeNode(resp.Data)
	if err != nil {
		return nil, err
	}
	return CheckNodeInWorld(node, b.world)
}

func (b *HTTPBackend) ListChildren(parentPath string) (*types.ListResult, error) {
	normalized := NormalizePath(parentPath)
	depth := ParentPathDepth(normalized)
	body := map[string]any{
		"parent_path": normalized,
		"table_name":  b.world,
		"depth":       depth,
	}
	var result types.ListResult
	if err := b.postJSON("/api/v1/items/list", body, &result); err != nil {
		return nil, err
	}
	if !result.Success {
		msg := result.Message
		if msg == "" {
			msg = "list children failed"
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return &result, nil
}

func (b *HTTPBackend) ReadFile(path string) ([]byte, error) {
	node, err := b.GetNode(path)
	if err != nil {
		return nil, err
	}
	if node.Type != types.NodeTypeFile {
		return nil, fmt.Errorf("not a file: %s", path)
	}

	url := fmt.Sprintf("%s/api/v1/items/%s/data?table_name=%s", b.baseURL, node.ID, b.world)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if b.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+b.accessToken)
	}
	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, &sdk.UnauthorizedError{Reason: "unauthorized"}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get file data: HTTP %d", resp.StatusCode)
	}

	var envelope types.APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, err
	}
	if !envelope.Success {
		return nil, fmt.Errorf("%s", envelope.Message)
	}

	payload, ok := envelope.Data.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected file data response")
	}
	raw, ok := payload["data"]
	if !ok {
		return nil, fmt.Errorf("missing data in response")
	}
	switch v := raw.(type) {
	case string:
		return base64.StdEncoding.DecodeString(v)
	case []byte:
		return v, nil
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		var out []byte
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, fmt.Errorf("unexpected data type in response")
		}
		return out, nil
	}
}

func (b *HTTPBackend) CreateFolder(parentPath, name string) (*types.Node, error) {
	body := map[string]string{
		"parent_path": NormalizePath(parentPath),
		"table_name":  b.world,
		"name":        name,
	}
	var resp types.APIResponse
	if err := b.postJSON("/api/v1/items/folder", body, &resp); err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("%s", resp.Message)
	}
	node, err := decodeNode(resp.Data)
	if err != nil {
		return nil, err
	}
	return CheckNodeInWorld(node, b.world)
}

func (b *HTTPBackend) CreateFile(parentPath, name string, size int64) (*types.Node, error) {
	body := map[string]any{
		"parent_path": NormalizePath(parentPath),
		"table_name":  b.world,
		"name":        name,
		"data":        make([]byte, size),
	}
	var resp types.APIResponse
	if err := b.postJSON("/api/v1/items/file", body, &resp); err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("%s", resp.Message)
	}
	node, err := decodeNode(resp.Data)
	if err != nil {
		return nil, err
	}
	return CheckNodeInWorld(node, b.world)
}

func (b *HTTPBackend) DeleteNode(path string) error {
	normalized := NormalizePath(path)
	if normalized == "/" {
		return fmt.Errorf("cannot delete root node")
	}
	body := map[string]string{
		"path":       normalized,
		"table_name": b.world,
	}
	var resp types.APIResponse
	if err := b.postJSON("/api/v1/items/delete", body, &resp); err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Message)
	}
	return nil
}

func (b *HTTPBackend) Close() error {
	return nil
}

func (b *HTTPBackend) postJSON(path string, body any, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	url := b.baseURL + path
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if b.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+b.accessToken)
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return &sdk.UnauthorizedError{Reason: "unauthorized"}
	}
	if resp.StatusCode == http.StatusNotFound {
		return ErrNotExist
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(raw))
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return err
	}
	return nil
}

func decodeNode(data any) (*types.Node, error) {
	if data == nil {
		return nil, fmt.Errorf("empty node data")
	}
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var node types.Node
	if err := json.Unmarshal(b, &node); err != nil {
		return nil, err
	}
	return &node, nil
}
