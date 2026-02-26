# EphemeralFS Package

EphemeralFS is a stateless, non-persistent filesystem implementation. It generates nodes and children on the fly from path and depth using the shared generator and deterministic IDs. Used when config `mode` is `"ephemeral"`.

## Structure

```
ephemeralfs/
├── ephemeralfs.go   # EphemeralFS type, ListChildren, GetNode, GetFileData, CreateFolder, UploadFile, no-ops (Delete, Reset, Close, stats)
└── ephemeralfs_test.go
```

## Behavior

- **No database**: Nothing is stored; every list/get/create returns synthetic or generated data.
- **ListChildren**: Requires `parent_path` and `depth`. RNG is seeded from (config seed, path or world//path, depth). Children are generated via `generator.GenerateChildren`; then `ExistenceMap[requestWorld]` is set to true for every returned child so callers that filter by world always see a consistent tree for that world.
- **Diverging tree mode**: If `seed.diverging_tree_mode` is true, the seed key for generation is `worldName//path` so each world gets a different tree shape; node paths in results remain normal (e.g. `/folder_1`), not prefixed with world.
- **GetNode**: Returns a synthetic node (root or generic) with full existence map; no persistence.
- **GetFileData**: Returns deterministic content and checksum from `generator.GenerateDeterministicFileData(config.Seed.FileBinarySeed)`.
- **CreateFolder / UploadFile**: Return synthetic nodes with deterministic IDs (`utils.DeterministicNodeID`); not persisted.
- **DeleteNode, Reset, Close, GetNodeCount, GetTableInfo, GetStats, FlushStats**: No-ops or return zero/empty.

## Interface

EphemeralFS implements the same interface as SpectraFS used by the SDK: ListChildren, GetNode, GetFileData, CreateFolder, UploadFile, DeleteNode, Reset, Close, GetConfig, GetNodeCount, GetTableInfo, GetSecondaryTables, GetStats, FlushStats. The SDK selects it when `config.Mode == "ephemeral"`.

## Usage

Instantiated by the SDK via `ephemeralfs.NewEphemeralFS(cfg)`. Callers use the SDK; in ephemeral mode they must supply `parent_path` and `depth` on ListChildren requests.
