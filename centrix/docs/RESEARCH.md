# Centrix — Research Foundation
> Status: Complete · All decisions implemented · v0.1

---

## Why This Exists

The dominant approach to machine intelligence — large language models deployed
on GPU clusters — has a structural resource problem.

During inference, the Transformer's self-attention mechanism must access every
previously generated token. Systems cache this data (the KV cache) to avoid
recomputation. But because sequence length is unpredictable, traditional systems
pre-allocate the maximum possible GPU memory for every request. Empirically,
this wastes **60–80% of KV cache memory** per request. A single 13B-parameter
model conversation can consume 1.6GB of GPU memory for 2048 tokens. At scale,
this makes LLM deployment expensive by default — not because of the intelligence
work, but because of memory management.

Specialised inference engines (vLLM's PagedAttention, continuous batching) reduce
this waste to under 4%. But the fundamental dependency remains: GPU hardware,
probability distributions, non-determinism, and an architecture that cannot
guarantee its own outputs.

Centrix takes a different position. Instead of managing the costs of probabilistic
inference on dense neural representations, it defines a mathematical substrate
for **deterministic reasoning over sparse signals** — a system where:

- Operations scale with active features (`k`), not dimension space size (`D`)
- A fully-loaded Signal costs ~4KB, not gigabytes
- CPU and RAM are the primary resources — accessible, measurable, bounded
- Every output is traceable, reproducible, and explainable by construction
- There is no attention mechanism, no KV cache, and no hallucination

The resource constraint is not an obstacle. It is the design target.

---

## Theoretical Foundations

The mathematics in Centrix is not invented. Each core mechanism traces to
established research.

### Sparse Distributed Representations (Kanerva, 1988)

Kanerva's Sparse Distributed Memory proved that high-dimensional sparse binary
vectors have powerful representational and retrieval properties. Two random sparse
vectors in a 10,000-dimension space with 20 active features each have an expected
cosine similarity near zero — the noise floor is mathematically bounded at
`√(k/D) ≈ 0.045`. This is the foundation for `minMatchScore = 0.15`: a
meaningful match is defined as 3× above the noise floor, not an arbitrary
threshold. The same principle justifies using `map[uint32]float64` for
SparseVector — storing only non-zero features is not an optimisation, it is the
correct representation.

### Prototype Theory (Rosch, 1975)

Rosch's prototype theory of categorisation established that concepts are not
defined by necessary and sufficient conditions but by similarity to a central
prototype. A sparrow is more "bird" than a penguin not because of logical
membership but because of graded similarity to the prototype. Centrix's
`Prototype` type directly encodes this: a SparseVector representing a concept
centre with a Weight encoding how trusted or canonical that prototype is. The
`Generate` operation is the computational realisation — producing features
implied by prototype similarity, scaled by match quality.

### Spreading Activation (Collins & Loftus, 1975)

Collins and Loftus modelled human semantic memory as a network where activation
spreads from one concept to related concepts based on associative strength.
Centrix's field dynamics implement this computationally. `SignalField.Propagate`
transfers energy between signals proportional to their Dot similarity — signals
that share concept dimensions amplify each other. `Attention` selects the most
activated signals. This is not a metaphor for spreading activation — it is a
sparse-vector implementation of it.

### Holographic Reduced Representations (Plate, 1995)

Plate's HRR demonstrated that compositional structure can be encoded in fixed-
width distributed vectors using circular convolution. The key insight for
Centrix: generation is bounded. `Generate` only produces features within the
prototype's vector — the output space is constrained by authored knowledge, not
open-ended. Nothing is invented. The composition space is closed at authoring
time. This is the mathematical basis for the determinism guarantee.

### Learning Rules (Rescorla-Wagner, Oja, Hebb)

The confidence update rule is a direct application of Rescorla-Wagner (1972):
belief updates proportionally to prediction error, bounded by learning rate α.
`confidence_new = confidence_old + α × (target − confidence_old)`. Oja's rule
(1982) provides the weight normalisation basis — learned Prototype weights remain
bounded. Hebb's rule (1949) underlies field propagation: signals that fire
together wire together, implemented as Dot-weighted energy transfer.

---

## Design Decisions

These are the resolved questions from the pre-build research phase. All are
implemented. None are open.

---

### R1 — Similarity Function Split

**Question:** Should Generate, Attention, and Propagation all use the same
similarity function?

**Resolution:** Split by operation semantics.

`Generate` uses **Cosine**. Prototype matching is about direction — semantic
alignment — not energy. A high-energy prototype pointing the wrong direction
must not dominate generation. Cosine normalises out magnitude, leaving only
directional agreement.

`Attention` and `Propagation` use **Dot**. Field dynamics need magnitude
influence. A high-energy signal aligned with the query should propagate more
aggressively than a weak signal with the same direction. Dot encodes both:
`a · b = |a||b|cosθ`. The magnitude factor is a feature, not noise.

```
Generate:    Cosine(query.Vector, prototype.Vector)   → [-1, 1] angular
Attention:   Dot(query.Vector, signal.Vector) × Energy → magnitude-scaled
Propagation: Dot(sᵢ.Vector, sⱼ.Vector)               → magnitude-scaled
```

---

### R2 — Field Constants

**Question:** What are safe defaults for α (propagation), λ (decay), ε (convergence)?

**Resolution:** Derived from the stability condition, validated for N ≤ 15.

Stability requires: `λ > α × (N − 1) × μ`

For N=15 signals with μ=0.05 (realistic: k=20 in D=10,000 produces very sparse
overlap), the constraint is `λ > 0.1 × 14 × 0.05 = 0.07`. The defaults give
λ/rhs = 0.3/0.07 = 4.3× margin. Benchmarks confirm convergence in under 50
ticks under these conditions.

```
α = 0.1    propagation coefficient
λ = 0.3    decay rate
ε = 1e-4   convergence threshold
MaxTicks = 50
StableN  = 15
```

Behaviour for N > 15 is uncharacterised. Centrix emits a warning on field
construction when N exceeds this bound. The warning is not a panic — the caller
decides how to respond.

---

### R3 — Compose Confidence Rule

**Question:** How should Compose combine the confidence of two signals?

**Resolution:** Adaptive — mode is caller-supplied via `ComposeMode`.

```
Independent → 1 − (1 − cA)(1 − cB)   OR-combination: accumulates distinct evidence
Correlated  → max(cA, cB)             Max: prevents inflation from redundant evidence
```

The caller knows whether two convergent paths represent independent sources or
the same evidence processed differently. Centrix cannot infer this from vectors
alone — cosine similarity is a hint, not a rule. The `ComposeMode` parameter
makes the intent explicit and auditable in the Trace.

---

### R4 — Confidence Gate Thresholds

**Question:** Under what conditions should confidence update?

**Resolution:** Four-condition gate, grounded in sparse vector baseline math.

Baseline cosine for two random sparse vectors (k=10, D=10,000):
`√(k/D) ≈ 0.032`. Meaningful match = 5× above noise floor.

```
minMatchScore  = 0.15   5× noise floor — excludes spurious matches
minProtoWeight = 0.30   reliable prototype threshold
α_confidence   = 0.10   conservative update rate
```

Gate: all four conditions must hold.
1. `signal.Vector` is non-empty
2. `node` string is non-empty
3. `matchScore > 0.15`
4. `protoWeight > 0.30`

When the gate fails, confidence is unchanged but the Step is still appended
with `ConfidenceBefore == ConfidenceAfter`. A gated update is information —
the learning system distinguishes "confidence was blocked" from "confidence
moved to the same value."

Formula when gate passes:
```
confidence_new = confidence_old + 0.10 × (matchScore × protoWeight − confidence_old)
```

---

### R5 — Trace Bounds

**Question:** How large can a Trace grow, and how is concurrency handled?

**Resolution:** Cap of 64, sliding window, no mutex.

Reasoning chains rarely exceed 10–20 steps. 64 covers 99% of expected usage
while bounding memory at ~5KB per Signal trace. Oldest steps are dropped when
the cap is reached; the most recently appended step is never dropped. On
Compose, both traces are merged before the Compose step is appended — the
Compose step is always the final entry.

No mutex on Trace. A Signal is owned by one goroutine at a time. Concurrent
access to a single Signal is a caller error, not a condition Centrix defends
against. Adding synchronisation would penalise correct usage to accommodate
incorrect usage. Tested with `-race`.

---

### R6 — Jaccard Variant

**Question:** Should Jaccard use weights or binary presence?

**Resolution:** Binary (presence-only) in v0.1.

```
Jaccard(a, b) = |features(a) ∩ features(b)| / |features(a) ∪ features(b)|
```

Intersection and union over active FeatureIndexes (non-zero = present). Weights
are deliberately ignored — Jaccard measures structural overlap, not magnitude
agreement. Weighted Jaccard deferred to v0.2 pending a use case that requires it.

---

### R7 — Signal Identity

**Question:** Is a Signal a value or an identity? What is its scope? How does it
relate to nodes and execution?

**Resolution:** Four sub-decisions, all implemented.

**Value, not identity.** `Signal` is returned by value, never by pointer.
Tier 2 operations take a Signal and return a new Signal — the input is
unchanged. Determinism holds by construction: same inputs to same operations
always produce the same output.

**Runtime scope.** Signals do not cross run boundaries. A Signal is created at
execution start and discarded at run end. Prototypes persist; Signals do not.
Confidence starts at a caller-supplied value each run — never inherited.

**Evolving state.** One Signal per execution thread. A node receives the current
Signal — the accumulated state of all reasoning so far — transforms it, and
returns the next state. Nodes are transformations. The Signal is what is being
transformed.

**Trace on Signal.** The Trace belongs to the Signal, not to the execution
context. A node can inspect the full history of how the Signal it receives was
built. On Compose, both traces merge — full auditability of both reasoning paths
is preserved.

---

### R8 — Feature Space Design

**Question:** What does a FeatureIndex represent? Who assigns them? Static or dynamic?

**Resolution:** Concept dimensions, global namespace, caller-assigned, static in v0.1.

A FeatureIndex represents a **concept dimension**, not a token or surface string.
Two tokens meaning the same thing map to the same FeatureIndex. Similarity is
semantic — FeatureIndex 1201 is `physics.gravity` everywhere, across every
Signal and Prototype in every run.

The namespace is global — one uint32 space for the entire system. Cross-domain
similarity is real when concepts genuinely overlap. Callers use ID range
conventions to avoid collisions; Centrix documents the pattern but does not
enforce it.

The Registry (`centrix/registry`) maps concept names to stable uint32 IDs.
It is authoring infrastructure, not a runtime constraint. Callers are not
required to use it.

The feature space is static in v0.1. No new FeatureIndexes are introduced at
runtime. Dynamic feature discovery is deferred to v0.2.

---

## Resolved Constants — Authoritative Table

These values are implemented. A change requires updating this table first.

| Decision | Value | Source |
|----------|-------|--------|
| Generate similarity | Cosine | R1 |
| Attention score | `Dot × Energy` | R1 |
| Propagation similarity | Dot | R1 |
| Propagation coefficient α | 0.1 | R2 |
| Decay rate λ | 0.3 | R2 |
| Convergence threshold ε | 1e-4 | R2 |
| Max stabilisation ticks | 50 | R2 |
| Field stability bound N | 15 | R2 |
| Compose mode | Caller-supplied `ComposeMode` | R3 |
| Correlated threshold | 0.5 cosine | R3 |
| Min match score | 0.15 | R4 |
| Min prototype weight | 0.30 | R4 |
| Confidence learning rate | 0.10 | R4 |
| Trace cap | 64, sliding window | R5 |
| Trace concurrency | None — sequential ownership | R5 |
| Jaccard type | Binary, presence-only | R6 |
| Signal semantics | Value, runtime-scoped, evolving state | R7 |
| Feature space | Concept dimensions, global, static v0.1 | R8 |
