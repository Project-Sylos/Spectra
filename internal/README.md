# Internal Package

This directory contains the internal implementation of Spectra. These packages are not intended for external use and may change without notice.

## Structure

- **`api/`** - HTTP API layer with handlers, middleware, and routing
- **`config/`** - Configuration management and validation
- **`db/`** - Database layer with BoltDB operations and multi-world support
- **`generator/`** - Procedural generation of nodes and file data
- **`spectrafs/`** - Core filesystem simulator logic
- **`types/`** - Type definitions and data structures

## Design Principles

- **Layered Architecture**: Clear separation between API, business logic, and data layers
- **Multi-World Support**: Primary and secondary worlds with probability-based distribution
- **UUID-based IDs**: Consistent identification across all worlds
- **Error Handling**: Comprehensive error handling with proper error propagation
- **Modular Design**: Each package has a single responsibility

## Usage

These packages are used internally by the SDK and should not be imported directly by external code. Use the public SDK interface in the `sdk/` directory instead.
