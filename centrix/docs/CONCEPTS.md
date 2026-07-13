# Centrix — Technical Reference
> v0.1 · `github.com/leraniode/xgo/centrix` (experimental)
> Graduating to `github.com/leraniode/centrix` when stable

---

## Contents

1. [What Centrix Is](#1-what-centrix-is)
2. [The Mental Model](#2-the-mental-model)
3. [Types](#3-types)
4. [Signal Lifecycle](#4-signal-lifecycle)
5. [Algebra — Tier 1](#5-algebra--tier-1)
6. [Signal Operations — Tier 2](#6-signal-operations--tier-2)
7. [Confidence](#7-confidence)
8. [Field Dynamics](#8-field-dynamics)
9. [Registry](#9-registry)
10. [Authoring Model](#10-authoring-model)
11. [Invariants](#11-invariants)

---

## 1. What Centrix Is

Centrix is a sparse signal mathematics library. It defines the types, algebra,
and field dynamics for systems that reason and generate using sparse distributed
representations. It is a mathematical layer — it has no knowledge of what builds
on it.

**What Centrix owns:**
- Canonical `Signal` and `Prototype` types
- Complete algebra over `SparseVector`
- Trace and observability types
- Action vocabulary for the learning system
- Field dynamics: propagation, decay, stabilisation, attention

**What Centrix does not own:**
- Nodes, flows, engines, knowledge stores
- File formats, encoding pipelines, data ingestion
- Feature encoding or feature space assignment
- Pipeline execution or routing logic
- Any import from the caller's ecosystem

```
Module:  github.com/leraniode/xgo/centrix   (experimental)
Future:  github.com/leraniode/centrix        (standalone, stable)
Deps:    none
```

Centrix imports nothing outside the Go standard library.

---

## 2. The Mental Model

Centrix is the **physics** of a reasoning system.

| Concept | Physics analogy |
|---------|-----------------|
| `SparseVector` | Matter — the substance a signal is made of |
| `Confidence` | Assertion strength — how strongly the signal asserts itself |
| `Trace` | History — the ordered record of what happened |
| `Prototype` | Memory — authored knowledge that persists between runs |
| `Algebra` | Transformations — how vectors change |
| `Field` | Mechanics — how signals interact when they share space |

A signal does not know what built it or what will consume it. It is a
mathematical object moving through a system that obeys Centrix's laws.

---

## 3. Types

All types are in package `core`. All are value types — no pointers in the public API.

### 3.1 FeatureIndex

```go
type FeatureIndex = uint32
```

A key in a `SparseVector`. Represents one dimension of concept space. Centrix
does not assign or interpret `FeatureIndex` values — that is the caller's
responsibility. The invariant: **a `FeatureIndex` always represents the same
concept**, across all signals, prototypes, and runs. If this breaks, similarity
math becomes meaningless.

### 3.2 SparseVector

```go
type SparseVector map[FeatureIndex]float64
```

A sparse point in concept space. Only non-zero features are stored — absence is
meaningful. Operations run in `O(k)` where `k` is active features, not `O(D)`
where `D` is the dimension space size. Typical usage: 10–30 active features in
a space of 10,000+ dimensions.

Zero weights must never be stored. Feature presence must be intentional.

### 3.3 Action

```go
type Action int

const (
    Generated  Action = iota
    Matched
    Propagated
    Attenuated
    Composed
    Filtered
)
```

Records what Tier 2 operation was applied at a `Step`. The learning system uses
`Action` to decide how to update weights and trust after a run. These six
constants map directly to the six Tier 2 operations.

Callers that build execution engines may define their own effect vocabulary
(tool calls, output emission, etc.) as separate types — those are not `Action`.

### 3.4 ComposeMode

```go
type ComposeMode int

const (
    Independent ComposeMode = iota // OR-combination: 1 − (1−cA)(1−cB)
    Correlated                     // Max: max(cA, cB)
)
```

Tells `Compose` how to combine confidence from two signals. The caller must
supply this — Centrix cannot infer whether two convergent paths represent
independent evidence or correlated evidence. The caller built the flow; they know.
Passing the wrong mode silently corrupts the learning signal downstream.

### 3.5 Step

```go
type Step struct {
    Node             string
    Action           Action
    Value            any
    ConfidenceBefore float64
    ConfidenceAfter  float64
}
```

One entry in a Signal's execution history. `Node` is the string identity of what
produced this step — required for confidence gate condition 2. `Value` is an
optional payload defined by the caller. When `ConfidenceBefore == ConfidenceAfter`,
the confidence gate was not satisfied — the update was blocked, not zero-delta.

### 3.6 Trace

```go
type Trace struct { /* unexported */ }
```

The ordered execution history of a Signal.

Rules:
- Append-only during execution
- Capped at 64 steps — sliding window, oldest dropped first
- The most recently appended step is **never** dropped
- No mutex — a Signal has single goroutine ownership; concurrent access is a caller error
- On `Compose`: a's history, then b's history, then the Compose step. Compose step is always last.

### 3.7 Signal

```go
type Signal struct {
    Vector     SparseVector
    Confidence float64
    Trace      Trace
}
```

The canonical runtime object.

**Value, not identity.** Every Tier 2 operation returns a new `Signal`. Inputs
are never mutated. Return type is `Signal`, not `*Signal`.

**Ephemeral.** A Signal does not exist before a run starts and does not survive
after it ends. Only Prototypes persist between runs.

**Evolving state.** One Signal per execution thread. It accumulates the full
reasoning history in its Trace. Nodes are transformations; the Signal is what
is being transformed.

**Energy is derived.** `Energy(signal.Vector)` is always the activation
strength — never a stored field (Invariant 10).

### 3.8 Prototype

```go
type Prototype struct {
    Vector SparseVector
    Weight float64 // [0.0, 1.0] — higher = more trusted
}
```

Persistent knowledge. Where a Signal is ephemeral (born and discarded each run),
a Prototype is authored or learned knowledge that lives in the knowledge layer.
It is never modified during a run.

| | Signal | Prototype |
|--|--------|-----------|
| Lifetime | One run | Persistent |
| Carries | Vector + Confidence + Trace | Vector + Weight |
| Purpose | Evolving reasoning state | Stored knowledge |
| Modified during run | Yes, via Tier 2 ops | No |

---

## 4. Signal Lifecycle

```
Construct → Transform (nodes) → Compose (if paths merge) → Return → Discard
```

A Signal is born once per run at a caller-supplied initial confidence. It passes
through nodes sequentially — each transforms it and returns the next state. If
parallel execution paths converge, their Signals are `Compose`d before continuing.
The final Signal is returned to the caller. After that, it is discarded — it does
not re-enter the system.

The SparseVector inside may inform construction of future Prototypes through the
learning system, but the Signal itself — with its Confidence and Trace — does not
carry forward.

---

## 5. Algebra — Tier 1

Package: `core`. These operate on `SparseVector` directly. No Trace written. No
Confidence changed. Pure mathematics.

| Function | Signature | Description |
|----------|-----------|-------------|
| `Energy` | `(v SparseVector) float64` | Σ\|wᵢ\| — activation strength (L1 norm) |
| `Dot` | `(a, b SparseVector) float64` | Σ aᵢbᵢ — magnitude-sensitive similarity |
| `Cosine` | `(a, b SparseVector) float64` | (a·b)/(‖a‖‖b‖) — direction-sensitive, [-1,1] |
| `Jaccard` | `(a, b SparseVector) float64` | \|a∩b\|/\|a∪b\| — binary set overlap, [0,1] |
| `Merge` | `(a, b SparseVector) SparseVector` | Union; shared features sum weights |
| `Normalize` | `(v SparseVector) SparseVector` | Scale to unit L2 norm |
| `Filter` | `(v SparseVector, θ float64) SparseVector` | Remove features below threshold |

**Similarity choice:**

```
Generate    → Cosine   angular alignment; high-energy misaligned proto must not dominate
Attention   → Dot      magnitude matters; high-energy signals should surface first
Propagation → Dot      same reasoning as Attention
```

`Energy` is always derived — never stored. Invariant 10 holds by construction.

`Cosine` returns `0.0` when either vector has zero L2 norm. Never returns NaN.

`Jaccard` is binary (presence-only) in v0.1. Weights are read but ignored. A
feature is either present or absent.

`Filter` uses absolute weight comparison: a feature with weight `-0.5` passes a
threshold of `0.3` because `|-0.5| = 0.5 ≥ 0.3`.

---

## 6. Signal Operations — Tier 2

Package: `core`. These operate on the full Signal. Each returns a new Signal with
an updated Confidence and a new Step appended. Inputs are never mutated.

The `node` parameter is the string identity of the caller. It is required for
Step attribution and for confidence gate condition 2. An empty `node` disables
the confidence update for that call — the Step is still appended.

### Generate

```go
func Generate(s Signal, proto Prototype, θ float64, node string) Signal
```

Produces features present in `proto` but absent in `s`, scaled by
`Cosine(s.Vector, proto.Vector)`:

```
for f in proto.Vector:
    if f not in s.Vector and |proto.Vector[f]| > θ:
        out[f] = proto.Vector[f] × Cosine(s.Vector, proto.Vector)
```

If Cosine ≤ 0 (opposite or orthogonal), no features are generated. Direction
defines semantic fit — a misaligned prototype produces nothing.

### Compose

```go
func Compose(a, b Signal, mode ComposeMode, node string) Signal
```

Merges two Signals. Vector is `Merge(a.Vector, b.Vector)`. Confidence is
combined per `mode`. Trace is a's history + b's history + Compose step.

### Attenuate

```go
func Attenuate(s Signal, λ float64, node string) Signal
```

Decays all feature weights by factor `λ`: `w_new = w_old × (1 − λ)`. λ is
clamped to [0, 1]. Features that reach zero are removed. Confidence is
unchanged — decay is structural, not a knowledge event.

### FilterSignal

```go
func FilterSignal(s Signal, θ float64, node string) Signal
```

Removes features whose absolute weight falls below `θ`. Delegates to Tier 1
`Filter`. Confidence is unchanged.

### Propagate

```go
func Propagate(s Signal, proto Prototype, α float64, node string) Signal
```

Signal-level propagation — one Signal absorbs energy from one Prototype,
weighted by `Dot(s.Vector, proto.Vector)`:

```
for f in proto.Vector:
    out[f] += α × Dot(s.Vector, proto.Vector) × proto.Vector[f]
```

If Dot ≤ 0, no energy is transferred. Full field-level propagation (multiple
signals interacting) is in package `field`.

---

## 7. Confidence

Confidence behaves like belief: bounded to `[0.0, 1.0]`, gated, never inflated
by correlated evidence.

### Gate conditions

All four must hold for confidence to update:

1. `signal.Vector` is non-empty
2. `node` is non-empty
3. `matchScore > 0.15`
4. `protoWeight > 0.30`

Thresholds are grounded in sparse vector math: baseline cosine for two random
sparse vectors (k=10, D=10,000) is `√(k/D) ≈ 0.032`. The match threshold is
~5× this baseline.

### Update formula

```
confidence_new = confidence_old + 0.10 × (matchScore × protoWeight − confidence_old)
```

This is Rescorla-Wagner: belief moves toward the target proportionally to
prediction error. It never overshoots in a single step.

### Compose confidence

```
Independent → 1 − (1 − cA)(1 − cB)   two weak signals can produce strong confidence
Correlated  → max(cA, cB)             the same evidence processed twice is not doubled
```

---

## 8. Field Dynamics

Package: `field`. A `SignalField` is a collection of Signals that interact.
Field dynamics implement associative reasoning — signals sharing concept
dimensions amplify each other.

### SignalField

```go
type SignalField struct {
    Signals  []core.Signal
    Alpha    float64 // propagation coefficient (default: 0.1)
    Lambda   float64 // decay rate (default: 0.3)
    Epsilon  float64 // convergence threshold (default: 1e-4)
    MaxTicks int     // hard stop (default: 50)
}

func New(signals []core.Signal) SignalField
```

`New` emits a warning to stderr when `len(signals) > 15` with default parameters
(stability not validated beyond this bound). Not a panic — the caller decides.

All field operations return a new `SignalField`. Inputs are not mutated.

### Propagate

```go
func Propagate(f SignalField) SignalField
```

One tick of energy spreading. For each signal `i`, every other signal `j`
contributes proportional to `Dot(sᵢ, sⱼ)`:

```
vector_i_new[f] += α × Dot(sᵢ, sⱼ) × sⱼ[f]   for all j ≠ i
```

**Snapshot semantics:** all Dot products are computed from the pre-tick state.
Signal i does not see signal j's updated vector within the same tick. This
makes propagation order-independent and deterministic (Invariant 1).

### Decay

```go
func Decay(f SignalField) SignalField
```

Applies `w_new = w_old × (1 − λ)` to all features across all signals.
Features that reach zero are removed (Invariant 3).

### Stabilize

```go
func Stabilize(f SignalField) StabilizeResult

type StabilizeResult struct {
    Field      SignalField
    Ticks      int
    Converged  bool
    FinalDelta float64
}
```

Alternates `Propagate → Decay` per tick until total field energy delta falls
below `ε`, or `MaxTicks` is reached. `StabilizeResult` reports which exit
condition was hit.

**Stability condition:** `λ > α × (N − 1) × μ`

Default values (α=0.1, λ=0.3) satisfy this for N ≤ 15 at μ=0.05 (realistic
sparse overlap). The condition is verified at 4.3× margin.

### Attention

```go
func Attention(f SignalField, query core.Signal, k int) []core.Signal
```

Returns the top-`k` signals ranked by `Dot(query, sᵢ) × Energy(sᵢ)`.
Signals with score ≤ 0 are excluded. Results are sorted descending.
If `k > len(qualifying)`, all qualifying signals are returned.

---

## 9. Registry

Package: `registry`. Maps concept names to stable `FeatureIndex` values.

```go
func New() *Registry
func NewFrom(entries map[string]FeatureIndex) (*Registry, error)

func (r *Registry) ID(name string) FeatureIndex     // register or retrieve
func (r *Registry) Name(id FeatureIndex) (string, bool)
func (r *Registry) Has(name string) bool
func (r *Registry) Len() int
func (r *Registry) Snapshot() map[string]FeatureIndex
func (r *Registry) Names() []string
func (r *Registry) IDs() []FeatureIndex
func (r *Registry) Merge(other *Registry) (*Registry, error)
```

Rules:
- A name always maps to the same ID — across runs, across systems
- IDs are never reassigned
- The mapping is append-only — existing entries are immutable
- `ID("")` panics — empty name is a caller error, not a runtime condition
- `FeatureIndex(0)` is reserved — never assigned
- IDs are assigned sequentially from 1

`NewFrom` validates that IDs are contiguous from 1, no duplicates, no zeros.
Returns an error otherwise.

`Merge` checks for conflicts (same name, different ID) and returns an error.
New names from `other` are assigned IDs after the receiver's maximum, sorted
alphabetically for determinism.

The Registry is safe for concurrent reads and writes. `ID` uses a double-checked
locking pattern — read lock for fast path, write lock only when registering new.

---

## 10. Authoring Model

Building on Centrix involves two distinct phases.

### Authoring (offline, before deployment)

1. Define all concept dimensions your system will reason about
2. Register them via the Registry: `registry.ID("concept.name")` → stable FeatureIndex
3. Build Prototypes: assign SparseVectors and weights for each knowledge unit
4. Encode Prototypes into `.pack` files via the caller's encoding pipeline
5. Ship `.pack` files with the system

### Runtime (online, during execution)

1. Load `.pack` files into the knowledge layer
2. Encode incoming input as a SparseVector using the same feature space
3. Run Signal through the reasoning pipeline
4. Signal is discarded at run end. Prototypes are unchanged.

No new FeatureIndexes are introduced at runtime. The feature space is closed
at authoring time. This is what makes Invariant 1 (Deterministic Execution)
hold by construction.

Both the authoring pipeline and the runtime encoder must use the same Registry —
concept names must map to the same FeatureIndexes on both sides.

---

## 11. Invariants

These are non-negotiable. Violating any one breaks correctness in ways the
algebra cannot recover from.

**1 — Deterministic Execution**
Same input signals + same Prototypes + same feature space + same constants =
same output, every run.

**2 — Feature Semantic Consistency**
A FeatureIndex always represents the same concept across all signals, prototypes,
and runs. This is the precondition for similarity math to be meaningful.

**3 — Sparse Vector Integrity**
Zero weights are never stored. Feature presence is intentional.

**4 — Signal Isolation**
One goroutine owns one Signal. Signals are never shared or mutated concurrently.
Parallelism happens between Signals — not within one.

**5 — Trace Completeness**
Every Tier 2 transformation appends a Step. No transformation is silent.
Bounded at 64 steps. Most recent Step is never dropped.

**6 — Energy Stability**
`λ > α × (N − 1) × μ`. Default values (α=0.1, λ=0.3) satisfy this for N ≤ 15.
`MaxTicks` acts as a hard stop regardless of convergence.

**7 — Confidence Integrity**
Bounded to [0.0, 1.0]. Gated. Not inflated by correlated evidence.

**8 — Signal Lifecycle**
Signals are ephemeral. Created at run start, discarded at run end. Never cross
run boundaries. Only Prototypes persist.

**9 — Memory Boundary**
Persistent knowledge is `Prototype { Vector, Weight }`. Signals cannot persist.
Each run starts from clean authored knowledge.

**10 — Energy is Derived**
A Signal's energy is always `Energy(signal.Vector)`. Never a stored field.
One source of truth for activation strength.
