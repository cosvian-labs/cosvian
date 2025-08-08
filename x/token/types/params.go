package types

import (
	"cosmossdk.io/math"
)

// NewParams creates a new Params instance.
func NewParams() Params {
	return Params{
		TokenPrefix:         "bto_",
		MaxTokensPerCreator: 10,
		ReservedSymbols:     []string{"btc", "eth", "bto", "sbtc", "usdt", "usdc", "bnb"},
		CreationFeeAmount:   math.NewInt(3_000_000), // 3 BTO (with 6 decimals)
	}
}

// DefaultParams returns a default set of parameters.
func DefaultParams() Params {
	return NewParams()
}

// Validate validates the set of params.
func (p Params) Validate() error {
	if p.TokenPrefix == "" {
		return ErrInvalidTokenName
	}
	if p.MaxTokensPerCreator == 0 {
		return ErrInvalidMaxSupply
	}
	if p.CreationFeeAmount.IsNegative() {
		return ErrInvalidInitialSupply
	}
	
	return nil
}
