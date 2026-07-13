package field_test

import (
	"testing"

	"github.com/leraniode/xgo/centrix/core"
	"github.com/leraniode/xgo/centrix/field"
)

// ─── helpers ──────────────────────────────────────────────────────────────────

func approx(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 1e-9
}

func sig(feats map[core.FeatureIndex]float64, conf float64) core.Signal {
	return core.NewSignalFromVector(core.SparseVector(feats), conf)
}

// ─── SignalField construction ─────────────────────────────────────────────────

func TestNew_DefaultConstants(t *testing.T) {
	f := field.New([]core.Signal{sig(map[core.FeatureIndex]float64{1: 0.5}, 0.5)})
	if f.Alpha != field.DefaultAlpha {
		t.Errorf("Alpha = %f, want %f", f.Alpha, field.DefaultAlpha)
	}
	if f.Lambda != field.DefaultLambda {
		t.Errorf("Lambda = %f, want %f", f.Lambda, field.DefaultLambda)
	}
	if f.Epsilon != field.DefaultEpsilon {
		t.Errorf("Epsilon = %f, want %f", f.Epsilon, field.DefaultEpsilon)
	}
	if f.MaxTicks != field.DefaultMaxTicks {
		t.Errorf("MaxTicks = %d, want %d", f.MaxTicks, field.DefaultMaxTicks)
	}
}

func TestNew_SignalsCopied(t *testing.T) {
	// Mutating the input slice after construction must not affect the field.
	signals := []core.Signal{sig(map[core.FeatureIndex]float64{1: 0.5}, 0.5)}
	f := field.New(signals)
	signals[0].Vector[1] = 99.0

	if f.Signals[0].Vector[1] == 99.0 {
		t.Error("New: signals not deep-copied — mutation of input affected field")
	}
}

func TestNew_Empty(t *testing.T) {
	f := field.New(nil)
	if len(f.Signals) != 0 {
		t.Errorf("New(nil): expected 0 signals, got %d", len(f.Signals))
	}
}

// ─── Propagate ────────────────────────────────────────────────────────────────

func TestPropagate_AlignedSignals_TransferEnergy(t *testing.T) {
	// Two signals sharing feature 1 → Dot > 0 → energy flows between them.
	a := sig(map[core.FeatureIndex]float64{1: 0.8}, 0.5)
	b := sig(map[core.FeatureIndex]float64{1: 0.6, 2: 0.4}, 0.5)
	f := field.New([]core.Signal{a, b})

	out := field.Propagate(f)

	// Signal 0 (a) should gain feature 2 from b via propagation.
	if _, ok := out.Signals[0].Vector[2]; !ok {
		t.Error("Propagate: aligned signals should transfer features across field")
	}
	// Signal 0's feature 1 should grow.
	if out.Signals[0].Vector[1] <= a.Vector[1] {
		t.Errorf("Propagate: feature weight should grow after energy transfer: got %f, want > %f",
			out.Signals[0].Vector[1], a.Vector[1])
	}
}

func TestPropagate_OrthogonalSignals_NoTransfer(t *testing.T) {
	// No shared features → Dot = 0 → no energy transfer.
	a := sig(map[core.FeatureIndex]float64{1: 0.8}, 0.5)
	b := sig(map[core.FeatureIndex]float64{2: 0.6}, 0.5)
	f := field.New([]core.Signal{a, b})

	out := field.Propagate(f)

	// Signal 0 must not gain feature 2.
	if _, ok := out.Signals[0].Vector[2]; ok {
		t.Error("Propagate: orthogonal signals should not transfer features")
	}
	if !approx(out.Signals[0].Vector[1], a.Vector[1]) {
		t.Errorf("Propagate: orthogonal — feature 1 changed: got %f, want %f",
			out.Signals[0].Vector[1], a.Vector[1])
	}
}

func TestPropagate_SnapshotSemantics(t *testing.T) {
	// Updates must use pre-tick state — signal i must not see signal j's
	// updated vector within the same tick (order-independent, Invariant 1).
	a := sig(map[core.FeatureIndex]float64{1: 1.0}, 0.5)
	b := sig(map[core.FeatureIndex]float64{1: 1.0}, 0.5)
	f := field.New([]core.Signal{a, b})

	// Run same field twice — result must be identical regardless of internal order.
	out1 := field.Propagate(f)
	out2 := field.Propagate(f)

	for feat, w1 := range out1.Signals[0].Vector {
		w2 := out2.Signals[0].Vector[feat]
		if !approx(w1, w2) {
			t.Errorf("Propagate not deterministic: feature %d: %f vs %f", feat, w1, w2)
		}
	}
}

func TestPropagate_DoesNotMutateInput(t *testing.T) {
	a := sig(map[core.FeatureIndex]float64{1: 0.8}, 0.5)
	b := sig(map[core.FeatureIndex]float64{1: 0.6, 2: 0.4}, 0.5)
	f := field.New([]core.Signal{a, b})
	origWeight := f.Signals[0].Vector[1]

	_ = field.Propagate(f)

	if f.Signals[0].Vector[1] != origWeight {
		t.Error("Propagate mutated input field")
	}
}

func TestPropagate_UsesAlpha(t *testing.T) {
	// Manual calculation for two signals:
	// Dot({1:1.0}, {1:1.0}) = 1.0
	// signal[0].Vector[1] += α × 1.0 × 1.0 = 0.1 → becomes 1.1
	a := sig(map[core.FeatureIndex]float64{1: 1.0}, 0.0)
	b := sig(map[core.FeatureIndex]float64{1: 1.0}, 0.0)
	f := field.New([]core.Signal{a, b})

	out := field.Propagate(f)

	want := 1.0 + field.DefaultAlpha*1.0*1.0
	if !approx(out.Signals[0].Vector[1], want) {
		t.Errorf("Propagate alpha: feature weight = %f, want %f", out.Signals[0].Vector[1], want)
	}
}

func TestPropagate_EmptyField(t *testing.T) {
	f := field.New(nil)
	out := field.Propagate(f)
	if len(out.Signals) != 0 {
		t.Error("Propagate on empty field should return empty field")
	}
}

func TestPropagate_PreservesSparsity(t *testing.T) {
	a := sig(map[core.FeatureIndex]float64{1: 0.5}, 0.5)
	b := sig(map[core.FeatureIndex]float64{1: 0.5}, 0.5)
	f := field.New([]core.Signal{a, b})
	out := field.Propagate(f)

	for i, s := range out.Signals {
		for feat, w := range s.Vector {
			if w == 0 {
				t.Errorf("Propagate: zero-weight feature %d in signal %d (sparsity violation)", feat, i)
			}
		}
	}
}

// ─── Decay ────────────────────────────────────────────────────────────────────

func TestDecay_ReducesWeights(t *testing.T) {
	// λ=0.3 → factor=0.7 → weight 1.0 becomes 0.7
	a := sig(map[core.FeatureIndex]float64{1: 1.0}, 0.5)
	f := field.New([]core.Signal{a})

	out := field.Decay(f)

	want := 1.0 * (1 - field.DefaultLambda)
	if !approx(out.Signals[0].Vector[1], want) {
		t.Errorf("Decay: weight = %f, want %f", out.Signals[0].Vector[1], want)
	}
}

func TestDecay_DoesNotMutateInput(t *testing.T) {
	a := sig(map[core.FeatureIndex]float64{1: 1.0}, 0.5)
	f := field.New([]core.Signal{a})
	_ = field.Decay(f)
	if f.Signals[0].Vector[1] != 1.0 {
		t.Error("Decay mutated input field")
	}
}

func TestDecay_RemovesZeroWeights(t *testing.T) {
	// After full decay (λ=1), all features should be absent (Invariant 3).
	a := sig(map[core.FeatureIndex]float64{1: 0.5}, 0.5)
	f := field.New([]core.Signal{a})
	f.Lambda = 1.0

	out := field.Decay(f)

	if len(out.Signals[0].Vector) != 0 {
		t.Errorf("Decay(λ=1): vector should be empty, got %v", out.Signals[0].Vector)
	}
}

func TestDecay_AllSignalsDecayed(t *testing.T) {
	signals := []core.Signal{
		sig(map[core.FeatureIndex]float64{1: 1.0}, 0.5),
		sig(map[core.FeatureIndex]float64{2: 0.8}, 0.4),
	}
	f := field.New(signals)
	out := field.Decay(f)

	for i, s := range out.Signals {
		for feat, w := range s.Vector {
			want := f.Signals[i].Vector[feat] * (1 - f.Lambda)
			if !approx(w, want) {
				t.Errorf("Decay signal %d feature %d: got %f, want %f", i, feat, w, want)
			}
		}
	}
}

// ─── Stabilize ────────────────────────────────────────────────────────────────

func TestStabilize_ConvergesWithValidatedDefaults(t *testing.T) {
	// Two similar signals, N=2, validated defaults → must converge.
	a := sig(map[core.FeatureIndex]float64{1: 0.5, 2: 0.3}, 0.5)
	b := sig(map[core.FeatureIndex]float64{1: 0.4, 3: 0.2}, 0.4)
	f := field.New([]core.Signal{a, b})

	result := field.Stabilize(f)

	if !result.Converged {
		t.Errorf("Stabilize: did not converge with N=2, defaults. Ticks=%d, delta=%f",
			result.Ticks, result.FinalDelta)
	}
	if result.Ticks == 0 {
		t.Error("Stabilize: reported 0 ticks")
	}
}

func TestStabilize_MaxTicksHardStop(t *testing.T) {
	// Force non-convergence by capping at 1 tick and tight epsilon.
	a := sig(map[core.FeatureIndex]float64{1: 100.0}, 0.5)
	b := sig(map[core.FeatureIndex]float64{1: 100.0}, 0.5)
	f := field.New([]core.Signal{a, b})
	f.MaxTicks = 1
	f.Epsilon = 1e-20 // impossible to reach in one tick

	result := field.Stabilize(f)

	if result.Ticks != 1 {
		t.Errorf("Stabilize: MaxTicks=1 should stop at 1 tick, got %d", result.Ticks)
	}
	if result.Converged {
		t.Error("Stabilize: should not report convergence when stopped by MaxTicks")
	}
}

func TestStabilize_DeltaBelowEpsilon(t *testing.T) {
	// After convergence, FinalDelta must be < Epsilon.
	a := sig(map[core.FeatureIndex]float64{1: 0.5}, 0.5)
	b := sig(map[core.FeatureIndex]float64{1: 0.4}, 0.4)
	f := field.New([]core.Signal{a, b})

	result := field.Stabilize(f)

	if result.Converged && result.FinalDelta >= f.Epsilon {
		t.Errorf("Stabilize: converged but FinalDelta=%f >= Epsilon=%f", result.FinalDelta, f.Epsilon)
	}
}

func TestStabilize_DoesNotMutateInput(t *testing.T) {
	a := sig(map[core.FeatureIndex]float64{1: 0.5}, 0.5)
	f := field.New([]core.Signal{a})
	origWeight := f.Signals[0].Vector[1]

	_ = field.Stabilize(f)

	if f.Signals[0].Vector[1] != origWeight {
		t.Error("Stabilize mutated input field")
	}
}

func TestStabilize_TicksMonotonicallyAdvance(t *testing.T) {
	// Ticks reported must be > 0 for a non-trivially converging field.
	a := sig(map[core.FeatureIndex]float64{1: 0.8, 2: 0.2}, 0.6)
	b := sig(map[core.FeatureIndex]float64{1: 0.6, 3: 0.4}, 0.5)
	f := field.New([]core.Signal{a, b})

	result := field.Stabilize(f)
	if result.Ticks <= 0 {
		t.Errorf("Stabilize: ticks = %d, want > 0", result.Ticks)
	}
}

func TestStabilize_EmptyField(t *testing.T) {
	f := field.New(nil)
	result := field.Stabilize(f)
	if !result.Converged {
		t.Error("Stabilize on empty field should report convergence immediately")
	}
	if result.Ticks > 1 {
		t.Errorf("Stabilize on empty field: expected ≤1 ticks, got %d", result.Ticks)
	}
}

// ─── Attention ────────────────────────────────────────────────────────────────

func TestAttention_ReturnsTopK(t *testing.T) {
	// Three signals. Query aligns with features 1 and 2.
	// Signal with highest Dot × Energy should rank first.
	high := sig(map[core.FeatureIndex]float64{1: 0.9, 2: 0.8}, 0.7) // strong overlap
	mid := sig(map[core.FeatureIndex]float64{1: 0.4}, 0.5)            // partial overlap
	low := sig(map[core.FeatureIndex]float64{3: 0.9}, 0.6)            // no overlap with query
	f := field.New([]core.Signal{low, mid, high})

	query := sig(map[core.FeatureIndex]float64{1: 1.0, 2: 1.0}, 0.0)
	result := field.Attention(f, query, 2)

	if len(result) != 2 {
		t.Fatalf("Attention: got %d results, want 2", len(result))
	}
	// First result should have the highest dot × energy — that's `high`.
	if result[0].Vector[1] != high.Vector[1] {
		t.Error("Attention: top result is not the highest-scoring signal")
	}
}

func TestAttention_ExcludesNonAligned(t *testing.T) {
	// Signal with no feature overlap → score = 0 → excluded.
	aligned := sig(map[core.FeatureIndex]float64{1: 0.8}, 0.5)
	unrelated := sig(map[core.FeatureIndex]float64{99: 0.9}, 0.5)
	f := field.New([]core.Signal{aligned, unrelated})

	query := sig(map[core.FeatureIndex]float64{1: 1.0}, 0.0)
	result := field.Attention(f, query, 10)

	for _, s := range result {
		if _, ok := s.Vector[99]; ok {
			t.Error("Attention: non-aligned signal (score ≤ 0) should be excluded")
		}
	}
}

func TestAttention_KLargerThanField(t *testing.T) {
	// k > number of qualifying signals → return all qualifying.
	a := sig(map[core.FeatureIndex]float64{1: 0.8}, 0.5)
	f := field.New([]core.Signal{a})
	query := sig(map[core.FeatureIndex]float64{1: 1.0}, 0.0)

	result := field.Attention(f, query, 100)
	if len(result) != 1 {
		t.Errorf("Attention: k > signals → should return all qualifying, got %d", len(result))
	}
}

func TestAttention_EmptyField(t *testing.T) {
	f := field.New(nil)
	query := sig(map[core.FeatureIndex]float64{1: 1.0}, 0.0)
	result := field.Attention(f, query, 5)
	if len(result) != 0 {
		t.Errorf("Attention on empty field: expected 0 results, got %d", len(result))
	}
}

func TestAttention_SortedDescending(t *testing.T) {
	// Result must be in descending score order.
	signals := make([]core.Signal, 5)
	for i := range signals {
		w := float64(i+1) * 0.2
		signals[i] = sig(map[core.FeatureIndex]float64{1: w}, 0.5)
	}
	f := field.New(signals)
	query := sig(map[core.FeatureIndex]float64{1: 1.0}, 0.0)

	result := field.Attention(f, query, 5)

	for i := 1; i < len(result); i++ {
		scoreA := core.Dot(query.Vector, result[i-1].Vector) * core.Energy(result[i-1].Vector)
		scoreB := core.Dot(query.Vector, result[i].Vector) * core.Energy(result[i].Vector)
		if scoreA < scoreB {
			t.Errorf("Attention not sorted descending at position %d: %.4f < %.4f", i, scoreA, scoreB)
		}
	}
}

func TestAttention_DoesNotMutateField(t *testing.T) {
	a := sig(map[core.FeatureIndex]float64{1: 0.8}, 0.5)
	f := field.New([]core.Signal{a})
	origWeight := f.Signals[0].Vector[1]
	query := sig(map[core.FeatureIndex]float64{1: 1.0}, 0.0)

	_ = field.Attention(f, query, 5)

	if f.Signals[0].Vector[1] != origWeight {
		t.Error("Attention mutated input field")
	}
}

// ─── Stability condition ──────────────────────────────────────────────────────

func TestStability_ConditionHolds_AtDefaultsAndRealisticN(t *testing.T) {
	// λ > α × (N-1) × μ must hold for N=15 at realistic sparse μ.
	α := field.DefaultAlpha
	λ := field.DefaultLambda
	N := field.StableN
	μ := 0.05 // realistic: k=20 in D=10,000

	rhs := α * float64(N-1) * μ
	if λ <= rhs {
		t.Errorf("Stability condition violated: λ=%.3f ≤ α×(N-1)×μ=%.3f (N=%d, μ=%.2f)",
			λ, rhs, N, μ)
	}
}

func TestStability_ConditionViolated_AtHighN(t *testing.T) {
	// At very high N with high overlap, condition should naturally fail.
	// This is a documentation test — it verifies the math, not runtime behaviour.
	α := field.DefaultAlpha
	λ := field.DefaultLambda
	N := 100
	μ := 0.5 // high overlap

	rhs := α * float64(N-1) * μ
	// We expect the condition to be violated here.
	if λ > rhs {
		t.Logf("Stability condition still holds at N=%d, μ=%.2f — field is more stable than expected", N, μ)
	} else {
		t.Logf("Stability condition correctly violated at N=%d, μ=%.2f: λ=%.2f ≤ %.3f", N, μ, λ, rhs)
	}
	// Not a hard failure — this is a characterisation test.
}
