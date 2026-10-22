package money

import (
	"errors"
	"fmt"
	"math"
)

const (
	// MaxFareRupees is the maximum allowable fare in rupees.
	MaxFareRupees = 100000.0
	// MaxFarePaise is the maximum allowable fare in paise (10,000,000 paise = 100,000 rupees).
	MaxFarePaise = int64(100000 * 100)
)

var (
	ErrNegativeFare    = errors.New("fare cannot be negative")
	ErrFareTooHigh     = errors.New("fare cannot exceed 100,000 rupees")
	ErrTooManyDecimals = errors.New("fare cannot have more than 2 decimal places")
	ErrInvalidNumber   = errors.New("fare must be a valid finite number")
)

// RupeesToPaise converts a decimal rupee amount to integer paise with strict validation:
// - Rejects NaN and +/-Inf
// - Rejects negative values (< 0)
// - Rejects amounts exceeding 100,000 rupees
// - Rejects values with more than 2 decimal places (e.g. 499.995)
// - Accurately converts IEEE 754 representations like 0.1 + 0.2
func RupeesToPaise(rupees float64) (int64, error) {
	if math.IsNaN(rupees) || math.IsInf(rupees, 0) {
		return 0, ErrInvalidNumber
	}
	if rupees < 0 {
		return 0, ErrNegativeFare
	}
	if rupees > MaxFareRupees {
		return 0, ErrFareTooHigh
	}

	// Validate maximum 2 decimal places
	cents := rupees * 100.0
	rounded := math.Round(cents)
	if math.Abs(cents-rounded) > 1e-4 {
		return 0, ErrTooManyDecimals
	}

	paise := int64(rounded)
	if paise < 0 {
		return 0, ErrNegativeFare
	}
	if paise > MaxFarePaise {
		return 0, ErrFareTooHigh
	}
	return paise, nil
}

// PaiseToRupeesString formats integer paise as a decimal rupee string with 2 decimal places (e.g. 49950 -> "499.50").
func PaiseToRupeesString(paise int64) string {
	sign := ""
	if paise < 0 {
		sign = "-"
		paise = -paise
	}
	rupees := paise / 100
	fraction := paise % 100
	return fmt.Sprintf("%s%d.%02d", sign, rupees, fraction)
}

// FormatINR formats integer paise into an Indian Rupee string with the currency symbol (e.g. 49950 -> "₹499.50").
func FormatINR(paise int64) string {
	return "₹" + PaiseToRupeesString(paise)
}
