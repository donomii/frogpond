# Frogpond CRDT Library Specification

## Overview
Frogpond is a lightweight, flexible Conflict-free Replicated Data Type (CRDT) library written in Go. It implements a key-value store where conflict resolution is handled using a Last-Write-Wins (LWW) strategy based on timestamps. It is designed for distributed systems where nodes need to share and synchronize state eventually.

## Architecture

### Data Model
The core unit of data is the `DataPoint`:
```go
type DataPoint struct {
    Key     []byte
    Value   []byte
    Name    string    // Optional metadata
    Updated time.Time // Timestamp for LWW resolution
    Deleted bool      // Tombstone for soft deletes
}
```

### Storage Engine
*   **Structure**: Uses a `patricia.Trie` (from `github.com/tchap/go-patricia/patricia`) for efficient prefix-based lookups and storage.
*   **Concurrency**: Protected by a `sync.RWMutex` to ensure thread safety during reads and writes.

### Conflict Resolution Strategy
Frogpond employs a **Last-Write-Wins (LWW)** strategy:
*   Each `DataPoint` carries an `Updated` timestamp.
*   When merging updates:
    *   If a key does not exist, the new entry is accepted.
    *   If a key exists, the library compares the `Updated` timestamps.
    *   The entry with the later timestamp wins.
    *   If timestamps are equal, the existing value is preserved (implicit tie-breaking, effectively "First-Write-Wins" for identical times, though broadly considered LWW).

## Core Operations

### 1. Set / Append
*   **Set**: Adds or updates a key-value pair.
*   **Append**: Takes a list of `DataPoint`s and merges them into the local store using the conflict resolution strategy.
*   **Logic**:
    ```go
    if new.Updated > current.Updated {
        store[key] = new
    }
    ```

### 2. Get
*   Retrieves a `DataPoint` by key.
*   Returns a deep copy to prevent external mutation of the internal store.

### 3. Delete
*   **Soft Delete**: Sets the `Deleted` flag to `true` and updates the timestamp.
*   **Backdating**: Supports backdating deletions to handle network partitions (e.g., a node disconnects, is deleted by peers, but rejoins with newer updates).

### 4. Synchronization
*   `ToList()`: Exports the entire store as a sorted list.
*   `FromList()`: Replaces the current store with a list.
*   `applyUpdate()`: Merges an incoming list of `DataPoint`s, returning the "delta" (items that actually caused a change).

### 5. Utilities
*   **Prefix Operations**: Get, Set, and Delete by string prefix.
*   **JSON Dump**: Serializes the store to JSON.
*   **Purge**: Removes soft-deleted items older than a specific time.



## Proposed Additions



### 3. Pub/Sub Logic
*   **Why**: Applications often need to react to changes immediately.
*   **Plan**: Add a subscription mechanism where clients can register callbacks for specific keys or prefixes.

### 4. Persistence Layer
*   **Why**: To survive restarts.
*   **Plan**: Implement a pluggable storage backend (e.g., generic `Save()` and `Load()` interfaces). Could simply dump JSON to disk on interval or write to an append-only log.

### 5. Typed Wrappers
*   **Why**: To improve developer experience and safety.
*   **Plan**: data generics or helper functions for common types (string, int, json) that automatically handle marshalling.
