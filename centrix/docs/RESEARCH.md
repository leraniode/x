# Centrix — Research Blueprint
> Status: 5/6 Resolved · R2 Partial · Ready to build

---

## R1 — Cosine vs Dot for similarity operations ✅ Resolved

**Resolution:** Split by operation type.

**Generate** uses Cosine. Prototype matching must prioritize angular alignment
over raw energy. A high-energy prototype misaligned with the query should not
dominate generation — direction defines semantic fit.

- Case A (identical direction, different magnitudes): Cosine ranks equally,
  Dot favors the larger. Cosine is correct — magnitude is post-match scaling,
  not a matching criterion.
- Case B (different directions, similar magnitudes): Cosine correctly penalizes
  misalignment regardless of scale.

**Attention and Propagation** use Dot. Field dynamics need magnitude influence —
high-energy signals should propagate more aggressively. Dot incorporates this
naturally: `a · b = |a||b|cosθ`. The magnitude factor is a feature, not noise.

```
Generate:    CosineSimilarity(query.Vector, prototype.Vector)  → [0, 1]
Attention:   DotProduct(signal.Vector, query.Vector)           → magnitude-scaled
Propagation: DotProduct(sᵢ.Vector, sⱼ.Vector)                → magnitude-scaled
```

---

## R2 — Default α, λ, ε values ⚠️ Partial

**Partial resolution:** Safe defaults derived for N ≤ 15.

Stability condition:
```
λ > α × (N − 1) × mean_cosine_similarity
```

For N=10 signals with 10% feature overlap in D=10k space,
max expected cosine ≈ 0.3. This gives:
```
λ > α × 9 × 0.3 = 2.7α
```

**Validated defaults:**
```
α = 0.1    (propagation coefficient)
λ = 0.3    (decay rate — ratio 3:1, stable for N ≤ 15)
ε = 1e-4   (convergence threshold, settles in < 50 iterations)
```

**Remaining gap:** Defaults are validated for N ≤ 15 and moderate feature
overlap. Behavior for large fields (N > 15) or high-overlap feature spaces
is uncharacterised. Acceptable for v0.1 — document the constraint explicitly
in field.go. Revisit in v0.2 with profiling data from real usage.

**Implementation note:** If a field exceeds N=15, emit a warning (not a panic).
The caller is responsible for field size — Centrix documents the bound.

---

## R3 — Confidence combination rule for Compose ✅ Resolved

**Resolution:** Adaptive combination based on signal correlation.

```
if Cosine(a.Vector, b.Vector) > 0.5:
    confidence = Max(a.Confidence, b.Confidence)
else:
    confidence = 1 − (1 − a.Confidence) × (1 − b.Confidence)
```

- Independent signals (corr < 0.5): OR-combination accumulates evidence
  from distinct sources correctly.
- Correlated signals (corr ≥ 0.5): Max prevents confidence inflation from
  redundant evidence. Two signals saying the same thing are not independent.

Threshold of 0.5 is defensible as a default. Configurable in v0.2 if needed.

---

## R4 — Confidence update thresholds ✅ Resolved

**Resolution:** Grounded thresholds derived from sparse vector baseline.

Baseline cosine for two random sparse vectors (k=10, D=10,000): `√(k/D) ≈ 0.03`.
Meaningful match = 5× above baseline.

```
minMatchScore  = 0.15   (5× noise floor)
minProtoWeight = 0.30   (reliable prototype threshold)
α_confidence   = 0.10   (conservative update rate)
```

**Update formula:**
```
confidence_new = confidence_old + α × (matchScore × protoWeight − confidence_old)
```

Applied only when ALL four conditions are met:
1. signal.Vector is non-empty
2. step.Node is non-empty string
3. matchScore > 0.15
4. protoWeight > 0.30

When any condition fails: confidence unchanged, Step still appended with
ConfidenceBefore == ConfidenceAfter. This is information — the learning system
distinguishes "confidence was gated" from "confidence moved."

---

## R5 — Trace size bounds ✅ Resolved

**Resolution:** Bounded Trace, cap = 64, sliding window (oldest dropped).

Reasoning chains rarely exceed 10–20 steps. Cap of 64 covers 99% of expected
usage while bounding memory at ~2KB per Signal trace. Caller can override cap.

**No mutex on Trace.** A Signal is owned by one goroutine at a time — passed
through nodes sequentially. Concurrent access to a single Signal is a caller
error. The mutex in the research proposal is rejected — it adds overhead for
a case that must not occur by design. Law 10 (Determinism) and Law 6
(Context-First Execution) both imply sequential signal ownership.

---

## R6 — Jaccard for weighted SparseVectors ✅ Resolved

**Resolution:** Binary Jaccard for v0.1.

```
Jaccard(a, b) = |features(a) ∩ features(b)| / |features(a) ∪ features(b)|
```

Intersection and union over active FeatureIndexes only (non-zero = present).
Weights are ignored — presence defines structural overlap; weights handle magnitude.

Returns 0 if both empty. Returns 1 if both have identical active feature sets.
Weighted Jaccard deferred to v0.2.

---

## R7 — The place and concept of a Signal ✅ Resolved

**Priority: Blocker for Phase 1 — determines the Signal type's fundamental design**

Four sub-questions resolved:

---

### R7.1 — Is a Signal a value or an identity?

**Resolution: Value.**

A Signal is not a persistent object with an identity that survives transformations.
It is a value — created, transformed, consumed. Each Tier 2 operation produces a
new Signal. The original is unchanged. Immutability is not a convention to follow
carefully — it is the definition of what a Signal is.

**Why this matters for the type:** `Generate`, `Compose`, `Attenuate`, and
`FilterSignal` all return `Signal`, not `*Signal`. There is no pointer — no shared
mutable state. The Trace accumulates by construction, not mutation.

**Why this matters for determinism:** Law 10 (Determinism) holds trivially when
Signals are values. Same inputs to same operations always produce the same output
because nothing is shared or mutated between calls.

---

### R7.2 — What is the Signal's scope?

**Resolution: Runtime scope only in v0.1. Option C.**

Signals do not cross run boundaries. A Signal is created at execution start and
discarded when the run ends. SparseVectors persist as prototypes — they are the
authored or learned knowledge. The Signal wrapping — Confidence and Trace — is
always fresh per run.

**Why this matters:** It means the Engine constructs the initial Signal from
scratch at the start of every `Run` call. There is no "resume" or "continue
from prior Signal" in v0.1. Prototypes are reusable; Signals are not.

**Corollary:** Confidence on a fresh Signal starts at a defined initial value
(0.0 or a caller-supplied value). It is not inherited from a previous run.

---

### R7.3 — What is the relationship between a Signal and a Node?

**Resolution: Signal as evolving state.**

A Signal is the accumulated state of reasoning in progress. One Signal per
execution thread. A node receives the current Signal — the full state of
reasoning so far — transforms it, and returns the next state. The Signal
grows richer as it moves through nodes.

Nodes are transformations. The Signal is what is being transformed.

**Why this matters for the type:** A node's function signature in Illygen is:

```
func(ctx Context, in Signal) Signal
```

Not `func(ctx Context, in []Signal) []Signal`. One Signal in, one Signal out.
Parallel paths that converge use `Compose` to merge two Signals into one before
continuing. The single-Signal-per-thread model holds throughout.

**Why this rules out the message model:** The message model (Signal as a
packet passing between nodes) implies nodes could receive multiple Signals
simultaneously. That requires the field to handle routing. That is the
ContextEngine's responsibility in Illygen, not Centrix's. Centrix defines
what a Signal is — Illygen's runtime handles how Signals are routed.

---

### R7.4 — Does the Signal carry its own history, or does the run carry it?

**Resolution: Trace on the Signal.**

The Trace belongs to the Signal, not to the execution context. This means a
node receiving a Signal can inspect the full history of how that Signal was
built — what actions were taken, what confidence changes occurred, which nodes
contributed. That is observability from inside the reasoning process, not just
from outside it.

**The Compose problem:** When two Signals are Composed, both carry Traces.
The resulting Signal's Trace is the merge of both, with the Compose Step
appended last:

```
result.Trace = append(a.Trace.Steps, b.Trace.Steps..., composeStep)
```

Both histories are preserved. The merge is ordered: a's history first, then
b's history, then the step that produced the composition. This is the correct
model because it preserves full auditability — you can reconstruct exactly
which two reasoning paths were combined and when.

**The cap interaction:** Trace cap of 64 (R5) applies after the merge.
If the merged Trace exceeds 64 steps, the oldest steps are dropped.
The compose step itself is always the last entry and is never dropped.

---

## R8 — Feature Space Design ✅ Resolved

**Priority: Blocker for Phase 1 — determines what FeatureIndex represents
and how SparseVectors are authored**

Four dimensions resolved:

**D1 — What does a feature represent?**
A feature represents a concept dimension, not a token or surface form.
Two tokens meaning the same thing map to the same FeatureIndex.
Similarity is semantic, not lexical. Feature 1201 = `physics.gravity`
regardless of what string was used to author it.

**D2 — Global or namespaced?**
Global. One uint32 space for the entire system. FeatureIndex 1201 means
`physics.gravity` in every Signal and Prototype in every run.
Cross-domain similarity is real when concepts genuinely overlap.
Callers use ID range conventions to avoid collisions — Centrix documents
the pattern but does not enforce it.

**D3 — Who assigns FeatureIndex values?**
Caller-defined. Centrix provides a Registry utility (`centrix/registry`)
but does not require its use. The Registry maps concept names to stable
uint32 IDs — append-only, deterministic, serializable. It is authoring
infrastructure, not a runtime constraint.

**D4 — Static or dynamic?**
Static in v0.1. The feature space is fully defined before any Signal is
created. No new FeatureIndexes are introduced at runtime. This preserves
determinism by construction. Dynamic discovery deferred to v0.2.

---

**Additional resolution from conflict analysis:**

**Energy is derived, not a field.**
`Signal.Vector.Energy()` (sum of absolute weights) is always the answer.
Energy is never stored separately on Signal. One source of truth for
activation strength. Two sources of truth would create a class of bugs
with no upside.

**Action naming conflict resolved.**
`Action` stays as defined in BUILD.md — operation-type constants on a Step:
`Generated`, `Matched`, `Propagated`, `Attenuated`, `Composed`, `Filtered`.
The execution-effect concept (`EmitToken`, `CallTool`, etc.) from external
design belongs to Illygen, not Centrix, and will be named there independently.

**float64 throughout.**
`SparseVector` uses `float64` weights. Revisit only if profiling on
constrained hardware shows a measurable impact — not before.

---

## Resolution Checklist

| ID | Question | Blocker | Target Phase | Status |
|----|----------|---------|--------------|--------|
| R1 | Cosine vs Dot — operation split | Yes | Phase 2 | ✅ Resolved |
| R2 | α, λ, ε default values | No | Phase 4 | ⚠️ Partial (N ≤ 15) |
| R3 | Compose confidence combination | No | Phase 3 | ✅ Resolved |
| R4 | Confidence update thresholds | Yes | Phase 3 | ✅ Resolved |
| R5 | Trace size bounds | No | Phase 1 | ✅ Resolved |
| R6 | Jaccard for weighted vectors | No | Phase 2 | ✅ Resolved |
| R7 | Signal place and concept | Yes | Phase 1 | ✅ Resolved |
| R8 | Feature space design | Yes | Phase 1 | ✅ Resolved |

---

## Decisions Carried Into Build

The implementation must use exactly these values. Changes require updating
this document first.

| Decision | Value | Source |
|----------|-------|--------|
| Generate similarity function | Cosine | R1 |
| Attention scoring function | Dot × energy | R1 |
| Propagation scoring function | Dot | R1 |
| Propagation coefficient α | 0.1 | R2 |
| Decay rate λ | 0.3 | R2 |
| Convergence threshold ε | 1e-4 | R2 |
| Field size warning threshold | N > 15 | R2 |
| Compose confidence rule | corr > 0.5 → Max, else OR-combination | R3 |
| Cosine correlation threshold | 0.5 | R3 |
| Min match score | 0.15 | R4 |
| Min prototype weight | 0.30 | R4 |
| Confidence learning rate | 0.10 | R4 |
| Trace cap | 64, sliding window | R5 |
| Trace concurrency model | None — sequential ownership enforced | R5 |
| Jaccard type | Binary (presence-only) | R6 |
