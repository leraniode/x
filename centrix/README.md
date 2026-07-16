# centrix

[![Go](https://img.shields.io/badge/go-1.22-00ADD8?logo=go)](https://go.dev)
[![Tests](https://img.shields.io/badge/tests-167%20passing-brightgreen)]()
[![License](https://img.shields.io/badge/license-MIT-green)](../LICENSE)
[![Status](https://img.shields.io/badge/status-experimental-orange)](https://github.com/leraniode/x)

Sparse signal mathematics library for Go.

Centrix defines the types, algebra, and field dynamics for deterministic reasoning
and generation systems. It has no knowledge of what builds on it — it is a pure
mathematical layer with zero external dependencies.

---

## Why

Large language models are memory-bound at inference. The KV cache alone can waste
60–80% of GPU memory per request. Specialised inference engines reduce this, but
the fundamental dependency on dense probabilistic representations remains.

Centrix takes a different approach: deterministic reasoning over sparse distributed
signals. Operations scale with active features (`k`), not dimension space size (`D`).
A fully-loaded Signal costs ~4KB. CPU and RAM are the primary resources.

```
Dot(k=20, D=100):       ~370ns
Dot(k=20, D=1,000,000): ~380ns   ← D grew 10,000×, time moved 2.7%
```

The resource constraint is the design target, not an obstacle.

---

## Install

```bash
go get github.com/leraniode/x/centrix@latest
```

This is an experimental package. APIs may change.

---

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/leraniode/x/centrix/core"
    "github.com/leraniode/x/centrix/field"
    "github.com/leraniode/x/centrix/registry"
)

func main() {
    // Build a feature space
    r := registry.New()
    gravity  := r.ID("physics.gravity")  // → 1
    mass     := r.ID("physics.mass")     // → 2
    force    := r.ID("physics.force")    // → 3

    // Author a prototype — persistent knowledge
    proto := core.NewPrototype(core.SparseVector{
        gravity: 0.9,
        mass:    0.8,
        force:   0.7,
    }, 0.85)

    // Create a query signal — ephemeral, one run
    query := core.NewSignalFromVector(core.SparseVector{
        gravity: 0.6,
        mass:    0.4,
    }, 0.0)

    // Generate: produce features implied by the prototype
    result := core.Generate(query, proto, 0.1, "physics-node")

    fmt.Printf("Confidence: %.3f\n", result.Confidence)
    fmt.Printf("Features:   %d active\n", result.Vector.Len())
    fmt.Printf("Trace:      %d steps\n", result.Trace.Len())

    // Field dynamics: multiple signals interact
    signals := []core.Signal{query, result}
    f := field.New(signals)
    stabilised := field.Stabilize(f)
    fmt.Printf("Converged in %d ticks: %v\n", stabilised.Ticks, stabilised.Converged)

    // Attention: most relevant signals for a query
    top := field.Attention(stabilised.Field, query, 1)
    fmt.Printf("Top signal energy: %.3f\n", core.Energy(top[0].Vector))
}
```

---

## Packages

### `core`

The foundation. All types and operations. Zero imports outside standard library.

**Types:**

| Type | Description |
|------|-------------|
| `FeatureIndex` | `uint32` — a concept dimension identifier |
| `SparseVector` | `map[FeatureIndex]float64` — sparse point in concept space |
| `Signal` | `{Vector, Confidence, Trace}` — ephemeral reasoning state |
| `Prototype` | `{Vector, Weight}` — persistent authored knowledge |
| `Trace` | Ordered execution history, capped at 64 steps |
| `Step` | One transformation record in a Trace |
| `Action` | `Generated \| Matched \| Propagated \| Attenuated \| Composed \| Filtered` |
| `ComposeMode` | `Independent \| Correlated` — how Compose combines confidence |

**Tier 1 — Algebra** (pure math, no Trace written):

```go
Energy(v SparseVector) float64
Dot(a, b SparseVector) float64
Cosine(a, b SparseVector) float64
Jaccard(a, b SparseVector) float64
Merge(a, b SparseVector) SparseVector
Normalize(v SparseVector) SparseVector
Filter(v SparseVector, θ float64) SparseVector
```

**Tier 2 — Signal Operations** (stateful, appends Step, updates Confidence):

```go
Generate(s Signal, proto Prototype, θ float64, node string) Signal
Compose(a, b Signal, mode ComposeMode, node string) Signal
Attenuate(s Signal, λ float64, node string) Signal
FilterSignal(s Signal, θ float64, node string) Signal
Propagate(s Signal, proto Prototype, α float64, node string) Signal
```

### `field`

Field dynamics for associative reasoning. Signals that share concept dimensions
amplify each other; signals that share nothing do not interact.

```go
field.New(signals []core.Signal) SignalField
field.Propagate(f SignalField) SignalField
field.Decay(f SignalField) SignalField
field.Stabilize(f SignalField) StabilizeResult
field.Attention(f SignalField, query core.Signal, k int) []core.Signal
```

Validated defaults: `α=0.1`, `λ=0.3`, `ε=1e-4`, stable for `N ≤ 15`.
A warning is emitted (not a panic) when N exceeds this bound.

### `registry`

Concept name → stable `FeatureIndex` mapping. Append-only. Deterministic.
No dependency on `core`.

```go
r := registry.New()
id := r.ID("physics.gravity")    // assigns or retrieves
r.ID("physics.gravity")          // always returns the same id

name, ok := r.Name(id)           // reverse lookup
snap := r.Snapshot()             // serialisable copy
restored, err := registry.NewFrom(snap) // restore from snapshot
merged, err := r1.Merge(r2)     // combine two registries
```

`ID` is safe for concurrent use. Double-checked locking — read lock fast path,
write lock only for new registrations.

---

## Validated Constants

These values are implemented and research-grounded. Changes require updating
[`docs/RESEARCH.md`](./docs/RESEARCH.md) first.

| Constant | Value | Purpose |
|----------|-------|---------|
| `minMatchScore` | `0.15` | Confidence gate — 5× sparse noise floor |
| `minProtoWeight` | `0.30` | Confidence gate — minimum prototype trust |
| `α_confidence` | `0.10` | Confidence learning rate |
| `α_propagation` | `0.10` | Field propagation coefficient |
| `λ_decay` | `0.30` | Field decay rate |
| `ε_convergence` | `1e-4` | Stabilize convergence threshold |
| `DefaultTraceCap` | `64` | Maximum steps per Trace |
| `StableN` | `15` | Maximum N for validated field defaults |

---

## Resource Profile

Measured on the actual implementation:

| Metric | Value |
|--------|-------|
| Signal memory (k=20, 64-step trace) | ~4KB |
| `Dot` allocations | 0 B/op, 0 allocs |
| `Cosine` allocations | 0 B/op, 0 allocs |
| `Propagate` allocations | ~4.6KB/op (new Signal) |
| `Dot` scaling with D (10× D growth) | <3% time increase |
| Tests | 167 passing |
| Race detector | Clean |

---

## Docs

- [`docs/CONCEPTS.md`](./docs/CONCEPTS.md) — full type reference, algebra spec, invariants
- [`docs/RESEARCH.md`](./docs/RESEARCH.md) — theoretical foundations and design decisions

