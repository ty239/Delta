package money

import (
	"errors"
	"slices"
	"testing"
)

func TestShare(t *testing.T) {
	tests := []struct {
		amount Cents
		bps    BasisPoints
		want   Cents
	}{
		{10000, 2500, 2500}, // 25% of $100.00
		{10000, 10000, 10000},
		{10000, 0, 0},
		{0, 2500, 0},
		{100, 3333, 33}, // 33.33¢ rounds down
	}
	for _, tt := range tests {
		if got := tt.amount.Share(tt.bps); got != tt.want {
			t.Errorf("Cents(%d).Share(%d) = %d, want %d", tt.amount, tt.bps, got, tt.want)
		}
	}
}

func TestSplit(t *testing.T) {
	tests := []struct {
		name   string
		total  Cents
		shares []BasisPoints
		want   []Cents
	}{
		{"even", 10000, []BasisPoints{2500, 2500, 5000}, []Cents{2500, 2500, 5000}},
		{"lost penny goes to largest remainder", 100, []BasisPoints{3333, 3333, 3334}, []Cents{33, 33, 34}},
		{"tie goes to earlier share", 1, []BasisPoints{5000, 5000}, []Cents{1, 0}},
		{"two leftover cents", 200, []BasisPoints{3333, 3333, 3334}, []Cents{67, 66, 67}},
		{"zero total", 0, []BasisPoints{5000, 5000}, []Cents{0, 0}},
		{"single rule", 12345, []BasisPoints{10000}, []Cents{12345}},
		{"zero share", 999, []BasisPoints{0, 10000}, []Cents{0, 999}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Split(tt.total, tt.shares)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("Split(%d, %v) = %v, want %v", tt.total, tt.shares, got, tt.want)
			}
		})
	}
}

func TestSplitNeverLosesAPenny(t *testing.T) {
	shares := []BasisPoints{2500, 3300, 1000, 3200}
	for total := Cents(0); total <= 10000; total++ {
		parts, err := Split(total, shares)
		if err != nil {
			t.Fatalf("Split(%d): %v", total, err)
		}
		var sum Cents
		for _, p := range parts {
			sum += p
		}
		if sum != total {
			t.Fatalf("Split(%d) parts sum to %d", total, sum)
		}
	}
}

func TestSplitErrors(t *testing.T) {
	tests := []struct {
		name   string
		total  Cents
		shares []BasisPoints
		want   error
	}{
		{"negative total", -1, []BasisPoints{10000}, ErrNegativeAmount},
		{"negative share", 100, []BasisPoints{11000, -1000}, ErrNegativeShare},
		{"under 100%", 100, []BasisPoints{5000}, ErrSharesNotWhole},
		{"over 100%", 100, []BasisPoints{6000, 5000}, ErrSharesNotWhole},
		{"no shares", 100, nil, ErrSharesNotWhole},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Split(tt.total, tt.shares); !errors.Is(err, tt.want) {
				t.Errorf("got error %v, want %v", err, tt.want)
			}
		})
	}
}
