package types

import (
	"fmt"
	"strings"

	sdkmath "cosmossdk.io/math"
)

// NewParams creates a new Params instance.
func NewParams() Params {
	return Params{
	ConnectionId:         "connection-0", // target connection to osmo-test-5
	UpdateIntervalSeconds: 30,
	// Defaults aligned with osmo-test-5 OSMO/USDC
	PoolId:               553,                   // OSMO/USDC pool id on osmo-test-5
	BaseDenom:            "uosmo",              // base denom on Osmosis
	QuoteDenom:           "ibc/DE6792CF9E521F6AD6E9A4BDF6225C9571A3B74ACC0A529F92BC5122A39D2E58", // USDC IBC denom on osmo-test-5
		UseTwap:              true,
		TwapWindowSeconds:    300,
		MinLiquidity:         "0",
		MaxDeviation:         "0.5", // 50% default guard
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
