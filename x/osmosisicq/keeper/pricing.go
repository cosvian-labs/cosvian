//go:build osmosis_removed

package keeper

import (
	sdkmath "cosmossdk.io/math"
)

// SpotPrice returns the quote/base price using legacy decimal math: price = quote / base.
// It expects non-zero base reserve. If base is zero, it returns 0.
func SpotPrice(quoteReserve, baseReserve sdkmath.Int) sdkmath.LegacyDec {
	if !baseReserve.IsPositive() {
		return sdkmath.LegacyZeroDec()
	}
	// Convert to Dec for division to preserve precision
	q := sdkmath.LegacyNewDecFromInt(quoteReserve)
	b := sdkmath.LegacyNewDecFromInt(baseReserve)
	return q.Quo(b)
}

// TwoHopPrice multiplies two prices (A/B) * (B/C) to derive A/C.
func TwoHopPrice(pab, pbc sdkmath.LegacyDec) sdkmath.LegacyDec {
	return pab.Mul(pbc)
}
