package core_test

import (
	"math"
	"testing"

	"github.com/leraniode/x/centrix/core"
)

// detVec builds a vector whose weights are chosen so that the order of
// floating-point additions changes the low bits of a sum: terms of very
// different magnitude and non-representable fractions.
func detVec(n int, scale float64) core.SparseVector {
	v := make(core.SparseVector, n)
	for i := 1; i <= n; i++ {
		// Mix magnitudes across ~8 orders so that addition order matters.
		w := scale * math.Pow(10, float64(i%9)-4) / float64(i+2)
		v[core.FeatureIndex(i)] = w
	}
	return v
}

// sameBits reports whether two floats are bit-for-bit identical.
func sameBits(a, b float64) bool { return math.Float64bits(a) == math.Float64bits(b) }

// Invariant 1 (Deterministic Execution) requires bit-identical results for
// identical inputs. Go randomises map iteration, so any sum taken in map
// order can differ in its last bits from one call to the next.
func TestDot_BitIdenticalAcrossCalls(t *testing.T) {
	a := detVec(64, 1.0)
	b := detVec(64, 0.7)

	want := core.Dot(a, b)
	for i := 0; i < 500; i++ {
		if got := core.Dot(a, b); !sameBits(got, want) {
			t.Fatalf("Dot not bit-identical across calls: %v vs %v (iteration %d)", got, want, i)
		}
	}
}

func TestDot_Symmetric_BitIdentical(t *testing.T) {
	a := detVec(64, 1.0)
	b := detVec(64, 0.7)

	ab := core.Dot(a, b)
	for i := 0; i < 200; i++ {
		if ba := core.Dot(b, a); !sameBits(ab, ba) {
			t.Fatalf("Dot(a,b) != Dot(b,a) bitwise: %v vs %v", ab, ba)
		}
	}
}

func TestEnergy_BitIdenticalAcrossCalls(t *testing.T) {
	v := detVec(64, 1.0)
	want := core.Energy(v)
	for i := 0; i < 500; i++ {
		if got := core.Energy(v); !sameBits(got, want) {
			t.Fatalf("Energy not bit-identical: %v vs %v", got, want)
		}
	}
}

func TestCosine_BitIdenticalAcrossCalls(t *testing.T) {
	a := detVec(64, 1.0)
	b := detVec(64, 0.7)
	want := core.Cosine(a, b)
	for i := 0; i < 500; i++ {
		if got := core.Cosine(a, b); !sameBits(got, want) {
			t.Fatalf("Cosine not bit-identical: %v vs %v", got, want)
		}
	}
}

func TestNormalize_BitIdenticalAcrossCalls(t *testing.T) {
	v := detVec(64, 1.0)
	want := core.Normalize(v)
	for i := 0; i < 200; i++ {
		got := core.Normalize(v)
		for f, w := range want {
			if !sameBits(got[f], w) {
				t.Fatalf("Normalize feature %d not bit-identical: %v vs %v", f, got[f], w)
			}
		}
	}
}
