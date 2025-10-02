//go:build osmosis_removed

package keeper

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildGammPoolKey(t *testing.T) {
	q := BuildGammPoolKey(42)
	require.Equal(t, "gamm", q.Store)
	require.Equal(t, []byte("pool/42"), q.Key)
}

func TestBuildTwapToNowKey(t *testing.T) {
	q := BuildTwapToNowKey("ucsv", "uusdc", 300)
	require.Equal(t, "twap", q.Store)
	require.Equal(t, []byte("twap/ucsv/uusdc/300"), q.Key)
}
