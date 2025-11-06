package spectrafs

import (
	"io"
	"io/fs"

	"github.com/Project-Sylos/Spectra/internal/types"
)

// spectraFile implements fs.File for regular files
type spectraFile struct {
	node    *types.Node
	data    []byte
	offset  int64
	closeFn func() error
}

// spectraDir implements fs.ReadDirFile for directories
type spectraDir struct {
	node    *types.Node
	entries []fs.DirEntry
	closeFn func() error
}

// Stat returns the FileInfo structure describing file
func (f *spectraFile) Stat() (fs.FileInfo, error) {
	return NewFileInfo(f.node), nil
}

// Read reads up to len(b) bytes from the file
func (f *spectraFile) Read(b []byte) (int, error) {
	if f.data == nil {
		return 0, io.EOF
	}

	if f.offset >= int64(len(f.data)) {
		return 0, io.EOF
	}

	n := copy(b, f.data[f.offset:])
	f.offset += int64(n)
	return n, nil
}

// Close closes the file
func (f *spectraFile) Close() error {
	if f.closeFn != nil {
		return f.closeFn()
	}
	return nil
}

// Stat returns the FileInfo structure describing dir
func (d *spectraDir) Stat() (fs.FileInfo, error) {
	return NewFileInfo(d.node), nil
}

// Read reads up to len(b) bytes from the directory
// For directories, this typically returns 0, io.EOF
func (d *spectraDir) Read(b []byte) (int, error) {
	return 0, io.EOF
}

// ReadDir reads the contents of the directory and returns
// a slice of up to n DirEntry values in directory order
func (d *spectraDir) ReadDir(n int) ([]fs.DirEntry, error) {
	if d.entries == nil {
		return nil, io.EOF
	}

	if n <= 0 {
		// Return all entries
		result := make([]fs.DirEntry, len(d.entries))
		copy(result, d.entries)
		d.entries = nil
		return result, nil
	}

	if len(d.entries) == 0 {
		return nil, io.EOF
	}

	// Return up to n entries
	count := n
	if count > len(d.entries) {
		count = len(d.entries)
	}

	result := make([]fs.DirEntry, count)
	copy(result, d.entries[:count])
	d.entries = d.entries[count:]

	var err error
	if len(d.entries) == 0 {
		err = io.EOF
	}

	return result, err
}

// Close closes the directory
func (d *spectraDir) Close() error {
	if d.closeFn != nil {
		return d.closeFn()
	}
	return nil
}
