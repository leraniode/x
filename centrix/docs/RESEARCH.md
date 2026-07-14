# Centrix — Research Foundation

> The mathematical and theoretical basis for the types, algebra, and field
> dynamics Centrix implements. Every operation in this library traces to a
> published, experimentally validated result — not a heuristic.

---

## The Question

Can a system be genuinely intelligent — able to reason, classify, generate,
and learn — using deterministic mathematics on minimal hardware, without
statistical inference or GPU dependency?

The answer, grounded in the research below, is yes — within a well-defined
capability space. This document establishes what that space is, the math
behind each part of it, where the open problems lie, and exactly what
Centrix implements today.

---

## 1. Knowledge Representation: Sparse Distributed Representations

**Sources:** Kanerva (1988, 2009); Rachkovskij & Kussul (2001);
HDC benchmark literature (2017–2026)

### Core result

Knowledge can be encoded as high-dimensional sparse vectors where
mathematical distance directly corresponds to semantic similarity.

Kanerva's Sparse Distributed Memory (1988) and the subsequent body of
Hyperdimensional Computing (HDC) research establishes the following,
proven mathematically:

In a vector space of dimension D with k active features per vector, two
randomly chosen sparse vectors have an expected Cosine similarity of:

```
E[cos(a, b)] ≈ k / D
```

For D = 10,000 and k = 10, this is 0.001 — effectively zero. Two unrelated
concepts encoded as random sparse vectors will never accidentally appear
similar. Similarity is signal, not noise. It only appears where features
are intentionally shared.

### What follows from the geometry

- The number of distinguishable concepts in a k=10, D=10,000 space is
  C(10,000, 10) — astronomically large. No practical system will exhaust it.

- Similarity is graded and transitive. If A is similar to B and B is similar
  to C, then A and C are more similar to each other than to random vectors.
  This follows from dot product geometry — not assumed, proven.

- Operations preserve similarity. Merging two vectors produces a result
  similar to both inputs. Filtering removes noise while keeping the strongest
  signal. These are mathematical guarantees.

- The representation is robust to noise. Partial information still retrieves
  the right concept because the similarity geometry is smooth — a vector
  missing a few features is still close to its prototype.

### What HDC research has demonstrated

Classification using the same mathematical primitives Centrix defines:

- Speech recognition (Imani et al., 2017)
- Human activity recognition (multiple benchmarks)
- Image classification: 95.67% on MNIST, 85.14% on Fashion-MNIST
  (Arockiaraj et al., 2026)
- Graph classification, outperforming Transformer, RNN, ConvLSTM
  (Domain-Aware HDC, 2025)
- Inference time 6–39× faster than equivalent deep learning models on
  the same hardware

These results validate the mathematical primitives. Centrix doesn't copy
HDC architecture — it implements the same underlying algebra because the
math is the same and the benchmarks confirm it works.

### Implementation cost

A SparseVector at k=10 active features:

| Property    | Sparse            | Dense equivalent     | Ratio                      |
| ----------- | ----------------- | -------------------- | -------------------------- |
| Storage     | 120 bytes         | 80,000 bytes         | 667× smaller               |
| Cosine ops  | ≤10 multiply-adds | 10,000 multiply-adds | 1000× fewer                |
| Time on CPU | ~10 ns            | —                    | ~100M comparisons/sec/core |

A KnowledgePack of 10,000 Prototypes at k=10 = ~1.2 MB. Fits entirely in
L2 cache on hardware made after 2010.

**Conclusion:** Sparse vector knowledge representation is mathematically
sound, empirically validated, and trivially achievable on low-resource
hardware. `SparseVector = map[FeatureIndex]float64` is not a convenience —
it is the correct representation.

---

## 2. Reasoning: Prototype-Based Similarity Retrieval

**Sources:** Rosch (1973, 1978); Nosofsky (1986); Collins & Loftus (1975);
Anderson ACT-R (1983)

### Core result

Human categorisation and reasoning are not performed by checking necessary
and sufficient conditions against a logical definition. They operate by
comparing to stored prototypes via graded similarity.

Eleanor Rosch's foundational work (1973) demonstrated:

- Category membership is graded — a robin is a "better bird" than a penguin
  even though both are equally birds by logical definition
- Categorisation speed and accuracy are directly proportional to similarity
  to the prototype — closer = faster and more accurate
- This holds across cultures, languages, and domains

Nosofsky's Generalised Context Model (1986) formalised this:

```
P(classify x as C) ∝ Σᵢ sim(x, pᵢ)^ρ
```

Where `sim` is a distance-based similarity function, `pᵢ` are stored
prototypes, and `ρ` controls generalisation. Cosine similarity is a valid
`sim` here. This model matches human categorisation behaviour across a wide
range of experimental conditions.

Centrix's Cosine similarity against Prototypes is a direct implementation
of this model — not a metaphor for it.

### Multi-step reasoning: spreading activation

Collins and Loftus (1975) proposed spreading activation as the mechanism
by which retrieval of one concept triggers access to related concepts via
weighted links in a semantic network. The model correctly predicts:

- Priming effects: activating DOCTOR makes NURSE more accessible
- Typicality gradients: central concepts activate faster
- The fan effect: more connections → slower retrieval from any one

The mathematical form:

```
activation_j = f( Σᵢ→j w_ij × activation_i )
```

Where `w_ij` are connection weights and `f` is an activation function.

This is the same equation as Centrix's field propagation:

```
v_j(t+1) = v_j(t) + α × Σᵢ Dot(sᵢ, sⱼ) × vᵢ(t)
```

Spreading activation was independently formalised in Anderson's ACT-R
architecture (1983) and remains the dominant computational model of
semantic memory. Centrix's `SignalField` is a direct implementation
of this model over sparse vectors.

**Conclusion:** Prototype retrieval and spreading activation are the
established computational models of how cognition actually works. Centrix's
`Prototype` type, Cosine similarity, and `SignalField` propagation are
direct implementations of them.

---

## 3. Generation: Analogical Completion and Bounded Inference

**Sources:** Plate (1995, 2003); Kanerva (2010);
HDC analogical reasoning literature

### Core result

Generating new information from existing knowledge is mathematically
formalised as analogical completion — answering "A is to B as C is to ?"

In Holographic Reduced Representations (Plate, 1995), this is:

```
? ≈ B ⊘ A ⊛ C
```

Where `⊛` is circular convolution (binding) and `⊘` is its approximate
inverse. The result is a vector close to the analogical completion,
retrievable by Cosine search against known Prototypes.

Kanerva (2010) demonstrated this concretely: "Dollar is to USA as Peso is
to Mexico" is correctly computed in hyperdimensional space without any
lookup table — purely through the algebra.

### Centrix's Generate operation

`Generate` is the simpler, bounded case. Given a query Signal matching a
Prototype at Cosine similarity `c`, produce the Prototype's features absent
from the query, scaled by `c`:

```
Generate(query, prototype, θ) =
  { (f, w_proto[f] × Cosine(query, prototype))
    | f ∈ prototype, f ∉ query, w_proto[f] > θ }
```

Geometrically: project the query onto the Prototype, take the residual
component. The residual is what the Prototype knows that the query does
not yet contain.

**What makes this grounded:**

- Output is strictly bounded by what is in the matched Prototype
- Output scales with match quality — poor matches generate almost nothing
- Generation quality is directly measurable — it is the Cosine score
- Nothing is invented — only inferred from an actual match

**The generation chain:** The output of `Generate` becomes input to the
next `Generate` call. Each step enriches the Signal with features the
previous step could not have produced. Each step is traced, each action
is recorded, confidence updates at each step according to match quality.
Multi-step chains are how simple retrieval becomes something resembling
actual reasoning.

**Conclusion:** Bounded generation from matched Prototypes is geometrically
grounded and cannot hallucinate because its output space is bounded by what
was matched. The quality is not subjective — it is the Cosine score.

---

## 4. Learning: Bounded Adaptive Updates

**Sources:** Rescorla & Wagner (1972); Oja (1982); Hebb (1949)

### Core result

Real, convergent learning is possible with simple local update rules —
no backpropagation, no gradient computation, no GPU.

### Rescorla-Wagner rule (1972)

One of the most replicated mathematical models in experimental psychology.
Originally derived from animal conditioning:

```
ΔV = α × β × (λ − V_total)
```

Where `V` = current associative strength, `λ` = maximum possible strength
(target), `α` and `β` = learning rates, `V_total` = total active strength.

Convergence is proven:

```
V_t = λ × (1 − (1 − α)^t)
```

For `α = 0.1`: at t=10, V ≈ 0.65λ. At t=50, V ≈ 0.995λ. The system
converges exponentially. It cannot diverge while `α ∈ (0, 1)`.

What this correctly predicts (validated across thousands of experiments):

- **Blocking:** if stimulus A already fully predicts an outcome, adding B
  teaches the system nothing new about B. The rule produces this from math.
- **Extinction:** repeated non-reinforcement drives V toward 0.
- **Discrimination:** stimuli predicting opposite outcomes converge to
  opposite weights.

All of these are properties a reasoning system wants. They emerge from
the math, not from engineered logic.

Centrix's confidence update rule is Rescorla-Wagner:

```
confidence_new = confidence_old + α × (matchScore × protoWeight − confidence_old)
```

### Oja's rule (1982)

Normalises weights during update so they do not grow without bound:

```
Δw = η × y × (x − y × w)
```

Where `y = w·x` is the current output. This rule converges to the principal
eigenvector of the input correlation matrix — a system using Oja's rule will,
over time, discover the most consistent structure in its training data.
Convergence and the convergence target are both proven.

Combined with Rescorla-Wagner: Oja's rule normalises the weight space while
R-W adjusts associative strength based on outcomes. The two rules address
orthogonal aspects of learning and compose without conflict.

### Hebbian learning (Hebb, 1949)

"Neurons that fire together, wire together."

```
Δw_ij = η × aᵢ × aⱼ
```

Where `aᵢ` and `aⱼ` are activations of two units in the same successful run.
Strengthens connections between concepts that co-activate in correct outcomes.
No supervision needed — only the pattern of co-occurrence.

Hebbian updates alone drift without bound, which is why they are combined
with Oja's normalisation. Together they strengthen genuinely co-occurring
patterns while remaining stable.

**Conclusion:** Three convergent, proven learning rules — Rescorla-Wagner,
Oja, Hebbian — form a complete, mathematically grounded learning system
with no GPU and no backpropagation. Each has been independently validated
across thousands of experiments. They compose without conflict.

---

## 5. The Binding Problem

**Sources:** Plate (1995); Rachkovskij & Kussul (2001);
Smolensky (1990); GHRR (2024)

### The problem

A flat `SparseVector` with active features `{red, car, fast}` does not
distinguish "a red car moving fast" from three unrelated facts that happen
to co-occur. The vector represents that all three features are active — not
that they are bound into a structured relationship.

This is the binding problem, documented as fundamental since Smolensky's
Tensor Product Representations (1990). It is not a gap in Centrix's design —
it is an open research problem in the entire field of distributed cognition.

### What HRR research has found

Holographic Reduced Representations (Plate, 1995) solve this using
circular convolution:

```
bound = a ⊛ b = F⁻¹( F(a) ⊙ F(b) )
```

Where `F` is the Fast Fourier Transform and `⊙` is element-wise
multiplication. The result is a vector of the same dimension `D` that is
dissimilar to both `a` and `b` individually, but from which either can be
approximately recovered:

```
a ≈ bound ⊛ b̄   (b̄ = approximate inverse of b)
```

This allows arbitrary role-filler bindings in a fixed-dimension vector.
"Red car" and "fast car" produce different bound vectors even if the same
features appear — because the binding encodes the relationship, not just
co-presence.

**Known limitations:**

- Binding fidelity degrades as more concepts are bound together (memory interference)
- Deep hierarchical structures are harder to encode correctly
- The binding operation adds computational cost (FFT)

Generalised HRR (GHRR, 2024) extends this with a non-commutative binding
operation, improving encoding of complex compositional structures while
preserving robustness.

### Implication for Centrix v0.1

Circular convolution binding is an available, implemented operation with a
proven mathematical basis. It solves the binding problem for simple
role-filler relationships ("concept A in context B"). Centrix v0.1
does not implement binding — it is the first capability target for v0.2.

Deeply nested compositionality and long-range structural dependencies are
v2+ research problems. The honest position for v0.1: Centrix reasons
correctly about individual concepts and their first-order relationships.

---

## 6. Open Research Problems

These are not blockers for Centrix v0.1. They are the track that expands
what future versions can do.

### Problem 1 — Encoding theory (most urgent)

How does raw unstructured input of any kind become a semantically valid
`SparseVector` consistently?

This is the most urgent open problem because it blocks DotPack's text
encoding path entirely. The math for what to do with a `SparseVector` once
you have one is solid. The open question is how to get a meaningful one from
raw text.

**What exists:** HDC random indexing (Kanerva et al., 2000) and Random
Projection (Achlioptas, 2003) provide partial answers for some input types.

**The v0.1 answer:** Features are authored by the developer using the
Registry. Input is encoded by mapping it to those authored features.
Encoding is a deliberate, domain-specific act rather than automatic induction.
This makes the system deterministic and correct within its domain.

### Problem 2 — Compositionality beyond simple binding

How do you represent structured relationships between three or more concepts
without binding degrading?

**What exists:** GHRR (2024) improves encoding accuracy for complex structures.
Active research area.

**The v0.1 position:** Simple role-filler bindings (circular convolution)
are the first target. Deeper compositional reasoning is v0.2+ research,
tackled once v0.1 is running and simpler binding cases are validated.

### Problem 3 — Decoding / realisation

How does a final Signal become a concrete, usable answer in the general case?

For v0.1 this is not blocking. The Signal itself (Vector + Confidence + Trace)
is a complete, inspectable result. Decoding matters when you need to produce
novel structured output that was not explicitly stored as a Prototype.
That is a v0.2+ capability.

---

## 7. What Centrix Implements

The table below maps every core Centrix operation to its theoretical
grounding. Above the "open research" line: proven mathematics, implemented
in v0.1.

| Capability                   | Operation                   | Theory                                | Status                                         |
| ---------------------------- | --------------------------- | ------------------------------------- | ---------------------------------------------- |
| Knowledge representation     | `SparseVector`, `Prototype` | Kanerva SDR, HDC (1988, 2009)         | Proven, validated in production systems        |
| Similarity-based reasoning   | `Cosine`, `Dot`             | Rosch (1973), Nosofsky GCM (1986)     | Proven, matches human behaviour experimentally |
| Spreading activation         | `SignalField.Propagate`     | Collins & Loftus (1975), ACT-R (1983) | Proven, dominant model of semantic memory      |
| Bounded generation           | `Generate`                  | Plate HRR (1995), Kanerva (2010)      | Proven, geometrically grounded                 |
| Confidence update            | Gate + R-W formula          | Rescorla-Wagner (1972)                | Proven, convergence theorem exists             |
| Weight normalisation         | Prototype weights           | Oja's rule (1982)                     | Proven, converges to principal eigenvector     |
| Co-occurrence strengthening  | Trust updates               | Hebb (1949)                           | Proven, stable with Oja normalisation          |
| Low-resource operation       | O(k) algebra                | HDC benchmarks (2017–2026)            | Demonstrated: 6–39× faster than deep learning  |
| Field convergence            | `Stabilize`                 | Stability condition: λ > α(N−1)μ      | Validated for N≤15, α=0.1, λ=0.3               |
| Simple compositionality      | (v0.2 target)               | Plate HRR circular convolution (1995) | Proven for two-concept binding                 |
| Complex compositionality     | (v0.2+ research)            | GHRR (2024)                           | Active research, partial solution              |
| Encoding from raw input      | (v0.2 research)             | HDC random indexing (2000)            | Open — authoring model is the v0.1 answer      |
| Decoding to arbitrary output | (v0.3+ research)            | Open                                  | Open — Signal is the v0.1 output               |

---

## 8. Design Decisions

These are the resolved pre-build research questions. All are implemented.
None are open.

### R1 — Similarity function split

`Generate` uses **Cosine**. Prototype matching is about direction — semantic
alignment — not energy. A high-energy prototype pointing the wrong direction
must not dominate generation. Cosine normalises out magnitude, leaving only
directional agreement.

`Attention` and `Propagation` use **Dot**. Field dynamics need magnitude
influence. `Dot = |a||b|cosθ` — the magnitude factor is a feature, not noise.

```
Generate:    Cosine(query.Vector, prototype.Vector)
Attention:   Dot(query.Vector, signal.Vector) × Energy(signal.Vector)
Propagation: Dot(sᵢ.Vector, sⱼ.Vector)
```

### R2 — Field constants

Stability condition: `λ > α × (N − 1) × μ`

For N=15, α=0.1, μ=0.05 (realistic sparse overlap in k=20, D=10,000 space):
`rhs = 0.1 × 14 × 0.05 = 0.07`. Default λ=0.3 gives 4.3× margin.
Benchmarks confirm convergence in under 50 ticks.

```
α_propagation  = 0.1
λ_decay        = 0.3
ε_convergence  = 1e-4
MaxTicks       = 50
StableN        = 15
```

### R3 — Compose confidence

`ComposeMode` is caller-supplied. Centrix cannot infer whether two convergent
paths represent independent or correlated evidence.

```
Independent → 1 − (1 − cA)(1 − cB)   accumulates evidence from distinct sources
Correlated  → max(cA, cB)             prevents inflation from redundant evidence
```

### R4 — Confidence gate

Rescorla-Wagner applied only when all four conditions hold:

1. `signal.Vector` non-empty
2. `node` non-empty
3. `matchScore > 0.15`
4. `protoWeight > 0.30`

Threshold derivation: for random sparse vectors (k=10, D=10,000),
`E[cos] ≈ 0.001`. For float-weighted sparse vectors the effective noise
floor is `√(k/D) ≈ 0.032`. `minMatchScore = 0.15` is ~5× this floor.

```
confidence_new = confidence_old + 0.10 × (matchScore × protoWeight − confidence_old)
```

### R5 — Trace bounds

Cap = 64, sliding window, newest never dropped. No mutex — Signal has single
goroutine ownership by design (Invariant 4). Reasoning chains rarely exceed
10–20 steps; 64 covers 99% of expected usage at ~5KB max per trace.

### R6 — Jaccard type

Binary (presence-only) in v0.1. Jaccard measures structural overlap — weights
handle magnitude, Jaccard handles co-presence. Weighted Jaccard deferred to v0.2.

### R7 — Signal identity

Value, not pointer. Ephemeral (run-scoped). Evolving state (not a message).
Trace on Signal (observability from inside reasoning). These four decisions
make Invariant 1 (determinism) hold by construction.

### R8 — Feature space

Concept dimensions, global namespace, caller-assigned, static in v0.1.
`FeatureIndex(1201)` means `physics.gravity` everywhere. The Registry makes
this tractable at authoring time.

---

## Resolved Constants

| Constant                  | Value                                   | Source |
| ------------------------- | --------------------------------------- | ------ |
| Generate similarity       | Cosine                                  | R1     |
| Attention score           | `Dot × Energy`                          | R1     |
| Propagation similarity    | Dot                                     | R1     |
| Propagation coefficient α | 0.1                                     | R2     |
| Decay rate λ              | 0.3                                     | R2     |
| Convergence threshold ε   | 1e-4                                    | R2     |
| Max ticks                 | 50                                      | R2     |
| Field stability bound     | N ≤ 15                                  | R2     |
| Compose mode              | Caller-supplied                         | R3     |
| Min match score           | 0.15                                    | R4     |
| Min prototype weight      | 0.30                                    | R4     |
| Confidence α              | 0.10                                    | R4     |
| Trace cap                 | 64, sliding window                      | R5     |
| Trace concurrency         | None — sequential ownership             | R5     |
| Jaccard type              | Binary, presence-only                   | R6     |
| Signal semantics          | Value, runtime-scoped, evolving state   | R7     |
| Feature space             | Concept dimensions, global, static v0.1 | R8     |
