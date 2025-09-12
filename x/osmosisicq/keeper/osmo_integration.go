package keeper

import (
	sdkmath "cosmossdk.io/math"
)

// osmoIntegration abstracts building keys and decoding values for Osmosis stores.
// The default implementation is a stub; a real implementation can be provided
// behind a build tag or via later wiring without changing callers.
type osmoIntegration interface {
	// BuildTwapKey builds the most-recent TWAP record key for a given pool and denom pair.
	// windowSeconds may be ignored at the key level (TWAP windowing is computed from values).
	BuildTwapKey(base, quote string, poolID uint64, windowSeconds uint64) (store string, key []byte)
	BuildGammPoolKey(poolID uint64) (store string, key []byte)
	// DecodeTwap decodes a TWAP record value into a quote/base price using the provided base & quote.
	DecodeTwap(key, value []byte, base, quote string) (price, liquidity sdkmath.LegacyDec, err error)
	// DecodeSpot decodes a GAMM spot record value into a quote/base price using the provided base & quote.
	DecodeSpot(key, value []byte, base, quote string) (price, liquidity sdkmath.LegacyDec, err error)
}

// stub implementation uses the existing placeholder formats.
type stubOsmoIntegration struct{}

func (stubOsmoIntegration) BuildTwapKey(base, quote string, poolID uint64, window uint64) (string, []byte) {
	// poolID is ignored in stub mode; placeholder key remains deterministic for tests.
	tk := BuildTwapToNowKey(base, quote, window)
	return tk.Store, tk.Key
}
func (stubOsmoIntegration) BuildGammPoolKey(poolID uint64) (string, []byte) {
	pk := BuildGammPoolKey(poolID)
	return pk.Store, pk.Key
}
func (stubOsmoIntegration) DecodeTwap(key, value []byte, base, quote string) (sdkmath.LegacyDec, sdkmath.LegacyDec, error) {
	return decodePriceLiquidity(value)
}
func (stubOsmoIntegration) DecodeSpot(key, value []byte, base, quote string) (sdkmath.LegacyDec, sdkmath.LegacyDec, error) {
	return decodePriceLiquidity(value)
}

// osmo is the active integration instance. Defaults to stub.
var osmo osmoIntegration = stubOsmoIntegration{}
