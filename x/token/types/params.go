package types

import (
	"cosmossdk.io/math"
)

// NewParams creates a new Params instance.
func NewParams() Params {
	return Params{
		MaxTokensPerCreator: 10,
		ReservedSymbols:     []string{"csv"}, // Only reserve native token
		CreationFeeAmount:   math.NewInt(0),  // 3 CSV (with 6 decimals)
	}
}

// DefaultParams returns a default set of parameters.
func DefaultParams() Params {
	return NewParams()
}

// Validate validates the set of params.
func (p Params) Validate() error {
	if p.MaxTokensPerCreator == 0 {
		return ErrInvalidMaxSupply
	}
	if p.CreationFeeAmount.IsNegative() {
		return ErrInvalidInitialSupply
	}

	return nil
}
