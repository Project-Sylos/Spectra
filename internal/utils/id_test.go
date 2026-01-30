package utils

import (
	"testing"
)

func TestDeterministicNodeID_Root(t *testing.T) {
	got := DeterministicNodeID("/", "folder")
	if got != "root" {
		t.Errorf("DeterministicNodeID(\"/\", \"folder\") = %q; want \"root\"", got)
	}
	// Variants that normalize to root
	for _, path := range []string{"", ".", "  /  ", "/"} {
		got := DeterministicNodeID(path, "folder")
		if got != "root" {
			t.Errorf("DeterministicNodeID(%q, \"folder\") = %q; want \"root\"", path, got)
		}
	}
}

func TestDeterministicNodeID_Prefix(t *testing.T) {
	got := DeterministicNodeID("/a/b", "folder")
	if len(got) < 4 || got[:4] != "spc:" {
		t.Errorf("DeterministicNodeID(\"/a/b\", \"folder\") = %q; want prefix \"spc:\"", got)
	}
}

func TestDeterministicNodeID_Deterministic(t *testing.T) {
	path, nodeType := "/some/path", "file"
	a := DeterministicNodeID(path, nodeType)
	b := DeterministicNodeID(path, nodeType)
	if a != b {
		t.Errorf("same inputs gave different IDs: %q vs %q", a, b)
	}
}

func TestDeterministicNodeID_DifferentPathDifferentID(t *testing.T) {
	a := DeterministicNodeID("/a", "folder")
	b := DeterministicNodeID("/b", "folder")
	if a == b {
		t.Errorf("different paths should give different IDs: both %q", a)
	}
}

func TestDeterministicNodeID_DifferentTypeDifferentID(t *testing.T) {
	a := DeterministicNodeID("/a", "folder")
	b := DeterministicNodeID("/a", "file")
	if a == b {
		t.Errorf("different types should give different IDs: both %q", a)
	}
}

func TestDeterministicNodeID_Normalization(t *testing.T) {
	// Paths that normalize to the same value should yield the same ID (except root)
	a := DeterministicNodeID("/foo/bar", "folder")
	b := DeterministicNodeID("foo/bar", "folder")
	c := DeterministicNodeID("/foo/bar/", "folder")
	if a != b || b != c {
		t.Errorf("normalized paths should match: %q, %q, %q", a, b, c)
	}
}
