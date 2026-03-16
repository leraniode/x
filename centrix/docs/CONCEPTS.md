# Centrix — Concepts, Blueprint & Mental Model
> Status: Pre-build canonical reference · Version: 0.2
> Module: github.com/leraniode/xgo/centrix (current)
> Future: github.com/leraniode/centrix (standalone, when stable)

---

## Table of Contents

1. [What Centrix Is](#1-what-centrix-is)
2. [The Mental Model](#2-the-mental-model)
3. [Core Primitives](#3-core-primitives)
4. [Signal Lifecycle](#4-signal-lifecycle)
5. [The Two-Tier Algebra](#5-the-two-tier-algebra)
6. [Confidence Update Rule](#6-confidence-update-rule)
7. [Generation](#7-generation)
8. [Field Dynamics](#8-field-dynamics)
9. [Package Structure](#9-package-structure)
10. [Feature Space Design](#10-feature-space-design)
11. [The Authoring Model](#11-the-authoring-model)
12. [System Invariants](#12-system-invariants)
13. [Example Concept Graph](#13-example-concept-graph)

---

## 1. What Centrix Is

Centrix is a sparse signal mathematics library for Go. It defines the primitives,
algebra, and field dynamics that power reasoning and generation in systems built
on top of it. Centrix has no knowledge of what builds on it — it is a pure
mathematical layer.

Centrix does not:
- Know about nodes, flows, engines, or knowledge stores
- Know about file formats, encoding pipelines, or data ingestion
- Impose a feature space or feature encoding
- Own any pipeline or execution logic
- Import any external system

Centrix does:
- Define the canonical Signal and Prototype types that reasoning systems use
- Provide a complete, correct algebra over sparse vectors
- Define the trace and observability types
- Define the action vocabulary that the learning system acts on
- Implement field dynamics: propagation, decay, stabilisation, attention

---

## 2. The Mental Model

Think of Centrix as the physics of the system.

Just as physics defines mass, energy, force, and motion — and everything built in
the physical world operates according to those laws — Centrix defines Signal,
SparseVector, Confidence, and Trace. Every system that reasons or generates using
Centrix operates according to Centrix's algebra.

The analogy holds further:
- SparseVector is matter — the substance a signal is made of
- Confidence is energy — how strongly the signal asserts itself
- Trace is history — the record of what happened to the signal
- Field dynamics are mechanics — how signals interact when they share space
- Algebra operations are transformations — how signals change
- Prototype is memory — authored or learned knowledge that persists between runs

A signal does not know what built it or what will consume it. It is just a
mathematical object moving through a system that obeys Centrix's rules.

---

## 3. Core Primitives

### 3.1 FeatureIndex

```go
type FeatureIndex = uint32
```

A key in a SparseVector. Centrix does not define what a FeatureIndex means —
that is the caller's responsibility. A FeatureIndex is just an unsigned integer
that identifies a dimension in the feature space.

### 3.2 SparseVector

```go
type SparseVector map[FeatureIndex]float64
```

The feature representation of a signal. Only non-zero features are stored.
This is the sparsity principle: absence is meaningful, and storing zeros wastes
space and time.

Operations over two SparseVectors run in O(k) where k is the number of active
features, not the size of the feature space. This is what makes Centrix viable
on constrained hardware.

All weights are float64. This is non-negotiable in v0.1 — revisit only if
profiling on constrained hardware shows a measurable impact.

### 3.3 Prototype

```go
// Prototype is persistent knowledge — a SparseVector that survives between runs.
// Where a Signal is ephemeral (created at run start, discarded at run end),
// a Prototype is authored or learned knowledge that lives in the knowledge layer.
// Weight reflects how trusted or reliable this prototype is.
type Prototype struct {
    Vector SparseVector
    Weight float64
}
```

The fundamental distinction: Signals are runtime objects. Prototypes are
persistent objects. A Signal wraps a SparseVector with Confidence and Trace.
A Prototype wraps a SparseVector with Weight. They are never the same thing.

Only derived knowledge — Prototypes — persists between runs. Signals do not.

### 3.4 Action

A typed constant representing what was done to a signal at a step.
The learning system uses Action to decide how to update weights and trust.

```go
type Action int

const (
    Generated  Action = iota // new features produced absent in the input signal
    Matched                  // signal compared against a prototype and scored
    Propagated               // energy spread through a field based on similarity
    Attenuated               // signal weights reduced by decay
    Composed                 // two signals merged into a higher-level signal
    Filtered                 // weak features removed below a threshold
)
```

Action is Centrix-defined because each constant corresponds directly to a
Centrix operation. Callers map their own execution events onto these constants.

Note: callers (such as execution engines) may define their own effect vocabulary
for execution-level events (tool calls, output emission, etc.). Those are not
Centrix Actions — they are a separate type defined by the caller.

### 3.5 Step

One entry in a Signal's execution history.

```go
type Step struct {
    Node             string
    Action           Action
    Value            any
    ConfidenceBefore float64
    ConfidenceAfter  float64
}
```

Node is the identifier of what produced this step — Centrix does not define what
a node is, only that it has a string identity. Action records the operation that
was applied. Value is an optional payload. The confidence delta records exactly
how this step changed the signal's certainty.

### 3.6 Trace

```go
type Trace []Step
```

The ordered log of every Step a Signal has accumulated since it was created.

Rules:
- Append-only during execution
- Capped at 64 steps — sliding window, oldest dropped first
- The most recent Step is never dropped
- On Compose: both Traces are merged (a first, then b), then the Compose Step
  is appended. The merged Trace is then capped. The Compose Step is always last
  and is never the one dropped.
- No mutex on Trace — a Signal is owned by one goroutine at a time.
  Concurrent access to a single Signal is a caller error, not a race to protect.

Cap reasoning: reasoning chains rarely exceed 10–20 steps. Cap of 64 covers
99% of expected usage while bounding memory at ~2KB per Signal trace.
Caller can override cap if required.

### 3.7 Signal

The canonical runtime object. Everything in Centrix operates on or produces Signals.

```go
type Signal struct {
    Vector     SparseVector
    Confidence float64
    Trace      Trace
}
```

**A Signal is a value, not an identity.**
Not a persistent object that survives transformations — a value that is created,
transformed, and consumed. Every Tier 2 operation takes a Signal and returns a
new Signal. The input is unchanged. Return type is `Signal`, not `*Signal`.
No pointer — no shared mutable state.

**A Signal is a runtime object.**
It does not exist before a flow run starts and does not survive after it ends.
Prototypes persist as authored or learned knowledge between runs. The Signal
wrapping (Confidence + Trace) is constructed fresh at the start of each run.
Confidence begins at a caller-supplied initial value, never inherited from
a prior run.

**A Signal is evolving state, not a message.**
One Signal per execution thread. A node receives the current Signal — the
accumulated state of all reasoning so far — transforms it, and returns the
next state. The Signal grows richer as it moves through nodes. Nodes are
transformations; the Signal is what is being transformed.

**The Trace belongs to the Signal.**
A node can inspect the full history of how the Signal it receives was built —
observability from inside the reasoning process, not just from outside.

**`Next` does not belong on Signal.**
Routing concerns (which node to visit next) are the caller's responsibility.
A `Next` field on Signal would give Signal knowledge of graph structure —
a direct violation of Centrix's boundary. Callers handle routing externally.

---

## 4. Signal Lifecycle

```
Construction   →   Node transforms   →   Composition (optional)   →   Discard
(run start)         (per node)             (paths merging)            (run end)
```

A Signal is born once per run. It passes through nodes sequentially — each node
receives it and returns a transformed version. If parallel execution paths
converge, their Signals are Composed into one before continuing. At run end,
the final Signal is returned to the caller as the result. After that, it is
discarded — it does not re-enter the system.

The SparseVector inside it may inform the construction of future Prototypes
(through the learning system), but the Signal itself — with its Confidence
and Trace — does not carry forward.

---

## 5. The Two-Tier Algebra

Centrix operations fall into two distinct tiers. This distinction is fundamental.

### Tier 1 — SparseVector Operations (Pure Math)

These operate on SparseVector directly. No Trace is written. No Confidence
is changed. These are pure mathematical transformations.

| Operation | Description |
|-----------|-------------|
| `Dot(a, b)` | Σ aᵢ × bᵢ — raw directional similarity, magnitude-sensitive |
| `Cosine(a, b)` | (a · b) / (‖a‖ × ‖b‖) — normalised similarity, range [-1, 1] |
| `Jaccard(a, b)` | \|a ∩ b\| / \|a ∪ b\| — binary set overlap (presence only, v0.1) |
| `Merge(a, b)` | Union of features; shared features sum their weights |
| `Normalize(v)` | Scale to unit L2 norm |
| `Filter(v, θ)` | Remove features below threshold θ |
| `Energy(v)` | Σ \|wᵢ\| — sum of absolute weights; activation strength of a vector |

**Operation-to-similarity mapping (from R1):**
```
Generate:    Cosine — prototype matching must prioritise angular alignment.
             High-energy misaligned prototype must not dominate.
Attention:   Dot   — magnitude influence required. High-energy signals
             should propagate more aggressively.
Propagation: Dot   — same reasoning as Attention.
```

`Energy` is always derived from the Vector — never a separate stored field.
Invariant 10 holds by construction: one source of truth for activation strength.

Use Tier 1 operations when you need fast similarity lookups, feature manipulation,
or preparation work before constructing a full Signal.

### Tier 2 — Signal Operations (Stateful Transformations)

These operate on the full Signal. They produce a new Signal with an updated
Confidence and a new Step appended to the Trace.

| Operation | Action Appended | Confidence Update |
|-----------|----------------|-------------------|
| `Generate(s, proto, θ)` | Generated | Updated if conditions met |
| `Compose(a, b, mode)` | Composed | Updated if conditions met |
| `Propagate(s, field, α)` | Propagated | Updated if conditions met |
| `Attenuate(s, λ)` | Attenuated | Reduced by decay factor |
| `FilterSignal(s, θ)` | Filtered | Unchanged |

Tier 2 operations are what reasoning nodes call. Every call is auditable.

### ComposeMode

Compose requires the caller to specify whether the two signals are independent
or correlated. The caller knows — Centrix cannot infer it.

```go
type ComposeMode int

const (
    Independent ComposeMode = iota // OR-combination: 1 − ∏ₖ(1 − cₖ)
    Correlated                     // Max: take the higher confidence
)
```

If `Cosine(a.Vector, b.Vector) > 0.5`, the signals are correlated.
Threshold of 0.5 is the default (from R3). Configurable in v0.2 if needed.

---

## 6. Confidence Update Rule

Confidence updates when all of the following conditions are satisfied:

1. **Signal is known** — the Signal's Vector has active features (non-empty)
2. **Step is known** — the Action is defined and the Node identity is non-empty
3. **Knowledge satisfies it** — match score against a Prototype exceeds minimum
   quality threshold: `matchScore > 0.15`
4. **Action produced good results** — Prototype weight exceeds minimum trust
   threshold: `protoWeight > 0.30`

Threshold derivation (from R4): baseline cosine for two random sparse vectors
(k=10, D=10,000) is `√(k/D) ≈ 0.03`. Meaningful match = 5× above baseline = 0.15.

When all four conditions are met, confidence updates as:

```
confidence_new = confidence_old + α × (matchScore × protoWeight − confidence_old)
```

Where `α = 0.10` (conservative update rate, from R4).

This is a bounded update — confidence moves toward `matchScore × protoWeight`
but never overshoots it in a single step. Confidence is always bounded to [0.0, 1.0].

When conditions are not fully met, confidence is unchanged. The Step is still
appended — ConfidenceBefore == ConfidenceAfter is itself information for the
learning system: it distinguishes "confidence was gated" from "confidence moved."

**Validated constants (from R4):**
```
minMatchScore    = 0.15
minProtoWeight   = 0.30
α_confidence     = 0.10
```

---

## 7. Generation

Generation is what makes Centrix generative. The operation:

```go
Generate(query Signal, prototype Prototype, θ float64) Signal
```

Produces features present in the Prototype's Vector but absent in the query's
Vector, scaled by the Cosine similarity between query and prototype:

```
gen(q, p) = { (f, wᵖ[f] × Cosine(q.Vector, p.Vector)) | f ∈ p, f ∉ q, wᵖ[f] > θ }
```

The output contains features that were not in the input. This is deterministic
generation: the output space is bounded by the Prototype, the output is scaled
by match quality, and nothing is invented — only inferred from the match.

Cosine is used here (not Dot) because prototype matching must prioritise angular
alignment over raw energy. A high-energy misaligned Prototype must not dominate
generation — direction defines semantic fit.

**Generation chain:** The output of Generate becomes input to the next Generate
call. Each step enriches the signal with features the previous step could not
have produced. Each chain step is traceable, each Action is appended, and
confidence updates according to the rule at each step.

---

## 8. Field Dynamics

A SignalField is a collection of Signals that interact. Field dynamics are how
Centrix implements associative reasoning — signals that share features amplify
each other; signals that share nothing do not interact.

### Propagation

Energy spreads through the field. Each signal's weights increase based on
similarity to its neighbours. Dot product is used — magnitude influence is
required here; high-energy signals should propagate more aggressively.

```
wᵢ(t+1) = wᵢ(t) + α × Σⱼ Dot(sᵢ.Vector, sⱼ.Vector) × wⱼ(t)   for j ≠ i
```

Where `α = 0.1` (propagation coefficient, from R2).

### Decay

Weights reduce by factor λ per tick to prevent runaway energy:

```
wᵢ(t+1) = wᵢ(t) × (1 − λ)
```

Where `λ = 0.3` (decay rate, from R2).

### Stabilisation

Propagation and decay run until the total field energy delta falls below
convergence threshold ε, or a maximum tick count is reached.

Where `ε = 1e-4` (from R2). Settles in < 50 iterations under normal conditions.

### Stability Condition

For the field to converge, the following must hold:

```
λ > α × (N − 1) × μ
```

Where:
- λ = decay factor
- α = propagation coefficient
- N = number of signals in the field
- μ = mean cosine similarity between signals

**Validated defaults** (α=0.1, λ=0.3) satisfy this condition for N ≤ 15
with moderate feature overlap (from R2).

When N > 15, callers must either increase λ, decrease α, or ensure the signal
population is sparse enough that μ is low. Centrix emits a warning (not a panic)
when N exceeds 15 with default parameters. The maximum tick count acts as a
hard stop regardless of convergence.

Behaviour for N > 15 or high-overlap feature spaces is uncharacterised in v0.1.
Document the constraint in field.go. Revisit in v0.2 with profiling data.

### Attention

Selects the top-K signals from a field ranked by `Dot(query, sᵢ) × Energy(sᵢ)`.
Dot product used — magnitude-scaled ranking is correct here; high-energy signals
that align with the query should surface first.

This is how a field narrows to the most relevant signals before Prototype matching.

---

## 9. Package Structure

```
centrix/
  core/
    types.go        — FeatureIndex, SparseVector, Prototype, Action,
                      Step, Trace, Signal, ComposeMode
    algebra.go      — Tier 1: Dot, Cosine, Jaccard, Merge, Normalize,
                      Filter, Energy
    signal_ops.go   — Tier 2: Generate, Compose, Attenuate, FilterSignal,
                      Propagate (signal-level)
  field/
    field.go        — SignalField, Propagate, Decay, Stabilize, Attention
  registry/
    registry.go     — concept name → FeatureIndex mapping
                      append-only, stable, deterministic
```

No other packages. No CLI. No knowledge layer. No pipeline. No imports from
any system built on Centrix.

---

## 10. Feature Space Design

The feature space is what gives SparseVector weights semantic meaning. Without
a well-defined feature space, similarity math is syntactically correct but
semantically meaningless.

### What a feature represents

A feature represents a **concept dimension** in a semantic space. Not a token,
not a word, not a surface form — a concept. Two tokens that mean the same thing
should map to the same FeatureIndex. Feature 1201 represents `physics.gravity`,
not the string "gravity."

This is what makes Cosine similarity semantically meaningful: two signals share
a feature only if they share the same concept, not merely similar-sounding words.

### The feature space is global

A single uint32 space spans the entire system. FeatureIndex 1201 means
`physics.gravity` in every Signal, every Prototype, every run. Cross-domain
similarity is real — signals from different domains can share features if the
underlying concepts genuinely overlap.

Callers use ID range conventions to avoid accidental collisions between domains.
Centrix documents the pattern but does not enforce it.

### The feature space is static in v0.1

The complete set of features is defined before any Signal is created. No new
FeatureIndexes are introduced at runtime. This preserves Invariant 1
(Deterministic Execution) by construction.

Dynamic feature discovery is deferred to v0.2, tied to the learning system.

### Feature semantic consistency (Invariant)

**A FeatureIndex must always represent the same concept — across signals,
across prototypes, across runs.** If this rule breaks, similarity math becomes
meaningless. This is the most fundamental correctness requirement in the system.

### SparseVector integrity

- Dimension space: 10,000+ possible FeatureIndexes
- Active features per vector: 10–30 in typical use
- Zero weights are never stored — absence is meaningful
- Feature presence must be intentional — a feature appears because the concept
  it represents genuinely contributes to the signal's meaning

### The Registry

The Registry maps concept names to stable uint32 FeatureIndexes:

```go
registry.ID("physics.gravity") // → 1201
```

Rules: append-only, stable, deterministic. A name always maps to the same ID.
IDs are never reassigned. Callers are not required to use the Registry — it is
infrastructure that makes stable feature space authoring tractable.

The Registry lives in `centrix/registry`. It has no dependency on core algebra.
It is used by authoring pipelines, not by Centrix's runtime operations.

---

## 11. The Authoring Model

The static feature space constraint implies an offline authoring step.

### Two phases

**AUTHORING (offline, before deployment)**
1. Define all concept dimensions your system will reason about
2. Register them: `registry.ID("concept.name")` → stable FeatureIndex
3. Build Prototypes: assign SparseVectors and weights for each knowledge unit
4. Encode Prototypes into `.pack` files via the caller's encoding pipeline
5. These `.pack` files are shipped with the system

**RUNTIME (online, during execution)**
1. Load `.pack` files into the knowledge layer
2. Encode incoming input as a SparseVector using the same feature space
3. Run Signal through the reasoning pipeline
4. Signal is discarded at run end. Prototypes are unchanged.

No new FeatureIndexes are introduced at runtime. No new concepts emerge during
execution. The feature space is closed at authoring time.

This is what makes Invariant 1 (Deterministic Execution) hold by construction.

### Feature space consistency across authoring and runtime

The caller is responsible for ensuring that Prototypes encoded into `.pack` files
and Signals constructed at runtime use the same Registry and the same feature
space. The Registry is the mechanism that makes this tractable — both the
authoring pipeline and the runtime input encoder use the same Registry, so
concept names map to the same FeatureIndexes on both sides.

---

## 12. System Invariants

These are the non-negotiable guarantees every Centrix implementation must uphold.
Violating any of these breaks correctness in ways the algebra cannot recover from.

**1. Deterministic Execution**
Same input signals + same Prototypes + same feature space + same constants =
same output, every run. Requires: static feature space, stable FeatureIndex
mapping, no concurrent signal mutation, no randomness.

**2. Feature Semantic Consistency**
A FeatureIndex always represents the same concept across all signals, prototypes,
and runs. This is the precondition for similarity math to be meaningful.

**3. Sparse Vector Integrity**
Zero weights are never stored. Feature presence is intentional. Vectors represent
semantic mixtures of concepts, not arbitrary numerical arrays.

**4. Signal Isolation**
One goroutine owns one Signal. Signals are not shared or mutated concurrently.
Parallelism happens between Signals (different execution threads), not within one.
This eliminates race conditions and nondeterministic traces.

**5. Trace Completeness**
Every Tier 2 transformation appends a Step. No transformation is silent.
The Trace is the audit log — always available, always accurate, always ordered.
Bounded at 64 steps (sliding window). The most recent Step is never dropped.

**6. Energy Stability**
Field propagation must converge. The stability condition must hold:
`λ > α × (N − 1) × μ`. Default values (α=0.1, λ=0.3) satisfy this for N ≤ 15.
Callers are warned when N exceeds this. Maximum tick count acts as hard stop.

**7. Confidence Integrity**
Confidence behaves like belief, not energy. Bounded to [0.0, 1.0]. Gated —
updates only when match quality and prototype weight satisfy their thresholds.
Not inflated by correlated evidence (Compose uses Max when correlated,
OR-combination when independent).

**8. Signal Lifecycle**
Signals are ephemeral. Created at run start, evolved through the reasoning
pipeline, discarded at run end. Signals never cross run boundaries.
Only Prototypes persist.

**9. Memory Boundary**
Persistent knowledge is represented as Prototypes `{ Vector, Weight }`.
Signals cannot persist. This prevents reasoning state from contaminating
future runs — each run starts from clean, authored knowledge.

**10. Energy is Derived**
A Signal's energy is always `Energy(signal.Vector)` — the sum of absolute
weights. Energy is never a separate stored field. One source of truth
for activation strength: the Vector itself.

---

## 13. Example Concept Graph

This shows how a caller might organise the layers from authoring through to
execution. This is not Centrix's responsibility — it is an illustration of how
the pieces connect for a caller building a full reasoning pipeline on Centrix.

```
KnowledgePack (caller)
      ↓
Feature Registry  ←  concept name → FeatureIndex mapping
      ↓
Feature Space     ←  the set of all valid FeatureIndexes and their meanings
      ↓
SparseVectors     ←  semantic composition over the feature space
      ↓
Prototypes        ←  SparseVector + Weight (persistent knowledge)
      ↓
Signals           ←  SparseVector + Confidence + Trace (runtime objects)
      ↓
Tier 2 Operations ←  Signal → Signal transformations (caller's nodes)
      ↓
Field Dynamics    ←  propagation, decay, stabilisation, attention
      ↓
Prototype Matching←  Cosine similarity between signal and Prototypes
      ↓
Compose           ←  merging reasoning paths
      ↓
Output Signal     ←  the result of the reasoning run
```

KnowledgePack authoring and node execution are the caller's responsibility.
Everything from Feature Registry to Output Signal is Centrix's domain.
