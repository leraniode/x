// Package field implements SignalField dynamics for Centrix.
//
// A SignalField is a collection of Signals that interact. Field dynamics are
// how associative reasoning emerges — signals that share concept dimensions
// amplify each other; signals that share nothing do not interact.
//
// This is field-level propagation. Signal-level propagation (one Signal
// absorbing energy from one Prototype) lives in core/signal_ops.go.
//
// Validated constants (R2):
//
//	α = 0.1   propagation coefficient
//	λ = 0.3   decay rate
//	ε = 1e-4  convergence threshold
//	MaxTicks = 50
//
// Stability condition:
//
//	λ > α × (N − 1) × μ
//
// where N = number of signals, μ = mean cosine similarity between signals.
// Default values satisfy this for N ≤ 15. A warning is emitted when N > 15.
// MaxTicks is a hard stop regardless of convergence.
package field

import (
	"fmt"
	"os"
	"sort"

	"github.com/leraniode/xgo/centrix/core"
)

// ─── Validated constants ───────────────────────────────────────────────────────

const (
	DefaultAlpha    = 0.1  // propagation coefficient
	DefaultLambda   = 0.3  // decay rate
	DefaultEpsilon  = 1e-4 // convergence threshold
	DefaultMaxTicks = 50   // hard stop on stabilisation
	StableN         = 15   // maximum N for which defaults are validated
)

// ─── SignalField ───────────────────────────────────────────────────────────────

// SignalField is a collection of Signals that interact through propagation
// and decay. It is the substrate for associative reasoning in Centrix.
//
// All Signals in the field share the same feature space. Cross-signal
// similarity is meaningful only if FeatureIndex semantics are consistent
// (Invariant 2).
//
// A SignalField is not safe for concurrent use. Field operations return a
// new SignalField — inputs are never mutated (value semantics, like Signal).
type SignalField struct {
	Signals  []core.Signal
	Alpha    float64 // propagation coefficient
	Lambda   float64 // decay rate
	Epsilon  float64 // convergence threshold
	MaxTicks int     // hard stop for Stabilize
}

// New creates a SignalField with the validated default constants.
func New(signals []core.Signal) SignalField {
	f := SignalField{
		Signals:  make([]core.Signal, len(signals)),
		Alpha:    DefaultAlpha,
		Lambda:   DefaultLambda,
		Epsilon:  DefaultEpsilon,
		MaxTicks: DefaultMaxTicks,
	}
	for i, s := range signals {
		f.Signals[i] = s.Clone()
	}

	if len(signals) > StableN {
		fmt.Fprintf(os.Stderr,
			"centrix/field: warning: N=%d exceeds validated stability bound (N≤%d). "+
				"Increase λ, decrease α, or ensure low inter-signal similarity. "+
				"Stability condition: λ > α×(N-1)×μ\n",
			len(signals), StableN,
		)
	}
	return f
}

// clone returns a deep copy of the SignalField.
// Used internally so field operations never mutate their receiver.
func (f SignalField) clone() SignalField {
	c := SignalField{
		Signals:  make([]core.Signal, len(f.Signals)),
		Alpha:    f.Alpha,
		Lambda:   f.Lambda,
		Epsilon:  f.Epsilon,
		MaxTicks: f.MaxTicks,
	}
	for i, s := range f.Signals {
		c.Signals[i] = s.Clone()
	}
	return c
}

// ─── Propagate ────────────────────────────────────────────────────────────────

// Propagate runs one tick of energy spreading across the field.
//
// For each signal i, its vector receives contributions from every other
// signal j, weighted by their Dot similarity and j's feature weights:
//
//	vector_i_new[f] += α × Dot(vector_i, vector_j) × vector_j[f]   ∀ j ≠ i
//
// Dot is used — magnitude influence is required. High-energy signals that
// align with signal i should propagate more aggressively (R1).
//
// All updates are computed from the pre-tick state (snapshot semantics):
// signal i does not see signal j's updated vector within the same tick.
// This makes propagation order-independent and deterministic (Invariant 1).
//
// Returns a new SignalField with updated Signal vectors.
// Traces and Confidence are not modified — field propagation is a structural
// operation at the field level, not a Tier 2 Signal transformation.
func Propagate(f SignalField) SignalField {
	n := len(f.Signals)
	if n == 0 {
		return f.clone()
	}

	// Snapshot: compute all Dot products from current state before any update.
	// dots[i][j] = Dot(signal_i, signal_j)
	dots := make([][]float64, n)
	for i := range dots {
		dots[i] = make([]float64, n)
		for j := range f.Signals {
			if i != j {
				dots[i][j] = core.Dot(f.Signals[i].Vector, f.Signals[j].Vector)
			}
		}
	}

	out := f.clone()
	for i := range out.Signals {
		for j, src := range f.Signals {
			if i == j {
				continue
			}
			sim := dots[i][j]
			if sim <= 0 {
				continue // no alignment — no energy transfer
			}
			for feat, w := range src.Vector {
				out.Signals[i].Vector[feat] += f.Alpha * sim * w
			}
		}
		// Remove zeros introduced by negative summation (Invariant 3).
		for feat, w := range out.Signals[i].Vector {
			if w == 0 {
				delete(out.Signals[i].Vector, feat)
			}
		}
	}
	return out
}

// ─── Decay ────────────────────────────────────────────────────────────────────

// Decay applies exponential weight reduction to all signals in the field.
//
//	w_new[f] = w_old[f] × (1 − λ)
//
// Prevents runaway energy accumulation from repeated Propagate ticks.
// Features that decay to exactly zero are removed (Invariant 3).
//
// Returns a new SignalField. The receiver is not modified.
func Decay(f SignalField) SignalField {
	out := f.clone()
	factor := 1 - f.Lambda
	for i := range out.Signals {
		for feat, w := range out.Signals[i].Vector {
			nw := w * factor
			if nw == 0 {
				delete(out.Signals[i].Vector, feat)
			} else {
				out.Signals[i].Vector[feat] = nw
			}
		}
	}
	return out
}

// ─── StabilizeResult ──────────────────────────────────────────────────────────

// StabilizeResult carries the outcome of a Stabilize call.
type StabilizeResult struct {
	Field      SignalField
	Ticks      int     // number of ticks executed
	Converged  bool    // true if energy delta fell below ε before MaxTicks
	FinalDelta float64 // last measured energy delta
}

// ─── Stabilize ────────────────────────────────────────────────────────────────

// Stabilize runs alternating Propagate and Decay ticks until the total field
// energy change between ticks falls below ε (convergence), or MaxTicks is
// reached (hard stop).
//
// Each tick: Propagate → Decay → measure energy delta.
//
// The stability condition λ > α × (N−1) × μ must hold for convergence.
// With defaults (α=0.1, λ=0.3) this is satisfied for N ≤ 15 at moderate
// inter-signal similarity. MaxTicks acts as a hard stop regardless.
//
// Returns a StabilizeResult with the final field, tick count, convergence
// status, and the last measured energy delta.
func Stabilize(f SignalField) StabilizeResult {
	current := f.clone()
	var ticks int
	var delta float64

	for ticks = 0; ticks < f.MaxTicks; ticks++ {
		energyBefore := totalEnergy(current)

		next := Propagate(current)
		next = Decay(next)

		energyAfter := totalEnergy(next)
		delta = energyAfter - energyBefore
		if delta < 0 {
			delta = -delta
		}

		current = next

		if delta < f.Epsilon {
			return StabilizeResult{
				Field:      current,
				Ticks:      ticks + 1,
				Converged:  true,
				FinalDelta: delta,
			}
		}
	}

	return StabilizeResult{
		Field:      current,
		Ticks:      ticks,
		Converged:  false,
		FinalDelta: delta,
	}
}

// ─── Attention ────────────────────────────────────────────────────────────────

// Attention returns the top-K signals from the field ranked by relevance to
// a query signal.
//
// Relevance score for signal i:
//
//	score_i = Dot(query.Vector, signal_i.Vector) × Energy(signal_i.Vector)
//
// Dot is used — magnitude influence is required. A high-energy signal that
// aligns with the query should rank above a weak signal with the same
// direction (R1). This is how the field narrows to the most active,
// most-aligned signals before Prototype matching.
//
// If k ≥ len(field.Signals), all signals are returned (sorted by score).
// Signals with score ≤ 0 (no alignment with query) are excluded.
//
// Returns signals in descending score order.
func Attention(f SignalField, query core.Signal, k int) []core.Signal {
	type scored struct {
		signal core.Signal
		score  float64
	}

	candidates := make([]scored, 0, len(f.Signals))
	for _, s := range f.Signals {
		score := core.Dot(query.Vector, s.Vector) * core.Energy(s.Vector)
		if score > 0 {
			candidates = append(candidates, scored{signal: s.Clone(), score: score})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	if k > len(candidates) {
		k = len(candidates)
	}

	result := make([]core.Signal, k)
	for i := range result {
		result[i] = candidates[i].signal
	}
	return result
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

// totalEnergy returns the sum of Energy across all signals in the field.
// Used by Stabilize to measure convergence.
func totalEnergy(f SignalField) float64 {
	var total float64
	for _, s := range f.Signals {
		total += core.Energy(s.Vector)
	}
	return total
}
