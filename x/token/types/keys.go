package types

import "cosmossdk.io/collections"

const (
	// ModuleName defines the module name
	ModuleName = "token"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// GovModuleName duplicates the gov module's name to avoid a dependency with x/gov.
	// It should be synced with the gov module's name if it is ever changed.
	// See: https://github.com/cosmos/cosmos-sdk/blob/v0.52.0-beta.2/x/gov/types/keys.go#L9
	GovModuleName = "gov"
)

// ParamsKey is the prefix to retrieve all Params
var ParamsKey = collections.NewPrefix("p_token")

// Store key prefixes for token wizard
var (
	TokenMetadataPrefix = collections.NewPrefix("tm_") // TokenMetadataPrefix + tokenID -> TokenMetadata
	TokenSequenceKey    = collections.NewPrefix("ts")  // TokenSequenceKey -> uint64 (next sequence)
	SymbolIndexPrefix   = collections.NewPrefix("si_") // SymbolIndexPrefix + symbol -> tokenID
	CreatorIndexPrefix  = collections.NewPrefix("ci_") // CreatorIndexPrefix + creator + sequence -> tokenID
	CreatorCountPrefix  = collections.NewPrefix("cc_") // CreatorCountPrefix + creator -> uint64 (token count)
)
