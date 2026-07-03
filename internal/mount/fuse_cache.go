//go:build linux || darwin

package mount

import (
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// fuseCacheTimeout is how long the kernel may cache FUSE dentries and attrs.
// Matches libfuse defaults; safe for deterministic ephemeral trees.
const fuseCacheTimeout = time.Second

func applyEntryTimeouts(out *fuse.EntryOut) {
	if out == nil {
		return
	}
	out.SetEntryTimeout(fuseCacheTimeout)
	out.SetAttrTimeout(fuseCacheTimeout)
}

func applyAttrTimeout(out *fuse.AttrOut) {
	if out == nil {
		return
	}
	out.SetTimeout(fuseCacheTimeout)
}

func fuseFSOptions(uid, gid uint32) *fs.Options {
	entryTTL := fuseCacheTimeout
	attrTTL := fuseCacheTimeout
	return &fs.Options{
		UID:          uid,
		GID:          gid,
		EntryTimeout: &entryTTL,
		AttrTimeout:  &attrTTL,
		MountOptions: fuse.MountOptions{
			Name: "spectra",
		},
	}
}
