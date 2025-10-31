# Generator Package

The generator package handles procedural generation of nodes and file data for the synthetic filesystem. It creates deterministic, reproducible structures based on configuration parameters.

## Structure

```
generator/
├── generator.go  # Main generation logic for nodes and children
└── checksum.go   # SHA256 checksum generation for file data
```

## Core Features

- **Deterministic Generation**: Seeded random number generator for reproducible results
- **Plain UUID IDs**: Simple unique identifiers without prefixes
- **Inline Existence Mapping**: World existence determined during generation and stored in `ExistenceMap`
- **File Data Generation**: 1KB random data with SHA256 checksums
- **Depth-Aware Generation**: Respects maximum depth constraints

## Key Components

### RNG (Random Number Generator)
- Wraps Go's `math/rand` with seeding support
- Provides deterministic random generation
- Used for all procedural generation decisions

### Node Generation
- `GenerateChildren()` - Generate child nodes with `ExistenceMap` populated
- `generateFolder()` - Create folder nodes with plain UUID IDs
- `generateFile()` - Create file nodes with plain UUID IDs

### File Data Generation
- `GenerateFileData()` - Generate 1KB random data with checksum
- `GenerateFileDataForUpload()` - Process uploaded data and generate checksum
- `GenerateChecksum()` - SHA256 checksum generation

## Generation Logic

### Unified Node Generation
1. Generate children based on configuration (min/max folders, files)
2. Create plain UUID IDs for each node
3. For each node, roll dice against world probabilities
4. Populate `ExistenceMap` based on probability rolls: `{"primary": true, "s1": true, "s2": false}`
5. Set appropriate depth levels, paths, and timestamps
6. Return single flat list of nodes

**Key Improvement:** All nodes generated in a single pass with existence information embedded, eliminating the need for separate primary/secondary generation steps.

### File Data Generation
- Generate 1KB of random data
- Compute SHA256 checksum
- Return both data and checksum for verification

## Configuration Integration

The generator uses configuration parameters for:
- `max_depth` - Maximum tree depth
- `min_folders` / `max_folders` - Folder count range
- `min_files` / `max_files` - File count range
- `seed` - Random number generator seed
- `secondary_tables` - Secondary table probabilities

## Usage

The generator is used internally by the SpectraFS implementation to create the synthetic filesystem structure. It provides the core procedural generation logic that makes Spectra's deterministic behavior possible.

## Example

```go
// Generate children for a parent node (returns single list with ExistenceMap)
children, err := generator.GenerateChildren(parentNode, depth, rng, config)
// Returns: []*types.Node with ExistenceMap populated per node

// Each child has existence information embedded
for _, child := range children {
    // child.ExistenceMap contains: {"primary": true, "s1": true, "s2": false}
    fmt.Printf("Node %s exists in worlds: %v\n", child.ID, child.ExistenceMap)
}

// Generate file data with checksum
data, checksum, err := generator.GenerateFileData(rng)
```
