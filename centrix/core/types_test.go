package core_test

import (
	"testing"

	"github.com/leraniode/xgo/centrix/core"
)

// ─── SparseVector ─────────────────────────────────────────────────────────────

func TestSparseVector_Len(t *testing.T) {
	v := core.SparseVector{1: 1.0, 2: 0.5, 3: 0.25}
	if v.Len() != 3 {
		t.Errorf("Len() = %d, want 3", v.Len())
	}
}

func TestSparseVector_Clone_Independence(t *testing.T) {
	original := core.SparseVector{1: 1.0, 2: 0.5}
	clone := original.Clone()

	clone[1] = 99.0
	clone[3] = 0.1

	if original[1] != 1.0 {
		t.Error("mutating clone changed original feature weight")
	}
	if _, exists := original[3]; exists {
		t.Error("adding to clone added to original")
	}
}

func TestSparseVector_Clone_ContentsMatch(t *testing.T) {
	v := core.SparseVector{1: 0.9, 42: 0.3}
	c := v.Clone()
	if c[1] != 0.9 || c[42] != 0.3 {
		t.Error("clone contents do not match original")
	}
}

// ─── Action ───────────────────────────────────────────────────────────────────

func TestAction_String_AllConstants(t *testing.T) {
	cases := []struct {
		action core.Action
		want   string
	}{
		{core.Generated, "Generated"},
		{core.Matched, "Matched"},
		{core.Propagated, "Propagated"},
		{core.Attenuated, "Attenuated"},
		{core.Composed, "Composed"},
		{core.Filtered, "Filtered"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.action.String(); got != tc.want {
				t.Errorf("Action.String() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAction_Unknown(t *testing.T) {
	var unknown core.Action = 99
	if s := unknown.String(); s == "" {
		t.Error("unknown Action should produce a non-empty string")
	}
}

// ─── ComposeMode ──────────────────────────────────────────────────────────────

func TestComposeMode_ZeroIsIndependent(t *testing.T) {
	var mode core.ComposeMode
	if mode != core.Independent {
		t.Error("zero value of ComposeMode should be Independent")
	}
}

func TestComposeMode_Distinct(t *testing.T) {
	if core.Independent == core.Correlated {
		t.Error("Independent and Correlated must be distinct constants")
	}
}

// ─── Trace ────────────────────────────────────────────────────────────────────

func TestTrace_Add_And_Len(t *testing.T) {
	tr := core.NewTrace(core.DefaultTraceCap)
	tr.Add(core.Step{Node: "n1", Action: core.Generated})
	if tr.Len() != 1 {
		t.Errorf("Len() after one Add = %d, want 1", tr.Len())
	}
}

func TestTrace_Last_NonEmpty(t *testing.T) {
	tr := core.NewTrace(core.DefaultTraceCap)
	tr.Add(core.Step{Node: "a", Action: core.Generated})
	tr.Add(core.Step{Node: "b", Action: core.Composed})

	last, ok := tr.Last()
	if !ok {
		t.Fatal("Last() returned false on non-empty trace")
	}
	if last.Node != "b" {
		t.Errorf("Last().Node = %q, want %q", last.Node, "b")
	}
}

func TestTrace_Last_Empty(t *testing.T) {
	tr := core.NewTrace(core.DefaultTraceCap)
	_, ok := tr.Last()
	if ok {
		t.Error("Last() on empty trace should return false")
	}
}

func TestTrace_Cap_SlidingWindow(t *testing.T) {
	tr := core.NewTrace(3)
	for i := 0; i < 4; i++ {
		tr.Add(core.Step{Node: "n", Value: i, Action: core.Filtered})
	}
	if tr.Len() != 3 {
		t.Errorf("trace length after overflow = %d, want 3", tr.Len())
	}
	steps := tr.Steps()
	if steps[0].Value.(int) == 0 {
		t.Error("oldest step should have been dropped by sliding window")
	}
	last, _ := tr.Last()
	if last.Value.(int) != 3 {
		t.Errorf("last step value = %v, want 3", last.Value)
	}
}

func TestTrace_Monotonicity(t *testing.T) {
	tr := core.NewTrace(10)
	for i := 0; i < 10; i++ {
		prev := tr.Len()
		tr.Add(core.Step{Node: "n", Action: core.Propagated})
		if tr.Len() < prev {
			t.Errorf("trace length decreased from %d at step %d", prev, i)
		}
	}
}

func TestTrace_Steps_Order(t *testing.T) {
	tr := core.NewTrace(core.DefaultTraceCap)
	tr.Add(core.Step{Node: "x", Action: core.Generated})
	tr.Add(core.Step{Node: "y", Action: core.Filtered})
	steps := tr.Steps()
	if len(steps) != 2 || steps[0].Node != "x" || steps[1].Node != "y" {
		t.Error("Steps() did not return expected ordered steps")
	}
}

// ─── Signal ───────────────────────────────────────────────────────────────────

func TestNewSignal_ZeroState(t *testing.T) {
	s := core.NewSignal(10)
	if s.Confidence != 0 {
		t.Errorf("new signal confidence = %f, want 0", s.Confidence)
	}
	if s.Trace.Len() != 0 {
		t.Errorf("new signal trace len = %d, want 0", s.Trace.Len())
	}
	if s.Vector.Len() != 0 {
		t.Errorf("new signal vector len = %d, want 0", s.Vector.Len())
	}
}

func TestNewSignalFromVector_ConfidenceClamped(t *testing.T) {
	v := core.SparseVector{1: 1.0}

	over := core.NewSignalFromVector(v, 1.5)
	if over.Confidence != 1.0 {
		t.Errorf("confidence above 1.0 should clamp to 1.0, got %f", over.Confidence)
	}
	under := core.NewSignalFromVector(v, -0.5)
	if under.Confidence != 0.0 {
		t.Errorf("confidence below 0.0 should clamp to 0.0, got %f", under.Confidence)
	}
}

func TestSignal_Clone_Independence(t *testing.T) {
	s := core.NewSignal(4)
	s.Vector[1] = 0.9
	s.Confidence = 0.7
	s.Trace.Add(core.Step{Node: "orig", Action: core.Generated})

	clone := s.Clone()
	clone.Vector[1] = 0.1
	clone.Vector[2] = 0.5
	clone.Confidence = 0.3
	clone.Trace.Add(core.Step{Node: "clone", Action: core.Filtered})

	if s.Vector[1] != 0.9 {
		t.Error("cloning did not produce independent Vector")
	}
	if _, exists := s.Vector[2]; exists {
		t.Error("adding to clone Vector affected original")
	}
	if s.Confidence != 0.7 {
		t.Error("mutating clone Confidence affected original")
	}
	if s.Trace.Len() != 1 {
		t.Error("adding to clone Trace affected original")
	}
}

func TestSignal_WithStep_Immutability(t *testing.T) {
	s := core.NewSignal(4)
	s.Vector[1] = 1.0

	s2 := s.WithStep(core.Step{Node: "node1", Action: core.Matched})

	if s.Trace.Len() != 0 {
		t.Error("WithStep mutated original signal trace")
	}
	if s2.Trace.Len() != 1 {
		t.Errorf("s2 trace len = %d, want 1", s2.Trace.Len())
	}
	last, _ := s2.Trace.Last()
	if last.Node != "node1" {
		t.Errorf("last step node = %q, want %q", last.Node, "node1")
	}
}

func TestSignal_ConfidenceBounds(t *testing.T) {
	v := core.SparseVector{1: 1.0}
	s := core.NewSignalFromVector(v, 0.75)
	if s.Confidence < 0 || s.Confidence > 1 {
		t.Errorf("confidence out of [0,1]: %f", s.Confidence)
	}
}

// ─── Prototype ────────────────────────────────────────────────────────────────

func TestPrototype_WeightClamped(t *testing.T) {
	v := core.SparseVector{1: 1.0}

	high := core.NewPrototype(v, 2.0)
	if high.Weight != 1.0 {
		t.Errorf("weight above 1.0 should clamp to 1.0, got %f", high.Weight)
	}
	low := core.NewPrototype(v, -1.0)
	if low.Weight != 0.0 {
		t.Errorf("weight below 0.0 should clamp to 0.0, got %f", low.Weight)
	}
}

func TestPrototype_VectorPreserved(t *testing.T) {
	v := core.SparseVector{7: 0.8, 42: 0.4}
	p := core.NewPrototype(v, 0.9)
	if p.Vector[7] != 0.8 || p.Vector[42] != 0.4 {
		t.Error("Prototype vector contents not preserved")
	}
}
