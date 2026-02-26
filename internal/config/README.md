# Config Package

The config package handles configuration management for Spectra: loading from JSON, validation, and default values. It supports both persistent (DB-backed) and ephemeral (no persistence) modes.

## Structure

```
config/
├── config.go     # LoadFromFile, DefaultConfig, Validate, SaveToFile
└── default.json  # Default configuration (mode, seed, api, secondary_tables)
```

## Core Features

- **Mode**: Top-level `mode` — `"persistent"` (default) or `"ephemeral"`. Drives which filesystem implementation the SDK uses.
- **Validation**: Mode, seed bounds, backoff/decay ranges, API port, secondary table probabilities.
- **Defaults**: Sensible defaults for all options; DB path resolved to absolute in persistent mode.
- **File loading**: JSON config with error handling; missing mode defaults to persistent.

## Configuration Sections

### Top-level
- `mode` (string) – `"persistent"` or `"ephemeral"`. Default: `"persistent"`.

### Seed configuration (`seed`)
- `max_depth` – Maximum tree depth (default: 4).
- `max_folders` – Max folder fanout; weighted distribution (default: 100).
- `folder_backoff_factor` – Exponent for folder bucket weights (default: 0.5).
- `folder_depth_decay_factor` – Depth decay for folder count (default: 0.8).
- `max_files` – Max file fanout (default: 100).
- `file_backoff_factor`, `file_depth_decay_factor` – Same for files (defaults: 0.5, 0.85).
- `seed` – RNG seed (default: 42).
- `db_path` – BoltDB path (default: `"./spectra.db"`; used only in persistent mode).
- `file_binary_seed` – Seed for deterministic file content (default: 0).
- `enable_cache` – Enable node cache in persistent mode (default: false).
- `diverging_tree_mode` – Ephemeral only: seed with `world//path` so each world gets a different tree (default: false).

### API configuration (`api`)
- `host` – Server host (default: `"localhost"`).
- `port` – Server port (default: 8086).

### Secondary tables (`secondary_tables`)
- Map of world name → probability (0.0–1.0), e.g. `"s1": 0.7`, `"s2": 0.3`.

## Core Functions

- **LoadFromFile(path)** – Load and validate config from JSON; set mode default, resolve DB path for persistent mode.
- **DefaultConfig()** – Return default `types.Config` (persistent mode, seed 42, etc.).
- **Validate(cfg)** – Validate mode, seed params, API port, secondary probabilities.
- **SaveToFile(cfg, path)** – Write config to JSON.

## Example Configuration

See `default.json` for the full structure. Minimal example:

```json
{
  "mode": "persistent",
  "seed": {
    "max_depth": 4,
    "max_folders": 100,
    "folder_backoff_factor": 0.5,
    "folder_depth_decay_factor": 0.8,
    "max_files": 100,
    "file_backoff_factor": 0.5,
    "file_depth_decay_factor": 0.85,
    "seed": 42,
    "db_path": "./spectra.db",
    "enable_cache": false,
    "diverging_tree_mode": false
  },
  "api": { "host": "localhost", "port": 8086 },
  "secondary_tables": { "s1": 0.7 }
}
```

## Usage

Used by the SDK (to choose implementation and pass config), SpectraFS (persistent), EphemeralFS (ephemeral), and the API server.
