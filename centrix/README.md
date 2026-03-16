# centrix

Sparse signal mathematics library for Go.

Defines the primitives, algebra, and field dynamics that power reasoning and
generation in systems built on top of it. Centrix has no knowledge of what
builds on it — it is a pure mathematical layer.


## Status

| Phase | Package | Description | Status |
|-------|---------|-------------|--------|
| 1 | `core` | Types — FeatureIndex, SparseVector, Action, Step, Trace, Signal, Prototype, ComposeMode | ✅ Complete |
| 2 | `core` | Tier 1 algebra — Dot, Cosine, Jaccard, Merge, Normalize, Filter, Energy | 🔲 Next |
| 3 | `core` | Tier 2 signal ops — Generate, Compose, Attenuate, FilterSignal | 🔲 Pending |
| 4 | `field` | Field dynamics — Propagate, Decay, Stabilize, Attention | 🔲 Pending |
| 5 | `registry` | Feature Registry — concept name → FeatureIndex | 🔲 Pending |

## Docs

See [`docs/CONCEPTS.md`](./docs/CONCEPTS.md) for the full mental model,
algebra specification, invariants, and authoring model.
