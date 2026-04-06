package core

import "math"

// algebra.go — Tier 1: pure SparseVector mathematics.
//
// These functions touch no Signal, no Trace, no Confidence.
// They are the mathematical substrate everything else is built on.
//
// Operation-to-similarity mapping (from R1 / CONCEPTS §5):
//   Generate  → Cosine  (angular alignment; high-energy misaligned proto must not dominate)
//   Attention → Dot     (magnitude influence required; high-energy signals surface first)
//   Propagate → Dot     (same reasoning as Attention)

// ─── Energy ───────────────────────────────────────────────────────────────────

// Energy returns the L1 norm of v — the sum of absolute feature weights.
// This is the activation strength of a vector.
//
// Energy is always derived, never stored. Invariant 10 holds by construction:
// there is one source of truth for activation strength.
func Energy(v SparseVector) float64 {
	var e float64
	for _, w := range v {
		if w < 0 {
			e -= w
		} else {
			e += w
		}
	}
	return e
}

// ─── Dot ──────────────────────────────────────────────────────────────────────

// Dot returns the dot product of a and b: Σ aᵢ × bᵢ over shared features.
//
// Magnitude-sensitive — a high-weight feature in both vectors dominates the
// result. Use Dot when energy should influence the score: attention ranking,
// field propagation. Use Cosine when direction matters more than magnitude.
//
// Runs over the smaller of the two maps for O(min(|a|, |b|)) performance.
func Dot(a, b SparseVector) float64 {
	// Always iterate the smaller map — fewer iterations, same result.
	if len(a) > len(b) {
		a, b = b, a
	}
	var sum float64
	for f, wa := range a {
		if wb, ok := b[f]; ok {
			sum += wa * wb
		}
	}
	return sum
}

// ─── Cosine ───────────────────────────────────────────────────────────────────

// Cosine returns the cosine similarity of a and b: (a·b) / (‖a‖ × ‖b‖).
// Result is in [-1.0, 1.0].
//
// Direction-sensitive — two vectors that point in the same concept direction
// score 1.0 regardless of their magnitudes. Use Cosine when semantic alignment
// matters more than energy: prototype matching, generation, compose gating.
//
// Returns 0.0 if either vector has zero L2 norm (avoids divide-by-zero).
func Cosine(a, b SparseVector) float64 {
	normA := l2norm(a)
	normB := l2norm(b)
	if normA == 0 || normB == 0 {
		return 0
	}
	return Dot(a, b) / (normA * normB)
}

// ─── Jaccard ──────────────────────────────────────────────────────────────────

// Jaccard returns the binary Jaccard similarity of a and b:
//
//	|a ∩ b| / |a ∪ b|
//
// Result is in [0.0, 1.0]. This is a presence-only operation in v0.1:
// a feature is either active or absent; its weight does not affect the score.
// This makes Jaccard appropriate for binary feature spaces where co-occurrence
// matters and magnitude does not.
//
// Returns 0.0 if both vectors are empty (no union = undefined → 0 by convention).
func Jaccard(a, b SparseVector) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	var intersection int
	// Count features present in both; iterate smaller map.
	small, large := a, b
	if len(a) > len(b) {
		small, large = b, a
	}
	for f := range small {
		if _, ok := large[f]; ok {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	return float64(intersection) / float64(union)
}

// ─── Merge ────────────────────────────────────────────────────────────────────

// Merge returns the union of a and b.
// Features present in only one vector are copied at their original weight.
// Features present in both vectors have their weights summed.
//
// Neither a nor b is modified. A new SparseVector is always returned.
func Merge(a, b SparseVector) SparseVector {
	result := make(SparseVector, len(a)+len(b))
	for f, w := range a {
		result[f] = w
	}
	for f, w := range b {
		result[f] += w // zero-initialised; adds b weight to whatever a contributed
	}
	// Remove any accidental zeros introduced by summation cancellation.
	for f, w := range result {
		if w == 0 {
			delete(result, f)
		}
	}
	return result
}

// ─── Normalize ────────────────────────────────────────────────────────────────

// Normalize returns a copy of v scaled to unit L2 norm.
//
// Use before Cosine-based comparisons when you want all vectors to have equal
// energy — direction only, no magnitude influence.
//
// Returns an empty SparseVector if v has zero L2 norm (cannot normalise a zero
// vector; dividing by zero would corrupt every weight).
func Normalize(v SparseVector) SparseVector {
	norm := l2norm(v)
	if norm == 0 {
		return make(SparseVector)
	}
	result := make(SparseVector, len(v))
	for f, w := range v {
		result[f] = w / norm
	}
	return result
}

// ─── Filter ───────────────────────────────────────────────────────────────────

// Filter returns a copy of v with all features whose absolute weight is below
// threshold θ removed.
//
// Use to prune noise from a vector before similarity scoring or generation.
// θ must be ≥ 0; a negative threshold keeps all features.
//
// Does not modify v. Returns a new SparseVector.
func Filter(v SparseVector, threshold float64) SparseVector {
	result := make(SparseVector, len(v))
	for f, w := range v {
		abs := w
		if abs < 0 {
			abs = -abs
		}
		if abs >= threshold {
			result[f] = w
		}
	}
	return result
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

// l2norm returns the L2 (Euclidean) norm of v: √(Σ wᵢ²).
// Used internally by Cosine and Normalize.
func l2norm(v SparseVector) float64 {
	var sum float64
	for _, w := range v {
		sum += w * w
	}
	return math.Sqrt(sum)
}
