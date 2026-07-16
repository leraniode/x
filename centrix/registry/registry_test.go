package registry_test

import (
	"sync"
	"testing"

	"github.com/leraniode/x/centrix/registry"
)

// ─── Core guarantee: a name always maps to the same ID ────────────────────────

func TestID_Stable(t *testing.T) {
	r := registry.New()
	id1 := r.ID("physics.gravity")
	id2 := r.ID("physics.gravity")
	if id1 != id2 {
		t.Errorf("ID not stable: first=%d second=%d", id1, id2)
	}
}

func TestID_DifferentNames_DifferentIDs(t *testing.T) {
	r := registry.New()
	a := r.ID("physics.gravity")
	b := r.ID("biology.cell")
	if a == b {
		t.Errorf("different names must have different IDs: both got %d", a)
	}
}

func TestID_Sequential(t *testing.T) {
	r := registry.New()
	ids := make([]registry.FeatureIndex, 5)
	names := []string{"a.one", "a.two", "a.three", "a.four", "a.five"}
	for i, name := range names {
		ids[i] = r.ID(name)
	}
	for i, id := range ids {
		want := registry.FeatureIndex(i + 1)
		if id != want {
			t.Errorf("ID(%q) = %d, want %d", names[i], id, want)
		}
	}
}

func TestID_NeverZero(t *testing.T) {
	r := registry.New()
	id := r.ID("any.concept")
	if id == 0 {
		t.Error("ID must never be 0 — 0 is reserved")
	}
}

func TestID_EmptyNamePanics(t *testing.T) {
	r := registry.New()
	defer func() {
		if rec := recover(); rec == nil {
			t.Error("ID with empty name should panic")
		}
	}()
	r.ID("")
}

// ─── Append-only: existing entries are immutable ──────────────────────────────

func TestID_AppendOnly(t *testing.T) {
	r := registry.New()
	first := r.ID("concept.x")

	// Register many other concepts.
	for i := 0; i < 100; i++ {
		r.ID("other.concept." + string(rune('a'+i%26)) + string(rune('a'+i/26)))
	}

	// Original entry must be unchanged.
	again := r.ID("concept.x")
	if first != again {
		t.Errorf("append-only violated: concept.x was %d, now %d", first, again)
	}
}

// ─── Name reverse lookup ──────────────────────────────────────────────────────

func TestName_Lookup(t *testing.T) {
	r := registry.New()
	r.ID("physics.gravity")
	r.ID("biology.cell")

	name, ok := r.Name(1)
	if !ok || name != "physics.gravity" {
		t.Errorf("Name(1) = %q, %v; want %q, true", name, ok, "physics.gravity")
	}
}

func TestName_UnknownID(t *testing.T) {
	r := registry.New()
	_, ok := r.Name(999)
	if ok {
		t.Error("Name(999) should return false for unregistered ID")
	}
}

func TestName_ZeroID(t *testing.T) {
	r := registry.New()
	_, ok := r.Name(0)
	if ok {
		t.Error("Name(0) should return false — 0 is reserved")
	}
}

func TestName_RoundTrip(t *testing.T) {
	r := registry.New()
	concepts := []string{"math.pi", "physics.c", "chemistry.avogadro"}
	for _, c := range concepts {
		id := r.ID(c)
		name, ok := r.Name(id)
		if !ok || name != c {
			t.Errorf("round-trip failed: ID(%q)=%d, Name(%d)=%q,%v", c, id, id, name, ok)
		}
	}
}

// ─── Has / Len ────────────────────────────────────────────────────────────────

func TestHas(t *testing.T) {
	r := registry.New()
	if r.Has("x.y") {
		t.Error("Has should return false before registration")
	}
	r.ID("x.y")
	if !r.Has("x.y") {
		t.Error("Has should return true after registration")
	}
}

func TestLen(t *testing.T) {
	r := registry.New()
	if r.Len() != 0 {
		t.Errorf("Len() on new registry = %d, want 0", r.Len())
	}
	r.ID("a")
	r.ID("b")
	r.ID("a") // duplicate — must not increment
	if r.Len() != 2 {
		t.Errorf("Len() = %d, want 2", r.Len())
	}
}

// ─── Snapshot / Names / IDs ───────────────────────────────────────────────────

func TestSnapshot_Independence(t *testing.T) {
	r := registry.New()
	r.ID("concept.a")
	snap := r.Snapshot()

	// Register more after snapshot — snapshot must not change.
	r.ID("concept.b")

	if _, ok := snap["concept.b"]; ok {
		t.Error("Snapshot is not independent — subsequent registration affected it")
	}
}

func TestSnapshot_Complete(t *testing.T) {
	r := registry.New()
	names := []string{"x.one", "x.two", "x.three"}
	for _, n := range names {
		r.ID(n)
	}
	snap := r.Snapshot()
	for _, n := range names {
		if _, ok := snap[n]; !ok {
			t.Errorf("Snapshot missing %q", n)
		}
	}
}

func TestNames_RegistrationOrder(t *testing.T) {
	r := registry.New()
	r.ID("first")
	r.ID("second")
	r.ID("third")

	names := r.Names()
	if len(names) != 3 {
		t.Fatalf("Names() len = %d, want 3", len(names))
	}
	if names[0] != "first" || names[1] != "second" || names[2] != "third" {
		t.Errorf("Names() not in registration order: %v", names)
	}
}

func TestIDs_Ascending(t *testing.T) {
	r := registry.New()
	r.ID("a")
	r.ID("b")
	r.ID("c")

	ids := r.IDs()
	for i, id := range ids {
		want := registry.FeatureIndex(i + 1)
		if id != want {
			t.Errorf("IDs()[%d] = %d, want %d", i, id, want)
		}
	}
}

// ─── NewFrom ──────────────────────────────────────────────────────────────────

func TestNewFrom_RestoresMapping(t *testing.T) {
	original := registry.New()
	original.ID("physics.gravity") // → 1
	original.ID("biology.cell")    // → 2

	snap := original.Snapshot()
	restored, err := registry.NewFrom(snap)
	if err != nil {
		t.Fatalf("NewFrom error: %v", err)
	}

	if restored.ID("physics.gravity") != 1 {
		t.Errorf("NewFrom: physics.gravity should be 1, got %d", restored.ID("physics.gravity"))
	}
	if restored.ID("biology.cell") != 2 {
		t.Errorf("NewFrom: biology.cell should be 2, got %d", restored.ID("biology.cell"))
	}
}

func TestNewFrom_DuplicateIDReturnsError(t *testing.T) {
	entries := map[string]registry.FeatureIndex{
		"concept.a": 1,
		"concept.b": 1, // duplicate ID
	}
	_, err := registry.NewFrom(entries)
	if err == nil {
		t.Error("NewFrom: duplicate ID should return an error")
	}
}

func TestNewFrom_ZeroIDReturnsError(t *testing.T) {
	entries := map[string]registry.FeatureIndex{
		"concept.a": 0, // reserved
	}
	_, err := registry.NewFrom(entries)
	if err == nil {
		t.Error("NewFrom: ID 0 should return an error")
	}
}

func TestNewFrom_GapReturnsError(t *testing.T) {
	entries := map[string]registry.FeatureIndex{
		"concept.a": 1,
		"concept.b": 3, // gap at 2
	}
	_, err := registry.NewFrom(entries)
	if err == nil {
		t.Error("NewFrom: non-contiguous IDs should return an error")
	}
}

func TestNewFrom_Empty(t *testing.T) {
	r, err := registry.NewFrom(nil)
	if err != nil {
		t.Fatalf("NewFrom(nil) error: %v", err)
	}
	if r.Len() != 0 {
		t.Errorf("NewFrom(nil): expected 0 entries, got %d", r.Len())
	}
}

// ─── Merge ────────────────────────────────────────────────────────────────────

func TestMerge_CombinesEntries(t *testing.T) {
	r1 := registry.New()
	r1.ID("physics.gravity")

	r2 := registry.New()
	r2.ID("biology.cell")

	merged, err := r1.Merge(r2)
	if err != nil {
		t.Fatalf("Merge error: %v", err)
	}
	if !merged.Has("physics.gravity") || !merged.Has("biology.cell") {
		t.Error("Merge: missing entries from one of the registries")
	}
}

func TestMerge_SharedEntriesCompatible(t *testing.T) {
	// Same name, same ID in both — must succeed.
	r1, _ := registry.NewFrom(map[string]registry.FeatureIndex{"shared.concept": 1})
	r2, _ := registry.NewFrom(map[string]registry.FeatureIndex{"shared.concept": 1})

	_, err := r1.Merge(r2)
	if err != nil {
		t.Errorf("Merge: compatible shared entries should not error: %v", err)
	}
}

func TestMerge_ConflictingIDsReturnsError(t *testing.T) {
	// r1: concept.x → 1
	r1, _ := registry.NewFrom(map[string]registry.FeatureIndex{
		"concept.x": 1,
	})
	// r2: padding.entry → 1, concept.x → 2
	// Same name, different ID — direct conflict.
	r2, _ := registry.NewFrom(map[string]registry.FeatureIndex{
		"padding.entry": 1,
		"concept.x":     2,
	})

	_, err := r1.Merge(r2)
	if err == nil {
		t.Error("Merge: conflicting IDs for same name should return error")
	}
}

func TestMerge_ReceiverIDsPreserved(t *testing.T) {
	r1 := registry.New()
	id := r1.ID("physics.gravity") // → 1

	r2 := registry.New()
	r2.ID("biology.cell")

	merged, err := r1.Merge(r2)
	if err != nil {
		t.Fatalf("Merge error: %v", err)
	}
	if merged.ID("physics.gravity") != id {
		t.Errorf("Merge: receiver ID not preserved: got %d, want %d",
			merged.ID("physics.gravity"), id)
	}
}

// ─── Concurrent safety ────────────────────────────────────────────────────────

func TestID_ConcurrentWrites_Stable(t *testing.T) {
	// Many goroutines registering the same name concurrently — all must get
	// the same ID. No panic, no data race (run with -race).
	r := registry.New()
	const goroutines = 50
	results := make([]registry.FeatureIndex, goroutines)
	var wg sync.WaitGroup

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			results[idx] = r.ID("concurrent.concept")
		}(i)
	}
	wg.Wait()

	first := results[0]
	for i, id := range results {
		if id != first {
			t.Errorf("goroutine %d got %d, want %d — concurrent ID not stable", i, id, first)
		}
	}
}

func TestID_ConcurrentDifferentNames_UniqueIDs(t *testing.T) {
	// Many goroutines registering different names — all IDs must be unique.
	r := registry.New()
	const goroutines = 50
	results := make([]registry.FeatureIndex, goroutines)
	var wg sync.WaitGroup

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			name := "concept." + string(rune('a'+idx%26)) + string(rune('0'+idx/26))
			results[idx] = r.ID(name)
		}(i)
	}
	wg.Wait()

	seen := make(map[registry.FeatureIndex]bool, goroutines)
	for _, id := range results {
		if seen[id] {
			t.Errorf("duplicate ID %d assigned to different names", id)
		}
		seen[id] = true
	}
}
