package math

import "github.com/james-orcales/golang_snacks/invariant"

func Add(x, y int) int {
	sum := x + y

	if x > 0 && y > 0 {
		invariant.Always(sum > max(x, y), "Sum is greater than the biggest addend when both addends are positive")
	}

	// Commutative Property
	invariant.Always(sum == x+y, "Addition is commutative")
	invariant.Always(sum == y+x, "Addition is commutative")

	// Identity Property
	if x == 0 {
		invariant.Always(sum == y, "Adding zero to a number should leave it unchanged")
	}
	if y == 0 {
		invariant.Always(sum == x, "Adding zero to a number should leave it unchanged")
	}

	// Inverse property
	if x == -y {
		invariant.Always(sum == 0, "Adding a number and its additive inverse should yield zero")
	}
	if y == -x {
		invariant.Always(sum == 0, "Adding a number and its additive inverse should yield zero")
	}

	return sum
}

func Subtract(subtrahend, minuend int) int {
	difference := subtrahend - minuend

	// Non-commutative: subtrahend - minuend
	//
	// Sometimes assertions only need a single example to succeed, while
	// Always assertions require a single counterexample to fail. Using a
	// combined Sometimes() like `invariant.Sometimes(multiplicand == 0 &&
	// product == 0, "")` would only exercise one specific example, which is
	// not what we want here.
	//
	// In this snippet, we are trying to establish the zero property for
	// multiplication, so we use Always assertions to ensure it holds in all
	// cases, not just one example.
	//
	// To ensure the Always assertion triggers at least once during testing,
	// we also include a `Sometimes(true)` call in the same scope. This
	// guarantees the assertion is exercised, albeit not extensively.
	//
	// By default, all assertions must be triggered in test runs at least
	// once. The explictit Sometimes() below demonstrates how it works under
	// the hood but is not required.
	if subtrahend != minuend {
		invariant.Sometimes(difference != minuend-subtrahend, "Subtrahend is not equal to minuend")
		invariant.Always(difference != minuend-subtrahend, "Subtraction is non-commutative")
	}

	// Non-associative: (subtrahend - minuend) - c != subtrahend - (minuend - c)
	c := 1
	invariant.Sometimes((subtrahend-minuend)-c != subtrahend-(minuend-c), "Subtraction is non-associative")

	// Identity element: subtrahend - 0 = subtrahend
	if minuend == 0 {
		invariant.Always(difference == subtrahend, "Subtracting zero leaves the number unchanged")
	}

	// Inverse of addition: subtrahend - minuend = subtrahend + (-minuend)
	invariant.Always(difference == Add(subtrahend, -minuend), "Subtraction equals addition of additive inverse")

	if minuend < 0 {
		invariant.Always(difference > subtrahend, "Subtrahend increases if minuend is negative")
	}

	return difference
}

func Multiply(multiplicand, multiplier int) int {
	product := multiplicand * multiplier

	// Zero properties
	if multiplicand == 0 {
		invariant.Always(product == 0, "Product must be zero when multiplicand is zero")
	}
	if multiplier == 0 {
		invariant.Always(product == 0, "Product must be zero when multiplier is zero")
	}

	// Identity properties
	// Avoid combining conditions if they can be separate assertions on their own
	if multiplicand == 1 || multiplier == 1 {
		invariant.Always(product == multiplicand || product == multiplier, "If some operand is one, then the product is equal to the other operand")
	}
	if multiplicand == 1 {
		invariant.Always(product == multiplier, "Product must equal multiplier when multiplicand is one")
	}
	if multiplier == 1 {
		invariant.Always(product == multiplicand, "Product must equal multiplicand when multiplier is one")
	}

	// Negation properties
	if multiplicand == -1 {
		invariant.Always(product == -multiplier, "Product must be negated when multiplicand is -1")
	}
	if multiplier == -1 {
		invariant.Always(product == -multiplicand, "Product must be negated when multiplier is -1")
	}

	// Sign properties
	// Positive * Positive
	if multiplicand > 0 && multiplier > 0 {
		invariant.Always(multiplicand*multiplier > 0, "Product must be positive when both operands are positive")
	}
	// Negative * Negative
	if multiplicand < 0 && multiplier < 0 {
		invariant.Always(multiplicand*multiplier > 0, "Product must be positive when both operands are negative")
	}
	// Positive * Negative
	if multiplicand > 0 && multiplier < 0 {
		invariant.Always(multiplicand*multiplier < 0, "Product must be negative when operands have opposing signs")
	}
	// Negative * Positive
	if multiplicand < 0 && multiplier > 0 {
		invariant.Always(multiplicand*multiplier < 0, "Product must be negative when operands have opposing signs")
	}

	// Repeated addition checks
	sum1 := 0
	sum2 := 0
	for i := 0; i < max(multiplier, -multiplier); i++ {
		sum1 = Add(sum1, multiplicand)
	}
	for i := 0; i < max(multiplicand, -multiplicand); i++ {
		sum2 = Add(sum2, multiplier)
	}
	if multiplier < 0 {
		sum1 *= -1
	}
	if multiplicand < 0 {
		sum2 *= -1
	}

	invariant.Always(sum1 == product, "Product must equal repeated addition of the multiplicand")
	invariant.Always(sum2 == product, "Product must equal repeated addition of the multiplier")
	invariant.Always(sum1 == sum2, "Multiplication is commutative")

	return product
}

func Divide(dividend, divisor int) (quotient, remainder int) {
	quotient = dividend / divisor
	remainder = dividend % divisor

	// Try to fill this one out yourself. You are encouraged to use AI, though I suspect it won't be of much help ;)
	// Food for thought: If it can't proof basic mathematical operations, what about your 20 javascript microservices?

	return quotient, remainder
}
