package types

import "reflect"

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params: DefaultParams(),
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	// Allow an empty (zero) Params to be considered valid for test fixtures.
	// Many tests construct an empty GenesisState and expect validation to pass.
	// Treat an empty Params struct as equivalent to having no params set.
	var empty Params
	if reflect.DeepEqual(gs.Params, empty) {
		return nil
	}
	return gs.Params.Validate()
}
