package math_test

import (
	"os"
	"testing"

	"golang_snacks/invariant"
	math "golang_snacks/invariant/examples/02_math"
)

func TestMain(m *testing.M) {
	invariant.RegisterPackagesForAnalysis()
	code := m.Run()
	invariant.AnalyzeAssertionFrequency()
	os.Exit(code)
}

func TestAdd(t *testing.T) {
	t.Parallel()

	cases := []struct {
		x, y, expected int
	}{
		{2, 3, 5},
		{2, -3, -1},
		{2, 0, 2},
		{0, 2, 2},
		{7, -7, 0},
		{-7, 7, 0},
	}

	for _, c := range cases {
		got := math.Add(c.x, c.y)
		if got != c.expected {
			t.Errorf("Add(%d, %d) = %d; want %d", c.x, c.y, got, c.expected)
		}
	}
}

func TestSubtract(t *testing.T) {
	t.Parallel()

	cases := []struct {
		a, b, expected int
	}{
		{5, 5, 0},
		{5, -7, 12},
		{5, 0, 5},
	}

	for _, c := range cases {
		got := math.Subtract(c.a, c.b)
		if got != c.expected {
			t.Errorf("Subtract(%d, %d) = %d; want %d", c.a, c.b, got, c.expected)
		}
	}
}

func TestMultiply(t *testing.T) {
	t.Parallel()

	cases := []struct {
		x, y, expected int
	}{
		// Zero properties
		{0, 0, 0},
		{0, 5, 0},
		{7, 0, 0},

		// Identity properties
		{1, 9, 9},
		{8, 1, 8},

		// Negation properties
		{-1, 3, -3},
		{4, -1, -4},

		// Sign properties
		{2, 3, 6},    // positive * positive
		{-2, -5, 10}, // negative * negative
		{6, -2, -12}, // positive * negative
		{-7, 3, -21}, // negative * positive
	}

	for _, c := range cases {
		got := math.Multiply(c.x, c.y)
		if got != c.expected {
			t.Errorf("Multiply(%d, %d) = %d; want %d", c.x, c.y, got, c.expected)
		}
	}
}
