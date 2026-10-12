// Package money holds the integer types and math for amounts and shares.
// Nothing here uses floats, so every result is exact to the cent.
package money

import (
	"errors"
	"slices"
)

// Cents is an amount of money in whole cents. $3.00 is Cents(300).
type Cents int64

// BasisPoints is a share of a whole, where 10000 is 100%. 25% is BasisPoints(2500).
type BasisPoints int64

// Whole is 100% in basis points. A ruleset's shares must sum to exactly this.
const Whole BasisPoints = 10000

var (
	ErrNegativeAmount = errors.New("money: amount is negative")
	ErrNegativeShare  = errors.New("money: share is negative")
	ErrSharesNotWhole = errors.New("money: shares do not sum to 10000 bps")
)

// Share returns bps of c, rounded down to the whole cent.
// On its own it can lose pennies; use Split to divide a whole amount.
func (c Cents) Share(bps BasisPoints) Cents {
	return Cents(int64(c) * int64(bps) / int64(Whole))
}

// Split divides total into one part per share. The parts always sum to
// exactly total: after rounding every part down, the leftover cents go one
// each to the parts that lost the most to rounding (ties go to the earlier
// share). shares must be non-negative and sum to Whole.
func Split(total Cents, shares []BasisPoints) ([]Cents, error) {
	if total < 0 {
		return nil, ErrNegativeAmount
	}
	var sum BasisPoints
	for _, s := range shares {
		if s < 0 {
			return nil, ErrNegativeShare
		}
		sum += s
	}
	if sum != Whole {
		return nil, ErrSharesNotWhole
	}

	parts := make([]Cents, len(shares))
	remainders := make([]int64, len(shares))
	leftover := total
	for i, s := range shares {
		product := int64(total) * int64(s)
		parts[i] = Cents(product / int64(Whole))
		remainders[i] = product % int64(Whole)
		leftover -= parts[i]
	}

	// Rank parts by how much rounding cut off, largest first.
	order := make([]int, len(shares))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int {
		switch {
		case remainders[a] > remainders[b]:
			return -1
		case remainders[a] < remainders[b]:
			return 1
		}
		return 0
	})

	// leftover is always smaller than len(shares), since each part lost
	// less than one cent to rounding.
	for _, i := range order[:leftover] {
		parts[i]++
	}
	return parts, nil
}
