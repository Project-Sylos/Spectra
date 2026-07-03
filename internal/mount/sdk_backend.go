package mount

import (
	"fmt"
	"strings"

	"codeberg.org/Sylos/Spectra/internal/spectrafs/models"
	"codeberg.org/Sylos/Spectra/internal/types"
	"codeberg.org/Sylos/Spectra/sdk"
)

// SDKBackend implements Backend using an in-process sdk.SpectraFS instance.
type SDKBackend struct {
	fs    *sdk.SpectraFS
	world string
}

// NewSDKBackend creates a backend bound to a specific world.
func NewSDKBackend(fs *sdk.SpectraFS, world string) *SDKBackend {
	if world == "" {
		world = "primary"
	}
	return &SDKBackend{fs: fs, world: world}
}

func (b *SDKBackend) World() string {
	return b.world
}

func (b *SDKBackend) GetNode(path string) (*types.Node, error) {
	node, err := b.fs.GetNode(&models.GetNodeRequest{
		Path:      NormalizePath(path),
		TableName: b.world,
	})
	if err != nil {
		return nil, mapNotExist(err)
	}
	return CheckNodeInWorld(node, b.world)
}

func (b *SDKBackend) ListChildren(parentPath string) (*types.ListResult, error) {
	normalized := NormalizePath(parentPath)
	req := &models.ListChildrenRequest{
		ParentPath: normalized,
		TableName:  b.world,
	}
	if cfg := b.fs.GetConfig(); cfg != nil && cfg.Mode == "ephemeral" {
		depth := ParentPathDepth(normalized)
		req.Depth = &depth
	}
	result, err := b.fs.ListChildren(req)
	if err != nil {
		return nil, err
	}
	if result == nil || !result.Success {
		msg := "list children failed"
		if result != nil && result.Message != "" {
			msg = result.Message
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return result, nil
}

func (b *SDKBackend) ReadFile(path string) ([]byte, error) {
	node, err := b.GetNode(path)
	if err != nil {
		return nil, err
	}
	if node.Type != types.NodeTypeFile {
		return nil, fmt.Errorf("not a file: %s", path)
	}
	data, _, err := b.fs.GetFileData(node.ID)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (b *SDKBackend) CreateFolder(parentPath, name string) (*types.Node, error) {
	node, err := b.fs.CreateFolder(&models.CreateFolderRequest{
		ParentPath: NormalizePath(parentPath),
		TableName:  b.world,
		Name:       name,
	})
	if err != nil {
		return nil, err
	}
	return CheckNodeInWorld(node, b.world)
}

func (b *SDKBackend) CreateFile(parentPath, name string, size int64) (*types.Node, error) {
	data := make([]byte, size)
	node, err := b.fs.UploadFile(&models.UploadFileRequest{
		ParentPath: NormalizePath(parentPath),
		TableName:  b.world,
		Name:       name,
		Data:       data,
	})
	if err != nil {
		return nil, err
	}
	return CheckNodeInWorld(node, b.world)
}

func (b *SDKBackend) DeleteNode(path string) error {
	normalized := NormalizePath(path)
	if normalized == "/" {
		return fmt.Errorf("cannot delete root node")
	}
	err := b.fs.DeleteNode(&models.DeleteNodeRequest{
		Path:      normalized,
		TableName: b.world,
	})
	return err
}

func (b *SDKBackend) Close() error {
	return nil
}

func mapNotExist(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if msg == "node not found" || strings.Contains(msg, "node not found") {
		return ErrNotExist
	}
	return err
}
