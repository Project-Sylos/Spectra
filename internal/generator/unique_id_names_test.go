// Copyright 2025 Sylos contributors
// SPDX-License-Identifier: LGPL-2.1-or-later

package generator

import (
	"strings"
	"testing"

	"codeberg.org/Sylos/Spectra/internal/types"
)

func TestUniqueIDNamesDisplay(t *testing.T) {
	parent := &types.Node{
		ID: "root", Path: "/", Type: types.NodeTypeFolder,
		ExistenceMap: map[string]bool{"primary": true},
	}
	cfg := &types.Config{
		Seed: types.SeedConfig{
			MaxDepth: 2, MaxFolders: 3, MaxFiles: 3,
			FolderBackoffFactor: 1, FolderDepthDecayFactor: 1,
			FileBackoffFactor: 1, FileDepthDecayFactor: 1,
			Seed: 1, UniqueIDNames: true,
		},
		SecondaryTables: map[string]float64{},
	}
	rng := NewRNG(1)
	folder, err := generateFolder(parent, 1, 1, cfg, rng)
	if err != nil {
		t.Fatal(err)
	}
	if folder.Name != folder.ID {
		t.Fatalf("folder name=%q want id=%q", folder.Name, folder.ID)
	}
	if !strings.HasPrefix(folder.Name, "spc:") {
		t.Fatalf("expected spc: id name, got %q", folder.Name)
	}
	if folder.Path != "/"+folder.ID {
		t.Fatalf("path=%q", folder.Path)
	}

	file, err := generateFile(parent, 1, 1, cfg, rng)
	if err != nil {
		t.Fatal(err)
	}
	wantFile := file.ID + ".txt"
	if file.Name != wantFile {
		t.Fatalf("file name=%q want %q", file.Name, wantFile)
	}
	if !strings.HasSuffix(file.Name, ".txt") {
		t.Fatalf("file should keep .txt: %q", file.Name)
	}

	// Classic mode still uses numbered names.
	cfg.Seed.UniqueIDNames = false
	classic, err := generateFolder(parent, 2, 1, cfg, rng)
	if err != nil {
		t.Fatal(err)
	}
	if classic.Name != "folder_2" {
		t.Fatalf("classic name=%q", classic.Name)
	}
}

func TestUniqueIDNamesDeterministicAcrossCalls(t *testing.T) {
	parent := &types.Node{
		ID: "root", Path: "/", Type: types.NodeTypeFolder,
		ExistenceMap: map[string]bool{"primary": true},
	}
	cfg := &types.Config{
		Seed: types.SeedConfig{
			MaxDepth: 2, MaxFolders: 2, MaxFiles: 2,
			FolderBackoffFactor: 1, FolderDepthDecayFactor: 1,
			FileBackoffFactor: 1, FileDepthDecayFactor: 1,
			Seed: 7, UniqueIDNames: true,
		},
	}
	a, err := generateFolder(parent, 1, 1, cfg, NewRNG(7))
	if err != nil {
		t.Fatal(err)
	}
	b, err := generateFolder(parent, 1, 1, cfg, NewRNG(7))
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != b.ID || a.Name != b.Name || a.Path != b.Path {
		t.Fatalf("not deterministic: %+v vs %+v", a, b)
	}
}
