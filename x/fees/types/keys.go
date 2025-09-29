package types

import (
	"encoding/binary"
	"time"

	"cosmossdk.io/collections"
)

const (
	// ModuleName defines the module name
	ModuleName = "fees"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// GovModuleName duplicates the gov module's name to avoid a dependency with x/gov.
	// It should be synced with the gov module's name if it is ever changed.
	// See: https://github.com/cosmos/cosmos-sdk/blob/v0.52.0-beta.2/x/gov/types/keys.go#L9
	GovModuleName = "gov"
)

// Store key prefixes
const (
	PricePointPrefix = "price_point/"
)

// Event types and attributes
const (
	EventTypeOracleUsed = "oracle_used"
	EventTypeFeeCharged = "fee_charged"

	AttributeKeyPrice      = "price"
	AttributeKeyTwapPrice  = "twap_price"
	AttributeKeyTimestamp  = "timestamp"
	AttributeKeyWindow     = "window"
	AttributeKeyFreshness  = "freshness"
	AttributeKeyIsStale    = "is_stale"
	AttributeKeyIsFallback = "is_fallback"
	AttributeKeyCategory   = "category"
	AttributeKeyUsdAmount  = "usd_amount"
	AttributeKeyCsvAmount  = "csv_amount"
	AttributeKeyTxHash     = "tx_hash"
)

// Context keys for storing fee information
type contextKey string

const (
	FeeEstimateContextKey contextKey = "fee_estimate"
	FeeMetadataContextKey contextKey = "fee_metadata"
)

// ParamsKey is the prefix to retrieve all Params
var ParamsKey = collections.NewPrefix("p_fees")

// GetPricePointKey returns the store key for a price point at a given timestamp
func GetPricePointKey(timestamp time.Time) []byte {
	// Convert timestamp to nanoseconds for precise ordering
	nanos := timestamp.UnixNano()
	key := make([]byte, len(PricePointPrefix)+8)
	copy(key, PricePointPrefix)
	binary.BigEndian.PutUint64(key[len(PricePointPrefix):], uint64(nanos))
	return key
}
