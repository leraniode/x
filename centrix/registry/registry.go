// Package registry maps concept names to stable FeatureIndex values.
//
// The Registry is the bridge between human-readable concept names and the
// integer feature space that Centrix algebra operates on. It enforces the
// rules that make Invariant 2 (Feature Semantic Consistency) tractable:
//
//   - A name always maps to the same FeatureIndex — across runs, across systems
//   - FeatureIndexes are never reassigned
//   - The mapping is append-only — existing entries are immutable
//
// The Registry has no dependency on core algebra. It is a pure mapping layer
// used during authoring (offline) and optionally at runtime for encoding.
// It is not required — callers can manage FeatureIndexes manually — but it
// makes stable feature space authoring tractable at scale.
//
// Usage:
//
//	r := registry.New()
//	idx := r.ID("physics.gravity") // → 1, always
//	idx  = r.ID("physics.gravity") // → 1, same call, same result
//	idx  = r.ID("biology.cell")    // → 2
//
// The Registry is not safe for concurrent writes. If concurrent registration
// is required, the caller is responsible for synchronisation. Concurrent reads
// after all writes are complete are safe.
package registry

import (
	"fmt"
	"sort"
	"sync"
)

// FeatureIndex is the stable integer identifier for a concept dimension.
// Defined here to avoid importing core — the registry has no algebra dependency.
type FeatureIndex = uint32

// Registry maps concept names to stable FeatureIndex values.
// The zero value is not valid — use New() or NewFrom() to create a Registry.
type Registry struct {
	mu       sync.RWMutex
	nameToID map[string]FeatureIndex
	idToName []string // index i holds the name for FeatureIndex i+1 (IDs are 1-based)
}

// New creates an empty Registry. IDs are assigned starting from 1.
// FeatureIndex 0 is reserved — it is never assigned to any concept.
func New() *Registry {
	return &Registry{
		nameToID: make(map[string]FeatureIndex),
	}
}

// NewFrom creates a Registry pre-loaded with the given name→ID mapping.
// Used to restore a Registry from a serialised snapshot.
//
// Returns an error if the mapping is inconsistent:
//   - Duplicate IDs
//   - ID 0 assigned (reserved)
//   - Gaps in the ID sequence (IDs must be contiguous from 1)
func NewFrom(entries map[string]FeatureIndex) (*Registry, error) {
	if len(entries) == 0 {
		return New(), nil
	}

	// Validate: no ID 0, no duplicates, contiguous from 1.
	seen := make(map[FeatureIndex]string, len(entries))
	maxID := FeatureIndex(0)
	for name, id := range entries {
		if id == 0 {
			return nil, fmt.Errorf("registry: ID 0 is reserved, cannot assign to %q", name)
		}
		if existing, dup := seen[id]; dup {
			return nil, fmt.Errorf("registry: duplicate ID %d assigned to %q and %q", id, existing, name)
		}
		seen[id] = name
		if id > maxID {
			maxID = id
		}
	}
	if int(maxID) != len(entries) {
		return nil, fmt.Errorf("registry: IDs must be contiguous from 1 to %d, got max=%d with %d entries",
			len(entries), maxID, len(entries))
	}

	r := &Registry{
		nameToID: make(map[string]FeatureIndex, len(entries)),
		idToName: make([]string, maxID),
	}
	for name, id := range entries {
		r.nameToID[name] = id
		r.idToName[id-1] = name
	}
	return r, nil
}

// ID returns the stable FeatureIndex for the given concept name.
// If the name has not been registered before, it is assigned the next
// available FeatureIndex and the mapping is stored permanently.
//
// The same name always returns the same FeatureIndex — this is the
// primary guarantee of the Registry.
//
// Name must not be empty. An empty name panics — it represents a caller
// error, not a runtime condition.
func (r *Registry) ID(name string) FeatureIndex {
	if name == "" {
		panic("registry: concept name must not be empty")
	}

	// Fast path: read lock for existing entries.
	r.mu.RLock()
	if id, ok := r.nameToID[name]; ok {
		r.mu.RUnlock()
		return id
	}
	r.mu.RUnlock()

	// Slow path: write lock to register new name.
	r.mu.Lock()
	defer r.mu.Unlock()

	// Re-check after acquiring write lock — another goroutine may have registered
	// the same name between the read unlock and write lock.
	if id, ok := r.nameToID[name]; ok {
		return id
	}

	id := FeatureIndex(len(r.idToName) + 1) // next sequential ID, 1-based
	r.nameToID[name] = id
	r.idToName = append(r.idToName, name)
	return id
}

// Name returns the concept name for the given FeatureIndex, and true.
// Returns "", false if the ID has not been assigned.
func (r *Registry) Name(id FeatureIndex) (string, bool) {
	if id == 0 {
		return "", false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if int(id) > len(r.idToName) {
		return "", false
	}
	return r.idToName[id-1], true
}

// Has reports whether name has already been registered.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.nameToID[name]
	return ok
}

// Len returns the number of registered concepts.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.nameToID)
}

// Snapshot returns a stable copy of the current name→ID mapping.
// The returned map is a snapshot — subsequent registrations do not
// affect it. Use with NewFrom to serialise and restore a Registry.
func (r *Registry) Snapshot() map[string]FeatureIndex {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]FeatureIndex, len(r.nameToID))
	for k, v := range r.nameToID {
		out[k] = v
	}
	return out
}

// Names returns all registered concept names in registration order
// (i.e. sorted by FeatureIndex ascending). The slice is a copy.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, len(r.idToName))
	copy(out, r.idToName)
	return out
}

// IDs returns all assigned FeatureIndexes in ascending order.
func (r *Registry) IDs() []FeatureIndex {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]FeatureIndex, len(r.idToName))
	for i := range out {
		out[i] = FeatureIndex(i + 1)
	}
	return out
}

// Merge returns a new Registry containing all entries from r plus all
// entries from other. Entries present in both registries must have the
// same ID — if they conflict, an error is returned.
//
// The merged Registry assigns new IDs to names present in other but not
// in r, starting after r's current maximum ID. This is not commutative
// when new names exist — r's ID space takes precedence.
func (r *Registry) Merge(other *Registry) (*Registry, error) {
	snap := r.Snapshot()
	otherSnap := other.Snapshot()

	// Check for conflicts: same name, different ID.
	for name, otherID := range otherSnap {
		if myID, exists := snap[name]; exists && myID != otherID {
			return nil, fmt.Errorf("registry: merge conflict: %q has ID %d in receiver and %d in other",
				name, myID, otherID)
		}
	}

	// Build merged map: start with r's entries, then add new names from other.
	merged := make(map[string]FeatureIndex, len(snap)+len(otherSnap))
	for k, v := range snap {
		merged[k] = v
	}

	// Collect new names from other (not already in r), sort for determinism.
	var newNames []string
	for name := range otherSnap {
		if _, exists := snap[name]; !exists {
			newNames = append(newNames, name)
		}
	}
	sort.Strings(newNames) // deterministic ID assignment for new entries

	nextID := FeatureIndex(len(snap) + 1)
	for _, name := range newNames {
		merged[name] = nextID
		nextID++
	}

	return NewFrom(merged)
}
