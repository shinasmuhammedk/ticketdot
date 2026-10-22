package money_test

import (
	"math"
	"testing"

	"ticketdot/internal/money"
)

func TestRupeesToPaise(t *testing.T) {
	tests := []struct {
		name        string
		input       float64
		wantPaise   int64
		wantErr     error
		expectError bool
	}{
		// Standard valid values
		{"zero", 0.0, 0, nil, false},
		{"single rupee", 1.0, 100, nil, false},
		{"with two decimals", 499.50, 49950, nil, false},
		{"with one decimal", 499.5, 49950, nil, false},
		{"odd paise", 123.45, 12345, nil, false},
		{"single paise", 0.01, 1, nil, false},
		{"large value max boundary", 100000.0, 10000000, nil, false},
		{"large value near max", 99999.99, 9999999, nil, false},

		// IEEE 754 precision inputs (0.1 + 0.2)
		{"float summation 0.1 + 0.2", 0.1 + 0.2, 30, nil, false},
		{"float summation 0.7 + 0.1", 0.7 + 0.1, 80, nil, false},

		// Rounding / too many decimals rejection
		{"fractional paise 499.995", 499.995, 0, money.ErrTooManyDecimals, true},
		{"three decimals 10.001", 10.001, 0, money.ErrTooManyDecimals, true},
		{"four decimals 0.0001", 0.0001, 0, money.ErrTooManyDecimals, true},

		// Out of range & special values
		{"negative fare", -0.01, 0, money.ErrNegativeFare, true},
		{"large negative fare", -500.0, 0, money.ErrNegativeFare, true},
		{"exceeds max fare", 100000.01, 0, money.ErrFareTooHigh, true},
		{"huge fare", 1000000.0, 0, money.ErrFareTooHigh, true},
		{"NaN", math.NaN(), 0, money.ErrInvalidNumber, true},
		{"positive infinity", math.Inf(1), 0, money.ErrInvalidNumber, true},
		{"negative infinity", math.Inf(-1), 0, money.ErrInvalidNumber, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := money.RupeesToPaise(tt.input)
			if tt.expectError {
				if err == nil {
					t.Fatalf("RupeesToPaise(%v) expected error, got nil (paise: %d)", tt.input, got)
				}
				if tt.wantErr != nil && err != tt.wantErr {
					t.Errorf("RupeesToPaise(%v) error = %v, want %v", tt.input, err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("RupeesToPaise(%v) unexpected error: %v", tt.input, err)
			}
			if got != tt.wantPaise {
				t.Errorf("RupeesToPaise(%v) = %d, want %d", tt.input, got, tt.wantPaise)
			}
		})
	}
}

func TestPaiseToRupeesString(t *testing.T) {
	tests := []struct {
		paise    int64
		expected string
	}{
		{0, "0.00"},
		{1, "0.01"},
		{9, "0.09"},
		{10, "0.10"},
		{50, "0.50"},
		{99, "0.99"},
		{100, "1.00"},
		{49950, "499.50"},
		{10000000, "100000.00"},
		{-49950, "-499.50"},
	}

	for _, tt := range tests {
		got := money.PaiseToRupeesString(tt.paise)
		if got != tt.expected {
			t.Errorf("PaiseToRupeesString(%d) = %s, want %s", tt.paise, got, tt.expected)
		}
	}
}

func TestFormatINR(t *testing.T) {
	if got := money.FormatINR(49950); got != "₹499.50" {
		t.Errorf("FormatINR(49950) = %s, want ₹499.50", got)
	}
	if got := money.FormatINR(0); got != "₹0.00" {
		t.Errorf("FormatINR(0) = %s, want ₹0.00", got)
	}
}

func TestMultiSeatTotalsAreExact(t *testing.T) {
	// Base fare of ₹499.50 = 49950 paise
	baseFareRupees := 499.50
	baseFarePaise, err := money.RupeesToPaise(baseFareRupees)
	if err != nil {
		t.Fatalf("failed to convert base fare: %v", err)
	}

	expectedTotals := map[int64]int64{
		1: 49950,  // ₹499.50
		2: 99900,  // ₹999.00
		3: 149850, // ₹1,498.50
		4: 199800, // ₹1,998.00
		5: 249750, // ₹2,497.50
		6: 299700, // ₹2,997.00
	}

	for seats := int64(1); seats <= 6; seats++ {
		total := baseFarePaise * seats
		expected := expectedTotals[seats]
		if total != expected {
			t.Errorf("multiplication for %d seats: got %d, want %d", seats, total, expected)
		}
	}
}
