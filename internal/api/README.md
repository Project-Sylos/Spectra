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
- **Request Model Conversion**: API models are converted to spectrafs request models for processing

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

- `/api/v1/folder/*` - Folder operations (list, create)
- `/api/v1/file/*` - File operations (upload, get metadata, get data)
- `/api/v1/node/*` - Node operations (get, delete)
- `/api/v1/reset` - System reset
- `/api/v1/config` - Configuration retrieval
- `/api/v1/tables/*` - Table information

## Usage

The API is automatically started when running in server mode:

```bash
go run main.go -mode server
```

The server will start on the configured host and port (default: localhost:8086).
