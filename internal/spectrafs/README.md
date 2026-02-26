# SpectraFS Package

SpectraFS is the **persistent** filesystem implementation: it uses the database layer and the generator to provide a stateful, BoltDB-backed synthetic filesystem. It is used by the SDK when config `mode` is `"persistent"` (the default). The SDK uses either SpectraFS or EphemeralFS, not both.

## Structure

```
spectrafs/
├── spectrafs.go   # SpectraFS type, NewSpectraFS, ListChildren, GetNode, CreateFolder, UploadFile, DeleteNode, Reset, GetFileData, etc.
├── file.go       # fs.File and fs.ReadDirFile implementations
├── fileinfo.go   # fs.FileInfo implementation
├── direntry.go   # fs.DirEntry implementation
└── models/       # Request interfaces and structs (ParentIdentifier, ListChildrenRequest, etc.)
```

## Core Responsibilities

- **Orchestration**: Loads config, opens DB (with buffer and optional cache), creates RNG. ListChildren resolves parent (by ID or path+world), fetches or lazily generates children, persists them, returns filtered result.
- **World-aware operations**: All node access respects ExistenceMap and requested world (table name).
- **Lazy generation**: Children are generated on first list at a given parent/depth and then stored; subsequent lists read from DB.
- **fs.FS**: SpectraFSWrapper projects one world as an `fs.FS` (ReadFile, ReadDir, Stat, Glob) for tools like Rclone. Only available in persistent mode from the SDK.

## Key Operations

- **ListChildren(req)** – Parent by ParentID or ParentPath+TableName; optional Depth in request (used for generation). Returns folders and files for that world.
- **GetNode(req)** – By ID or Path+TableName.
- **CreateFolder(req)**, **UploadFile(req)** – Create nodes with deterministic IDs; persisted via db buffer.
- **DeleteNode(req)**, **Reset()** – Delete node or clear all and recreate root.
- **GetFileData(id)** – Generate file content and checksum (deterministic or RNG-based).
- **GetConfig**, **GetTableInfo**, **GetNodeCount**, **GetSecondaryTables**, **GetStats**, **FlushStats** – Config and metadata from DB.

## Configuration

Uses config for DB path, cache flag, generator parameters (depth, fanout, seed), and secondary tables. NewSpectraFS takes a config file path and loads it via config.LoadFromFile.

## Usage

Created by the SDK when `config.Mode != "ephemeral"`. The API server and CLI use `sdk.New(configPath)`, which returns a wrapper that delegates to SpectraFS or EphemeralFS based on mode.
