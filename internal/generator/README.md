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
- **UUID-based IDs**: Consistent unique identifiers across all tables
- **Probability-Based Secondary Nodes**: Secondary table generation based on configurable probabilities
- **File Data Generation**: 1KB random data with SHA256 checksums
- **Depth-Aware Generation**: Respects maximum depth constraints

## Key Components

### RNG (Random Number Generator)
- Wraps Go's `math/rand` with seeding support
- Provides deterministic random generation
- Used for all procedural generation decisions

### Node Generation
- `GenerateChildren()` - Generate child nodes for a parent
- `GenerateSecondaryNodes()` - Determine secondary table existence based on probabilities
- `generateFolder()` - Create folder nodes with UUID-based IDs
- `generateFile()` - Create file nodes with UUID-based IDs

### File Data Generation
- `GenerateFileData()` - Generate 1KB random data with checksum
- `GenerateFileDataForUpload()` - Process uploaded data and generate checksum
- `GenerateChecksum()` - SHA256 checksum generation

## Generation Logic

### Primary Node Generation
1. Generate children based on configuration (min/max folders, files)
2. Create UUID-based IDs with `p-` prefix
3. Set appropriate depth levels and paths
4. Generate random names and timestamps

### Secondary Node Generation
1. For each primary node, roll dice against secondary table probabilities
2. If probability check passes, create secondary node with same UUID but different prefix
3. Secondary nodes reference primary parent ID
4. Update primary node's existence map

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
// Generate children for a parent node
children, err := generator.GenerateChildren(parentNode, config, rng)

// Generate secondary nodes based on probabilities
secondaryNodes, err := generator.GenerateSecondaryNodes(primaryNode, config, rng)

// Generate file data with checksum
data, checksum, err := generator.GenerateFileData(rng)
```
