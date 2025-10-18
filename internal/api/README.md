# API Package

The API package provides the HTTP interface for Spectra, built on top of the Chi router. It's organized into modular handlers, middleware, and models for maintainability.

## Structure

```
api/
├── handlers/          # Endpoint handlers organized by domain
│   ├── base.go       # Common handler functionality
│   ├── health.go     # Health check endpoints
│   ├── folder.go     # Folder operations
│   ├── file.go       # File operations
│   ├── node.go       # Node operations
│   └── system.go     # System operations
├── middleware/        # HTTP middleware
│   └── cors.go       # CORS middleware
├── models/           # Request/response models
│   └── requests.go   # API request structures
├── router.go         # Route configuration and handler wiring
└── server.go         # HTTP server setup and lifecycle
```

## Design Principles

- **Modular Handlers**: Each handler focuses on a specific domain (folder, file, node, system)
- **Base Handler**: Common functionality shared across all handlers
- **Middleware Support**: Extensible middleware system for cross-cutting concerns
- **Type Safety**: Strongly typed request/response models
- **Error Handling**: Consistent error responses with proper HTTP status codes

## Handlers

### BaseHandler
Provides common functionality for all handlers:
- `sendJSON()` - Send JSON responses
- `sendError()` - Send error responses
- `sendSuccess()` - Send success responses

### Domain Handlers
- **HealthHandler**: Health check endpoints
- **FolderHandler**: Folder CRUD operations
- **FileHandler**: File upload and data retrieval
- **NodeHandler**: Generic node operations (get, delete)
- **SystemHandler**: System operations (reset, config, tables)

## Middleware

- **CORS**: Cross-origin resource sharing support
- **Chi Middleware**: Logger, recoverer, request ID, real IP, timeout

## Routes

All API routes are prefixed with `/api/v1/` and organized by domain:

- `/api/v1/folder/*` - Folder operations
- `/api/v1/file/*` - File operations
- `/api/v1/node/*` - Node operations
- `/api/v1/reset` - System reset
- `/api/v1/config` - Configuration retrieval
- `/api/v1/tables/*` - Table information

## Usage

The API is automatically started when running in server mode:

```bash
go run main.go -mode server
```

The server will start on the configured host and port (default: localhost:8086).
