// Package core defines Centrix's fundamental types and invariants.
//
// Dependency order — nothing in this package imports from anywhere else:
//
//	FeatureIndex → SparseVector → Action → ComposeMode → Step → Trace → Signal → Prototype
//
// Every other Centrix package builds on these types. They are the foundation.
package core

import "fmt"

// ─── FeatureIndex ─────────────────────────────────────────────────────────────

// FeatureIndex is the key type for a concept dimension in the semantic space.
//
// A FeatureIndex always represents the same concept — across all signals,
// prototypes, and runs. This is the precondition for similarity math to be
// meaningful. If the same index means different things in different contexts,
// every cosine and dot product becomes semantically undefined.
//
// Centrix does not assign or interpret FeatureIndex values. That is the
// caller's responsibility, typically managed through the Registry package.
type FeatureIndex = uint32

// ─── SparseVector ─────────────────────────────────────────────────────────────

// SparseVector is a sparse representation of a point in concept space.
// Only non-zero features are stored — absence is meaningful; storing zeros
// wastes memory and corrupts similarity computations.
//
// Typical usage: 10–30 active features in a space of 10,000+ dimensions.
// All algebra operations run in O(k) where k is the number of active features.
type SparseVector map[FeatureIndex]float64

// Len returns the number of active (non-zero) features.
func (v SparseVector) Len() int {
	return len(v)
}

// Clone returns a deep copy of the SparseVector.
// Mutating the copy does not affect the original.
func (v SparseVector) Clone() SparseVector {
	c := make(SparseVector, len(v))
	for f, w := range v {
		c[f] = w
	}
	return c
}

// ─── Action ───────────────────────────────────────────────────────────────────

// Action is a typed constant recording what Tier 2 operation was applied to a
// Signal at a given Step. The learning system uses Action to decide how to
// update weights and trust after a run completes.
//
// These six constants correspond to the six Tier 2 Signal operations.
// Callers that build execution engines may define their own effect vocabulary
// (tool calls, output emission, etc.) — those are separate types, not Actions.
type Action int

const (
	// Generated — new features were produced that were absent in the input signal.
	Generated Action = iota

	// Matched — the signal was compared against a Prototype and scored.
	Matched

	// Propagated — energy was spread through a SignalField based on similarity.
	Propagated

	// Attenuated — signal weights were reduced by a decay factor.
	Attenuated

	// Composed — two signals were merged into a higher-level signal.
	Composed

	// Filtered — features below a weight threshold were removed.
	Filtered
)

// String returns a human-readable name for logging and trace inspection.
func (a Action) String() string {
	switch a {
	case Generated:
		return "Generated"
	case Matched:
		return "Matched"
	case Propagated:
		return "Propagated"
	case Attenuated:
		return "Attenuated"
	case Composed:
		return "Composed"
	case Filtered:
		return "Filtered"
	default:
		return fmt.Sprintf("Action(%d)", int(a))
	}
}

// ─── ComposeMode ──────────────────────────────────────────────────────────────

// ComposeMode specifies how Compose combines the confidence of two Signals.
//
// The caller must supply this — Centrix cannot infer it. The caller built the
// flow and knows whether two convergent paths represent genuinely independent
// evidence (different sources, different angles) or correlated evidence (the
// same facts processed two ways).
//
// Passing the wrong mode silently inflates or suppresses confidence, which
// corrupts the downstream learning signal.
type ComposeMode int

const (
	// Independent — the two signals come from genuinely separate evidence sources.
	// Confidence is combined with the OR-rule:
	//   1 − (1 − cA) × (1 − cB)
	// Two weak independent signals can together produce strong confidence.
	Independent ComposeMode = iota

	// Correlated — the two signals share the same underlying evidence.
	// Confidence is the Max of the two:
	//   max(cA, cB)
	// Combining correlated signals does not multiply evidence — it selects the
	// more confident reading of the same facts.
	Correlated
)

// ─── Step ─────────────────────────────────────────────────────────────────────

// Step records one transformation in a Signal's execution history.
//
// Every Tier 2 operation appends a Step to the Signal's Trace (Invariant 5).
// No transformation is silent. When ConfidenceBefore == ConfidenceAfter, the
// confidence update was gated — all four conditions were not met. This is
// information: the learning system distinguishes gated updates from zero-delta
// updates.
type Step struct {
	// Node is the string identity of what produced this step.
	// Centrix does not define what a node is — only that it has a string identity.
	// A non-empty Node is required for the confidence gate (condition 2).
	Node string

	// Action is the Tier 2 operation that was applied.
	Action Action

	// Value is an optional payload — the result or output of the operation.
	// Its type and meaning are defined by the caller.
	Value any

	// ConfidenceBefore is the signal's confidence before this step.
	ConfidenceBefore float64

	// ConfidenceAfter is the signal's confidence after this step.
	// Equal to ConfidenceBefore when the confidence gate was not satisfied.
	ConfidenceAfter float64
}

// ─── Trace ────────────────────────────────────────────────────────────────────

// DefaultTraceCap is the maximum number of Steps a Trace retains.
// When full, the oldest Step is dropped to make room (sliding window).
// The most recently appended Step is never dropped.
//
// Reasoning chains rarely exceed 10–20 steps. 64 covers 99% of expected usage
// while bounding memory at ~2KB per Signal trace.
const DefaultTraceCap = 64

// Trace is the ordered execution history of a Signal.
// It accumulates Steps as the Signal moves through Tier 2 operations.
//
// No mutex. A Signal is owned by one goroutine at a time (Invariant 4).
// Concurrent access to a Signal is a caller error, not a race to protect.
// Adding synchronisation here would penalise correct usage to accommodate incorrect usage.
type Trace struct {
	steps []Step
	cap   int
}

// NewTrace creates an empty Trace with the given capacity.
// Pass DefaultTraceCap for standard usage. Values ≤ 0 fall back to DefaultTraceCap.
func NewTrace(cap int) Trace {
	if cap <= 0 {
		cap = DefaultTraceCap
	}
	return Trace{
		steps: make([]Step, 0, min(cap, DefaultTraceCap)),
		cap:   cap,
	}
}

// Add appends a Step to the Trace.
// If the cap is reached, the oldest Step is dropped (sliding window).
// The newly added Step is always retained.
func (t *Trace) Add(step Step) {
	if len(t.steps) >= t.cap {
		copy(t.steps, t.steps[1:])
		t.steps[len(t.steps)-1] = step
		return
	}
	t.steps = append(t.steps, step)
}

// Steps returns a read-only view of accumulated Steps in chronological order.
// The returned slice must not be modified by the caller.
func (t Trace) Steps() []Step {
	return t.steps
}

// Len returns the number of Steps currently held.
func (t Trace) Len() int {
	return len(t.steps)
}

// Last returns the most recently added Step and true, or a zero Step and false
// if the Trace is empty.
func (t Trace) Last() (Step, bool) {
	if len(t.steps) == 0 {
		return Step{}, false
	}
	return t.steps[len(t.steps)-1], true
}

// clone returns a deep copy. Used by Signal.Clone().
func (t Trace) clone() Trace {
	c := Trace{
		steps: make([]Step, len(t.steps), t.cap),
		cap:   t.cap,
	}
	copy(c.steps, t.steps)
	return c
}

// merge returns a new Trace containing all steps from t followed by all steps
// from other. If the combined length exceeds the cap, oldest steps are dropped.
// One slot is reserved for the Compose Step the caller appends afterward —
// the Compose Step is always the final entry and is never dropped (CONCEPTS §3.6).
func (t Trace) merge(other Trace) Trace {
	combined := make([]Step, 0, len(t.steps)+len(other.steps))
	combined = append(combined, t.steps...)
	combined = append(combined, other.steps...)

	cap := t.cap
	if other.cap > cap {
		cap = other.cap
	}

	result := Trace{cap: cap}
	if len(combined) <= cap-1 {
		// Fits with room for the Compose Step.
		result.steps = combined
		return result
	}
	// Trim oldest, preserve one slot for Compose Step.
	keep := cap - 1
	start := len(combined) - keep
	if start < 0 {
		start = 0
	}
	result.steps = make([]Step, len(combined)-start, cap)
	copy(result.steps, combined[start:])
	return result
}

// ─── Signal ───────────────────────────────────────────────────────────────────

// Signal is the canonical runtime object in Centrix.
//
// A Signal is a value, not an identity. Return type is Signal, not *Signal.
// Every Tier 2 operation returns a new Signal — the input is never mutated.
// Determinism holds by construction, not by discipline (Invariant 1).
//
// A Signal is ephemeral. It is created at run start, transformed through nodes,
// and discarded at run end. It never crosses run boundaries. Only Prototypes
// persist between runs (Invariant 8).
//
// A Signal is evolving state, not a message. One Signal per execution thread.
// It accumulates the full reasoning history in its Trace.
//
// Energy is always derived: Energy(signal.Vector). There is no separate Energy
// field — one source of truth for activation strength (Invariant 10).
type Signal struct {
	Vector     SparseVector
	Confidence float64
	Trace      Trace
}

// NewSignal creates a Signal with a pre-allocated Vector and a fresh Trace.
// capacity hints at the expected number of active features.
func NewSignal(capacity int) Signal {
	return Signal{
		Vector: make(SparseVector, capacity),
		Trace:  NewTrace(DefaultTraceCap),
	}
}

// NewSignalFromVector creates a Signal wrapping an existing SparseVector
// with the given initial confidence and a fresh Trace.
// Confidence is clamped to [0.0, 1.0].
func NewSignalFromVector(v SparseVector, confidence float64) Signal {
	return Signal{
		Vector:     v,
		Confidence: clampUnit(confidence),
		Trace:      NewTrace(DefaultTraceCap),
	}
}

// Clone returns a deep copy of the Signal.
// Mutating the clone does not affect the original — they share no state.
func (s Signal) Clone() Signal {
	return Signal{
		Vector:     s.Vector.Clone(),
		Confidence: s.Confidence,
		Trace:      s.Trace.clone(),
	}
}

// WithStep returns a new Signal identical to s but with step appended to the Trace.
// The original Signal is not modified.
func (s Signal) WithStep(step Step) Signal {
	c := s.Clone()
	c.Trace.Add(step)
	return c
}

// ─── Prototype ────────────────────────────────────────────────────────────────

// Prototype is persistent knowledge — a SparseVector that survives between runs.
//
// Where a Signal is ephemeral (created at run start, discarded at run end),
// a Prototype is authored or learned knowledge that lives in the knowledge layer.
// It is never modified during a run. Only Prototypes persist (Invariant 9).
//
// Weight reflects how trusted or reliable this prototype is.
// Higher weight prototypes have more influence in matching and generation.
type Prototype struct {
	Vector SparseVector
	Weight float64 // [0.0, 1.0] — higher weight = more trusted
}

// NewPrototype creates a Prototype with the given Vector and Weight.
// Weight is clamped to [0.0, 1.0].
func NewPrototype(v SparseVector, weight float64) Prototype {
	return Prototype{
		Vector: v,
		Weight: clampUnit(weight),
	}
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

// clampUnit clamps v to [0.0, 1.0].
// Used for Confidence and Prototype.Weight — both represent bounded beliefs.
func clampUnit(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// min returns the smaller of two ints.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
