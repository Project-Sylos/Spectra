package ephemeralfs

import (
	"encoding/json"
	"sort"
	"testing"

	"codeberg.org/Sylos/Spectra/internal/spectrafs/models"
	"codeberg.org/Sylos/Spectra/internal/types"
)

func testConfig() *types.Config {
	return &types.Config{
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
			DBPath:                 "",
			FileBinarySeed:         0,
			EnableCache:            false,
		},
		API: types.APIConfig{Host: "localhost", Port: 8086},
		SecondaryTables: map[string]float64{"s1": 0.7},
	}
}

// stableFingerprint returns a comparable representation of ListResult (paths and IDs only)
// so we can assert determinism without depending on timestamps.
func stableFingerprint(r *types.ListResult) string {
	type entry struct {
		Path string `json:"path"`
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	}
	var entries []entry
	for _, f := range r.Folders {
		entries = append(entries, entry{f.Path, f.ID, f.Name, f.Type})
	}
	for _, f := range r.Files {
		entries = append(entries, entry{f.Path, f.ID, f.Name, f.Type})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Path != entries[j].Path {
			return entries[i].Path < entries[j].Path
		}
		return entries[i].ID < entries[j].ID
	})
	out, _ := json.Marshal(entries)
	return string(out)
}

func TestListChildren_Determinism(t *testing.T) {
	cfg := testConfig()
	fs, err := NewEphemeralFS(cfg)
	if err != nil {
		t.Fatalf("NewEphemeralFS: %v", err)
	}
	depth := 1
	req := &models.ListChildrenRequest{
		ParentPath: "/",
		TableName:  "primary",
		Depth:      &depth,
	}

	r1, err := fs.ListChildren(req)
	if err != nil {
		t.Fatalf("ListChildren: %v", err)
	}
	if !r1.Success {
		t.Fatalf("ListChildren failed: %s", r1.Message)
	}

	r2, err := fs.ListChildren(req)
	if err != nil {
		t.Fatalf("ListChildren (2nd): %v", err)
	}
	if !r2.Success {
		t.Fatalf("ListChildren (2nd) failed: %s", r2.Message)
	}

	fp1 := stableFingerprint(r1)
	fp2 := stableFingerprint(r2)
	if fp1 != fp2 {
		t.Errorf("ListChildren not deterministic: fingerprints differ\n%q\n%q", fp1, fp2)
	}
}

func TestListChildren_NoDuplicatePaths(t *testing.T) {
	cfg := testConfig()
	fs, err := NewEphemeralFS(cfg)
	if err != nil {
		t.Fatalf("NewEphemeralFS: %v", err)
	}
	depth := 2
	req := &models.ListChildrenRequest{
		ParentPath: "/",
		TableName:  "primary",
		Depth:      &depth,
	}

	r, err := fs.ListChildren(req)
	if err != nil {
		t.Fatalf("ListChildren: %v", err)
	}
	if !r.Success {
		t.Fatalf("ListChildren failed: %s", r.Message)
	}

	seen := make(map[string]bool)
	for _, f := range r.Folders {
		if seen[f.Path] {
			t.Errorf("duplicate folder path: %q", f.Path)
		}
		seen[f.Path] = true
	}
	for _, f := range r.Files {
		if seen[f.Path] {
			t.Errorf("duplicate file path: %q", f.Path)
		}
		seen[f.Path] = true
	}
}

func TestListChildren_RequiresPathAndDepth(t *testing.T) {
	cfg := testConfig()
	fs, err := NewEphemeralFS(cfg)
	if err != nil {
		t.Fatalf("NewEphemeralFS: %v", err)
	}

	// Missing path
	reqNoPath := &models.ListChildrenRequest{TableName: "primary", Depth: intPtr(1)}
	r, _ := fs.ListChildren(reqNoPath)
	if r.Success {
		t.Error("expected failure when path is missing")
	}
	if r.Message == "" {
		t.Error("expected message when path is missing")
	}

	// Missing depth
	reqNoDepth := &models.ListChildrenRequest{ParentPath: "/", TableName: "primary"}
	r, _ = fs.ListChildren(reqNoDepth)
	if r.Success {
		t.Error("expected failure when depth is missing")
	}
	if r.Message == "" {
		t.Error("expected message when depth is missing")
	}
}

func intPtr(n int) *int { return &n }

func TestListChildren_DivergingTreeMode(t *testing.T) {
	// With DivergingTreeMode false: same path in any world → same children (identical trees).
	cfgOff := testConfig()
	cfgOff.Seed.DivergingTreeMode = false
	fsOff, _ := NewEphemeralFS(cfgOff)
	depth := 1
	reqPrimary := &models.ListChildrenRequest{ParentPath: "/", TableName: "primary", Depth: &depth}
	reqS1 := &models.ListChildrenRequest{ParentPath: "/", TableName: "s1", Depth: &depth}
	rPrimary, _ := fsOff.ListChildren(reqPrimary)
	rS1, _ := fsOff.ListChildren(reqS1)
	if !rPrimary.Success || !rS1.Success {
		t.Fatalf("ListChildren failed: %s / %s", rPrimary.Message, rS1.Message)
	}
	fpOffP := stableFingerprint(rPrimary)
	fpOffS := stableFingerprint(rS1)
	if fpOffP != fpOffS {
		t.Errorf("with DivergingTreeMode false, same path in different worlds should yield same children; got different fingerprints")
	}

	// With DivergingTreeMode true: same path in different worlds → different children (diverging trees).
	cfgOn := testConfig()
	cfgOn.Seed.DivergingTreeMode = true
	fsOn, _ := NewEphemeralFS(cfgOn)
	rPrimary2, _ := fsOn.ListChildren(reqPrimary)
	rS1_2, _ := fsOn.ListChildren(reqS1)
	if !rPrimary2.Success || !rS1_2.Success {
		t.Fatalf("ListChildren failed: %s / %s", rPrimary2.Message, rS1_2.Message)
	}
	fpOnP := stableFingerprint(rPrimary2)
	fpOnS := stableFingerprint(rS1_2)
	if fpOnP == fpOnS {
		t.Errorf("with DivergingTreeMode true, same path in different worlds (primary vs s1) should yield different children; got identical fingerprints")
	}
}
