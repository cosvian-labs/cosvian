//go:build osmosis_removed

package keeper

import (
	"fmt"
)

// QueryKey represents a target KV store and key for ICQ.
type QueryKey struct {
	Store string
	Key   []byte
}

// BuildGammPoolKey builds a pseudo key for querying Osmosis GAMM pool by poolID.
// Note: This is a placeholder encoding kept deterministic for testing and eventing.
func BuildGammPoolKey(poolID uint64) QueryKey {
	return QueryKey{
		Store: "gamm",
		Key:   []byte(fmt.Sprintf("pool/%d", poolID)),
	}
}

// BuildTwapToNowKey builds a pseudo key for Osmosis TWAP arithmetic-to-now for base/quote and window seconds.
// Ordering is preserved as provided by caller.
func BuildTwapToNowKey(baseDenom, quoteDenom string, windowSeconds uint64) QueryKey {
	return QueryKey{
		Store: "twap",
		Key:   []byte(fmt.Sprintf("twap/%s/%s/%d", baseDenom, quoteDenom, windowSeconds)),
	}
}
