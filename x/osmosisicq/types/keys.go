package types

import "cosmossdk.io/collections"

const (
	// ModuleName defines the module name
	ModuleName = "osmosisicq"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// GovModuleName duplicates the gov module's name to avoid a dependency with x/gov.
	// It should be synced with the gov module's name if it is ever changed.
	// See: https://github.com/cosmos/cosmos-sdk/blob/v0.52.0-beta.2/x/gov/types/keys.go#L9
	GovModuleName = "gov"
)

// ParamsKey is the prefix to retrieve all Params
var ParamsKey = collections.NewPrefix("p_osmosisicq")

// LastUpdateKey stores the unix timestamp of last successful schedule/attempt
var LastUpdateKey = collections.NewPrefix("lu_osmosisicq")

// NextUpdateKey stores the next unix timestamp when scheduler should run
var NextUpdateKey = collections.NewPrefix("nu_osmosisicq")

// LastResultTimeKey stores the unix timestamp of last successful result handled
var LastResultTimeKey = collections.NewPrefix("lrt_osmosisicq")

// QueryIDsKey is a prefix for persisted query IDs (per logical query name)
// Map key example: "twap:BTO:USDC" -> id string
var QueryIDsKey = collections.NewPrefix("qid_osmosisicq")
