package core_test

import (
	"math"
	"testing"

	"github.com/leraniode/xgo/centrix/core"
)

// ─── helpers ──────────────────────────────────────────────────────────────────

func approxEqual(a, b float64) bool {
	const eps = 1e-9
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < eps
}

func approxEqualTol(a, b, eps float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < eps
}

// ─── Energy ───────────────────────────────────────────────────────────────────

func TestEnergy_Positive(t *testing.T) {
	v := core.SparseVector{1: 0.5, 2: 0.3, 3: 0.2}
	if !approxEqual(core.Energy(v), 1.0) {
		t.Errorf("Energy = %f, want 1.0", core.Energy(v))
	}
}

func TestEnergy_Negative(t *testing.T) {
	// Energy is L1 norm — negative weights count positively.
	v := core.SparseVector{1: -0.5, 2: 0.5}
	if !approxEqual(core.Energy(v), 1.0) {
		t.Errorf("Energy with negatives = %f, want 1.0", core.Energy(v))
	}
}

func TestEnergy_Empty(t *testing.T) {
	if core.Energy(core.SparseVector{}) != 0 {
		t.Error("Energy of empty vector should be 0")
	}
}

func TestEnergy_Mixed(t *testing.T) {
	v := core.SparseVector{1: 1.0, 2: -2.0, 3: 3.0}
	if !approxEqual(core.Energy(v), 6.0) {
		t.Errorf("Energy = %f, want 6.0", core.Energy(v))
	}
}

// Invariant 10: Energy is always derived from the Vector.
// Mutating the vector must immediately change Energy — no cached field.
func TestEnergy_DerivedNotCached(t *testing.T) {
	v := core.SparseVector{1: 0.6, 2: 0.4}
	if !approxEqual(core.Energy(v), 1.0) {
		t.Fatalf("initial Energy = %f, want 1.0", core.Energy(v))
	}
	delete(v, 1)
	if !approxEqual(core.Energy(v), 0.4) {
		t.Errorf("Energy after mutation = %f, want 0.4", core.Energy(v))
	}
}

// ─── Dot ──────────────────────────────────────────────────────────────────────

func TestDot_Basic(t *testing.T) {
	a := core.SparseVector{1: 1.0, 2: 2.0}
	b := core.SparseVector{1: 3.0, 2: 4.0}
	// 1×3 + 2×4 = 11
	if !approxEqual(core.Dot(a, b), 11.0) {
		t.Errorf("Dot = %f, want 11.0", core.Dot(a, b))
	}
}

func TestDot_NoOverlap(t *testing.T) {
	a := core.SparseVector{1: 1.0}
	b := core.SparseVector{2: 1.0}
	if !approxEqual(core.Dot(a, b), 0.0) {
		t.Errorf("Dot with no overlap = %f, want 0.0", core.Dot(a, b))
	}
}

func TestDot_EmptyVectors(t *testing.T) {
	if core.Dot(core.SparseVector{}, core.SparseVector{}) != 0 {
		t.Error("Dot of two empty vectors should be 0")
	}
}

func TestDot_OneEmpty(t *testing.T) {
	a := core.SparseVector{1: 5.0}
	if core.Dot(a, core.SparseVector{}) != 0 {
		t.Error("Dot with empty vector should be 0")
	}
}

func TestDot_Commutativity(t *testing.T) {
	a := core.SparseVector{1: 0.9, 2: 0.1, 3: 0.5}
	b := core.SparseVector{1: 0.2, 3: 0.8}
	if !approxEqual(core.Dot(a, b), core.Dot(b, a)) {
		t.Errorf("Dot not commutative: Dot(a,b)=%f Dot(b,a)=%f", core.Dot(a, b), core.Dot(b, a))
	}
}

func TestDot_IteratesSmaller(t *testing.T) {
	// Large a, small b — result must be identical regardless of argument order.
	a := make(core.SparseVector, 100)
	for i := 0; i < 100; i++ {
		a[core.FeatureIndex(i)] = float64(i) * 0.01
	}
	b := core.SparseVector{1: 1.0, 50: 1.0}
	if !approxEqual(core.Dot(a, b), core.Dot(b, a)) {
		t.Error("Dot result differs by argument order (iteration optimisation broken)")
	}
}

func TestDot_NegativeWeights(t *testing.T) {
	a := core.SparseVector{1: -1.0}
	b := core.SparseVector{1: 2.0}
	if !approxEqual(core.Dot(a, b), -2.0) {
		t.Errorf("Dot with negative weight = %f, want -2.0", core.Dot(a, b))
	}
}

// ─── Cosine ───────────────────────────────────────────────────────────────────

func TestCosine_IdenticalVectors(t *testing.T) {
	v := core.SparseVector{1: 0.6, 2: 0.8}
	got := core.Cosine(v, v)
	if !approxEqualTol(got, 1.0, 1e-9) {
		t.Errorf("Cosine(v, v) = %f, want 1.0", got)
	}
}

func TestCosine_OppositeVectors(t *testing.T) {
	a := core.SparseVector{1: 1.0}
	b := core.SparseVector{1: -1.0}
	got := core.Cosine(a, b)
	if !approxEqualTol(got, -1.0, 1e-9) {
		t.Errorf("Cosine of opposite vectors = %f, want -1.0", got)
	}
}

func TestCosine_OrthogonalVectors(t *testing.T) {
	a := core.SparseVector{1: 1.0}
	b := core.SparseVector{2: 1.0}
	if !approxEqual(core.Cosine(a, b), 0.0) {
		t.Errorf("Cosine of orthogonal vectors = %f, want 0.0", core.Cosine(a, b))
	}
}

func TestCosine_ZeroVector(t *testing.T) {
	// Invariant: zero vector must return 0, not NaN or panic.
	a := core.SparseVector{1: 1.0}
	z := core.SparseVector{}
	got := core.Cosine(a, z)
	if math.IsNaN(got) || math.IsInf(got, 0) {
		t.Errorf("Cosine with zero vector = %v, want 0.0", got)
	}
	if got != 0 {
		t.Errorf("Cosine with zero vector = %f, want 0.0", got)
	}
}

func TestCosine_BothZero(t *testing.T) {
	got := core.Cosine(core.SparseVector{}, core.SparseVector{})
	if got != 0 {
		t.Errorf("Cosine(zero, zero) = %f, want 0.0", got)
	}
}

func TestCosine_MagnitudeIndependence(t *testing.T) {
	// Scaling a vector should not change cosine similarity.
	a := core.SparseVector{1: 1.0, 2: 1.0}
	b := core.SparseVector{1: 1.0, 2: 1.0}
	scaled := core.SparseVector{1: 100.0, 2: 100.0}
	if !approxEqualTol(core.Cosine(a, b), core.Cosine(a, scaled), 1e-9) {
		t.Error("Cosine should be independent of vector magnitude")
	}
}

func TestCosine_Commutativity(t *testing.T) {
	a := core.SparseVector{1: 0.5, 3: 0.9}
	b := core.SparseVector{1: 0.2, 2: 0.7}
	if !approxEqualTol(core.Cosine(a, b), core.Cosine(b, a), 1e-12) {
		t.Errorf("Cosine not commutative: %f vs %f", core.Cosine(a, b), core.Cosine(b, a))
	}
}

func TestCosine_Range(t *testing.T) {
	cases := []struct{ a, b core.SparseVector }{
		{core.SparseVector{1: 0.9, 2: 0.1}, core.SparseVector{1: 0.4, 3: 0.6}},
		{core.SparseVector{1: -0.5, 2: 0.5}, core.SparseVector{1: 0.5, 2: -0.5}},
		{core.SparseVector{1: 1.0}, core.SparseVector{1: 0.001}},
	}
	for _, tc := range cases {
		got := core.Cosine(tc.a, tc.b)
		if got < -1.0-1e-9 || got > 1.0+1e-9 {
			t.Errorf("Cosine out of [-1,1]: %f", got)
		}
	}
}

// ─── Jaccard ──────────────────────────────────────────────────────────────────

func TestJaccard_IdenticalSets(t *testing.T) {
	v := core.SparseVector{1: 0.9, 2: 0.1, 3: 0.5}
	got := core.Jaccard(v, v)
	if !approxEqual(got, 1.0) {
		t.Errorf("Jaccard(v, v) = %f, want 1.0", got)
	}
}

func TestJaccard_DisjointSets(t *testing.T) {
	a := core.SparseVector{1: 1.0, 2: 1.0}
	b := core.SparseVector{3: 1.0, 4: 1.0}
	if !approxEqual(core.Jaccard(a, b), 0.0) {
		t.Errorf("Jaccard of disjoint sets = %f, want 0.0", core.Jaccard(a, b))
	}
}

func TestJaccard_PartialOverlap(t *testing.T) {
	// a={1,2}, b={2,3} → intersection={2}, union={1,2,3} → 1/3
	a := core.SparseVector{1: 1.0, 2: 1.0}
	b := core.SparseVector{2: 1.0, 3: 1.0}
	want := 1.0 / 3.0
	if !approxEqualTol(core.Jaccard(a, b), want, 1e-9) {
		t.Errorf("Jaccard partial overlap = %f, want %f", core.Jaccard(a, b), want)
	}
}

func TestJaccard_BothEmpty(t *testing.T) {
	got := core.Jaccard(core.SparseVector{}, core.SparseVector{})
	if got != 0 {
		t.Errorf("Jaccard(empty, empty) = %f, want 0.0", got)
	}
}

func TestJaccard_OneEmpty(t *testing.T) {
	a := core.SparseVector{1: 1.0}
	got := core.Jaccard(a, core.SparseVector{})
	if !approxEqual(got, 0.0) {
		t.Errorf("Jaccard with empty = %f, want 0.0", got)
	}
}

func TestJaccard_WeightIgnored(t *testing.T) {
	// Jaccard is presence-only — weight must not affect the score.
	a := core.SparseVector{1: 0.001, 2: 100.0}
	b := core.SparseVector{1: 999.0, 2: 0.001}
	if !approxEqual(core.Jaccard(a, b), 1.0) {
		t.Errorf("Jaccard should ignore weights, got %f, want 1.0", core.Jaccard(a, b))
	}
}

func TestJaccard_Commutativity(t *testing.T) {
	a := core.SparseVector{1: 1.0, 2: 1.0, 5: 1.0}
	b := core.SparseVector{2: 1.0, 3: 1.0}
	if !approxEqual(core.Jaccard(a, b), core.Jaccard(b, a)) {
		t.Errorf("Jaccard not commutative: %f vs %f", core.Jaccard(a, b), core.Jaccard(b, a))
	}
}

func TestJaccard_Range(t *testing.T) {
	a := core.SparseVector{1: 1.0, 2: 1.0}
	b := core.SparseVector{2: 1.0, 3: 1.0}
	got := core.Jaccard(a, b)
	if got < 0 || got > 1 {
		t.Errorf("Jaccard out of [0,1]: %f", got)
	}
}

// ─── Merge ────────────────────────────────────────────────────────────────────

func TestMerge_DisjointFeatures(t *testing.T) {
	a := core.SparseVector{1: 0.5}
	b := core.SparseVector{2: 0.7}
	m := core.Merge(a, b)
	if m[1] != 0.5 || m[2] != 0.7 {
		t.Errorf("Merge disjoint: got %v", m)
	}
}

func TestMerge_SharedFeaturesSum(t *testing.T) {
	a := core.SparseVector{1: 0.3}
	b := core.SparseVector{1: 0.4}
	m := core.Merge(a, b)
	if !approxEqual(m[1], 0.7) {
		t.Errorf("Merge shared feature = %f, want 0.7", m[1])
	}
}

func TestMerge_DoesNotMutateInputs(t *testing.T) {
	a := core.SparseVector{1: 0.5}
	b := core.SparseVector{1: 0.5, 2: 0.3}
	_ = core.Merge(a, b)
	if a[1] != 0.5 {
		t.Error("Merge mutated input a")
	}
	if b[1] != 0.5 || b[2] != 0.3 {
		t.Error("Merge mutated input b")
	}
}

func TestMerge_CancellationDropsZero(t *testing.T) {
	// If weights sum to zero, the feature should be absent (sparsity invariant).
	a := core.SparseVector{1: 0.5}
	b := core.SparseVector{1: -0.5}
	m := core.Merge(a, b)
	if _, exists := m[1]; exists {
		t.Error("Merge: feature with zero summed weight should be removed")
	}
}

func TestMerge_EmptyInputs(t *testing.T) {
	m := core.Merge(core.SparseVector{}, core.SparseVector{})
	if len(m) != 0 {
		t.Error("Merge of two empty vectors should be empty")
	}
}

func TestMerge_OneEmpty(t *testing.T) {
	a := core.SparseVector{1: 0.9, 2: 0.1}
	m := core.Merge(a, core.SparseVector{})
	if m[1] != 0.9 || m[2] != 0.1 {
		t.Errorf("Merge with empty: got %v", m)
	}
}

func TestMerge_Commutativity_NonOverlap(t *testing.T) {
	a := core.SparseVector{1: 0.5}
	b := core.SparseVector{2: 0.7}
	mAB := core.Merge(a, b)
	mBA := core.Merge(b, a)
	for f := range mAB {
		if !approxEqual(mAB[f], mBA[f]) {
			t.Errorf("Merge not commutative for feature %d: %f vs %f", f, mAB[f], mBA[f])
		}
	}
}

// ─── Normalize ────────────────────────────────────────────────────────────────

func TestNormalize_UnitNorm(t *testing.T) {
	v := core.SparseVector{1: 3.0, 2: 4.0} // L2 norm = 5
	n := core.Normalize(v)
	// Compute resulting norm manually.
	var sum float64
	for _, w := range n {
		sum += w * w
	}
	if !approxEqualTol(math.Sqrt(sum), 1.0, 1e-9) {
		t.Errorf("Normalized L2 norm = %f, want 1.0", math.Sqrt(sum))
	}
}

func TestNormalize_ZeroVector(t *testing.T) {
	// Must not panic or return NaN.
	n := core.Normalize(core.SparseVector{})
	if len(n) != 0 {
		t.Errorf("Normalize of zero vector should be empty, got %v", n)
	}
}

func TestNormalize_PreservesDirection(t *testing.T) {
	// After normalisation, cosine similarity to original must be 1.0.
	v := core.SparseVector{1: 3.0, 2: 4.0}
	n := core.Normalize(v)
	got := core.Cosine(v, n)
	if !approxEqualTol(got, 1.0, 1e-9) {
		t.Errorf("Cosine(v, Normalize(v)) = %f, want 1.0", got)
	}
}

func TestNormalize_DoesNotMutateInput(t *testing.T) {
	v := core.SparseVector{1: 3.0, 2: 4.0}
	_ = core.Normalize(v)
	if v[1] != 3.0 || v[2] != 4.0 {
		t.Error("Normalize mutated input vector")
	}
}

func TestNormalize_IdempotentOnUnitVector(t *testing.T) {
	v := core.SparseVector{1: 0.6, 2: 0.8} // already unit
	n1 := core.Normalize(v)
	n2 := core.Normalize(n1)
	if !approxEqualTol(n1[1], n2[1], 1e-12) || !approxEqualTol(n1[2], n2[2], 1e-12) {
		t.Error("Normalize of already-unit vector should be stable")
	}
}

// ─── Filter ───────────────────────────────────────────────────────────────────

func TestFilter_RemovesBelowThreshold(t *testing.T) {
	v := core.SparseVector{1: 0.9, 2: 0.05, 3: 0.3}
	f := core.Filter(v, 0.1)
	if _, exists := f[2]; exists {
		t.Error("Filter: feature below threshold should be removed")
	}
	if f[1] != 0.9 || f[3] != 0.3 {
		t.Error("Filter: features above threshold should be preserved")
	}
}

func TestFilter_ExactThreshold_Kept(t *testing.T) {
	// Feature at exactly θ should be kept (≥ not >).
	v := core.SparseVector{1: 0.1}
	f := core.Filter(v, 0.1)
	if _, exists := f[1]; !exists {
		t.Error("Filter: feature at exactly θ should be retained")
	}
}

func TestFilter_NegativeWeightAbsoluteComparison(t *testing.T) {
	// A feature with weight -0.5 and threshold 0.3 should be kept (|−0.5| ≥ 0.3).
	v := core.SparseVector{1: -0.5}
	f := core.Filter(v, 0.3)
	if _, exists := f[1]; !exists {
		t.Error("Filter should use absolute weight for threshold comparison")
	}
	if f[1] != -0.5 {
		t.Errorf("Filter preserved wrong value: %f, want -0.5", f[1])
	}
}

func TestFilter_ZeroThreshold_KeepsAll(t *testing.T) {
	v := core.SparseVector{1: 0.001, 2: 0.999}
	f := core.Filter(v, 0.0)
	if len(f) != len(v) {
		t.Errorf("Filter(θ=0): got %d features, want %d", len(f), len(v))
	}
}

func TestFilter_DoesNotMutateInput(t *testing.T) {
	v := core.SparseVector{1: 0.9, 2: 0.05}
	_ = core.Filter(v, 0.1)
	if v[2] != 0.05 {
		t.Error("Filter mutated input vector")
	}
}

func TestFilter_EmptyInput(t *testing.T) {
	f := core.Filter(core.SparseVector{}, 0.1)
	if len(f) != 0 {
		t.Error("Filter of empty vector should be empty")
	}
}

func TestFilter_AllRemoved(t *testing.T) {
	v := core.SparseVector{1: 0.01, 2: 0.02}
	f := core.Filter(v, 1.0)
	if len(f) != 0 {
		t.Errorf("Filter(θ=1.0): expected empty, got %d features", len(f))
	}
}
