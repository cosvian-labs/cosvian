package types

// DefaultDistributionRatio represents the DistributionRatio default value.
// TODO: Determine the default value.
var DefaultDistributionRatio string = "distribution_ratio"

// DefaultValidatorRewardPercentage represents the ValidatorRewardPercentage default value.
// TODO: Determine the default value.
var DefaultValidatorRewardPercentage string = "validator_reward_percentage"

// DefaultDevRewardPercentage represents the DevRewardPercentage default value.
// TODO: Determine the default value.
var DefaultDevRewardPercentage string = "dev_reward_percentage"

// NewParams creates a new Params instance.
func NewParams(
	distributionRatio string,
	validatorRewardPercentage string,
	devRewardPercentage string,
) Params {
	return Params{
		DistributionRatio:         distributionRatio,
		ValidatorRewardPercentage: validatorRewardPercentage,
		DevRewardPercentage:       devRewardPercentage,
	}
}

// DefaultParams returns a default set of parameters.
func DefaultParams() Params {
	return NewParams(
		DefaultDistributionRatio,
		DefaultValidatorRewardPercentage,
		DefaultDevRewardPercentage,
	)
}

// Validate validates the set of params.
func (p Params) Validate() error {
	if err := validateDistributionRatio(p.DistributionRatio); err != nil {
		return err
	}

	if err := validateValidatorRewardPercentage(p.ValidatorRewardPercentage); err != nil {
		return err
	}

	if err := validateDevRewardPercentage(p.DevRewardPercentage); err != nil {
		return err
	}

	return nil
}

// validateDistributionRatio validates the DistributionRatio parameter.
func validateDistributionRatio(v string) error {
	// TODO implement validation
	return nil
}

// validateValidatorRewardPercentage validates the ValidatorRewardPercentage parameter.
func validateValidatorRewardPercentage(v string) error {
	// TODO implement validation
	return nil
}

// validateDevRewardPercentage validates the DevRewardPercentage parameter.
func validateDevRewardPercentage(v string) error {
	// TODO implement validation
	return nil
}
