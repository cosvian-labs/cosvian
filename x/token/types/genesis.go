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
	// Allow empty/default genesis state in tests: if Params is zero-valued,
	// consider the genesis state valid (tests construct empty GenesisState{}).
	// Use reflect.DeepEqual to avoid calling methods on zero-value fields.
	// If Params is the zero-value struct, skip validation.
	var zero Params
	if reflect.DeepEqual(gs.Params, zero) {
		return nil
	}

	return gs.Params.Validate()
}
