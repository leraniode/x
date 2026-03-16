# Centrix — Build Blueprint
> Status: Pre-build · Version: 0.1

---

## 1. Module Identity

```
module: github.com/leraniode/xgo/centrix
go:     1.22
deps:   none (zero external dependencies)
```

> Centrix lives in xgo (leraniode's experimental Go repository) until the full
> algebra is built, field dynamics are proven, and at least one real Illygen
> integration has been validated. At graduation it moves to
> `github.com/leraniode/centrix` with a stable public API.

---

## 2. Package Layout

```
centrix/
  core/
    types.go        Phase 1 — all type definitions
    algebra.go      Phase 2 — Tier 1 SparseVector operations
    signal_ops.go   Phase 3 — Tier 2 Signal operations
    types_test.go   Phase 1 — type construction and invariant tests
    algebra_test.go Phase 2 — algebra correctness tests
    signal_test.go  Phase 3 — Tier 2 operation tests
  field/
    field.go        Phase 4 — SignalField and dynamics
    field_test.go   Phase 4 — field correctness tests
  registry/
    registry.go     Phase 5 — concept name → FeatureIndex mapping
    registry_test.go Phase 5 — stability and determinism tests
```

**Energy note:** `Signal` has no `Energy` field. Energy is always derived:
`signal.Vector.Energy()` returns the sum of absolute weights. This is the
only source of truth for activation strength. No separate field, no sync burden.

---

## 3. Build Phases

### Phase 1 — Types

File: `core/types.go`

Define in this order, no skipping:

```go
// The feature dimension key. Caller-defined meaning.
type FeatureIndex = uint32

// The sparse feature representation.
// Only non-zero features are stored.
type SparseVector map[FeatureIndex]float64

// Typed action constants — exactly these six, no others in v0.1.
type Action uint8
const (
    Generated  Action = iota // new features produced from prototype
    Matched                  // signal scored against prototype
    Propagated               // energy spread through field
    Attenuated               // weights reduced by decay
    Composed                 // two signals merged
    Filtered                 // weak features removed
)

// One step in a signal's execution history.
type Step struct {
    Node             string
    Action           Action
    Value            any
    ConfidenceBefore float64
    ConfidenceAfter  float64
}

// The full ordered execution history of a signal.
type Trace []Step

// The canonical runtime object.
// Immutable by convention — operations return new Signals.
type Signal struct {
    Vector     SparseVector
    Confidence float64
    Trace      Trace
}
```

Constructor functions defined in this phase:
- `NewSignal(capacity int) Signal` — empty signal with pre-allocated map
- `NewSignalWithConfidence(v SparseVector, confidence float64) Signal`
- `(s Signal) Clone() Signal` — deep copy, Trace included
- `(s Signal) WithStep(step Step) Signal` — return new Signal with step appended
- `(v SparseVector) Energy() float64` — sum of absolute weights
- `(v SparseVector) Len() int` — number of active features

Tests for Phase 1:
- NewSignal produces correct zero-state
- Clone is a deep copy (mutating clone does not affect original)
- WithStep appends and does not mutate original
- Action string representations are correct
- Energy returns correct L1 norm

---

### Phase 2 — Tier 1 Algebra (SparseVector operations)

File: `core/algebra.go`

All operations take SparseVectors, return SparseVectors or float64.
No Signal, no Trace, no Confidence touched.

```
Dot(a, b SparseVector) float64
    Iterates the smaller map for efficiency.
    Returns Σ aᵢ × bᵢ over the intersection.

Cosine(a, b SparseVector) float64
    Returns (Dot(a,b)) / (‖a‖ × ‖b‖).
    Returns 0 if either vector is zero.

Jaccard(a, b SparseVector) float64
    Intersection size / Union size.
    For binary feature spaces (weights treated as present/absent).
    Returns |features(a) ∩ features(b)| / |features(a) ∪ features(b)|.
    Returns 0 if both are empty.

Merge(a, b SparseVector) SparseVector
    Union of features.
    Shared features: weights sum.
    Exclusive features: carried over unchanged.

Normalize(v SparseVector) SparseVector
    Scale to unit L2 norm.
    Returns clone unchanged if ‖v‖ = 0.

Filter(v SparseVector, theta float64) SparseVector
    Remove features where |w| < theta.
    Returns new SparseVector.
```

Tests for Phase 2:
- Dot of orthogonal vectors = 0
- Dot of identical normalised vector = 1
- Cosine of identical vectors = 1
- Cosine of orthogonal vectors = 0
- Jaccard of identical sets = 1
- Jaccard of disjoint sets = 0
- Merge union correctness (shared features sum, exclusive features carry)
- Normalize produces unit norm
- Normalize of zero vector returns zero vector unchanged
- Filter removes below threshold
- Filter retains at and above threshold

---

### Phase 3 — Tier 2 Signal Operations

File: `core/signal_ops.go`

All operations take Signals, return new Signals.
Every operation appends a Step to the Trace.
Confidence is updated according to the confidence update rule where conditions are met.

```
Generate(query Signal, prototype Signal, theta float64, node string) Signal
    Produces features in prototype.Vector absent from query.Vector,
    scaled by Cosine(query.Vector, prototype.Vector).
    Only features where prototype weight > theta are emitted.
    Appends Step{Node: node, Action: Generated, ...}.
    Updates confidence if all four conditions are met.

Compose(a, b Signal, node string) Signal
    Merges two Signals: Merge(a.Vector, b.Vector), then Normalize.
    Confidence: uses OR-combination rule.
    Appends Step{Node: node, Action: Composed, ...}.

Attenuate(s Signal, lambda float64, node string) Signal
    Applies decay: each weight × (1 − lambda).
    Confidence: reduced proportionally.
    Appends Step{Node: node, Action: Attenuated, ...}.

FilterSignal(s Signal, theta float64, node string) Signal
    Calls Filter on s.Vector.
    Confidence: unchanged.
    Appends Step{Node: node, Action: Filtered, ...}.
```

Confidence update rule (internal helper):
```
func updateConfidence(current, matchScore, protoWeight, alpha float64,
                      vectorNonEmpty bool, nodeNonEmpty bool,
                      minMatchScore, minProtoWeight float64) float64
```
Returns current unchanged if any condition fails.
Returns current + alpha × (matchScore × protoWeight − current) when all pass.

OR-combination (used by Compose):
```
c_total = 1 − (1 − cA) × (1 − cB)
```

Tests for Phase 3:
- Generate emits only features absent from query
- Generate emits nothing when similarity is zero
- Generate correctly scales by similarity
- Generate confidence updates when conditions are met
- Generate confidence unchanged when conditions not met
- Generate trace grows by exactly one Step with Action=Generated
- Compose produces correct merged vector
- Compose OR-combines confidence correctly
- Attenuate reduces all weights by lambda
- FilterSignal removes sub-threshold features from Vector
- Immutability: original Signal unchanged after every operation

---

### Phase 4 — Field Dynamics

File: `field/field.go`

```go
type SignalField struct {
    Signals []Signal
    Alpha   float64  // propagation coefficient (default 0.1)
    Lambda  float64  // decay rate (default 0.05)
}

func New() *SignalField
func (f *SignalField) Add(s Signal)
func (f *SignalField) TotalEnergy() float64
func (f *SignalField) Propagate()
func (f *SignalField) Decay()
func (f *SignalField) Prune(minEnergy float64)
func (f *SignalField) Stabilize() int      // returns ticks to convergence
func (f *SignalField) Attention(query Signal, k int) []Signal
```

Propagate:
  Compute deltas before applying (order-independent updates).
  Each signal i receives contribution from each signal j ≠ i:
  contribution = Alpha × Cosine(i.Vector, j.Vector) × j.Vector.Energy()
  Distributed across i's features proportionally.

Decay:
  Every weight in every Signal's Vector × (1 − Lambda).
  Updates each Signal's Confidence proportionally.

Stabilize:
  Loop: Propagate → Decay → check |ΔEnergy| < ε (default 0.001).
  Hard cap: 50 ticks maximum.
  Returns actual tick count.

Attention:
  Score each signal: Cosine(query.Vector, s.Vector) × s.Vector.Energy()
  Return top-K sorted descending.
  Return all if K > len(Signals).

Tests for Phase 4:
- Single-signal field: Propagate is no-op
- Identical signals amplify each other after Propagate
- Orthogonal signals do not interact after Propagate
- Decay reduces all weights
- Stabilize converges within tick limit for small field
- Prune removes below-energy signals
- Attention returns correct top-K order
- Attention returns all when K exceeds field size

---

## 4. Invariants (Enforced in Tests)

These must hold across all phases:

1. **Sparsity** — SparseVectors never store zero-value features after any operation
2. **Immutability** — no Tier 2 operation mutates its input Signal
3. **Trace monotonicity** — Trace length never decreases
4. **Confidence bounds** — Confidence is always in [0.0, 1.0]
5. **Type safety** — no `int` for FeatureIndex, no `float32` anywhere

---

## 5. What Is Not Built in v0.1

These are explicitly out of scope. Do not add them:

- Feature encoding (text → FeatureIndex mapping)
- Prototype storage or retrieval
- Knowledge packs or knowledge stores
- Pipeline execution
- CLI or example programs
- Any import of an external system

---

## 6. Commit Sequence

Each phase is one commit. Commit format:

```
feat(core): define types — Signal, SparseVector, Action, Step, Trace
feat(core): implement Tier 1 algebra — Dot, Cosine, Jaccard, Merge, Normalize, Filter
feat(core): implement Tier 2 signal operations — Generate, Compose, Attenuate, Filter
feat(field): implement SignalField — Propagate, Decay, Stabilize, Attention
feat(registry): implement Registry — concept name to FeatureIndex mapping
```

No phase is committed until its tests pass. No phase is skipped.
