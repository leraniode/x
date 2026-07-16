# Changelog

All notable changes to x packages are documented here.

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
Versioning: [Semantic Versioning](https://semver.org/spec/v2.0.0.html)

Packages in leraniode/x are experimental. APIs may break between commits until a package
graduates to its own repository.

---

## centrix

### [0.1.0] — 2026-07-14

First complete implementation of the Centrix sparse signal mathematics library.
All five build phases complete. 167 tests passing. Race detector clean.

#### Added

**`core` — Types (Phase 1)**

- `FeatureIndex` (`uint32` alias) — concept dimension identifier
- `SparseVector` (`map[FeatureIndex]float64`) — sparse point in concept space
- `Action` — typed constants: `Generated`, `Matched`, `Propagated`, `Attenuated`, `Composed`, `Filtered`
- `ComposeMode` — `Independent` (OR-combination) and `Correlated` (Max)
- `Step` — one transformation record with before/after confidence
- `Trace` — capped at 64 steps, sliding window, newest never dropped, no mutex
- `Signal` — `{Vector, Confidence, Trace}`, value type, ephemeral
- `Prototype` — `{Vector, Weight}`, persistent knowledge type

**`core` — Tier 1 Algebra (Phase 2)**

- `Energy(v)` — L1 norm, always derived, never stored (Invariant 10)
- `Dot(a, b)` — magnitude-sensitive similarity, O(min(|a|,|b|))
- `Cosine(a, b)` — direction-sensitive similarity, safe on zero vectors
- `Jaccard(a, b)` — binary set overlap, presence-only in v0.1
- `Merge(a, b)` — union with weight summation, zero-cancellation cleanup
- `Normalize(v)` — unit L2 norm, safe on zero vector
- `Filter(v, θ)` — absolute weight threshold, preserves sign

**`core` — Tier 2 Signal Operations (Phase 3)**

- `Generate(s, proto, θ, node)` — deterministic feature generation via Cosine match
- `Compose(a, b, mode, node)` — signal merge with configurable confidence combination
- `Attenuate(s, λ, node)` — exponential weight decay
- `FilterSignal(s, θ, node)` — feature pruning with Step appended
- `Propagate(s, proto, α, node)` — signal-level energy absorption via Dot similarity
- Confidence gate: four-condition check, Rescorla-Wagner update formula

**`field` — Field Dynamics (Phase 4)**

- `SignalField` — collection of interacting Signals, value type
- `New(signals)` — constructs field with validated defaults, warns when N > 15
- `Propagate(f)` — one tick of energy spreading, snapshot semantics (order-independent)
- `Decay(f)` — exponential weight reduction across all signals
- `Stabilize(f)` — alternating Propagate+Decay until convergence or MaxTicks
- `StabilizeResult` — reports ticks, convergence status, final delta
- `Attention(f, query, k)` — top-K signals by `Dot × Energy`, excludes score ≤ 0

**`registry` — Feature Registry (Phase 5)**

- `Registry` — append-only, deterministic concept name → FeatureIndex mapping
- `New()` — empty registry, IDs from 1
- `NewFrom(entries)` — restore from snapshot, validates contiguity and uniqueness
- `ID(name)` — assign or retrieve, panics on empty name, thread-safe
- `Name(id)` — reverse lookup
- `Has(name)` — existence check
- `Snapshot()` — serialisable copy, independent of subsequent registrations
- `Names()` — registration-order slice
- `IDs()` — ascending slice
- `Merge(other)` — combine registries, error on conflict

**Validated constants (all packages)**

- `α_propagation = 0.1`, `λ_decay = 0.3`, `ε_convergence = 1e-4`
- `minMatchScore = 0.15`, `minProtoWeight = 0.30`, `α_confidence = 0.10`
- `DefaultTraceCap = 64`, `StableN = 15`, `DefaultMaxTicks = 50`

**Resource verification**

- O(k) property confirmed: Dot with k=20 across D=100 to D=1,000,000 shows <3% variance
- Signal memory: ~4KB fully loaded (k=20, 64-step trace)
- Tier 1 algebra: 0 allocs/op
- Race detector: clean across all packages and concurrent registry tests

---

## kernel

### [Unreleased]

Design and project planning documents. No implementation yet.
