// Package testutil provides lightweight test assertion helpers.

package testutil

import (
	"fmt"
	"math"
	"testing"
)

// Equal asserts that expected == actual using fmt.Sprintf comparison.
func Equal(t *testing.T, expected, actual interface{}, msgAndArgs ...interface{}) bool {
	t.Helper()
	if fmt.Sprintf("%v", expected) == fmt.Sprintf("%v", actual) {
		return true
	}
	t.Errorf("Not equal:\n  expected: %v\n  actual:   %v\n%s",
		expected, actual, formatMsg(msgAndArgs...))
	return false
}

// InDelta asserts that |expected - actual| <= delta.
func InDelta(t *testing.T, expected, actual, delta float64, msgAndArgs ...interface{}) bool {
	t.Helper()
	diff := math.Abs(expected - actual)
	if diff <= delta {
		return true
	}
	t.Errorf("Delta exceeded:\n  expected: %v\n  actual:   %v\n  diff: %v > delta: %v\n%s",
		expected, actual, diff, delta, formatMsg(msgAndArgs...))
	return false
}

// True asserts that condition is true.
func True(t *testing.T, condition bool, msgAndArgs ...interface{}) bool {
	t.Helper()
	if condition {
		return true
	}
	t.Errorf("Expected true\n%s", formatMsg(msgAndArgs...))
	return false
}

// False asserts that condition is false.
func False(t *testing.T, condition bool, msgAndArgs ...interface{}) bool {
	t.Helper()
	if !condition {
		return true
	}
	t.Errorf("Expected false\n%s", formatMsg(msgAndArgs...))
	return false
}

// Greater asserts that a > b.
func Greater(t *testing.T, a, b float64, msgAndArgs ...interface{}) bool {
	t.Helper()
	if a > b {
		return true
	}
	t.Errorf("Expected %v > %v\n%s", a, b, formatMsg(msgAndArgs...))
	return false
}

// Less asserts that a < b.
func Less(t *testing.T, a, b float64, msgAndArgs ...interface{}) bool {
	t.Helper()
	if a < b {
		return true
	}
	t.Errorf("Expected %v < %v\n%s", a, b, formatMsg(msgAndArgs...))
	return false
}

// GreaterOrEqual asserts that a >= b.
func GreaterOrEqual(t *testing.T, a, b float64, msgAndArgs ...interface{}) bool {
	t.Helper()
	if a >= b {
		return true
	}
	t.Errorf("Expected %v >= %v\n%s", a, b, formatMsg(msgAndArgs...))
	return false
}

// LessOrEqual asserts that a <= b.
func LessOrEqual(t *testing.T, a, b float64, msgAndArgs ...interface{}) bool {
	t.Helper()
	if a <= b {
		return true
	}
	t.Errorf("Expected %v <= %v\n%s", a, b, formatMsg(msgAndArgs...))
	return false
}

// Error asserts that err is non-nil.
func Error(t *testing.T, err error, msgAndArgs ...interface{}) bool {
	t.Helper()
	if err != nil {
		return true
	}
	t.Errorf("Expected an error but got nil\n%s", formatMsg(msgAndArgs...))
	return false
}

// NoError asserts that err is nil and fails the test immediately if not.
func NoError(t *testing.T, err error, msgAndArgs ...interface{}) bool {
	t.Helper()
	if err == nil {
		return true
	}
	t.Fatalf("Unexpected error: %v\n%s", err, formatMsg(msgAndArgs...))
	return false
}

func formatMsg(msgAndArgs ...interface{}) string {
	if len(msgAndArgs) == 0 {
		return ""
	}
	if format, ok := msgAndArgs[0].(string); ok && len(msgAndArgs) > 1 {
		return fmt.Sprintf(format, msgAndArgs[1:]...)
	}
	return fmt.Sprintf("%v", msgAndArgs[0])
}
