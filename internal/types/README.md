# Types Package

The types package defines shared data structures and constants used across Spectra: configuration, nodes, list results, and API responses. Used by config, db, generator, spectrafs, ephemeralfs, API, and SDK.

## Structure

```
types/
└── types.go   # Config, SeedConfig, APIConfig, Node, Folder, File, ListResult, TableInfo, Stats, constants
```

## Core Types

### Config
- **Mode** – `"persistent"` or `"ephemeral"` (drives SDK implementation choice).
- **Seed** – SeedConfig (generation and DB/cache options).
- **API** – Host and port.
- **SecondaryTables** – map[world]probability (e.g. `"s1": 0.7`).

### SeedConfig
- **MaxDepth**, **MaxFolders**, **MaxFiles** – Depth and fanout limits.
- **FolderBackoffFactor**, **FolderDepthDecayFactor** (and file variants) – Weighted distribution and depth decay.
- **Seed** – RNG seed.
- **DBPath**, **FileBinarySeed**, **EnableCache** – DB path, deterministic file seed, optional cache.
- **DivergingTreeMode** – Ephemeral only: seed with world//path for per-world tree shape.

### Node
- **ID** – `"root"` or `spc:` + hex (deterministic from path and type).
- **ParentID**, **Name**, **Path**, **ParentPath**, **Type** (`folder`/`file`), **DepthLevel**, **Size**, **LastUpdated**, **Checksum**, **ExistenceMap**, **ChildIDs**.

### ListResult
- **Success**, **Message** – Result status.
- **Folders** – `[]Folder` (each embeds Node).
- **Files** – `[]File` (each embeds Node).

### Other
- **APIResponse** – Success, Message, Data.
- **TableInfo** – Name, RowCount, TableType.
- **Stats** – FileCount, FolderCount, TotalFileSize, SecondaryNodes.

## Constants

- **NodeTypeFolder**, **NodeTypeFile**
- **StatusPending**, **StatusSuccessful**, **StatusFailed**
- **CopyStatusPending**, **CopyStatusInProgress**, **CopyStatusCompleted**

## Usage

Imported by all internal packages and re-exported by the SDK. JSON and db tags align with API and BoltDB usage.
