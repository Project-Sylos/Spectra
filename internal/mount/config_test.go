package mount

import (
	"testing"

	"codeberg.org/Sylos/Spectra/internal/types"
)

func TestResolveMounts_FromConfig(t *testing.T) {
	cfg := &types.Config{
		SecondaryTables: map[string]float64{"s1": 0.7},
		Mount: &types.MountConfig{
			Enabled: true,
			Paths: map[string]string{
				"primary": "/mnt/spectra",
			},
		},
	}
	mounts, err := ResolveMounts(cfg, nil)
	if err != nil {
		t.Fatalf("ResolveMounts: %v", err)
	}
	if len(mounts) != 2 {
		t.Fatalf("expected 2 mounts, got %d", len(mounts))
	}
	if mounts[0].World != "primary" || mounts[0].Path == "" {
		t.Errorf("unexpected primary mount: %+v", mounts[0])
	}
	foundS1 := false
	for _, m := range mounts {
		if m.World == "s1" {
			foundS1 = true
			if m.Path != "/mnt/spectra-s1" {
				t.Errorf("s1 path = %q, want /mnt/spectra-s1", m.Path)
			}
		}
	}
	if !foundS1 {
		t.Error("expected s1 mount")
	}
}

func TestParseMountFlag(t *testing.T) {
	world, path, err := ParseMountFlag("primary:/tmp/x")
	if err != nil {
		t.Fatal(err)
	}
	if world != "primary" || path != "/tmp/x" {
		t.Errorf("got %q %q", world, path)
	}
	_, _, err = ParseMountFlag("invalid")
	if err == nil {
		t.Error("expected error for invalid mount flag")
	}
}
