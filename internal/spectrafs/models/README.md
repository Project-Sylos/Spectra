# Request Models and Interface System

This package provides a flexible, interface-based request system for Spectra operations. The design allows users to pass simple struct literals while maintaining type safety and validation through Go interfaces.

## Overview

The request system is built on three core components:

1. **Interfaces** (`interfaces.go`) - Define contracts for request types
2. **Request Structs** (`requests.go`) - Concrete implementations of request interfaces
3. **Helper Functions** (`helpers.go`) - Convenient constructors for common use cases

## Design Philosophy

### Simple Struct Literals

Users can create requests using straightforward struct literals:

```go
// Simple, clean syntax
req := &models.GetNodeRequest{
    ID: "root",
}

// Or with path-based lookup
req := &models.GetNodeRequest{
    Path:      "/folder1",
    TableName: "primary",
}
```

### Interface-Based Validation

Each request struct implements one or more interfaces that define its capabilities:

- **`NodeIdentifier`**: Identifies a node by ID or Path+TableName
- **`ParentIdentifier`**: Identifies a parent node by ParentID or ParentPath+TableName
- **`NamedRequest`**: Provides a Name field for creation operations
- **`DataRequest`**: Provides a Data field for file uploads
- **`StatusRequest`**: Provides a Status field for traversal status updates

### Type Safety

The interface system provides compile-time guarantees:

```go
// This function only accepts requests that can identify a node
func ProcessNode(req models.NodeIdentifier) error {
    // Validate that either ID or (Path + TableName) is provided
    if err := models.ValidateNodeIdentifier(req); err != nil {
        return err
    }
    // ... process node
}
```

## Core Interfaces

### NodeIdentifier

Identifies a node using either ID or Path+TableName:

```go
type NodeIdentifier interface {
    GetID() string
    GetPath() string
    GetTableName() string
}
```

**Usage:** `GetNodeRequest`, `DeleteNodeRequest`, `UpdateTraversalStatusRequest`

### ParentIdentifier

Identifies a parent node using either ParentID or ParentPath+TableName:

```go
type ParentIdentifier interface {
    GetParentID() string
    GetParentPath() string
    GetTableName() string
}
```

**Usage:** `ListChildrenRequest`, `CreateFolderRequest`, `UploadFileRequest`

### NamedRequest

Provides a name for resource creation:

```go
type NamedRequest interface {
    GetName() string
}
```

**Usage:** `CreateFolderRequest`, `UploadFileRequest`

### DataRequest

Provides data payload for file operations:

```go
type DataRequest interface {
    GetData() []byte
}
```

**Usage:** `UploadFileRequest`

### StatusRequest

Provides status information for traversal tracking:

```go
type StatusRequest interface {
    GetStatus() string
}
```

**Usage:** `UpdateTraversalStatusRequest`

## Request Structs

### GetNodeRequest

Retrieve a node by ID or Path+TableName.

**Implements:** `NodeIdentifier`

**Fields:**
- `ID` (string): Direct node ID (e.g., "root", "s1-abc123")
- `Path` (string): Node path (e.g., "/", "/folder1")
- `TableName` (string): Table name ("primary", "s1", "s2", etc.)

**Examples:**
```go
// By ID
req := &models.GetNodeRequest{ID: "root"}

// By Path
req := &models.GetNodeRequest{
    Path:      "/folder1",
    TableName: "primary",
}
```

### ListChildrenRequest

List children of a parent node with lazy generation.

**Implements:** `ParentIdentifier`

**Fields:**
- `ParentID` (string): Direct parent node ID
- `ParentPath` (string): Parent node path
- `TableName` (string): Table name (required when using ParentPath)

**Examples:**
```go
// By ParentID
req := &models.ListChildrenRequest{ParentID: "root"}

// By ParentPath
req := &models.ListChildrenRequest{
    ParentPath: "/",
    TableName:  "primary",
}
```

### CreateFolderRequest

Create a new folder node.

**Implements:** `ParentIdentifier`, `NamedRequest`

**Fields:**
- `ParentID` (string): Direct parent node ID
- `ParentPath` (string): Parent node path
- `TableName` (string): Table name (required when using ParentPath)
- `Name` (string, required): Name of the folder to create

**Examples:**
```go
// By ParentID
req := &models.CreateFolderRequest{
    ParentID: "root",
    Name:     "new-folder",
}

// By ParentPath
req := &models.CreateFolderRequest{
    ParentPath: "/",
    TableName:  "primary",
    Name:       "new-folder",
}
```

### UploadFileRequest

Upload a file with data processing.

**Implements:** `ParentIdentifier`, `NamedRequest`, `DataRequest`

**Fields:**
- `ParentID` (string): Direct parent node ID
- `ParentPath` (string): Parent node path
- `TableName` (string): Table name (required when using ParentPath)
- `Name` (string, required): Name of the file to upload
- `Data` ([]byte, required): File content

**Examples:**
```go
// By ParentID
req := &models.UploadFileRequest{
    ParentID: "root",
    Name:     "data.txt",
    Data:     []byte("file content"),
}

// By ParentPath
req := &models.UploadFileRequest{
    ParentPath: "/folder1",
    TableName:  "primary",
    Name:       "data.txt",
    Data:       []byte("file content"),
}
```

### DeleteNodeRequest

Delete a node by ID or Path+TableName.

**Implements:** `NodeIdentifier`

**Fields:**
- `ID` (string): Direct node ID
- `Path` (string): Node path
- `TableName` (string): Table name (required when using Path)

**Examples:**
```go
// By ID
req := &models.DeleteNodeRequest{ID: "p-abc123"}

// By Path
req := &models.DeleteNodeRequest{
    Path:      "/folder1/subfolder",
    TableName: "primary",
}
```

### UpdateTraversalStatusRequest

Update the traversal status of a node.

**Implements:** `NodeIdentifier`, `StatusRequest`

**Fields:**
- `ID` (string): Direct node ID
- `Path` (string): Node path
- `TableName` (string): Table name (required when using Path)
- `Status` (string, required): New status ("pending", "successful", or "failed")

**Examples:**
```go
// By ID
req := &models.UpdateTraversalStatusRequest{
    ID:     "p-abc123",
    Status: "successful",
}

// By Path
req := &models.UpdateTraversalStatusRequest{
    Path:      "/folder1",
    TableName: "primary",
    Status:    "successful",
}
```

## Helper Functions

The `helpers.go` file provides convenient constructor functions for common use cases:

```go
// Get node helpers
req := models.NewGetNodeRequest("root")
req := models.NewGetNodeRequestByPath("/folder1", "primary")

// List children helpers
req := models.NewListChildrenRequest("root")
req := models.NewListChildrenRequestByPath("/", "primary")

// Create folder helpers
req := models.NewCreateFolderRequest("root", "new-folder")
req := models.NewCreateFolderRequestByPath("/", "primary", "new-folder")

// Upload file helpers
req := models.NewUploadFileRequest("root", "file.txt", data)
req := models.NewUploadFileRequestByPath("/", "primary", "file.txt", data)

// Delete node helpers
req := models.NewDeleteNodeRequest("p-abc123")
req := models.NewDeleteNodeRequestByPath("/folder1", "primary")

// Update status helpers
req := models.NewUpdateTraversalStatusRequest("p-abc123", "successful")
req := models.NewUpdateTraversalStatusRequestByPath("/folder1", "primary", "successful")
```

## Validation

The package provides validation functions for each interface:

```go
// Validate NodeIdentifier
if err := models.ValidateNodeIdentifier(req); err != nil {
    // Either ID or (Path + TableName) must be provided
    return err
}

// Validate ParentIdentifier
if err := models.ValidateParentIdentifier(req); err != nil {
    // Either ParentID or (ParentPath + TableName) must be provided
    return err
}
```

## Extending the System

To add a new request type:

1. **Define the struct** in `requests.go`:
```go
type MyCustomRequest struct {
    ID        string `json:"id,omitempty"`
    CustomField string `json:"custom_field"`
}
```

2. **Implement required interfaces**:
```go
func (r *MyCustomRequest) GetID() string { return r.ID }
func (r *MyCustomRequest) GetCustomField() string { return r.CustomField }
```

3. **Add helper constructors** in `helpers.go`:
```go
func NewMyCustomRequest(id, customField string) *MyCustomRequest {
    return &MyCustomRequest{
        ID:          id,
        CustomField: customField,
    }
}
```

4. **Use in SDK methods**:
```go
func (s *SpectraFS) MyCustomOperation(req models.NodeIdentifier) error {
    if err := models.ValidateNodeIdentifier(req); err != nil {
        return err
    }
    // ... implementation
}
```

## Benefits

This design provides several advantages:

1. **Clean API**: Users write simple, readable struct literals
2. **Type Safety**: Compile-time guarantees through interfaces
3. **Flexibility**: Multiple lookup methods (ID vs Path+TableName)
4. **Validation**: Centralized validation logic
5. **Extensibility**: Easy to add new request types or fields
6. **Documentation**: Self-documenting through interface contracts
7. **Testing**: Easy to mock with interface-based testing

## JSON Compatibility

All request structs include JSON tags for API compatibility:

```json
{
  "parent_id": "root",
  "name": "new-folder"
}
```

Or with path-based lookup:

```json
{
  "parent_path": "/",
  "table_name": "primary",
  "name": "new-folder"
}
```

The `omitempty` tag ensures unused fields are not serialized.

