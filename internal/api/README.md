# API Package

The API package provides the HTTP interface for Spectra, built on top of the Chi router. It's organized into modular handlers, middleware, and models for maintainability.

## Structure

```
api/
├── handlers/          # Endpoint handlers organized by domain
│   ├── base.go       # Common handler functionality
│   ├── health.go     # Health check endpoints
│   ├── item.go       # Item operations (files and folders)
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

- **Modular Handlers**: Each handler focuses on a specific domain (items, node, system)
- **Base Handler**: Common functionality shared across all handlers
- **Middleware Support**: Extensible middleware system for cross-cutting concerns
- **Type Safety**: Strongly typed request/response models
- **Error Handling**: Consistent error responses with proper HTTP status codes
- **Request Model Conversion**: API models are converted to spectrafs request models for processing

## Handlers

### BaseHandler
Provides common functionality for all handlers:
- `sendJSON()` - Send JSON responses
- `sendError()` - Send error responses
- `sendSuccess()` - Send success responses

### Domain Handlers
- **HealthHandler**: Health check endpoints
- **ItemHandler**: Item operations (list, create folder, upload file, get file data)
- **NodeHandler**: Generic node operations (get, delete)
- **SystemHandler**: System operations (reset, config, world information)

## Middleware

- **CORS**: Cross-origin resource sharing support
- **Chi Middleware**: Logger, recoverer, request ID, real IP, timeout

## Request Models

The API uses its own request models in `models/requests.go` that mirror the structure of spectrafs request models. These are converted internally to spectrafs models for processing:

```go
// API request model (JSON from HTTP)
type ListChildrenRequest struct {
    ParentID   string `json:"parent_id,omitempty"`
    ParentPath string `json:"parent_path,omitempty"`
    TableName  string `json:"table_name,omitempty"`
}

// Converted to spectrafs request model
spectrafsRequest := &spectrafsmodels.ListChildrenRequest{
    ParentID:   apiRequest.ParentID,
    ParentPath: apiRequest.ParentPath,
    TableName:  apiRequest.TableName,
}
```

This separation allows the API layer to evolve independently from the core business logic.

## Routes

All API routes are prefixed with `/api/v1/` and organized by domain:

- `/api/v1/items/*` - Item operations (list, create folder, upload file, get metadata, get file data)
- `/api/v1/node/*` - Node operations (get, delete)
- `/api/v1/reset` - System reset
- `/api/v1/config` - Configuration retrieval
- `/api/v1/tables/*` - World information (kept as "tables" for API compatibility)

## Usage

The API is automatically started when running in server mode:

```bash
go run main.go -mode server
```

The server will start on the configured host and port (default: localhost:8086).
