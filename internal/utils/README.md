# Utils Package

Shared utilities used by spectrafs, ephemeralfs, generator, and db. No external API.

## Structure

```
utils/
├── id.go    # DeterministicNodeID(path, nodeType), normalizePath
├── path.go  # JoinPath(parts...) — root-relative path with "/"
└── id_test.go
```

## id.go

- **DeterministicNodeID(path, nodeType string) string**  
  Returns a stable ID: `"root"` when normalized path is `"/"` and nodeType is `"folder"`; otherwise `"spc:"` + 32-char hex (FNV128a of normalized path + `"|"` + nodeType). Same path and type always yield the same ID across all worlds; used for node identity in both persistent and ephemeral implementations.

- **normalizePath** (internal): Trim space, strip trailing `/`, ensure leading `/`; empty or `"."` → `"/"`.

## path.go

- **JoinPath(parts ...string) string**  
  Joins path segments with `/`; strips leading/trailing slashes from each part; returns `"/"` if no non-empty parts; otherwise `"/" + joined`. Used for building node paths (e.g. parent path + name).

## Usage

Referenced by generator (path and ID for nodes), spectrafs and ephemeralfs (path joining and node IDs), and db (ID format is consistent with these utilities).
