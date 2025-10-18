# Command Line Applications

This directory contains the main entry points for different Spectra applications.

## Structure

```
cmd/
├── api/           # API server application
│   └── main.go    # HTTP API server entry point
└── README.md      # This file
```

## Applications

### API Server (`cmd/api/main.go`)

The API server provides the HTTP interface for Spectra. It starts a web server that exposes all the filesystem operations via RESTful endpoints.

#### Usage

```bash
# Use default configuration
go run cmd/api/main.go

# Use custom configuration
go run cmd/api/main.go configs/custom.json
```

#### Features

- **HTTP Server**: Runs on configurable host and port
- **Graceful Shutdown**: Handles SIGINT/SIGTERM signals
- **Timeout Support**: Configurable read/write timeouts
- **Health Check**: Available at `/health`
- **API Endpoints**: All CRUD operations via `/api/v1/`

#### Configuration

The API server uses the same configuration file as the main application, with the API section controlling server settings:

```json
{
  "api": {
    "host": "localhost",
    "port": 8086
  }
}
```

#### Example Output

```
Spectra API Server
==================
Loading configuration from: configs/default.json
Initializing SpectraFS...
SpectraFS initialized successfully
API config: Host=localhost, Port=8086
Creating API server...
API server created successfully
Starting HTTP server on localhost:8086
API endpoints available at http://localhost:8086/api/v1/
Health check available at http://localhost:8086/health
Press Ctrl+C to stop the server
```

## Future Applications

Additional command-line applications may be added:

- **`cmd/cli/`** - Interactive command-line interface
- **`cmd/benchmark/`** - Performance benchmarking tool
- **`cmd/migrate/`** - Database migration utilities
- **`cmd/test/`** - Test data generation tools

## Development

To add a new command-line application:

1. Create a new directory under `cmd/`
2. Add a `main.go` file with the application logic
3. Update this README with usage instructions
4. Add any necessary configuration or documentation

## Building

To build the applications:

```bash
# Build API server
go build -o bin/spectra-api cmd/api/main.go

# Build all applications
go build -o bin/spectra-api cmd/api/main.go
```

## Docker

The applications can be containerized:

```dockerfile
# API server
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o spectra-api cmd/api/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/spectra-api .
COPY --from=builder /app/configs ./configs
CMD ["./spectra-api"]
```
