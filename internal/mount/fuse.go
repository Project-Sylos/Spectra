//go:build linux || darwin

package mount

import (
	"context"
	"errors"
	"hash/fnv"
	"io"
	"os"
	"syscall"

	"codeberg.org/Sylos/Spectra/internal/types"
	"codeberg.org/Sylos/Spectra/sdk"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

type spectraNode struct {
	fs.Inode
	backend Backend
	path    string
}

type spectraFileHandle struct {
	backend Backend
	path    string
	data    []byte
	loaded  bool
}

var (
	_ fs.NodeLookuper  = (*spectraNode)(nil)
	_ fs.NodeGetattrer = (*spectraNode)(nil)
	_ fs.NodeReaddirer = (*spectraNode)(nil)
	_ fs.NodeOpener    = (*spectraNode)(nil)
	_ fs.NodeReader    = (*spectraNode)(nil)
	_ fs.NodeMkdirer   = (*spectraNode)(nil)
	_ fs.NodeCreater   = (*spectraNode)(nil)
	_ fs.NodeUnlinker  = (*spectraNode)(nil)
	_ fs.NodeRmdirer   = (*spectraNode)(nil)
	_ fs.NodeWriter    = (*spectraNode)(nil)
	_ fs.FileReader    = (*spectraFileHandle)(nil)
	_ fs.FileWriter    = (*spectraFileHandle)(nil)
)

func newRootNode(backend Backend) *spectraNode {
	return &spectraNode{
		backend: backend,
		path:    "/",
	}
}

func pathIno(path string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(path))
	return h.Sum64()
}

func nodeMode(node *types.Node) uint32 {
	if node.Type == types.NodeTypeFolder {
		return fuse.S_IFDIR | 0755
	}
	return fuse.S_IFREG | 0644
}

func fillAttrFromNode(node *types.Node, attr *fuse.Attr) {
	attr.Mode = nodeMode(node)
	attr.Size = uint64(node.Size)
	sec := uint64(node.LastUpdated.Unix())
	attr.Mtime = sec
	attr.Ctime = sec
	attr.Atime = sec
	attr.Ino = pathIno(node.Path)
	attr.Nlink = 1
}

func fillAttr(node *types.Node, out *fuse.AttrOut) {
	fillAttrFromNode(node, &out.Attr)
	applyAttrTimeout(out)
}

func fillEntry(node *types.Node, out *fuse.EntryOut) {
	fillAttrFromNode(node, &out.Attr)
	applyEntryTimeouts(out)
}

func (n *spectraNode) resolveNode(path string) (*types.Node, error) {
	if node, ok := listingChildByPath(path); ok {
		return node, nil
	}
	return n.backend.GetNode(path)
}

func (n *spectraNode) resolveChild(name string) (*types.Node, error) {
	if node, ok := listingChild(n.path, name); ok {
		return node, nil
	}
	return n.backend.GetNode(JoinChildPath(n.path, name))
}

func errnoFor(err error) syscall.Errno {
	if err == nil {
		return 0
	}
	if errors.Is(err, ErrNotExist) {
		return syscall.ENOENT
	}
	if _, ok := sdk.IsUnauthorized(err); ok {
		return syscall.EACCES
	}
	return syscall.EIO
}

func (n *spectraNode) Getattr(_ context.Context, _ fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	node, err := n.resolveNode(n.path)
	if err != nil {
		return errnoFor(err)
	}
	fillAttr(node, out)
	return 0
}

func (n *spectraNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	childPath := JoinChildPath(n.path, name)
	node, err := n.resolveChild(name)
	if err != nil {
		return nil, errnoFor(err)
	}

	child := &spectraNode{
		backend: n.backend,
		path:    childPath,
	}
	stable := fs.StableAttr{
		Mode: nodeMode(node),
		Ino:  pathIno(childPath),
	}
	inode := n.NewInode(ctx, child, stable)
	fillEntry(node, out)
	return inode, 0
}

func (n *spectraNode) Readdir(_ context.Context) (fs.DirStream, syscall.Errno) {
	result, err := n.backend.ListChildren(n.path)
	if err != nil {
		return nil, errnoFor(err)
	}
	publishListingCache(n.path, result)

	entries := make([]fuse.DirEntry, 0, len(result.Folders)+len(result.Files))
	for _, folder := range result.Folders {
		entries = append(entries, fuse.DirEntry{
			Name: folder.Name,
			Ino:  pathIno(folder.Path),
			Mode: fuse.S_IFDIR | 0755,
		})
	}
	for _, file := range result.Files {
		entries = append(entries, fuse.DirEntry{
			Name: file.Name,
			Ino:  pathIno(file.Path),
			Mode: fuse.S_IFREG | 0644,
		})
	}
	return fs.NewListDirStream(entries), 0
}

func (n *spectraNode) Open(_ context.Context, _ uint32) (fs.FileHandle, uint32, syscall.Errno) {
	node, err := n.backend.GetNode(n.path)
	if err != nil {
		return nil, 0, errnoFor(err)
	}
	if node.Type == types.NodeTypeFolder {
		return nil, 0, syscall.EISDIR
	}
	return &spectraFileHandle{
		backend: n.backend,
		path:    n.path,
	}, 0, 0
}

func (n *spectraNode) Read(_ context.Context, fh fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	reader, ok := fh.(*spectraFileHandle)
	if !ok {
		return nil, syscall.EIO
	}
	return reader.Read(context.Background(), dest, off)
}

func (fh *spectraFileHandle) Read(_ context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	if err := fh.ensureLoaded(); err != nil {
		return nil, errnoFor(err)
	}
	if off >= int64(len(fh.data)) {
		return fuse.ReadResultData(nil), 0
	}
	end := int(off) + len(dest)
	if end > len(fh.data) {
		end = len(fh.data)
	}
	return fuse.ReadResultData(fh.data[int(off):end]), 0
}

func (fh *spectraFileHandle) Write(_ context.Context, data []byte, off int64) (uint32, syscall.Errno) {
	// Metadata-only writes: update declared size via UploadFile semantics.
	node, err := fh.backend.GetNode(fh.path)
	if err != nil {
		return 0, errnoFor(err)
	}
	newSize := off + int64(len(data))
	if newSize < node.Size {
		newSize = node.Size
	}
	parentPath := node.ParentPath
	if parentPath == "" {
		parentPath = "/"
	}
	_, err = fh.backend.CreateFile(parentPath, node.Name, newSize)
	if err != nil {
		return 0, errnoFor(err)
	}
	fh.loaded = false
	return uint32(len(data)), 0
}

func (fh *spectraFileHandle) ensureLoaded() error {
	if fh.loaded {
		return nil
	}
	data, err := fh.backend.ReadFile(fh.path)
	if err != nil {
		return err
	}
	fh.data = data
	fh.loaded = true
	return nil
}

func (n *spectraNode) Mkdir(ctx context.Context, name string, _ uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	invalidateListingCache(n.path)
	node, err := n.backend.CreateFolder(n.path, name)
	if err != nil {
		return nil, errnoFor(err)
	}
	childPath := node.Path
	child := &spectraNode{
		backend: n.backend,
		path:    childPath,
	}
	stable := fs.StableAttr{
		Mode: nodeMode(node),
		Ino:  pathIno(childPath),
	}
	inode := n.NewInode(ctx, child, stable)
	fillEntry(node, out)
	return inode, 0
}

func (n *spectraNode) Create(ctx context.Context, name string, _ uint32, mode uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	invalidateListingCache(n.path)
	node, err := n.backend.CreateFile(n.path, name, 0)
	if err != nil {
		return nil, nil, 0, errnoFor(err)
	}
	childPath := node.Path
	child := &spectraNode{
		backend: n.backend,
		path:    childPath,
	}
	stable := fs.StableAttr{
		Mode: nodeMode(node),
		Ino:  pathIno(childPath),
	}
	inode := n.NewInode(ctx, child, stable)
	fillEntry(node, out)
	fh := &spectraFileHandle{
		backend: n.backend,
		path:    childPath,
		data:    []byte{},
		loaded:  true,
	}
	_ = mode
	return inode, fh, 0, 0
}

func (n *spectraNode) Unlink(_ context.Context, name string) syscall.Errno {
	invalidateListingCache(n.path)
	childPath := JoinChildPath(n.path, name)
	node, err := n.backend.GetNode(childPath)
	if err != nil {
		return errnoFor(err)
	}
	if node.Type == types.NodeTypeFolder {
		return syscall.EISDIR
	}
	if err := n.backend.DeleteNode(childPath); err != nil {
		return errnoFor(err)
	}
	return 0
}

func (n *spectraNode) Rmdir(_ context.Context, name string) syscall.Errno {
	invalidateListingCache(n.path)
	childPath := JoinChildPath(n.path, name)
	node, err := n.backend.GetNode(childPath)
	if err != nil {
		return errnoFor(err)
	}
	if node.Type != types.NodeTypeFolder {
		return syscall.ENOTDIR
	}
	if err := n.backend.DeleteNode(childPath); err != nil {
		return errnoFor(err)
	}
	return 0
}

func (n *spectraNode) Write(ctx context.Context, fh fs.FileHandle, data []byte, off int64) (uint32, syscall.Errno) {
	writer, ok := fh.(*spectraFileHandle)
	if !ok {
		return 0, syscall.EIO
	}
	return writer.Write(ctx, data, off)
}

// MountServer mounts a Spectra world at mountPoint and returns the FUSE mount handle.
func MountServer(mountPoint string, backend Backend) (*FUSEMount, error) {
	if err := os.MkdirAll(mountPoint, 0755); err != nil {
		return nil, err
	}

	root := newRootNode(backend)
	opts := fuseFSOptions(uint32(os.Getuid()), uint32(os.Getgid()))

	server, err := fs.Mount(mountPoint, root, opts)
	if err != nil {
		return nil, err
	}
	return &FUSEMount{
		mountPoint: mountPoint,
		wait:       server.Wait,
		unmount:    server.Unmount,
	}, nil
}

// IsNotSupported returns false on supported platforms.
func IsNotSupported() bool {
	return false
}

var _ = io.EOF