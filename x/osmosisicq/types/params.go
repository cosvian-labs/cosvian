//go:build osmosis_removed

package types

import (
	"fmt"
	"strings"

	sdkmath "cosmossdk.io/math"
)

// NewParams creates a new Params instance.
func NewParams() Params {
	return Params{
		ConnectionId:          "connection-0", // placeholder; set to real Osmosis connection ID
		UpdateIntervalSeconds: 30,
		// Practical defaults to enable ICQ registration out-of-the-box in local setups
		PoolId:            1464,                   // OSMO/USDC main pool on Osmosis (example)
		BaseDenom:         "uosmo",                // Osmosis on-chain base denom
		QuoteDenom:        "ibc/PLACEHOLDER_HASH", // Replace with real USDC ibc/<HASH> on Osmosis
		UseTwap:           true,
		TwapWindowSeconds: 300,
		MinLiquidity:      "0",
		MaxDeviation:      "0.5", // 50% default guard
	}
}

// DefaultParams returns a default set of parameters.
func DefaultParams() Params {
	return NewParams()
}

// Validate validates the set of params.
func (p Params) Validate() error {
	// ConnectionId can be empty at genesis, but if set must be non-whitespace
	// Basic numeric checks
	if p.UpdateIntervalSeconds == 0 {
		// allow zero only if explicitly desired; default is non-zero
		// keep as-is, not an error
	}
	if p.UseTwap && p.TwapWindowSeconds == 0 {
		return fmt.Errorf("twap_window_seconds must be > 0 when use_twap is true")
	}
	// PoolId 0 means unset; allowed at genesis
	if strings.TrimSpace(p.BaseDenom) == "" {
		return fmt.Errorf("base_denom cannot be empty")
	}
	if strings.TrimSpace(p.QuoteDenom) == "" {
		return fmt.Errorf("quote_denom cannot be empty")
	}
	// MinLiquidity and MaxDeviation should be parseable decimals (non-negative)
	if _, err := sdkmath.LegacyNewDecFromStr(p.MinLiquidity); err != nil {
		return fmt.Errorf("invalid min_liquidity: %w", err)
	}
	dev, err := sdkmath.LegacyNewDecFromStr(p.MaxDeviation)
	if err != nil {
		return fmt.Errorf("invalid max_deviation: %w", err)
	}
	if dev.IsNegative() {
		return fmt.Errorf("max_deviation cannot be negative")
	}
	return nil
}
