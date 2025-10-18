# Development Setup Scripts

This directory contains scripts to set up the development environment for Spectra on different platforms.

## Windows Setup

### `windows.ps1`
PowerShell script for setting up the development environment on Windows.

#### What it does:
- **Checks for MSYS2**: Verifies MSYS2 installation for CGO support
- **Installs GCC**: Sets up the MinGW-w64 GCC compiler
- **Downloads DuckDB**: Downloads and sets up DuckDB binaries
- **Configures Environment**: Sets up CGO environment variables
- **Installs Dependencies**: Runs `go mod tidy` to install Go dependencies

#### Prerequisites:
- Windows 10/11
- PowerShell 5.1 or later
- Go 1.24.2 or later installed
- Internet connection for downloading dependencies

#### Usage:
```powershell
# Run from the project root directory
.\dev_setup_scripts\windows.ps1
```

#### Environment Variables Set:
- `CGO_ENABLED=1`
- `CC=x86_64-w64-mingw32-gcc`
- `CGO_CFLAGS=-I/path/to/duckdb/include`
- `CGO_LDFLAGS=-L/path/to/duckdb/lib -lduckdb`

#### What gets installed:
- MSYS2 (if not already installed)
- MinGW-w64 GCC compiler
- DuckDB binaries and headers
- Go dependencies via `go mod tidy`

#### DuckDB Version Compatibility:
Spectra uses specific DuckDB versions for Windows compatibility:
- `go-duckdb v1.7.0`
- `apache/arrow/go/v14 v14.0.2`

These versions are known to work well with the Windows CGO setup and avoid common linking issues.

## Manual Setup

If the automated script doesn't work or you prefer manual setup:

### Windows Manual Setup
1. Install MSYS2 from https://www.msys2.org/
2. Install MinGW-w64: `pacman -S mingw-w64-x86_64-gcc`
3. Download DuckDB binaries from https://duckdb.org/
4. Set environment variables:
   ```cmd
   set CGO_ENABLED=1
   set CC=x86_64-w64-mingw32-gcc
   set CGO_CFLAGS=-I/path/to/duckdb/include
   set CGO_LDFLAGS=-L/path/to/duckdb/lib -lduckdb
   ```
5. Run `go mod tidy`

### Linux Setup
```bash
# Install build dependencies
sudo apt-get update
sudo apt-get install build-essential

# Install DuckDB
# Follow DuckDB installation instructions for your distribution

# Set environment variables
export CGO_ENABLED=1
export CC=gcc
export CGO_CFLAGS=-I/path/to/duckdb/include
export CGO_LDFLAGS=-L/path/to/duckdb/lib -lduckdb

# Install Go dependencies
go mod tidy
```

### macOS Setup
```bash
# Install build dependencies
xcode-select --install

# Install DuckDB via Homebrew
brew install duckdb

# Set environment variables
export CGO_ENABLED=1
export CC=clang
export CGO_CFLAGS=-I/opt/homebrew/include
export CGO_LDFLAGS=-L/opt/homebrew/lib -lduckdb

# Install Go dependencies
go mod tidy
```

## Troubleshooting

### Common Issues

#### CGO Compilation Errors
- Ensure GCC is properly installed and in PATH
- Verify DuckDB headers and libraries are accessible
- Check that CGO environment variables are set correctly

#### DuckDB Linking Errors
- Verify DuckDB binaries are compatible with your system
- Check that library paths are correct
- Ensure DuckDB version is compatible with go-duckdb

#### Go Module Issues
- Run `go mod tidy` to resolve dependencies
- Check that Go version is 1.24.2 or later
- Verify internet connection for dependency downloads

### Getting Help

If you encounter issues:
1. Check the error messages carefully
2. Verify all prerequisites are installed
3. Try manual setup steps
4. Check the project's issue tracker
5. Ask for help in the project's community channels

## Future Scripts

Additional setup scripts may be added for:
- Linux (Ubuntu/Debian, CentOS/RHEL, etc.)
- macOS
- Docker-based development environment
- CI/CD environment setup
