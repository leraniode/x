package core

// signal_ops.go — Tier 2: stateful Signal transformations.
//
// Every function here:
//   1. Takes one or more Signals (by value — no mutation of input)
//   2. Applies a mathematical transformation
//   3. Appends a Step to the output Signal's Trace
//   4. Updates Confidence if the four gate conditions are met
//   5. Returns a new Signal
//
// The node parameter is the string identity of what called the operation.
// It is required for Step attribution and for confidence gate condition 2.
// An empty node string disables the confidence update for that call —
// the Step is still appended, ConfidenceBefore == ConfidenceAfter.
//
// Confidence gate (from CONCEPTS §6, R4):
//   All four must be true for confidence to update:
//   1. Signal.Vector is non-empty
//   2. node is non-empty
//   3. matchScore > minMatchScore (0.15)
//   4. protoWeight > minProtoWeight (0.30)
//
//   Formula: confidence_new = confidence_old + αConf × (matchScore × protoWeight − confidence_old)

// Validated constants (R4).
const (
	minMatchScore  = 0.15
	minProtoWeight = 0.30
	αConf          = 0.10
)

// ─── Generate ─────────────────────────────────────────────────────────────────

// Generate produces a new Signal enriched with features from proto that are
// absent in s, scaled by the Cosine similarity between s and the prototype.
//
// Only prototype features where abs(weight) > θ and the feature is absent from
// s.Vector are emitted. The output weight for each such feature is:
//
//	protoWeight × Cosine(s.Vector, proto.Vector)
//
// Cosine is used — not Dot — because prototype matching must prioritise angular
// alignment. A high-energy misaligned prototype must not dominate generation.
//
// If Cosine similarity ≤ 0 (opposite or orthogonal direction), no features are
// generated: the prototype does not semantically align with the query signal.
//
// The returned Signal carries s's full history plus one Generated step.
func Generate(s Signal, proto Prototype, θ float64, node string) Signal {
	sim := Cosine(s.Vector, proto.Vector)

	// Build the output vector: start from s, add qualifying proto features.
	out := s.Vector.Clone()
	if sim > 0 {
		for f, pw := range proto.Vector {
			abs := pw
			if abs < 0 {
				abs = -abs
			}
			// Only emit features absent in s and above the weight threshold.
			if _, exists := s.Vector[f]; !exists && abs > θ {
				out[f] = pw * sim
			}
		}
	}

	confidenceBefore := s.Confidence
	confidenceAfter := updateConfidence(s.Confidence, sim, proto.Weight, node, len(s.Vector))

	result := Signal{
		Vector:     out,
		Confidence: confidenceAfter,
		Trace:      s.Trace.clone(),
	}
	result.Trace.Add(Step{
		Node:             node,
		Action:           Generated,
		Value:            sim,
		ConfidenceBefore: confidenceBefore,
		ConfidenceAfter:  confidenceAfter,
	})
	return result
}

// ─── Compose ──────────────────────────────────────────────────────────────────

// Compose merges two Signals into a single higher-level Signal.
//
// Vector: Merge(a.Vector, b.Vector) — union with weight summation on shared features.
//
// Confidence: determined by mode.
//   Independent → OR-combination: 1 − (1 − cA)(1 − cB)
//   Correlated  → Max: max(cA, cB)
//
// Trace: a's history followed by b's history, then the Compose Step.
// The merged Trace is capped at DefaultTraceCap; the Compose Step is never dropped.
//
// The caller must supply mode — Centrix cannot infer whether the two convergent
// paths represent independent or correlated evidence.
func Compose(a, b Signal, mode ComposeMode, node string) Signal {
	mergedVector := Merge(a.Vector, b.Vector)

	var composedConf float64
	switch mode {
	case Correlated:
		composedConf = a.Confidence
		if b.Confidence > composedConf {
			composedConf = b.Confidence
		}
	default: // Independent
		composedConf = 1 - (1-a.Confidence)*(1-b.Confidence)
	}
	composedConf = clampUnit(composedConf)

	mergedTrace := a.Trace.merge(b.Trace)
	mergedTrace.Add(Step{
		Node:             node,
		Action:           Composed,
		Value:            mode,
		ConfidenceBefore: (a.Confidence + b.Confidence) / 2, // pre-compose average for audit
		ConfidenceAfter:  composedConf,
	})

	return Signal{
		Vector:     mergedVector,
		Confidence: composedConf,
		Trace:      mergedTrace,
	}
}

// ─── Attenuate ────────────────────────────────────────────────────────────────

// Attenuate decays all feature weights in s by factor λ:
//
//	w_new = w_old × (1 − λ)
//
// λ is clamped to [0.0, 1.0]. λ=0 is a no-op. λ=1 zeroes all weights.
// Features that decay to exactly zero are removed to preserve sparsity (Invariant 3).
//
// Confidence is not updated by Attenuate — decay is a structural operation,
// not a knowledge-matching event. The confidence gate does not apply here.
// ConfidenceBefore == ConfidenceAfter in the appended Step.
func Attenuate(s Signal, λ float64, node string) Signal {
	if λ < 0 {
		λ = 0
	}
	if λ > 1 {
		λ = 1
	}

	factor := 1 - λ
	out := make(SparseVector, len(s.Vector))
	for f, w := range s.Vector {
		nw := w * factor
		if nw != 0 {
			out[f] = nw
		}
	}

	result := Signal{
		Vector:     out,
		Confidence: s.Confidence,
		Trace:      s.Trace.clone(),
	}
	result.Trace.Add(Step{
		Node:             node,
		Action:           Attenuated,
		Value:            λ,
		ConfidenceBefore: s.Confidence,
		ConfidenceAfter:  s.Confidence,
	})
	return result
}

// ─── FilterSignal ─────────────────────────────────────────────────────────────

// FilterSignal removes features from s whose absolute weight is below θ.
//
// Delegates to Tier 1 Filter for the vector transformation. Wraps the result
// in a new Signal with a Filtered Step appended.
//
// Confidence is unchanged — filtering is a structural pruning operation, not a
// knowledge-matching event. ConfidenceBefore == ConfidenceAfter in the Step.
func FilterSignal(s Signal, θ float64, node string) Signal {
	out := Filter(s.Vector, θ)

	result := Signal{
		Vector:     out,
		Confidence: s.Confidence,
		Trace:      s.Trace.clone(),
	}
	result.Trace.Add(Step{
		Node:             node,
		Action:           Filtered,
		Value:            θ,
		ConfidenceBefore: s.Confidence,
		ConfidenceAfter:  s.Confidence,
	})
	return result
}

// ─── Propagate ────────────────────────────────────────────────────────────────

// Propagate absorbs energy from proto into s, weighted by their Dot similarity.
//
// This is Signal-level propagation — one Signal drawing energy from one Prototype.
// Full field propagation (multiple signals interacting) lives in the field package.
//
// The energy transfer formula:
//
//	w_new[f] = w_old[f] + α × Dot(s.Vector, proto.Vector) × proto.Vector[f]
//
// for all features f in proto.Vector. α = 0.1 (validated constant, R2).
// Dot is used — magnitude influence is required here; high-energy prototypes
// should propagate more aggressively than direction alone would imply.
//
// Confidence updates through the standard gate using Dot as the match score.
// A zero or negative Dot means no semantic alignment — no energy transfer,
// no confidence update, Step still appended.
func Propagate(s Signal, proto Prototype, α float64, node string) Signal {
	similarity := Dot(s.Vector, proto.Vector)

	out := s.Vector.Clone()
	if similarity > 0 {
		for f, pw := range proto.Vector {
			out[f] += α * similarity * pw
		}
		// Remove any features that decayed to zero via negative summation.
		for f, w := range out {
			if w == 0 {
				delete(out, f)
			}
		}
	}

	confidenceBefore := s.Confidence
	confidenceAfter := updateConfidence(s.Confidence, similarity, proto.Weight, node, len(s.Vector))

	result := Signal{
		Vector:     out,
		Confidence: confidenceAfter,
		Trace:      s.Trace.clone(),
	}
	result.Trace.Add(Step{
		Node:             node,
		Action:           Propagated,
		Value:            similarity,
		ConfidenceBefore: confidenceBefore,
		ConfidenceAfter:  confidenceAfter,
	})
	return result
}

// ─── Confidence gate ──────────────────────────────────────────────────────────

// updateConfidence applies the confidence update rule from CONCEPTS §6 (R4).
// Returns the updated confidence if all four gate conditions are met,
// otherwise returns confidenceOld unchanged.
//
// Gate conditions:
//  1. vectorLen > 0      — Signal has active features
//  2. node != ""         — Step is attributed to an identified caller
//  3. matchScore > 0.15  — knowledge match exceeds noise floor
//  4. protoWeight > 0.30 — prototype is sufficiently trusted
//
// Formula when gate passes:
//
//	c_new = c_old + αConf × (matchScore × protoWeight − c_old)
func updateConfidence(confidenceOld, matchScore, protoWeight float64, node string, vectorLen int) float64 {
	if vectorLen == 0 || node == "" || matchScore <= minMatchScore || protoWeight <= minProtoWeight {
		return confidenceOld
	}
	target := matchScore * protoWeight
	updated := confidenceOld + αConf*(target-confidenceOld)
	return clampUnit(updated)
}
