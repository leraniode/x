package registry_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/leraniode/x/centrix/registry"
)

func TestAlias_ResolvesToConcept(t *testing.T) {
	r := registry.New()
	id := r.ID("lang.python")

	if err := r.Alias("python", "lang.python"); err != nil {
		t.Fatalf("Alias: %v", err)
	}
	if err := r.Alias("py", "lang.python"); err != nil {
		t.Fatalf("Alias: %v", err)
	}

	for _, surface := range []string{"python", "py"} {
		got, ok := r.Resolve(surface)
		if !ok || got != id {
			t.Errorf("Resolve(%q) = %d, %v; want %d, true", surface, got, ok, id)
		}
	}
}

func TestResolve_ConceptNameResolvesDirectly(t *testing.T) {
	r := registry.New()
	id := r.ID("nil pointer")
	got, ok := r.Resolve("nil pointer")
	if !ok || got != id {
		t.Errorf("Resolve(concept name) = %d, %v; want %d, true", got, ok, id)
	}
}

func TestResolve_UnknownRegistersNothing(t *testing.T) {
	r := registry.New()
	r.ID("lang.go")
	before := r.Len()

	if _, ok := r.Resolve("xyzzy"); ok {
		t.Error("Resolve of an unknown surface must report false")
	}
	if r.Len() != before {
		t.Errorf("Resolve must never register: Len %d → %d", before, r.Len())
	}
}

func TestAlias_UnknownConceptRejected(t *testing.T) {
	r := registry.New()
	err := r.Alias("python", "lang.python")
	if !errors.Is(err, registry.ErrUnknownConcept) {
		t.Fatalf("want ErrUnknownConcept, got %v", err)
	}
	if r.Len() != 0 || r.AliasLen() != 0 {
		t.Error("a rejected alias must register nothing")
	}
}

func TestAlias_EmptySurfaceRejected(t *testing.T) {
	r := registry.New()
	r.ID("lang.go")
	if err := r.Alias("", "lang.go"); !errors.Is(err, registry.ErrEmptySurface) {
		t.Fatalf("want ErrEmptySurface, got %v", err)
	}
}

func TestAlias_IdempotentForSameConcept(t *testing.T) {
	r := registry.New()
	r.ID("lang.go")
	if err := r.Alias("go", "lang.go"); err != nil {
		t.Fatal(err)
	}
	if err := r.Alias("go", "lang.go"); err != nil {
		t.Fatalf("repeating an identical alias must be a no-op, got %v", err)
	}
	if r.AliasLen() != 1 {
		t.Errorf("AliasLen = %d, want 1", r.AliasLen())
	}
}

func TestAlias_ConflictWithAlias(t *testing.T) {
	r := registry.New()
	r.ID("lang.go")
	r.ID("game.go")
	if err := r.Alias("go", "lang.go"); err != nil {
		t.Fatal(err)
	}
	err := r.Alias("go", "game.go")
	if !errors.Is(err, registry.ErrAliasConflict) {
		t.Fatalf("want ErrAliasConflict, got %v", err)
	}
	// The first mapping must be untouched.
	got, _ := r.Resolve("go")
	if want := r.ID("lang.go"); got != want {
		t.Errorf("conflict must not change the existing alias: got %d want %d", got, want)
	}
}

func TestAlias_ConflictWithConceptName(t *testing.T) {
	r := registry.New()
	r.ID("error")
	r.ID("concept.fault")
	err := r.Alias("error", "concept.fault")
	if !errors.Is(err, registry.ErrAliasConflict) {
		t.Fatalf("want ErrAliasConflict, got %v", err)
	}
}

func TestAlias_SurfaceEqualToOwnConceptNameIsNoOp(t *testing.T) {
	r := registry.New()
	r.ID("error")
	if err := r.Alias("error", "error"); err != nil {
		t.Fatalf("aliasing a concept to its own name must be a no-op, got %v", err)
	}
	if r.AliasLen() != 0 {
		t.Errorf("AliasLen = %d, want 0 (nothing to record)", r.AliasLen())
	}
}

func TestAlias_DoesNotDisturbConceptSnapshot(t *testing.T) {
	r := registry.New()
	r.ID("a.one")
	r.ID("a.two")
	_ = r.Alias("one", "a.one")
	_ = r.Alias("uno", "a.one")

	snap := r.Snapshot()
	if len(snap) != 2 {
		t.Errorf("Snapshot must hold concepts only, got %d entries", len(snap))
	}
	if _, err := registry.NewFrom(snap); err != nil {
		t.Errorf("concept snapshot must still satisfy NewFrom: %v", err)
	}
}

func TestAliasSnapshot_RoundTrip(t *testing.T) {
	r := registry.New()
	r.ID("lang.python")
	r.ID("lang.go")
	_ = r.Alias("python", "lang.python")
	_ = r.Alias("py", "lang.python")
	_ = r.Alias("golang", "lang.go")

	restored, err := registry.NewWithAliases(r.Snapshot(), r.AliasSnapshot())
	if err != nil {
		t.Fatalf("NewWithAliases: %v", err)
	}
	for _, s := range []string{"python", "py", "golang", "lang.go"} {
		want, _ := r.Resolve(s)
		got, ok := restored.Resolve(s)
		if !ok || got != want {
			t.Errorf("Resolve(%q) after restore = %d, %v; want %d", s, got, ok, want)
		}
	}
}

func TestAliasSnapshot_IsACopy(t *testing.T) {
	r := registry.New()
	r.ID("lang.go")
	_ = r.Alias("go", "lang.go")
	snap := r.AliasSnapshot()
	snap["evil"] = 1
	if _, ok := r.Resolve("evil"); ok {
		t.Error("mutating a snapshot must not affect the Registry")
	}
}

func TestNewWithAliases_RejectsBadInput(t *testing.T) {
	entries := map[string]registry.FeatureIndex{"a": 1, "b": 2}

	cases := map[string]map[string]registry.FeatureIndex{
		"unknown id":     {"x": 9},
		"zero id":        {"x": 0},
		"empty surface":  {"": 1},
		"name for other": {"b": 1}, // "b" is a concept name for ID 2
	}
	for name, aliases := range cases {
		if _, err := registry.NewWithAliases(entries, aliases); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestAliasLastSegments_Basic(t *testing.T) {
	r := registry.New()
	py := r.ID("lang.python")
	str := r.ID("concept.String") // capitalised segment is lower-cased
	r.ID("nil pointer")           // no dot: skipped

	if err := r.AliasLastSegments("lang.python", "concept.String", "nil pointer"); err != nil {
		t.Fatalf("AliasLastSegments: %v", err)
	}
	if got, ok := r.Resolve("python"); !ok || got != py {
		t.Errorf("Resolve(python) = %d, %v", got, ok)
	}
	if got, ok := r.Resolve("string"); !ok || got != str {
		t.Errorf("Resolve(string) = %d, %v", got, ok)
	}
	if r.AliasLen() != 2 {
		t.Errorf("AliasLen = %d, want 2 (dotless concept skipped)", r.AliasLen())
	}
}

func TestAliasLastSegments_CollisionInBatchIsAllOrNothing(t *testing.T) {
	r := registry.New()
	r.ID("lang.go")
	r.ID("game.go")
	r.ID("lang.python")

	err := r.AliasLastSegments("lang.python", "lang.go", "game.go")
	if !errors.Is(err, registry.ErrAliasConflict) {
		t.Fatalf("want ErrAliasConflict, got %v", err)
	}
	if !strings.Contains(err.Error(), `"go"`) {
		t.Errorf("error should name the colliding surface, got: %v", err)
	}
	if r.AliasLen() != 0 {
		t.Errorf("a failed batch must register nothing, AliasLen = %d", r.AliasLen())
	}
}

func TestAliasLastSegments_CollisionWithExistingAliasAndName(t *testing.T) {
	r := registry.New()
	r.ID("lang.go")
	r.ID("game.go")
	_ = r.Alias("go", "lang.go")
	if err := r.AliasLastSegments("game.go"); !errors.Is(err, registry.ErrAliasConflict) {
		t.Errorf("collision with existing alias: want ErrAliasConflict, got %v", err)
	}

	r2 := registry.New()
	r2.ID("error")
	r2.ID("concept.error")
	r2.ID("concept.fault")
	if err := r2.AliasLastSegments("concept.error"); !errors.Is(err, registry.ErrAliasConflict) {
		t.Errorf("collision with concept name: want ErrAliasConflict, got %v", err)
	}
}

func TestAliasLastSegments_UnknownConcept(t *testing.T) {
	r := registry.New()
	if err := r.AliasLastSegments("lang.nope"); !errors.Is(err, registry.ErrUnknownConcept) {
		t.Fatalf("want ErrUnknownConcept, got %v", err)
	}
}

func TestMerge_CarriesAliasesAcrossRenumbering(t *testing.T) {
	a := registry.New()
	a.ID("shared.one") // 1
	a.ID("a.only")     // 2
	_ = a.Alias("one", "shared.one")

	b := registry.New()
	b.ID("shared.one") // 1 — agrees with a
	b.ID("b.first")    // 2 here, but a.only already holds 2, so it is renumbered
	_ = b.Alias("first", "b.first")
	_ = b.Alias("uno", "shared.one")

	m, err := a.Merge(b)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if got := m.ID("b.first"); got == b.ID("b.first") {
		t.Fatalf("test premise broken: b.first was not renumbered (still %d)", got)
	}

	for surface, concept := range map[string]string{"one": "shared.one", "uno": "shared.one", "first": "b.first"} {
		got, ok := m.Resolve(surface)
		want := m.ID(concept)
		if !ok || got != want {
			t.Errorf("merged Resolve(%q) = %d, %v; want %d (%s)", surface, got, ok, want, concept)
		}
	}
}

func TestMerge_AliasConflictIsAnError(t *testing.T) {
	a := registry.New()
	a.ID("lang.go")
	a.ID("game.go")
	_ = a.Alias("go", "lang.go")

	b := registry.New()
	b.ID("lang.go")
	b.ID("game.go")
	_ = b.Alias("go", "game.go")

	if _, err := a.Merge(b); !errors.Is(err, registry.ErrAliasConflict) {
		t.Fatalf("want ErrAliasConflict from Merge, got %v", err)
	}
}
