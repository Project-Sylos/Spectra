package models

// NewGetNodeRequest creates a GetNodeRequest by ID
func NewGetNodeRequest(id string) *GetNodeRequest {
	return &GetNodeRequest{ID: id}
}

// NewGetNodeRequestByPath creates a GetNodeRequest by path and table name
func NewGetNodeRequestByPath(path, tableName string) *GetNodeRequest {
	return &GetNodeRequest{
		Path:      path,
		TableName: tableName,
	}
}

// NewListChildrenRequest creates a ListChildrenRequest by parent ID
func NewListChildrenRequest(parentID string) *ListChildrenRequest {
	return &ListChildrenRequest{ParentID: parentID}
}

// NewListChildrenRequestByPath creates a ListChildrenRequest by parent path and table name
func NewListChildrenRequestByPath(parentPath, tableName string) *ListChildrenRequest {
	return &ListChildrenRequest{
		ParentPath: parentPath,
		TableName:  tableName,
	}
}

// NewCreateFolderRequest creates a CreateFolderRequest
func NewCreateFolderRequest(parentID, name string) *CreateFolderRequest {
	return &CreateFolderRequest{
		ParentID: parentID,
		Name:     name,
	}
}

// NewCreateFolderRequestByPath creates a CreateFolderRequest by parent path
func NewCreateFolderRequestByPath(parentPath, tableName, name string) *CreateFolderRequest {
	return &CreateFolderRequest{
		ParentPath: parentPath,
		TableName:  tableName,
		Name:       name,
	}
}

// NewUploadFileRequest creates an UploadFileRequest
func NewUploadFileRequest(parentID, name string, data []byte) *UploadFileRequest {
	return &UploadFileRequest{
		ParentID: parentID,
		Name:     name,
		Data:     data,
	}
}

// NewUploadFileRequestByPath creates an UploadFileRequest by parent path
func NewUploadFileRequestByPath(parentPath, tableName, name string, data []byte) *UploadFileRequest {
	return &UploadFileRequest{
		ParentPath: parentPath,
		TableName:  tableName,
		Name:       name,
		Data:       data,
	}
}

// NewDeleteNodeRequest creates a DeleteNodeRequest by ID
func NewDeleteNodeRequest(id string) *DeleteNodeRequest {
	return &DeleteNodeRequest{ID: id}
}

// NewDeleteNodeRequestByPath creates a DeleteNodeRequest by path
func NewDeleteNodeRequestByPath(path, tableName string) *DeleteNodeRequest {
	return &DeleteNodeRequest{
		Path:      path,
		TableName: tableName,
	}
}

// NewUpdateTraversalStatusRequest creates an UpdateTraversalStatusRequest
func NewUpdateTraversalStatusRequest(id, status string) *UpdateTraversalStatusRequest {
	return &UpdateTraversalStatusRequest{
		ID:     id,
		Status: status,
	}
}

// NewUpdateTraversalStatusRequestByPath creates an UpdateTraversalStatusRequest by path
func NewUpdateTraversalStatusRequestByPath(path, tableName, status string) *UpdateTraversalStatusRequest {
	return &UpdateTraversalStatusRequest{
		Path:      path,
		TableName: tableName,
		Status:    status,
	}
}
