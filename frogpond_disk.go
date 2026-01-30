package frogpond

import (
	"fmt"

	"gitlab.com/donomii/ensemblekv"
)

// NewDiskNode creates a new frogpond node with disk persistence enabled using ExtentKV.
// This preserves the "append-only" nature which is efficient for spinning disks.
func NewDiskNode(path string, size int64) (*Node, error) {
	backend, err := ensemblekv.ExtentCreator(path, 4096, size)
	if err != nil {
		return nil, fmt.Errorf("failed to create extent backend: %w", err)
	}

	return &Node{
		DataPool: &DataPoolMap{
			trie:    nil,
			backend: backend,
		},
	}, nil
}

// NewMegapoolNode creates a new frogpond node with disk persistence using MegaPoolKV.
// MegaPoolKV offers better crash resistance using a tree structure.
func NewMegapoolNode(path string, size int64) (*Node, error) {
	backend, err := ensemblekv.MegapoolCreator(path, 4096, size)
	if err != nil {
		return nil, fmt.Errorf("failed to create megapool backend: %w", err)
	}

	return &Node{
		DataPool: &DataPoolMap{
			trie:    nil,
			backend: backend,
		},
	}, nil
}

// Close closes the underlying backend connection if one exists.
func (n *Node) Close() error {
	n.DataPool.mu.Lock()
	defer n.DataPool.mu.Unlock()

	if n.DataPool.backend != nil {
		return n.DataPool.backend.Close()
	}
	return nil
}

// Flush ensures all data is written to disk.
func (n *Node) Flush() error {
	n.DataPool.mu.Lock()
	defer n.DataPool.mu.Unlock()

	if n.DataPool.backend != nil {
		return n.DataPool.backend.Flush()
	}
	return nil
}
