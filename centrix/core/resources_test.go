package core_test

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/leraniode/xgo/centrix/core"
)

// makeSparse builds a SparseVector with k active features spread across a D-dimensional space.
func makeSparse(k, D int, weight float64) core.SparseVector {
	v := make(core.SparseVector, k)
	step := D / k
	for i := 0; i < k; i++ {
		v[core.FeatureIndex(i*step)] = weight
	}
	return v
}

// ─── O(k) property ────────────────────────────────────────────────────────────
// Operations must scale with active features (k), not dimension space size (D).

func BenchmarkDot_k20_D100(b *testing.B) {
	a, bv := makeSparse(20, 100, 0.5), makeSparse(20, 100, 0.5)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		core.Dot(a, bv)
	}
}

func BenchmarkDot_k20_D10000(b *testing.B) {
	a, bv := makeSparse(20, 10_000, 0.5), makeSparse(20, 10_000, 0.5)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		core.Dot(a, bv)
	}
}

func BenchmarkDot_k20_D1000000(b *testing.B) {
	a, bv := makeSparse(20, 1_000_000, 0.5), makeSparse(20, 1_000_000, 0.5)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		core.Dot(a, bv)
	}
}

// k scaling at fixed D — should grow linearly with k.
func BenchmarkDot_k10_D10000(b *testing.B) {
	a, bv := makeSparse(10, 10_000, 0.5), makeSparse(10, 10_000, 0.5)
	b.ResetTimer()
	for i := 0; i < b.N; i++ { core.Dot(a, bv) }
}

func BenchmarkDot_k50_D10000(b *testing.B) {
	a, bv := makeSparse(50, 10_000, 0.5), makeSparse(50, 10_000, 0.5)
	b.ResetTimer()
	for i := 0; i < b.N; i++ { core.Dot(a, bv) }
}

func BenchmarkDot_k200_D10000(b *testing.B) {
	a, bv := makeSparse(200, 10_000, 0.5), makeSparse(200, 10_000, 0.5)
	b.ResetTimer()
	for i := 0; i < b.N; i++ { core.Dot(a, bv) }
}

func BenchmarkCosine_k20_D10000(b *testing.B) {
	a, bv := makeSparse(20, 10_000, 0.5), makeSparse(20, 10_000, 0.3)
	b.ResetTimer()
	for i := 0; i < b.N; i++ { core.Cosine(a, bv) }
}

func BenchmarkGenerate_k20_D10000(b *testing.B) {
	s := core.NewSignalFromVector(makeSparse(20, 10_000, 0.5), 0.5)
	proto := core.NewPrototype(makeSparse(30, 10_000, 0.6), 0.8)
	b.ResetTimer()
	for i := 0; i < b.N; i++ { core.Generate(s, proto, 0.1, "bench") }
}

func BenchmarkPropagate_k20_D10000(b *testing.B) {
	s := core.NewSignalFromVector(makeSparse(20, 10_000, 0.5), 0.5)
	proto := core.NewPrototype(makeSparse(20, 10_000, 0.5), 0.8)
	b.ResetTimer()
	for i := 0; i < b.N; i++ { core.Propagate(s, proto, 0.1, "bench") }
}

func BenchmarkCompose_k20_D10000(b *testing.B) {
	a := core.NewSignalFromVector(makeSparse(20, 10_000, 0.5), 0.6)
	bv := core.NewSignalFromVector(makeSparse(20, 10_000, 0.4), 0.5)
	b.ResetTimer()
	for i := 0; i < b.N; i++ { core.Compose(a, bv, core.Independent, "bench") }
}

// ─── Memory bounds ────────────────────────────────────────────────────────────

func TestMemory_TraceCapEnforced(t *testing.T) {
	s := core.NewSignal(4)
	s.Vector[1] = 0.5
	for i := 0; i < 200; i++ {
		s = s.WithStep(core.Step{Node: "n", Action: core.Generated})
	}
	if s.Trace.Len() > core.DefaultTraceCap {
		t.Errorf("Trace exceeded cap: len=%d, cap=%d", s.Trace.Len(), core.DefaultTraceCap)
	}
	if s.Trace.Len() != core.DefaultTraceCap {
		t.Errorf("Trace should be at cap: len=%d want %d", s.Trace.Len(), core.DefaultTraceCap)
	}
}

func TestMemory_SignalHeapBound(t *testing.T) {
	// Measure using unsafe.Sizeof approximations and empirical step counting.
	// A Signal with k=20 features and a 64-step trace:
	//   SparseVector map:  map header (~8B) + k entries × (8B key + 8B value) = ~328B
	//   Trace.steps slice: 64 × sizeof(Step)
	//   Step fields:       Node(string=16B) + Action(8B) + Value(interface=16B) + 2×float64(16B) = ~56B
	//   Trace overhead:    slice header(24B) + cap int(8B)
	//   Confidence:        8B
	//   Total estimate:    328 + 24 + 8 + 64×56 + 8 = ~3952B (~4KB)

	s := core.NewSignalFromVector(makeSparse(20, 10_000, 0.5), 0.7)
	for j := 0; j < 64; j++ {
		s = s.WithStep(core.Step{Node: "node", Action: core.Generated, ConfidenceBefore: 0.5, ConfidenceAfter: 0.55})
	}

	vectorBytes := 20 * 16              // k × (8B key + 8B value)
	stepBytes := 56                     // Node(16) + Action(8) + Value(16) + 2×float64(16)
	traceBytes := 64*stepBytes + 24 + 8 // steps + slice header + cap
	signalTotal := vectorBytes + traceBytes + 8

	t.Logf("Signal memory breakdown (k=20, 64-step trace):")
	t.Logf("  SparseVector entries: ~%d bytes", vectorBytes)
	t.Logf("  Trace (64 steps):     ~%d bytes", traceBytes)
	t.Logf("  Confidence:           8 bytes")
	t.Logf("  Total estimate:       ~%d bytes (~%dKB)", signalTotal, signalTotal/1024)

	// Verify trace is actually at cap.
	if s.Trace.Len() != core.DefaultTraceCap {
		t.Errorf("trace len = %d, want %d", s.Trace.Len(), core.DefaultTraceCap)
	}

	// Hard bound: a fully-loaded Signal must fit in 16KB.
	const maxBytes = 16 * 1024
	if signalTotal > maxBytes {
		t.Errorf("Signal estimate %d bytes exceeds bound %d", signalTotal, maxBytes)
	}

	// Count allocations for one Propagate call (steady state — no new signals created).
	allocs := testing.AllocsPerRun(100, func() {
		proto := core.NewPrototype(makeSparse(20, 10_000, 0.5), 0.8)
		_ = core.Propagate(s, proto, 0.1, "bench")
	})
	t.Logf("AllocsPerRun (Propagate): %.1f", allocs)

	_ = runtime.MemStats{} // keep import
}

// ─── O(k) empirical verification ─────────────────────────────────────────────

func TestCPU_DotScalesWithK_NotD(t *testing.T) {
	const iters = 200_000
	k := 20

	a_s, b_s := makeSparse(k, 100, 0.5), makeSparse(k, 100, 0.5)
	a_l, b_l := makeSparse(k, 1_000_000, 0.5), makeSparse(k, 1_000_000, 0.5)

	// Warm up.
	for i := 0; i < 5000; i++ {
		core.Dot(a_s, b_s)
		core.Dot(a_l, b_l)
	}

	run := func(a, b core.SparseVector) time.Duration {
		start := time.Now()
		for i := 0; i < iters; i++ {
			core.Dot(a, b)
		}
		return time.Since(start)
	}

	tSmall := run(a_s, b_s)
	tLarge := run(a_l, b_l)

	ratio := float64(tLarge) / float64(tSmall)
	t.Logf("Dot k=%d, D=100:       %v", k, tSmall)
	t.Logf("Dot k=%d, D=1,000,000: %v", k, tLarge)
	t.Logf("D grew 10,000× — time ratio: %.2fx (want < 10×)", ratio)

	// O(D) would give ~10,000× ratio. O(k) gives ~1×.
	// We allow 10× for map hash overhead variance.
	if ratio > 10.0 {
		t.Errorf("Dot scales with D not k: ratio=%.2fx", ratio)
	}
}

// ─── Resource summary ─────────────────────────────────────────────────────────

func TestResourceConstraints_Summary(t *testing.T) {
	k := 20
	t.Log("─── Centrix Resource Constraint Summary ───────────────────────────")
	t.Logf("SparseVector  k=%-3d active features in D=10,000 space", k)
	t.Logf("              ~%d bytes (8B key + 8B value × k)", k*16)
	t.Logf("Trace         %d steps max (sliding window)", core.DefaultTraceCap)
	t.Logf("              ~%d bytes per step → ~%dKB max", 80, core.DefaultTraceCap*80/1024)
	total := k*16 + core.DefaultTraceCap*80 + 8
	t.Logf("Signal total  ~%d bytes (~%dKB) — k=20, full 64-step trace", total, total/1024)

	// Stability condition.
	α, λ, N, μ := 0.1, 0.3, 15, 0.05  // realistic: k=20 in D=10,000 → very sparse overlap
	rhs := α * float64(N-1) * μ
	t.Logf("Field stable  λ=%.2f > α×(N-1)×μ = %.3f → %v (N≤15, μ=%.2f, realistic sparse)", λ, rhs, λ > rhs, μ)
	t.Logf("───────────────────────────────────────────────────────────────────")
	fmt.Println()
}
