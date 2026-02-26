# Generator Package

The generator package handles procedural generation of nodes and file data. It is shared by both the persistent (SpectraFS) and ephemeral (EphemeralFS) implementations. It uses a seeded RNG for reproducible, deterministic output and produces stable node IDs via path + type (no randomness in IDs).

## Structure

```
generator/
├── generator.go   # RNG, sampleWeightedRange, GenerateChildren, generateFolder, generateFile
├── checksum.go   # ComputeChecksum, GenerateFileData (RNG-based 1KB + SHA256)
└── deterministic.go  # GenerateDeterministicFileData(seed) for ephemeral file content
```

## Core Features

- **Seeded RNG**: Thread-safe wrapper around `math/rand`; same seed yields same sequence.
- **Deterministic node IDs**: Node IDs are not generated here; callers use `utils.DeterministicNodeID(path, type)` (root or `spc:` + hex). Generator uses parent path to build child paths only.
- **Weighted fanout**: Logarithmic buckets (e.g. [0–10), [10–100), …) with exponential backoff; depth decay applied to effective max.
- **Existence map**: For each generated child, primary is true; secondaries are rolled with config probability (parent must exist in that world).
- **File data**: 1KB random data + SHA256 checksum; optional deterministic variant via `GenerateDeterministicFileData(seed)`.

## Key Components

### RNG
- **NewRNG(seed)** – Create seeded RNG.
- **Intn(n)**, **Float64()**, **Read(buf)** – Thread-safe random values.

### Node generation
- **GenerateChildren(parent, depth, rng, cfg)** – Returns a slice of child nodes (folders then files). Uses `sampleWeightedRange` for folder and file counts; calls `generateFolder` / `generateFile` which build path via `utils.JoinPath(parent.Path, name)` and ID via `utils.DeterministicNodeID(pathStr, type)`.
- **generateFolder** / **generateFile** – Build Node with path, ParentPath, ExistenceMap, etc.; IDs come from utils.

### File data
- **GenerateFileData(rng)** – 1KB random bytes + checksum (for persistent/lazy content).
- **GenerateDeterministicFileData(baseSeed)** – Same content every time for given seed (used by ephemeral).
- **ComputeChecksum(data)** – SHA256 hex string.

## Configuration

Uses `types.Config` for: max_depth, max_folders/max_files, folder/file backoff and depth-decay factors, seed, secondary_tables (world probabilities). No min_folders/min_files; distribution is weighted by backoff and decay.

## Usage

- **SpectraFS**: Uses a single long-lived RNG for lazy generation; generates children when a level is first listed and persists them via db.
- **EphemeralFS**: Creates a fresh RNG per ListChildren call, seeded from (config seed, path or world//path, depth); does not persist; all returned children are marked existing in the requested world.
