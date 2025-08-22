package types

// DefaultPosCompatibleTokens represents the PosCompatibleTokens default value.
// TODO: Determine the default value.
var DefaultPosCompatibleTokens string = "pos_compatible_tokens"

// DefaultWalletCompatibleTokens represents the WalletCompatibleTokens default value.
// TODO: Determine the default value.
var DefaultWalletCompatibleTokens string = "wallet_compatible_tokens"

// DefaultMinTokenStandard represents the MinTokenStandard default value.
// TODO: Determine the default value.
var DefaultMinTokenStandard string = "min_token_standard"

// NewParams creates a new Params instance.
func NewParams(
	posCompatibleTokens string,
	walletCompatibleTokens string,
	minTokenStandard string,
) Params {
	return Params{
		PosCompatibleTokens:    posCompatibleTokens,
		WalletCompatibleTokens: walletCompatibleTokens,
		MinTokenStandard:       minTokenStandard,
	}
}

// DefaultParams returns a default set of parameters.
func DefaultParams() Params {
	return NewParams(
		DefaultPosCompatibleTokens,
		DefaultWalletCompatibleTokens,
		DefaultMinTokenStandard,
	)
}

// Validate validates the set of params.
func (p Params) Validate() error {
	if err := validatePosCompatibleTokens(p.PosCompatibleTokens); err != nil {
		return err
	}

	if err := validateWalletCompatibleTokens(p.WalletCompatibleTokens); err != nil {
		return err
	}

	if err := validateMinTokenStandard(p.MinTokenStandard); err != nil {
		return err
	}

	return nil
}

// validatePosCompatibleTokens validates the PosCompatibleTokens parameter.
func validatePosCompatibleTokens(v string) error {
	// TODO implement validation
	return nil
}

// validateWalletCompatibleTokens validates the WalletCompatibleTokens parameter.
func validateWalletCompatibleTokens(v string) error {
	// TODO implement validation
	return nil
}

// validateMinTokenStandard validates the MinTokenStandard parameter.
func validateMinTokenStandard(v string) error {
	// TODO implement validation
	return nil
}
