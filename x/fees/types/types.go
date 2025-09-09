package types

// Fee mode constants
const (
	FeeModeTable  = "table"
	FeeModeHybrid = "hybrid"
	FeeModeGasOnly = "gas_only"
)

// IsHybridEnabled returns true if params fee mode is hybrid.
func (p Params) IsHybridEnabled() bool { return p.FeeMode == FeeModeHybrid }

// IsGasOnly returns true if USD table should be ignored entirely.
func (p Params) IsGasOnly() bool { return p.FeeMode == FeeModeGasOnly }
