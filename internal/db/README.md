# Database Package

The database package provides the persistence layer for Spectra when `mode` is persistent. It uses BoltDB with a single-bucket design, write-ahead buffering, optional node cache, and per-world existence tracking.

## Structure

```
db/
├── db.go      # DB type, New(), CRUD, GetParentAndChildren, stats, root lifecycle
├── schema.go  # Bucket names and InitializeBuckets / VerifyBucketsExist
├── buffer.go  # OutputBuffer: batched inserts/updates, write-ahead queue, flush
└── cache.go   # NodeCache: optional sliding-window cache (3-generation), eviction
```

## Single-Bucket Architecture

### Unified `nodes` bucket
- All nodes in one bucket. Keys are node IDs (string): `"root"` or `spc:` + hex (deterministic from path and type).
- Value: JSON-serialized `types.Node` (path, type, existence_map, child_ids, etc.).
- Existence map records which worlds (primary, s1, s2, …) each node exists in.

### Index buckets
- **index_path**: path → node ID (path lookups).
- **index_parent_path**: `parentPath|nodeID` → empty (parent-path index).
- **stats**: filesystem stats.

### Parent–child relationships
- Each node has `ChildIDs []string`. Children are read via parent’s ChildIDs (O(1) per child), not by scanning indexes.
- OutputBuffer batches parent ChildIDs updates at flush time.

## Key Types and Behavior

- **DB**: Holds BoltDB connection, secondary table list, StatsBuffer, OutputBuffer, NodeCache. Created with `New(dbPath, secondaryTables, enableCache)`.
- **OutputBuffer** (buffer.go): Queues insert and update operations; flushes when batch size or interval is reached; runs inserts then updates for referential integrity.
- **NodeCache** (cache.go): If enabled, caches nodes with a 3-generation sliding window; evicts by depth to bound memory during BFS.
- **StatsBuffer**: Async stats updates (file/folder counts, sizes).

## Core Operations

- **InsertNode / BufferInsertNode** – Insert node and update indexes; parent ChildIDs updated on flush.
- **GetNodeByID**, **GetNodeByPath** – Lookup by ID or path (with world filtering).
- **GetParentAndChildren** – Fetch parent and children in one vectorized flow using ChildIDs.
- **DeleteNode**, **Flush** – Delete node; flush buffered writes.
- **CreateRootNode**, **DeleteAllNodes** – Root lifecycle and full reset.
- **GetTableInfo**, **GetNodeCount** – World metadata and counts.

## Node IDs

Node IDs are deterministic: root folder is `"root"`; all others are `spc:` + 32-char hex (FNV128a of path and type). See `internal/utils/id.go`. No UUIDs/ULIDs.

## Usage

Used only by `internal/spectrafs` when running in persistent mode. Ephemeral mode does not use this package.
