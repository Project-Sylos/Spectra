# Configuration Files

This directory contains configuration files for Spectra. Configuration files are in JSON format and control various aspects of the synthetic filesystem behavior.

## Default Configuration

### `default.json`
The default configuration file that provides sensible defaults for all settings:

```json
{
  "seed": {
    "max_depth": 4,
    "min_folders": 1,
    "max_folders": 3,
    "min_files": 2,
    "max_files": 5,
    "seed": 42,
    "db_path": "./spectra.db"
  },
  "api": {
    "host": "localhost",
    "port": 8086
  },
  "secondary_tables": {
    "s1": 0.7,
    "s2": 0.3
  }
}
```

## Configuration Sections

### Seed Configuration (`seed`)
Controls procedural generation parameters:

- **`max_depth`** (int): Maximum tree depth (default: 4)
- **`min_folders`** (int): Minimum folders per parent (default: 1)
- **`max_folders`** (int): Maximum folders per parent (default: 3)
- **`min_files`** (int): Minimum files per parent (default: 2)
- **`max_files`** (int): Maximum files per parent (default: 5)
- **`seed`** (int64): Random number generator seed (default: 42)
- **`db_path`** (string): Database file path (default: "./spectra.db")

### API Configuration (`api`)
Controls HTTP server settings:

- **`host`** (string): Server host (default: "localhost")
- **`port`** (int): Server port (default: 8086)

### Secondary Tables Configuration (`secondary_tables`)
Defines secondary table probabilities:

- **`s1`, `s2`, etc.** (float): Table names with probability values (0.0-1.0)
- Example: `"s1": 0.7` means 70% of nodes will exist in the s1 table

## Custom Configurations

You can create custom configuration files for different scenarios:

### High Complexity Configuration
```json
{
  "seed": {
    "max_depth": 8,
    "min_folders": 2,
    "max_folders": 8,
    "min_files": 5,
    "max_files": 20,
    "seed": 12345,
    "db_path": "./complex.db"
  },
  "api": {
    "host": "0.0.0.0",
    "port": 8080
  },
  "secondary_tables": {
    "s1": 0.5,
    "s2": 0.3,
    "s3": 0.2
  }
}
```

### Simple Configuration
```json
{
  "seed": {
    "max_depth": 2,
    "min_folders": 1,
    "max_folders": 2,
    "min_files": 1,
    "max_files": 3,
    "seed": 1,
    "db_path": "./simple.db"
  },
  "api": {
    "host": "localhost",
    "port": 8086
  },
  "secondary_tables": {
    "s1": 0.8
  }
}
```

## Usage

### Command Line
```bash
# Use default configuration
go run main.go -mode server

# Use custom configuration
go run main.go -mode server -config configs/custom.json
```

### Programmatic
```go
config, err := config.LoadFromFile("configs/custom.json")
if err != nil {
    log.Fatal(err)
}

fs, err := sdk.NewSpectraFS(config)
```

## Validation

Configuration files are validated when loaded. Invalid configurations will result in clear error messages indicating what needs to be fixed.

### Common Validation Rules
- `max_depth` must be positive
- `min_folders` and `max_folders` must be positive
- `min_files` and `max_files` must be positive
- `port` must be between 1 and 65535
- Secondary table probabilities must be between 0.0 and 1.0
- `db_path` must be a valid file path

## Environment Variables

While not currently supported, future versions may support environment variable overrides for configuration values.
