//go:build linux || darwin

package mount

import (
	"testing"
	"time"

	"codeberg.org/Sylos/Spectra/internal/types"
)

func TestListingCache_hitAndExpire(t *testing.T) {
	t.Cleanup(func() { invalidateListingCache("/") })

	now := time.Now()
	publishListingCache("/", &types.ListResult{
		Folders: []types.Folder{
			{Node: types.Node{Name: "a", Path: "/a", Type: types.NodeTypeFolder}},
		},
		Files: []types.File{
			{Node: types.Node{Name: "b.txt", Path: "/b.txt", Type: types.NodeTypeFile, Size: 12}},
		},
	})

	node, ok := listingChild("/", "a")
	if !ok || node.Name != "a" {
		t.Fatalf("listingChild folder: ok=%v node=%v", ok, node)
	}
	node, ok = listingChildByPath("/b.txt")
	if !ok || node.Size != 12 {
		t.Fatalf("listingChildByPath file: ok=%v node=%v", ok, node)
	}

	v, _ := dirListings.Load("/")
	v.(*dirListingEntry).expires = now.Add(-time.Millisecond)
	if _, ok := listingChild("/", "a"); ok {
		t.Fatal("expected expired cache miss")
	}
}

func TestParentNameFromPath(t *testing.T) {
	parent, name, ok := parentNameFromPath("/foo/bar")
	if !ok || parent != "/foo" || name != "bar" {
		t.Fatalf("got parent=%q name=%q ok=%v", parent, name, ok)
	}
	if _, _, ok := parentNameFromPath("/"); ok {
		t.Fatal("root should not split")
	}
}

func TestInvalidateListingCache(t *testing.T) {
	t.Cleanup(func() { invalidateListingCache("/parent") })

	publishListingCache("/parent", &types.ListResult{
		Folders: []types.Folder{{Node: types.Node{Name: "x", Path: "/parent/x", Type: types.NodeTypeFolder}}},
	})
	invalidateListingCache("/parent")
	if _, ok := listingChild("/parent", "x"); ok {
		t.Fatal("expected miss after invalidate")
	}
}
