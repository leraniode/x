package core_test

import (
	"testing"

	"github.com/leraniode/x/centrix/core"
)

// ─── Generate ─────────────────────────────────────────────────────────────────

func TestGenerate_AddsAbsentFeatures(t *testing.T) {
	// Query has feature 1; proto has feature 1 and 2.
	// Feature 2 is absent in query → must appear in output.
	s := core.NewSignalFromVector(core.SparseVector{1: 0.8}, 0.5)
	proto := core.NewPrototype(core.SparseVector{1: 0.9, 2: 0.7}, 0.8)

	out := core.Generate(s, proto, 0.0, "gen-node")

	if _, ok := out.Vector[2]; !ok {
		t.Error("Generate: absent proto feature should appear in output")
	}
}

func TestGenerate_DoesNotOverwritePresentFeatures(t *testing.T) {
	// Feature 1 is present in query — Generate must not touch it.
	s := core.NewSignalFromVector(core.SparseVector{1: 0.3}, 0.5)
	proto := core.NewPrototype(core.SparseVector{1: 0.9, 2: 0.6}, 0.8)

	out := core.Generate(s, proto, 0.0, "gen-node")

	if out.Vector[1] != 0.3 {
		t.Errorf("Generate overwrote present feature: got %f, want 0.3", out.Vector[1])
	}
}

func TestGenerate_ScalesByCosine(t *testing.T) {
	// Identical direction → Cosine = 1.0 → generated weight = proto weight × 1.
	s := core.NewSignalFromVector(core.SparseVector{1: 0.5}, 0.0)
	proto := core.NewPrototype(core.SparseVector{1: 0.5, 2: 0.8}, 0.9)

	out := core.Generate(s, proto, 0.0, "gen-node")

	sim := core.Cosine(s.Vector, proto.Vector)
	want := proto.Vector[2] * sim
	if !approxEqualTol(out.Vector[2], want, 1e-9) {
		t.Errorf("Generate: output weight = %f, want %f", out.Vector[2], want)
	}
}

func TestGenerate_NegativeCosine_NoFeatures(t *testing.T) {
	// Opposite direction — Cosine < 0 → no features generated.
	s := core.NewSignalFromVector(core.SparseVector{1: 1.0}, 0.5)
	proto := core.NewPrototype(core.SparseVector{1: -1.0, 2: 0.9}, 0.8)

	out := core.Generate(s, proto, 0.0, "gen-node")

	// Feature 2 must not appear — negative cosine, no generation.
	if _, ok := out.Vector[2]; ok {
		t.Error("Generate: negative Cosine should produce no new features")
	}
}

func TestGenerate_ThresholdFiltersProtoFeatures(t *testing.T) {
	// Proto feature 2 has weight 0.05, θ=0.1 → must not be generated.
	s := core.NewSignalFromVector(core.SparseVector{1: 0.8}, 0.0)
	proto := core.NewPrototype(core.SparseVector{1: 0.8, 2: 0.05}, 0.8)

	out := core.Generate(s, proto, 0.1, "gen-node")

	if _, ok := out.Vector[2]; ok {
		t.Error("Generate: proto feature below θ should not be emitted")
	}
}

func TestGenerate_DoesNotMutateInput(t *testing.T) {
	s := core.NewSignalFromVector(core.SparseVector{1: 0.5}, 0.4)
	proto := core.NewPrototype(core.SparseVector{1: 0.5, 2: 0.7}, 0.8)
	_ = core.Generate(s, proto, 0.0, "gen-node")

	if s.Vector[1] != 0.5 {
		t.Error("Generate mutated input signal vector")
	}
	if s.Trace.Len() != 0 {
		t.Error("Generate mutated input signal trace")
	}
}

func TestGenerate_AppendsStep(t *testing.T) {
	s := core.NewSignalFromVector(core.SparseVector{1: 0.5}, 0.4)
	proto := core.NewPrototype(core.SparseVector{1: 0.5, 2: 0.7}, 0.8)

	out := core.Generate(s, proto, 0.0, "gen-node")

	if out.Trace.Len() != s.Trace.Len()+1 {
		t.Errorf("Generate: trace len = %d, want %d", out.Trace.Len(), s.Trace.Len()+1)
	}
	last, _ := out.Trace.Last()
	if last.Action != core.Generated {
		t.Errorf("Generate: step action = %v, want Generated", last.Action)
	}
	if last.Node != "gen-node" {
		t.Errorf("Generate: step node = %q, want %q", last.Node, "gen-node")
	}
}

func TestGenerate_ConfidenceGate_Passes(t *testing.T) {
	// All four gate conditions met: non-empty vector, named node,
	// matchScore (cosine) > 0.15, protoWeight > 0.30.
	s := core.NewSignalFromVector(core.SparseVector{1: 0.8}, 0.5)
	proto := core.NewPrototype(core.SparseVector{1: 0.8, 2: 0.6}, 0.9)

	out := core.Generate(s, proto, 0.0, "gen-node")

	last, _ := out.Trace.Last()
	if last.ConfidenceBefore == last.ConfidenceAfter {
		t.Error("Generate: confidence should have updated when gate conditions are met")
	}
}

func TestGenerate_ConfidenceGate_EmptyNode(t *testing.T) {
	// Empty node → gate fails → confidence unchanged.
	s := core.NewSignalFromVector(core.SparseVector{1: 0.8}, 0.5)
	proto := core.NewPrototype(core.SparseVector{1: 0.8, 2: 0.6}, 0.9)

	out := core.Generate(s, proto, 0.0, "")

	last, _ := out.Trace.Last()
	if last.ConfidenceBefore != last.ConfidenceAfter {
		t.Error("Generate: empty node should gate confidence update")
	}
}

// ─── Compose ──────────────────────────────────────────────────────────────────

func TestCompose_VectorIsUnion(t *testing.T) {
	a := core.NewSignalFromVector(core.SparseVector{1: 0.5}, 0.6)
	b := core.NewSignalFromVector(core.SparseVector{2: 0.7}, 0.4)

	out := core.Compose(a, b, core.Independent, "compose-node")

	if !approxEqual(out.Vector[1], 0.5) || !approxEqual(out.Vector[2], 0.7) {
		t.Errorf("Compose: vector union incorrect: %v", out.Vector)
	}
}

func TestCompose_SharedFeaturesSum(t *testing.T) {
	a := core.NewSignalFromVector(core.SparseVector{1: 0.3}, 0.6)
	b := core.NewSignalFromVector(core.SparseVector{1: 0.4}, 0.4)

	out := core.Compose(a, b, core.Independent, "compose-node")

	if !approxEqual(out.Vector[1], 0.7) {
		t.Errorf("Compose: shared feature should sum weights, got %f want 0.7", out.Vector[1])
	}
}

func TestCompose_Independent_ORConfidence(t *testing.T) {
	// OR-combination: 1 − (1−0.6)(1−0.4) = 1 − 0.4×0.6 = 1 − 0.24 = 0.76
	a := core.NewSignalFromVector(core.SparseVector{1: 1.0}, 0.6)
	b := core.NewSignalFromVector(core.SparseVector{2: 1.0}, 0.4)

	out := core.Compose(a, b, core.Independent, "compose-node")

	want := 1 - (1-0.6)*(1-0.4)
	if !approxEqualTol(out.Confidence, want, 1e-9) {
		t.Errorf("Compose Independent confidence = %f, want %f", out.Confidence, want)
	}
}

func TestCompose_Correlated_MaxConfidence(t *testing.T) {
	a := core.NewSignalFromVector(core.SparseVector{1: 1.0}, 0.6)
	b := core.NewSignalFromVector(core.SparseVector{2: 1.0}, 0.9)

	out := core.Compose(a, b, core.Correlated, "compose-node")

	if !approxEqual(out.Confidence, 0.9) {
		t.Errorf("Compose Correlated confidence = %f, want 0.9 (max)", out.Confidence)
	}
}

func TestCompose_TraceOrder(t *testing.T) {
	// Trace must be: a's steps, then b's steps, then Compose step.
	a := core.NewSignal(4)
	a.Trace.Add(core.Step{Node: "a1", Action: core.Generated})

	b := core.NewSignal(4)
	b.Trace.Add(core.Step{Node: "b1", Action: core.Filtered})

	out := core.Compose(a, b, core.Independent, "compose-node")

	steps := out.Trace.Steps()
	if len(steps) != 3 {
		t.Fatalf("Compose trace len = %d, want 3", len(steps))
	}
	if steps[0].Node != "a1" {
		t.Errorf("Compose: step[0] node = %q, want a1", steps[0].Node)
	}
	if steps[1].Node != "b1" {
		t.Errorf("Compose: step[1] node = %q, want b1", steps[1].Node)
	}
	if steps[2].Action != core.Composed {
		t.Errorf("Compose: last step action = %v, want Composed", steps[2].Action)
	}
}

func TestCompose_ComposeStepAlwaysLast(t *testing.T) {
	// Fill both traces to near-cap, verify Compose step is still last after cap.
	a := core.NewSignal(4)
	b := core.NewSignal(4)
	for i := 0; i < 40; i++ {
		a.Trace.Add(core.Step{Node: "a", Action: core.Generated})
		b.Trace.Add(core.Step{Node: "b", Action: core.Filtered})
	}

	out := core.Compose(a, b, core.Independent, "compose-node")

	last, ok := out.Trace.Last()
	if !ok {
		t.Fatal("Compose trace is empty")
	}
	if last.Action != core.Composed {
		t.Errorf("Compose step must always be last, got %v", last.Action)
	}
}

func TestCompose_DoesNotMutateInputs(t *testing.T) {
	a := core.NewSignalFromVector(core.SparseVector{1: 0.5}, 0.6)
	b := core.NewSignalFromVector(core.SparseVector{2: 0.7}, 0.4)

	_ = core.Compose(a, b, core.Independent, "compose-node")

	if a.Vector[1] != 0.5 || a.Trace.Len() != 0 {
		t.Error("Compose mutated signal a")
	}
	if b.Vector[2] != 0.7 || b.Trace.Len() != 0 {
		t.Error("Compose mutated signal b")
	}
}

func TestCompose_ConfidenceClamped(t *testing.T) {
	// OR-combination of two high-confidence signals must stay ≤ 1.0.
	a := core.NewSignalFromVector(core.SparseVector{1: 1.0}, 0.95)
	b := core.NewSignalFromVector(core.SparseVector{2: 1.0}, 0.95)

	out := core.Compose(a, b, core.Independent, "compose-node")

	if out.Confidence > 1.0 {
		t.Errorf("Compose confidence exceeded 1.0: %f", out.Confidence)
	}
}

// ─── Attenuate ────────────────────────────────────────────────────────────────

func TestAttenuate_DecaysWeights(t *testing.T) {
	// λ=0.3 → factor=0.7 → weight 1.0 becomes 0.7
	s := core.NewSignalFromVector(core.SparseVector{1: 1.0}, 0.5)
	out := core.Attenuate(s, 0.3, "att-node")

	if !approxEqualTol(out.Vector[1], 0.7, 1e-9) {
		t.Errorf("Attenuate: weight = %f, want 0.7", out.Vector[1])
	}
}

func TestAttenuate_ZeroLambda_NoChange(t *testing.T) {
	s := core.NewSignalFromVector(core.SparseVector{1: 0.8, 2: 0.4}, 0.5)
	out := core.Attenuate(s, 0.0, "att-node")

	if !approxEqual(out.Vector[1], 0.8) || !approxEqual(out.Vector[2], 0.4) {
		t.Error("Attenuate(λ=0): weights should be unchanged")
	}
}

func TestAttenuate_FullDecay_ZeroWeights(t *testing.T) {
	s := core.NewSignalFromVector(core.SparseVector{1: 0.8}, 0.5)
	out := core.Attenuate(s, 1.0, "att-node")

	if len(out.Vector) != 0 {
		t.Errorf("Attenuate(λ=1): vector should be empty (all weights zero), got %v", out.Vector)
	}
}

func TestAttenuate_LambdaClamped(t *testing.T) {
	s := core.NewSignalFromVector(core.SparseVector{1: 1.0}, 0.5)
	// λ > 1 should clamp to 1 → full decay
	out := core.Attenuate(s, 5.0, "att-node")
	if len(out.Vector) != 0 {
		t.Error("Attenuate(λ>1): should clamp to 1.0 and zero all weights")
	}
}

func TestAttenuate_ConfidenceUnchanged(t *testing.T) {
	s := core.NewSignalFromVector(core.SparseVector{1: 1.0}, 0.7)
	out := core.Attenuate(s, 0.3, "att-node")

	if out.Confidence != 0.7 {
		t.Errorf("Attenuate must not change confidence: got %f, want 0.7", out.Confidence)
	}
	last, _ := out.Trace.Last()
	if last.ConfidenceBefore != last.ConfidenceAfter {
		t.Error("Attenuate: Step confidence delta should be zero")
	}
}

func TestAttenuate_AppendsStep(t *testing.T) {
	s := core.NewSignalFromVector(core.SparseVector{1: 1.0}, 0.5)
	out := core.Attenuate(s, 0.3, "att-node")

	if out.Trace.Len() != 1 {
		t.Errorf("Attenuate: trace len = %d, want 1", out.Trace.Len())
	}
	last, _ := out.Trace.Last()
	if last.Action != core.Attenuated {
		t.Errorf("Attenuate: step action = %v, want Attenuated", last.Action)
	}
}

func TestAttenuate_DoesNotMutateInput(t *testing.T) {
	s := core.NewSignalFromVector(core.SparseVector{1: 1.0}, 0.5)
	_ = core.Attenuate(s, 0.3, "att-node")

	if s.Vector[1] != 1.0 {
		t.Error("Attenuate mutated input signal vector")
	}
	if s.Trace.Len() != 0 {
		t.Error("Attenuate mutated input signal trace")
	}
}

func TestAttenuate_PreservesSparsity(t *testing.T) {
	// After decay, zero-weight features must be absent (Invariant 3).
	s := core.NewSignalFromVector(core.SparseVector{1: 1e-300}, 0.5)
	out := core.Attenuate(s, 1.0, "att-node")
	for f, w := range out.Vector {
		if w == 0 {
			t.Errorf("Attenuate: zero-weight feature %d stored in output (sparsity violation)", f)
		}
	}
}

// ─── FilterSignal ─────────────────────────────────────────────────────────────

func TestFilterSignal_RemovesBelowThreshold(t *testing.T) {
	s := core.NewSignalFromVector(core.SparseVector{1: 0.9, 2: 0.05, 3: 0.3}, 0.5)
	out := core.FilterSignal(s, 0.1, "filter-node")

	if _, ok := out.Vector[2]; ok {
		t.Error("FilterSignal: feature below θ should be removed")
	}
	if !approxEqual(out.Vector[1], 0.9) || !approxEqual(out.Vector[3], 0.3) {
		t.Error("FilterSignal: features above θ should be preserved")
	}
}

func TestFilterSignal_ConfidenceUnchanged(t *testing.T) {
	s := core.NewSignalFromVector(core.SparseVector{1: 0.9, 2: 0.05}, 0.6)
	out := core.FilterSignal(s, 0.1, "filter-node")

	if out.Confidence != 0.6 {
		t.Errorf("FilterSignal must not change confidence: got %f, want 0.6", out.Confidence)
	}
}

func TestFilterSignal_AppendsStep(t *testing.T) {
	s := core.NewSignalFromVector(core.SparseVector{1: 0.9}, 0.5)
	out := core.FilterSignal(s, 0.1, "filter-node")

	if out.Trace.Len() != 1 {
		t.Errorf("FilterSignal: trace len = %d, want 1", out.Trace.Len())
	}
	last, _ := out.Trace.Last()
	if last.Action != core.Filtered {
		t.Errorf("FilterSignal: step action = %v, want Filtered", last.Action)
	}
}

func TestFilterSignal_DoesNotMutateInput(t *testing.T) {
	s := core.NewSignalFromVector(core.SparseVector{1: 0.9, 2: 0.05}, 0.5)
	_ = core.FilterSignal(s, 0.1, "filter-node")

	if s.Vector[2] != 0.05 {
		t.Error("FilterSignal mutated input signal vector")
	}
	if s.Trace.Len() != 0 {
		t.Error("FilterSignal mutated input signal trace")
	}
}

// ─── Propagate ────────────────────────────────────────────────────────────────

func TestPropagate_EnrichesVector(t *testing.T) {
	// Signal and proto overlap on feature 1 → Dot > 0 → proto features absorbed.
	s := core.NewSignalFromVector(core.SparseVector{1: 0.8}, 0.5)
	proto := core.NewPrototype(core.SparseVector{1: 0.8, 2: 0.6}, 0.8)

	out := core.Propagate(s, proto, 0.1, "prop-node")

	// Feature 1 should grow; feature 2 should appear.
	if out.Vector[1] <= s.Vector[1] {
		t.Errorf("Propagate: existing feature should grow, got %f want > %f", out.Vector[1], s.Vector[1])
	}
	if _, ok := out.Vector[2]; !ok {
		t.Error("Propagate: proto feature should be absorbed into signal")
	}
}

func TestPropagate_ZeroDot_NoChange(t *testing.T) {
	// Orthogonal signal and proto → Dot = 0 → no energy transfer.
	s := core.NewSignalFromVector(core.SparseVector{1: 1.0}, 0.5)
	proto := core.NewPrototype(core.SparseVector{2: 1.0}, 0.8)

	out := core.Propagate(s, proto, 0.1, "prop-node")

	if _, ok := out.Vector[2]; ok {
		t.Error("Propagate: zero Dot should produce no energy transfer")
	}
	if !approxEqual(out.Vector[1], s.Vector[1]) {
		t.Error("Propagate: zero Dot should leave existing features unchanged")
	}
}

func TestPropagate_UsesAlphaCoefficient(t *testing.T) {
	// Manual calculation: Dot({1:1},{1:1,2:0.5}) = 1.0
	// Feature 1: 1.0 + 0.1 × 1.0 × 1.0 = 1.1
	// Feature 2: 0.0 + 0.1 × 1.0 × 0.5 = 0.05
	s := core.NewSignalFromVector(core.SparseVector{1: 1.0}, 0.0)
	proto := core.NewPrototype(core.SparseVector{1: 1.0, 2: 0.5}, 0.9)

	out := core.Propagate(s, proto, 0.1, "prop-node")

	if !approxEqualTol(out.Vector[1], 1.1, 1e-9) {
		t.Errorf("Propagate feature 1: got %f, want 1.1", out.Vector[1])
	}
	if !approxEqualTol(out.Vector[2], 0.05, 1e-9) {
		t.Errorf("Propagate feature 2: got %f, want 0.05", out.Vector[2])
	}
}

func TestPropagate_ConfidenceGate_Passes(t *testing.T) {
	s := core.NewSignalFromVector(core.SparseVector{1: 0.8}, 0.3)
	proto := core.NewPrototype(core.SparseVector{1: 0.8, 2: 0.6}, 0.9)

	out := core.Propagate(s, proto, 0.1, "prop-node")

	last, _ := out.Trace.Last()
	if last.ConfidenceBefore == last.ConfidenceAfter {
		t.Error("Propagate: confidence should update when gate conditions are met")
	}
}

func TestPropagate_ConfidenceGate_LowDot(t *testing.T) {
	// Very weak overlap → Dot ≤ 0.15 → gate fails.
	s := core.NewSignalFromVector(core.SparseVector{1: 0.1}, 0.5)
	proto := core.NewPrototype(core.SparseVector{1: 0.1}, 0.9)

	out := core.Propagate(s, proto, 0.1, "prop-node")

	last, _ := out.Trace.Last()
	if last.ConfidenceBefore != last.ConfidenceAfter {
		t.Error("Propagate: low Dot score should gate confidence update")
	}
}

func TestPropagate_AppendsStep(t *testing.T) {
	s := core.NewSignalFromVector(core.SparseVector{1: 0.8}, 0.5)
	proto := core.NewPrototype(core.SparseVector{1: 0.8}, 0.8)

	out := core.Propagate(s, proto, 0.1, "prop-node")

	if out.Trace.Len() != 1 {
		t.Errorf("Propagate: trace len = %d, want 1", out.Trace.Len())
	}
	last, _ := out.Trace.Last()
	if last.Action != core.Propagated {
		t.Errorf("Propagate: step action = %v, want Propagated", last.Action)
	}
}

func TestPropagate_DoesNotMutateInput(t *testing.T) {
	s := core.NewSignalFromVector(core.SparseVector{1: 0.8}, 0.5)
	proto := core.NewPrototype(core.SparseVector{1: 0.8, 2: 0.6}, 0.8)

	_ = core.Propagate(s, proto, 0.1, "prop-node")

	if s.Vector[1] != 0.8 {
		t.Error("Propagate mutated input signal vector")
	}
	if _, ok := s.Vector[2]; ok {
		t.Error("Propagate added proto feature to input signal")
	}
	if s.Trace.Len() != 0 {
		t.Error("Propagate mutated input signal trace")
	}
}

func TestPropagate_PreservesSparsity(t *testing.T) {
	// No zero weights in output.
	s := core.NewSignalFromVector(core.SparseVector{1: 0.8}, 0.5)
	proto := core.NewPrototype(core.SparseVector{1: 0.8, 2: 0.6}, 0.8)
	out := core.Propagate(s, proto, 0.1, "prop-node")

	for f, w := range out.Vector {
		if w == 0 {
			t.Errorf("Propagate: zero-weight feature %d in output (sparsity violation)", f)
		}
	}
}

// ─── Confidence gate (shared across ops) ─────────────────────────────────────

func TestConfidenceGate_AllConditionsMet_Updates(t *testing.T) {
	// Use Generate as the gate vehicle — easiest to fully control.
	s := core.NewSignalFromVector(core.SparseVector{1: 0.9}, 0.2)
	proto := core.NewPrototype(core.SparseVector{1: 0.9, 2: 0.7}, 0.9)

	out := core.Generate(s, proto, 0.0, "node")

	last, _ := out.Trace.Last()
	if last.ConfidenceBefore == last.ConfidenceAfter {
		t.Error("confidence should have updated: all gate conditions met")
	}
	if out.Confidence < 0 || out.Confidence > 1 {
		t.Errorf("confidence out of [0,1]: %f", out.Confidence)
	}
}

func TestConfidenceGate_EmptyVector_Blocked(t *testing.T) {
	s := core.NewSignal(4) // empty vector
	proto := core.NewPrototype(core.SparseVector{1: 0.9}, 0.9)

	out := core.Generate(s, proto, 0.0, "node")

	last, _ := out.Trace.Last()
	if last.ConfidenceBefore != last.ConfidenceAfter {
		t.Error("confidence gate: empty vector should block update")
	}
}

func TestConfidenceGate_LowProtoWeight_Blocked(t *testing.T) {
	s := core.NewSignalFromVector(core.SparseVector{1: 0.9}, 0.2)
	// Weight ≤ 0.30 → gate blocked.
	proto := core.NewPrototype(core.SparseVector{1: 0.9, 2: 0.7}, 0.3)

	out := core.Generate(s, proto, 0.0, "node")

	last, _ := out.Trace.Last()
	if last.ConfidenceBefore != last.ConfidenceAfter {
		t.Error("confidence gate: proto weight ≤ 0.30 should block update")
	}
}

func TestConfidenceGate_MovesTowardTarget(t *testing.T) {
	// confidence_new = c_old + 0.10 × (score × weight − c_old)
	// score = Cosine({1:0.8}, {1:0.8, 2:0.6}) — verify direction of movement.
	s := core.NewSignalFromVector(core.SparseVector{1: 0.8}, 0.1)
	proto := core.NewPrototype(core.SparseVector{1: 0.8, 2: 0.6}, 0.9)

	out := core.Generate(s, proto, 0.0, "node")

	last, _ := out.Trace.Last()
	target := core.Cosine(s.Vector, proto.Vector) * proto.Weight
	expected := 0.1 + 0.10*(target-0.1)

	if !approxEqualTol(out.Confidence, expected, 1e-9) {
		t.Errorf("confidence update incorrect: got %f, want %f", out.Confidence, expected)
	}
	_ = last
}
