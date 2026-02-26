# Internal Package

This directory contains the internal implementation of Spectra. These packages are not intended for external use and may change without notice.

## Structure

- **`api/`** – HTTP API layer with handlers, middleware, and routing. Binds the SDK (persistent or ephemeral) to REST endpoints.
- **`config/`** – Configuration loading, validation, and defaults. Supports `mode` (persistent vs ephemeral) and seed/API/secondary_tables.
- **`db/`** – Persistent storage layer: BoltDB, write-ahead buffer, optional node cache, and schema. Used only when `mode` is persistent.
- **`ephemeralfs/`** – Stateless, on-the-fly filesystem implementation. No DB; children generated deterministically from path and depth. Used when `mode` is ephemeral.
- **`generator/`** – Procedural generation: seeded RNG, weighted fanout, deterministic node IDs, file data and checksums. Shared by both SpectraFS and EphemeralFS.
- **`spectrafs/`** – Persistent filesystem implementation. Orchestrates db + generator; lazy generation, world-aware operations, fs.FS wrapper. Used when `mode` is persistent.
- **`types/`** – Shared types: Config, Node, ListResult, SeedConfig, etc.
- **`utils/`** – Path joining and deterministic node ID generation (path + type → stable `root` or `spc:...` IDs).

## Design Principles

- **Layered architecture**: API → SDK (factory) → implementation (spectrafs or ephemeralfs) → db or generator.
- **Dual mode**: Config `mode` selects persistent (DB-backed, SpectraFS) or ephemeral (no persistence, EphemeralFS). Same SDK surface for both.
- **Deterministic IDs**: Node IDs are `root` for root folder or `spc:` + FNV128a hex from path and type; world-agnostic and reproducible.
- **Multi-world support**: Primary and secondary worlds with existence maps; diverging-tree option in ephemeral for different tree shapes per world.
- **Modular design**: Each package has a single responsibility; generator and types shared across both implementations.

## Usage

These packages are used internally by the SDK and API. External code should use the public SDK in `sdk/` (e.g. `sdk.New(configPath)`), which selects the implementation based on config.
