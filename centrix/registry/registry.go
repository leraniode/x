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
	"errors"
	"fmt"
	"sort"
	"strings"
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

	// aliases maps a surface form (a word or phrase as it appears in input
	// text) to the FeatureIndex of the concept it denotes. Several surfaces
	// may denote one concept, so this cannot live in nameToID, which is
	// strictly one name per ID. Like nameToID it is append-only.
	aliases map[string]FeatureIndex
}

// New creates an empty Registry. IDs are assigned starting from 1.
// FeatureIndex 0 is reserved — it is never assigned to any concept.
func New() *Registry {
	return &Registry{
		nameToID: make(map[string]FeatureIndex),
		aliases:  make(map[string]FeatureIndex),
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
		aliases:  make(map[string]FeatureIndex),
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

	out, err := NewFrom(merged)
	if err != nil {
		return nil, err
	}

	// Carry aliases across by concept name, not by ID: IDs for names that only
	// exist in other may have been renumbered above. Receiver aliases go first,
	// both sets in sorted order, so the result is deterministic.
	for _, src := range []*Registry{r, other} {
		for _, surface := range src.aliasSurfaces() {
			name, _ := src.Name(src.aliasID(surface))
			if err := out.Alias(surface, name); err != nil {
				return nil, fmt.Errorf("registry: merge: %w", err)
			}
		}
	}
	return out, nil
}

// ─── Aliases ──────────────────────────────────────────────────────────────────

// Errors returned by the alias API. Use errors.Is to test for them.
var (
	// ErrEmptySurface is returned when a surface form is empty.
	ErrEmptySurface = errors.New("registry: surface form must not be empty")

	// ErrUnknownConcept is returned when an alias names a concept that has not
	// been registered. Aliases never register concepts: ID auto-registers, and
	// an alias typo must not silently grow the feature space.
	ErrUnknownConcept = errors.New("registry: alias targets unregistered concept")

	// ErrAliasConflict is returned when a surface form already denotes a
	// different concept, either as an earlier alias or as a concept name.
	ErrAliasConflict = errors.New("registry: alias conflict")
)

// Alias records that the surface form denotes the named concept.
//
// A concept name is a stable identifier ("lang.python"); a surface form is
// what text actually contains ("python", "py"). The Registry keeps the two
// apart so concept names can stay namespaced while an encoder still finds
// them. Surface forms are matched exactly as stored — normalisation (case,
// whitespace) is the caller's policy, and the same policy must be applied when
// resolving.
//
// The concept must already be registered (ErrUnknownConcept otherwise).
// Aliasing a surface to the concept it already denotes is a no-op. Aliasing a
// surface that already denotes a different concept — as an alias or as a
// concept name — returns ErrAliasConflict and changes nothing. One surface
// maps to exactly one concept in v0.1; word-sense ambiguity is left to the
// layer above.
func (r *Registry) Alias(surface, concept string) error {
	if surface == "" {
		return ErrEmptySurface
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.aliasLocked(surface, concept)
}

func (r *Registry) aliasLocked(surface, concept string) error {
	id, ok := r.nameToID[concept]
	if !ok {
		return fmt.Errorf("%w: %q (alias %q)", ErrUnknownConcept, concept, surface)
	}
	if existing, ok := r.nameToID[surface]; ok {
		if existing == id {
			return nil // the surface is already the concept's own name
		}
		return fmt.Errorf("%w: %q is a concept name (ID %d), cannot alias it to %q",
			ErrAliasConflict, surface, existing, concept)
	}
	if existing, ok := r.aliases[surface]; ok {
		if existing == id {
			return nil
		}
		other := r.idToName[existing-1]
		return fmt.Errorf("%w: %q already denotes %q, cannot alias it to %q",
			ErrAliasConflict, surface, other, concept)
	}
	r.aliases[surface] = id
	return nil
}

// Resolve returns the FeatureIndex a surface form denotes: the concept
// registered under exactly that name, or else the concept it was aliased to.
// It never registers anything, so — unlike ID — it is safe to call with
// arbitrary input text.
func (r *Registry) Resolve(surface string) (FeatureIndex, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if id, ok := r.nameToID[surface]; ok {
		return id, true
	}
	id, ok := r.aliases[surface]
	return id, ok
}

// AliasLen returns the number of registered aliases.
func (r *Registry) AliasLen() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.aliases)
}

// AliasSnapshot returns a copy of the current surface→ID alias mapping,
// separate from Snapshot so the concept mapping keeps its one-name-per-ID
// shape. Use with NewWithAliases to serialise and restore a Registry.
func (r *Registry) AliasSnapshot() map[string]FeatureIndex {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]FeatureIndex, len(r.aliases))
	for k, v := range r.aliases {
		out[k] = v
	}
	return out
}

// NewWithAliases restores a Registry from a concept snapshot plus an alias
// snapshot. It applies NewFrom's validation to entries, and additionally
// rejects an alias with an empty surface, an ID that is not in entries, or a
// surface that is also a concept name for a different ID.
func NewWithAliases(entries map[string]FeatureIndex, aliases map[string]FeatureIndex) (*Registry, error) {
	r, err := NewFrom(entries)
	if err != nil {
		return nil, err
	}
	surfaces := make([]string, 0, len(aliases))
	for s := range aliases {
		surfaces = append(surfaces, s)
	}
	sort.Strings(surfaces) // deterministic error reporting
	for _, surface := range surfaces {
		id := aliases[surface]
		if id == 0 || int(id) > len(r.idToName) {
			return nil, fmt.Errorf("registry: alias %q has unknown ID %d", surface, id)
		}
		if err := r.Alias(surface, r.idToName[id-1]); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// AliasLastSegments is an authoring helper for registries that name concepts
// as dotted paths. For each named concept it registers the lower-cased final
// segment as an alias: "lang.python" → "python". Concepts without a dot are
// skipped; they are already their own surface form.
//
// It is all-or-nothing. If any derived surface collides — with another
// concept in the batch, an existing alias, or a concept name — it returns one
// error naming every collision (wrapping ErrAliasConflict) and registers
// nothing. An unregistered concept returns ErrUnknownConcept.
func (r *Registry) AliasLastSegments(concepts ...string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	type pair struct{ surface, concept string }
	var pairs []pair
	for _, c := range concepts {
		if _, ok := r.nameToID[c]; !ok {
			return fmt.Errorf("%w: %q", ErrUnknownConcept, c)
		}
		i := strings.LastIndex(c, ".")
		if i < 0 || i == len(c)-1 {
			continue
		}
		pairs = append(pairs, pair{strings.ToLower(c[i+1:]), c})
	}

	// Detect every collision before changing anything.
	claimed := make(map[string]string, len(pairs))
	var problems []string
	for _, p := range pairs {
		if prev, ok := claimed[p.surface]; ok && prev != p.concept {
			problems = append(problems, fmt.Sprintf("%q derives from both %q and %q", p.surface, prev, p.concept))
			continue
		}
		claimed[p.surface] = p.concept
		id := r.nameToID[p.concept]
		if existing, ok := r.nameToID[p.surface]; ok && existing != id {
			problems = append(problems, fmt.Sprintf("%q is already the concept name %q", p.surface, r.idToName[existing-1]))
		} else if existing, ok := r.aliases[p.surface]; ok && existing != id {
			problems = append(problems, fmt.Sprintf("%q already denotes %q", p.surface, r.idToName[existing-1]))
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("%w: %s", ErrAliasConflict, strings.Join(problems, "; "))
	}
	for _, p := range pairs {
		if err := r.aliasLocked(p.surface, p.concept); err != nil {
			return err // unreachable after the checks above; kept for safety
		}
	}
	return nil
}

// aliasSurfaces returns every alias surface form in sorted order.
func (r *Registry) aliasSurfaces() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.aliases))
	for s := range r.aliases {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// aliasID returns the ID an alias surface denotes (0 if none).
func (r *Registry) aliasID(surface string) FeatureIndex {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.aliases[surface]
}
