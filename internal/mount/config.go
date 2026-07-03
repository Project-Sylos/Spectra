package mount

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"codeberg.org/Sylos/Spectra/internal/types"
)

// ResolvedMount maps a world name to an absolute mount path.
type ResolvedMount struct {
	World string
	Path  string
}

// ResolveMounts builds the world→path map from config and CLI overrides.
func ResolveMounts(cfg *types.Config, overrides map[string]string) ([]ResolvedMount, error) {
	paths, err := mountPaths(cfg, overrides)
	if err != nil {
		return nil, err
	}

	primaryPath := paths["primary"]
	worlds := worldsToMount(cfg, paths)

	seen := make(map[string]string)
	mounts := make([]ResolvedMount, 0, len(worlds))
	for _, world := range worlds {
		mountPath, ok := paths[world]
		if !ok || strings.TrimSpace(mountPath) == "" {
			if world == "primary" {
				return nil, fmt.Errorf("mount path for primary world is required")
			}
			mountPath = deriveSecondaryPath(primaryPath, world)
		}
		absPath, err := filepath.Abs(mountPath)
		if err != nil {
			return nil, fmt.Errorf("resolve mount path for %s: %w", world, err)
		}
		if other, exists := seen[absPath]; exists {
			return nil, fmt.Errorf("duplicate mount path %s for worlds %s and %s", absPath, other, world)
		}
		seen[absPath] = world
		mounts = append(mounts, ResolvedMount{World: world, Path: absPath})
	}
	return mounts, nil
}

func mountPaths(cfg *types.Config, overrides map[string]string) (map[string]string, error) {
	if len(overrides) > 0 {
		if _, ok := overrides["primary"]; !ok {
			return nil, fmt.Errorf("mount path for primary world is required (use --mount primary:PATH)")
		}
		return overrides, nil
	}
	if cfg.Mount == nil || !cfg.Mount.Enabled {
		return nil, fmt.Errorf("mount is not enabled in config; use --mount WORLD:PATH or set mount.enabled=true")
	}
	if len(cfg.Mount.Paths) == 0 {
		return nil, fmt.Errorf("no mount paths configured")
	}
	if _, ok := cfg.Mount.Paths["primary"]; !ok {
		return nil, fmt.Errorf("mount path for primary world is required")
	}
	return cfg.Mount.Paths, nil
}

func worldsToMount(cfg *types.Config, paths map[string]string) []string {
	set := map[string]struct{}{"primary": {}}
	for world := range cfg.SecondaryTables {
		set[world] = struct{}{}
	}
	for world := range paths {
		set[world] = struct{}{}
	}
	list := make([]string, 0, len(set))
	for world := range set {
		if world != "primary" {
			list = append(list, world)
		}
	}
	sort.Strings(list)
	return append([]string{"primary"}, list...)
}

func deriveSecondaryPath(primaryPath, world string) string {
	primaryPath = strings.TrimRight(primaryPath, "/")
	return fmt.Sprintf("%s-%s", primaryPath, world)
}

// ParseMountFlag parses WORLD:PATH mount flag values.
func ParseMountFlag(value string) (string, string, error) {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid mount %q, expected WORLD:PATH", value)
	}
	return parts[0], parts[1], nil
}
