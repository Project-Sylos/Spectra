# Config Package

The config package handles configuration management for Spectra, including loading, validating, and providing default configurations.

## Structure

```
config/
└── config.go  # Configuration loading, validation, and defaults
```

## Core Features

- **Multi-Section Configuration**: Supports seed, API, and secondary table configurations
- **Validation**: Comprehensive validation of configuration parameters
- **Default Values**: Sensible defaults for all configuration options
- **File Loading**: JSON configuration file loading with error handling

## Configuration Sections

### Seed Configuration
Controls procedural generation parameters:
- `max_depth` - Maximum tree depth (default: 4)
- `min_folders` / `max_folders` - Folder count range (default: 1-3)
- `min_files` / `max_files` - File count range (default: 2-5)
- `seed` - Random number generator seed (default: 42)
- `db_path` - Database file path (default: "./spectra.db")

### API Configuration
Controls HTTP server settings:
- `host` - Server host (default: "localhost")
- `port` - Server port (default: 8086)

### Secondary Tables Configuration
Defines secondary table probabilities:
- `s1`, `s2`, etc. - Table names with probability values (0.0-1.0)

## Core Functions

### Configuration Loading
- `LoadFromFile(path)` - Load configuration from JSON file
- `DefaultConfig()` - Get default configuration
- `SaveToFile(config, path)` - Save configuration to file

### Validation
- `Validate(config)` - Validate configuration parameters
- Checks for valid ranges, required fields, and data types
- Validates secondary table probabilities (0.0-1.0)
- Validates API port range (1-65535)

## Example Configuration

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

## Usage

The config package is used throughout the application to provide consistent configuration management. It's used by the main application, SpectraFS, and API layers.

## Error Handling

Comprehensive error handling for:
- File not found errors
- JSON parsing errors
- Validation errors
- Invalid parameter ranges

## Default Behavior

If no configuration file is provided, the application uses sensible defaults that allow it to run immediately without setup.
