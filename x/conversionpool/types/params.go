package types

// DefaultMaxConversionSupply represents the MaxConversionSupply default value.
// TODO: Determine the default value.
var DefaultMaxConversionSupply string = "max_conversion_supply"

// DefaultConversionRate represents the ConversionRate default value.
// TODO: Determine the default value.
var DefaultConversionRate string = "conversion_rate"

// DefaultMinPoolReserve represents the MinPoolReserve default value.
// TODO: Determine the default value.
var DefaultMinPoolReserve string = "min_pool_reserve"

// NewParams creates a new Params instance.
func NewParams(
	maxConversionSupply string,
	conversionRate string,
	minPoolReserve string,
) Params {
	return Params{
		MaxConversionSupply: maxConversionSupply,
		ConversionRate:      conversionRate,
		MinPoolReserve:      minPoolReserve,
	}
}

// DefaultParams returns a default set of parameters.
func DefaultParams() Params {
	return NewParams(
		DefaultMaxConversionSupply,
		DefaultConversionRate,
		DefaultMinPoolReserve,
	)
}

// Validate validates the set of params.
func (p Params) Validate() error {
	if err := validateMaxConversionSupply(p.MaxConversionSupply); err != nil {
		return err
	}

	if err := validateConversionRate(p.ConversionRate); err != nil {
		return err
	}

	if err := validateMinPoolReserve(p.MinPoolReserve); err != nil {
		return err
	}

	return nil
}

// validateMaxConversionSupply validates the MaxConversionSupply parameter.
func validateMaxConversionSupply(v string) error {
	// TODO implement validation
	return nil
}

// validateConversionRate validates the ConversionRate parameter.
func validateConversionRate(v string) error {
	// TODO implement validation
	return nil
}

// validateMinPoolReserve validates the MinPoolReserve parameter.
func validateMinPoolReserve(v string) error {
	// TODO implement validation
	return nil
}
