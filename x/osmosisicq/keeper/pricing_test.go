//go:build osmosis_removed

package keeper

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/stretchr/testify/require"
)

func TestSpotPrice(t *testing.T) {
	// 200 quote, 100 base -> 2.0
	p := SpotPrice(sdkmath.NewInt(200), sdkmath.NewInt(100))
	require.True(t, p.Equal(sdkmath.LegacyNewDec(2)))

	// zero base -> 0
	p = SpotPrice(sdkmath.NewInt(123), sdkmath.NewInt(0))
	require.True(t, p.IsZero())

	// larger numbers
	p = SpotPrice(sdkmath.NewInt(1_000_000_000), sdkmath.NewInt(4_000_000)) // 250
	require.True(t, p.Equal(sdkmath.LegacyNewDec(250)))
}

func TestTwoHopPrice(t *testing.T) {
	a := sdkmath.LegacyMustNewDecFromStr("1.5")
	b := sdkmath.LegacyMustNewDecFromStr("0.2")
	out := TwoHopPrice(a, b)
	require.True(t, out.Equal(sdkmath.LegacyMustNewDecFromStr("0.3")))
}
