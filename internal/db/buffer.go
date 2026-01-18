// Copyright 2025 Sylos contributors
// SPDX-License-Identifier: LGPL-2.1-or-later

package db

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Project-Sylos/Spectra/internal/types"
	"go.etcd.io/bbolt"
)

// WriteOperation represents a buffered database write operation.
// All operations must implement Execute() to perform the actual DB write.
type WriteOperation interface {
	Execute(tx *bbolt.Tx) error
	GetBucketKeys() []string // Returns all bucket+nodeID keys this operation touches
	GetNodeIDs() []string    // Returns all node IDs this operation affects
}

// InsertNodeOperation represents a single node insertion.
type InsertNodeOperation struct {
	Node *types.Node
}

// GetNodeIDs returns the node IDs this operation affects.
func (op *InsertNodeOperation) GetNodeIDs() []string {
	nodeIDs := []string{op.Node.ID}
	// Also include parent ID since we modify parent's ChildIDs array
	if op.Node.ParentID != "" {
		nodeIDs = append(nodeIDs, op.Node.ParentID)
	}
	return nodeIDs
}

// Execute performs the node insertion within a transaction.
// Note: Parent ChildIDs updates are deferred and batched by the buffer's Flush() method.
func (op *InsertNodeOperation) Execute(tx *bbolt.Tx) error {
	nodesBucket := tx.Bucket([]byte(bucketNodes))
	if nodesBucket == nil {
		return fmt.Errorf("nodes bucket does not exist")
	}

	// Initialize ChildIDs if nil
	if op.Node.ChildIDs == nil {
		op.Node.ChildIDs = []string{}
	}

	// Serialize node to JSON
	nodeJSON, err := json.Marshal(op.Node)
	if err != nil {
		return fmt.Errorf("failed to marshal node %s: %w", op.Node.ID, err)
	}

	// Store node in nodes bucket
	if err := nodesBucket.Put([]byte(op.Node.ID), nodeJSON); err != nil {
		return fmt.Errorf("failed to insert node %s: %w", op.Node.ID, err)
	}

	// Update index_path: key format "{path}" -> value "{nodeID}"
	indexPath := tx.Bucket([]byte(bucketIndexPath))
	if indexPath == nil {
		return fmt.Errorf("index_path bucket does not exist")
	}
	if err := indexPath.Put([]byte(op.Node.Path), []byte(op.Node.ID)); err != nil {
		return fmt.Errorf("failed to update path index for node %s: %w", op.Node.ID, err)
	}

	// Update index_parent_path: key format "{parentPath}|{nodeID}"
	indexParentPath := tx.Bucket([]byte(bucketIndexParentPath))
	if indexParentPath == nil {
		return fmt.Errorf("index_parent_path bucket does not exist")
	}
	parentPathKey := fmt.Sprintf("%s|%s", op.Node.ParentPath, op.Node.ID)
	if err := indexParentPath.Put([]byte(parentPathKey), []byte{}); err != nil {
		return fmt.Errorf("failed to update parent_path index for node %s: %w", op.Node.ID, err)
	}

	return nil
}

// GetBucketKeys returns all bucket keys this operation touches.
func (op *InsertNodeOperation) GetBucketKeys() []string {
	keys := []string{
		fmt.Sprintf("%s-%s", bucketNodes, op.Node.ID),
		fmt.Sprintf("%s-%s", bucketIndexPath, op.Node.Path),
		fmt.Sprintf("%s-%s|%s", bucketIndexParentPath, op.Node.ParentPath, op.Node.ID),
	}
	// Also touches parent node (to update ChildIDs)
	if op.Node.ParentID != "" {
		keys = append(keys, fmt.Sprintf("%s-%s", bucketNodes, op.Node.ParentID))
	}
	return keys
}

// BulkInsertNodeOperation represents a batch of node insertions.
type BulkInsertNodeOperation struct {
	Nodes []*types.Node
}

// GetNodeIDs returns the node IDs this operation affects.
func (op *BulkInsertNodeOperation) GetNodeIDs() []string {
	// Collect all node IDs and unique parent IDs
	nodeIDSet := make(map[string]bool)

	for _, node := range op.Nodes {
		nodeIDSet[node.ID] = true
		// Also include parent ID since we modify parent's ChildIDs array
		if node.ParentID != "" {
			nodeIDSet[node.ParentID] = true
		}
	}

	// Convert set to slice
	ids := make([]string, 0, len(nodeIDSet))
	for id := range nodeIDSet {
		ids = append(ids, id)
	}
	return ids
}

// Execute performs the bulk insert within a transaction.
func (op *BulkInsertNodeOperation) Execute(tx *bbolt.Tx) error {
	nodesBucket := tx.Bucket([]byte(bucketNodes))
	if nodesBucket == nil {
		return fmt.Errorf("nodes bucket does not exist")
	}

	indexPath := tx.Bucket([]byte(bucketIndexPath))
	if indexPath == nil {
		return fmt.Errorf("index_path bucket does not exist")
	}

	indexParentPath := tx.Bucket([]byte(bucketIndexParentPath))
	if indexParentPath == nil {
		return fmt.Errorf("index_parent_path bucket does not exist")
	}

	// Group children by parent for efficient ChildIDs updates
	parentToChildren := make(map[string][]string)

	// Insert all nodes
	for _, node := range op.Nodes {
		// Check if node already exists (INSERT OR IGNORE behavior)
		existingData := nodesBucket.Get([]byte(node.ID))
		if existingData != nil {
			continue // Skip if node already exists
		}

		// Initialize ChildIDs if nil
		if node.ChildIDs == nil {
			node.ChildIDs = []string{}
		}

		// Serialize node to JSON
		nodeJSON, err := json.Marshal(node)
		if err != nil {
			return fmt.Errorf("failed to marshal node %s: %w", node.ID, err)
		}

		// Store node in nodes bucket
		if err := nodesBucket.Put([]byte(node.ID), nodeJSON); err != nil {
			return fmt.Errorf("failed to insert node %s: %w", node.ID, err)
		}

		// Track children by parent
		if node.ParentID != "" {
			parentToChildren[node.ParentID] = append(parentToChildren[node.ParentID], node.ID)
		}

		// Update index_path: key format "{path}" -> value "{nodeID}"
		if err := indexPath.Put([]byte(node.Path), []byte(node.ID)); err != nil {
			return fmt.Errorf("failed to update path index for node %s: %w", node.ID, err)
		}

		// Update index_parent_path: key format "{parentPath}|{nodeID}"
		parentPathKey := fmt.Sprintf("%s|%s", node.ParentPath, node.ID)
		if err := indexParentPath.Put([]byte(parentPathKey), []byte{}); err != nil {
			return fmt.Errorf("failed to update parent_path index for node %s: %w", node.ID, err)
		}
	}

	// Update all parent nodes' ChildIDs arrays in batch
	for parentID, childIDs := range parentToChildren {
		parentData := nodesBucket.Get([]byte(parentID))
		if parentData == nil {
			continue // Parent doesn't exist, skip
		}

		var parent types.Node
		if err := json.Unmarshal(parentData, &parent); err != nil {
			return fmt.Errorf("failed to unmarshal parent node %s: %w", parentID, err)
		}

		// Initialize parent's ChildIDs if nil
		if parent.ChildIDs == nil {
			parent.ChildIDs = []string{}
		}

		// Add new children (check for duplicates)
		for _, childID := range childIDs {
			childExists := false
			for _, existingChildID := range parent.ChildIDs {
				if existingChildID == childID {
					childExists = true
					break
				}
			}
			if !childExists {
				parent.ChildIDs = append(parent.ChildIDs, childID)
			}
		}

		// Save updated parent
		parentJSON, err := json.Marshal(parent)
		if err != nil {
			return fmt.Errorf("failed to marshal parent node %s: %w", parent.ID, err)
		}
		if err := nodesBucket.Put([]byte(parent.ID), parentJSON); err != nil {
			return fmt.Errorf("failed to update parent node %s: %w", parent.ID, err)
		}
	}

	return nil
}

// GetBucketKeys returns all bucket keys this operation touches.
func (op *BulkInsertNodeOperation) GetBucketKeys() []string {
	keys := make([]string, 0, len(op.Nodes)*3)
	parentIDs := make(map[string]bool)

	for _, node := range op.Nodes {
		keys = append(keys,
			fmt.Sprintf("%s-%s", bucketNodes, node.ID),
			fmt.Sprintf("%s-%s", bucketIndexPath, node.Path),
			fmt.Sprintf("%s-%s|%s", bucketIndexParentPath, node.ParentPath, node.ID),
		)
		// Track unique parent IDs
		if node.ParentID != "" {
			parentIDs[node.ParentID] = true
		}
	}

	// Add parent nodes (to update ChildIDs)
	for parentID := range parentIDs {
		keys = append(keys, fmt.Sprintf("%s-%s", bucketNodes, parentID))
	}

	return keys
}

// DeleteNodeOperation represents deletion of a node from all relevant buckets.
type DeleteNodeOperation struct {
	NodeID   string
	ParentID string // Parent ID for updating parent's ChildIDs
}

// GetNodeIDs returns the node IDs this operation affects.
func (op *DeleteNodeOperation) GetNodeIDs() []string {
	nodeIDs := []string{op.NodeID}
	// Also include parent ID since we modify parent's ChildIDs array
	if op.ParentID != "" {
		nodeIDs = append(nodeIDs, op.ParentID)
	}
	return nodeIDs
}

// Execute performs the node deletion within a transaction.
// Note: Parent ChildIDs updates are deferred and batched by the buffer's Flush() method.
func (op *DeleteNodeOperation) Execute(tx *bbolt.Tx) error {
	nodesBucket := tx.Bucket([]byte(bucketNodes))
	if nodesBucket == nil {
		return fmt.Errorf("nodes bucket does not exist")
	}

	// Get the node to retrieve its path info for cleanup
	nodeData := nodesBucket.Get([]byte(op.NodeID))
	if nodeData == nil {
		return fmt.Errorf("node not found: %s", op.NodeID)
	}

	var node types.Node
	if err := json.Unmarshal(nodeData, &node); err != nil {
		return fmt.Errorf("failed to unmarshal node %s: %w", op.NodeID, err)
	}

	// Delete from nodes bucket
	if err := nodesBucket.Delete([]byte(op.NodeID)); err != nil {
		return fmt.Errorf("failed to delete node %s: %w", op.NodeID, err)
	}

	// Delete from index_path
	indexPath := tx.Bucket([]byte(bucketIndexPath))
	if indexPath != nil {
		if err := indexPath.Delete([]byte(node.Path)); err != nil {
			return fmt.Errorf("failed to delete from path index: %w", err)
		}
	}

	// Delete from index_parent_path
	indexParentPath := tx.Bucket([]byte(bucketIndexParentPath))
	if indexParentPath != nil {
		parentPathKey := fmt.Sprintf("%s|%s", node.ParentPath, node.ID)
		if err := indexParentPath.Delete([]byte(parentPathKey)); err != nil {
			return fmt.Errorf("failed to delete from parent_path index: %w", err)
		}
	}

	return nil
}

// GetBucketKeys returns all bucket keys this operation touches.
// Note: We can't know the exact parent ID without the node data, so we return a conservative set.
// The actual keys will be determined during Execute.
func (op *DeleteNodeOperation) GetBucketKeys() []string {
	// We return a key for the node being deleted
	// We can't know the parent ID without fetching the node first,
	// but Execute will handle updating the parent's ChildIDs
	return []string{
		fmt.Sprintf("%s-%s", bucketNodes, op.NodeID),
	}
}

// UpdateExistenceMapOperation represents an existence map update for a node.
type UpdateExistenceMapOperation struct {
	NodeID       string
	ExistenceMap map[string]bool
}

// GetNodeIDs returns the node IDs this operation affects.
func (op *UpdateExistenceMapOperation) GetNodeIDs() []string {
	return []string{op.NodeID}
}

// Execute performs the existence map update within a transaction.
func (op *UpdateExistenceMapOperation) Execute(tx *bbolt.Tx) error {
	nodesBucket := tx.Bucket([]byte(bucketNodes))
	if nodesBucket == nil {
		return fmt.Errorf("nodes bucket does not exist")
	}

	// Get existing node
	nodeData := nodesBucket.Get([]byte(op.NodeID))
	if nodeData == nil {
		return fmt.Errorf("node %s not found", op.NodeID)
	}

	var node types.Node
	if err := json.Unmarshal(nodeData, &node); err != nil {
		return fmt.Errorf("failed to unmarshal node %s: %w", op.NodeID, err)
	}

	// Update existence map
	node.ExistenceMap = op.ExistenceMap

	// Serialize updated node
	updatedNodeData, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("failed to marshal node %s: %w", op.NodeID, err)
	}

	// Store updated node
	if err := nodesBucket.Put([]byte(op.NodeID), updatedNodeData); err != nil {
		return fmt.Errorf("failed to update existence map for %s: %w", op.NodeID, err)
	}

	return nil
}

// GetBucketKeys returns all bucket keys this operation touches.
func (op *UpdateExistenceMapOperation) GetBucketKeys() []string {
	return []string{
		fmt.Sprintf("%s-%s", bucketNodes, op.NodeID),
	}
}

// OutputBuffer batches write operations for efficient database writes.
// It supports three flush triggers: forced, size threshold, and time-based.
type OutputBuffer struct {
	db             *DB
	mu             sync.Mutex
	insertOps      []WriteOperation // Insert operations (executed first)
	updateOps      []WriteOperation // Update operations (executed second)
	pendingNodeIDs map[string]bool  // Set of nodeIDs that have pending operations
	opCount        int              // Total number of operations (for efficient size check)
	batchSize      int
	flushTicker    *time.Ticker
	stopChan       chan struct{}
	wg             sync.WaitGroup
	paused         bool
	stopOnce       sync.Once // Ensures Stop() is idempotent
	flushing       bool
	flushCond      *sync.Cond // Condition variable for threads to wait on during flush
}

// NewOutputBuffer creates a new output buffer that will flush every N operations or every interval.
func NewOutputBuffer(db *DB, batchSize int, flushInterval time.Duration) *OutputBuffer {
	ob := &OutputBuffer{
		db:             db,
		insertOps:      make([]WriteOperation, 0, batchSize),
		updateOps:      make([]WriteOperation, 0, batchSize),
		pendingNodeIDs: make(map[string]bool),
		opCount:        0,
		batchSize:      batchSize,
		flushTicker:    time.NewTicker(flushInterval),
		stopChan:       make(chan struct{}),
		paused:         false,
		flushing:       false,
	}
	ob.flushCond = sync.NewCond(&ob.mu)

	ob.wg.Add(1)
	go ob.flushLoop()

	return ob
}

// isInsertOp returns true if the operation is an insert operation
func isInsertOp(op WriteOperation) bool {
	switch op.(type) {
	case *InsertNodeOperation, *BulkInsertNodeOperation:
		return true
	default:
		return false
	}
}

// waitForFlush blocks if a flush is currently in progress.
func (ob *OutputBuffer) waitForFlush() {
	ob.mu.Lock()
	for ob.flushing {
		ob.flushCond.Wait() // Releases lock, waits, re-acquires lock
	}
	ob.mu.Unlock()
}

// markFlushing sets the flushing flag and creates a new wait channel.
func (ob *OutputBuffer) markFlushing() bool {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	for ob.flushing {
		ob.flushCond.Wait() // Wait for other flush to complete
	}

	// Mark that we're flushing
	ob.flushing = true
	return true
}

// unmarkFlushing clears the flushing flag and wakes all waiting threads.
func (ob *OutputBuffer) unmarkFlushing() {
	ob.mu.Lock()
	ob.flushing = false
	ob.flushCond.Broadcast() // Wake all waiters
	ob.mu.Unlock()
}

// Add adds a write operation to the buffer. If batch size is reached, it triggers a flush.
func (ob *OutputBuffer) Add(op WriteOperation) {
	// Wait if a flush is currently in progress
	ob.waitForFlush()

	ob.mu.Lock()
	// Route operation to appropriate queue
	if isInsertOp(op) {
		ob.insertOps = append(ob.insertOps, op)
	} else {
		ob.updateOps = append(ob.updateOps, op)
	}
	ob.opCount++

	// Update cache immediately if operation affects cached nodes
	if ob.db.nodeCache != nil {
		switch typedOp := op.(type) {
		case *InsertNodeOperation:
			ob.db.nodeCache.Set(typedOp.Node)
		case *BulkInsertNodeOperation:
			for _, node := range typedOp.Nodes {
				ob.db.nodeCache.Set(node)
			}
		case *UpdateExistenceMapOperation:
			// For updates, we'd need to fetch the node, update it, and re-cache
			// For now, we can leave this as-is since most operations are inserts
			// and the cache will be updated when the node is read next time
		}
	}

	// Track which nodeIDs have pending operations
	for _, nodeID := range op.GetNodeIDs() {
		ob.pendingNodeIDs[nodeID] = true
	}

	// Check if we should flush due to batch size
	shouldFlush := ob.opCount >= ob.batchSize
	ob.mu.Unlock()

	if shouldFlush {
		ob.Flush()
	}
}

// HasPendingWrites returns true if there are pending operations for the given nodeID.
func (ob *OutputBuffer) HasPendingWrites(nodeID string) bool {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	return ob.pendingNodeIDs[nodeID]
}

// Flush writes all buffered operations to BoltDB in a single transaction.
// Operations are executed in the order they were added to the buffer.
// This is synchronous and blocks until the flush completes.
func (ob *OutputBuffer) Flush() {
	// Mark that we're flushing (blocks new operations from being added)
	// If another goroutine is already flushing, wait and return
	if !ob.markFlushing() {
		return // Another goroutine handled the flush
	}
	defer ob.unmarkFlushing()

	// Quick lock to take snapshot
	ob.mu.Lock()
	if len(ob.insertOps) == 0 && len(ob.updateOps) == 0 {
		ob.mu.Unlock()
		return
	}

	// Take snapshot of both queues and combine (inserts first, then updates)
	insertBatch := make([]WriteOperation, len(ob.insertOps))
	updateBatch := make([]WriteOperation, len(ob.updateOps))
	copy(insertBatch, ob.insertOps)
	copy(updateBatch, ob.updateOps)

	// Combine batches: inserts first, then updates
	batch := make([]WriteOperation, 0, len(insertBatch)+len(updateBatch))
	batch = append(batch, insertBatch...)
	batch = append(batch, updateBatch...)

	// Clear buffer operations (but keep pendingNodeIDs until write completes)
	ob.insertOps = make([]WriteOperation, 0, ob.batchSize)
	ob.updateOps = make([]WriteOperation, 0, ob.batchSize)
	ob.opCount = 0
	ob.mu.Unlock()

	// Execute all operations in a single transaction (no lock held during DB write)
	err := ob.db.db.Update(func(tx *bbolt.Tx) error {
		// Execute all operations first (inserts, then updates)
		for i, op := range batch {
			if err := op.Execute(tx); err != nil {
				opType := fmt.Sprintf("%T", op)
				return fmt.Errorf("failed to execute operation %d of %d (type: %s): %w", i+1, len(batch), opType, err)
			}
		}

		// Batch parent ChildIDs updates to avoid O(n²) behavior
		// Collect all parent-to-children mappings from insert/delete operations
		parentAdditions := make(map[string][]string)       // parentID -> childIDs to add
		parentRemovals := make(map[string]map[string]bool) // parentID -> childIDs to remove

		for _, op := range batch {
			switch typedOp := op.(type) {
			case *InsertNodeOperation:
				if typedOp.Node.ParentID != "" {
					parentAdditions[typedOp.Node.ParentID] = append(
						parentAdditions[typedOp.Node.ParentID],
						typedOp.Node.ID,
					)
				}
			case *DeleteNodeOperation:
				if typedOp.ParentID != "" {
					if parentRemovals[typedOp.ParentID] == nil {
						parentRemovals[typedOp.ParentID] = make(map[string]bool)
					}
					parentRemovals[typedOp.ParentID][typedOp.NodeID] = true
				}
			}
		}

		// Update each parent's ChildIDs array once
		nodesBucket := tx.Bucket([]byte(bucketNodes))
		if nodesBucket == nil {
			return fmt.Errorf("nodes bucket does not exist for parent updates")
		}

		// Process all unique parents
		allParentIDs := make(map[string]bool)
		for parentID := range parentAdditions {
			allParentIDs[parentID] = true
		}
		for parentID := range parentRemovals {
			allParentIDs[parentID] = true
		}

		for parentID := range allParentIDs {
			parentData := nodesBucket.Get([]byte(parentID))
			if parentData == nil {
				continue // Parent doesn't exist, skip
			}

			var parent types.Node
			if err := json.Unmarshal(parentData, &parent); err != nil {
				return fmt.Errorf("failed to unmarshal parent node %s: %w", parentID, err)
			}

			// Initialize ChildIDs if nil
			if parent.ChildIDs == nil {
				parent.ChildIDs = []string{}
			}

			// Apply removals first
			if removals := parentRemovals[parentID]; removals != nil {
				newChildIDs := make([]string, 0, len(parent.ChildIDs))
				for _, childID := range parent.ChildIDs {
					if !removals[childID] {
						newChildIDs = append(newChildIDs, childID)
					}
				}
				parent.ChildIDs = newChildIDs
			}

			// Apply additions (check for duplicates)
			if additions := parentAdditions[parentID]; additions != nil {
				existingChildren := make(map[string]bool, len(parent.ChildIDs))
				for _, childID := range parent.ChildIDs {
					existingChildren[childID] = true
				}
				for _, childID := range additions {
					if !existingChildren[childID] {
						parent.ChildIDs = append(parent.ChildIDs, childID)
						existingChildren[childID] = true
					}
				}
			}

			// Save updated parent
			parentJSON, err := json.Marshal(parent)
			if err != nil {
				return fmt.Errorf("failed to marshal parent node %s: %w", parent.ID, err)
			}
			if err := nodesBucket.Put([]byte(parent.ID), parentJSON); err != nil {
				return fmt.Errorf("failed to update parent node %s: %w", parent.ID, err)
			}
		}

		return nil
	})

	if err != nil {
		// Log error with details
		fmt.Printf("ERROR flushing output buffer (%d operations): %v\n", len(batch), err)
		// Re-add operations to buffer for retry (route back to correct queues)
		ob.mu.Lock()
		for _, op := range batch {
			if isInsertOp(op) {
				ob.insertOps = append(ob.insertOps, op)
			} else {
				ob.updateOps = append(ob.updateOps, op)
			}
		}
		ob.opCount = len(ob.insertOps) + len(ob.updateOps)
		// Note: pendingNodeIDs already contains these operations, no need to rebuild
		ob.mu.Unlock()
	} else {
		// Success - now clear the pending node IDs for the flushed operations
		ob.mu.Lock()
		for _, op := range batch {
			for _, nodeID := range op.GetNodeIDs() {
				delete(ob.pendingNodeIDs, nodeID)
			}
		}
		ob.mu.Unlock()
	}
}

// flushLoop runs in a goroutine and periodically flushes the buffer.
func (ob *OutputBuffer) flushLoop() {
	defer ob.wg.Done()

	for {
		select {
		case <-ob.flushTicker.C:
			// Check if paused (quick lock)
			ob.mu.Lock()
			paused := ob.paused
			ob.mu.Unlock()

			if !paused {
				ob.Flush() // Flush handles its own locking
			}
		case <-ob.stopChan:
			ob.flushTicker.Stop()
			ob.Flush() // Final flush before stopping (Flush handles its own locking)
			return
		}
	}
}

// Pause pauses the buffer (stops time-based flushing).
// Force-flushes before pausing to ensure state is persisted.
func (ob *OutputBuffer) Pause() {
	ob.Flush() // Force flush before pausing
	ob.mu.Lock()
	ob.paused = true
	ob.mu.Unlock()
}

// Resume resumes the buffer (resumes time-based flushing).
func (ob *OutputBuffer) Resume() {
	ob.mu.Lock()
	ob.paused = false
	ob.mu.Unlock()
}

// Stop gracefully stops the output buffer and flushes remaining operations.
// Uses a timeout to prevent indefinite blocking if the flush loop is stuck.
// This method is idempotent - it can be called multiple times safely.
func (ob *OutputBuffer) Stop() {
	ob.stopOnce.Do(func() {
		close(ob.stopChan)

		// Wait for flush loop to finish, but with a timeout to prevent hanging
		done := make(chan struct{}, 1)
		go func() {
			ob.wg.Wait()
			done <- struct{}{}
		}()

		select {
		case <-done:
			// Flush loop completed successfully
		case <-time.After(2 * time.Second):
			// Timeout - flush loop may be stuck or slow
			// Continue anyway to prevent blocking the entire shutdown
		}
	})
}
