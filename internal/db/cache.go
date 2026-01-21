package db

import (
	"sync"

	"codeberg.org/Sylos/Spectra/internal/types"
)

// NodeCache provides a thread-safe cache for nodes with sliding window eviction
type NodeCache struct {
	mu       sync.RWMutex
	nodes    map[string]*types.Node // nodeID -> Node
	maxDepth int                    // Track max depth for eviction
	enabled  bool
}

// NewNodeCache creates a new node cache
func NewNodeCache(enabled bool) *NodeCache {
	return &NodeCache{
		nodes:    make(map[string]*types.Node),
		maxDepth: 0,
		enabled:  enabled,
	}
}

// Get retrieves a node from the cache
// Returns the node and true if found, nil and false otherwise
func (nc *NodeCache) Get(nodeID string) (*types.Node, bool) {
	if !nc.enabled || nodeID == "" {
		return nil, false
	}

	nc.mu.RLock()
	defer nc.mu.RUnlock()

	node, ok := nc.nodes[nodeID]
	return node, ok
}

// Set adds or updates a node in the cache
// Updates maxDepth if the node's depth is greater
func (nc *NodeCache) Set(node *types.Node) {
	if !nc.enabled || node == nil {
		return
	}

	nc.mu.Lock()
	defer nc.mu.Unlock()

	nc.nodes[node.ID] = node

	// Update maxDepth if this node is deeper
	if node.DepthLevel > nc.maxDepth {
		nc.maxDepth = node.DepthLevel
	}
}

// GetNodesForEviction returns node IDs that should be evicted
// Evicts nodes where depth < maxDepth-2 (keeps 3-generation window)
func (nc *NodeCache) GetNodesForEviction() []string {
	if !nc.enabled {
		return nil
	}

	nc.mu.RLock()
	defer nc.mu.RUnlock()

	// Don't evict until we have at least 3 generations
	if nc.maxDepth < 3 {
		return nil
	}

	evictionThreshold := nc.maxDepth - 2
	var toEvict []string

	for nodeID, node := range nc.nodes {
		if node.DepthLevel < evictionThreshold {
			toEvict = append(toEvict, nodeID)
		}
	}

	return toEvict
}

// EvictNodes removes specified nodes from the cache
func (nc *NodeCache) EvictNodes(nodeIDs []string) {
	if !nc.enabled || len(nodeIDs) == 0 {
		return
	}

	nc.mu.Lock()
	defer nc.mu.Unlock()

	for _, nodeID := range nodeIDs {
		delete(nc.nodes, nodeID)
	}
}

// Clear removes all nodes from the cache and resets maxDepth
func (nc *NodeCache) Clear() {
	if !nc.enabled {
		return
	}

	nc.mu.Lock()
	defer nc.mu.Unlock()

	nc.nodes = make(map[string]*types.Node)
	nc.maxDepth = 0
}
