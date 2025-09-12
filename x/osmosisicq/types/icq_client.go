package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ICQClient is an optional dependency used to register interchain KV queries.
// Implementations are provided by an ICQ controller module or adapter.
// If absent (nil), osmosisicq will emit intent events but not register queries.
type ICQClient interface {
	// RegisterKVQuery registers a KV query against a remote chain over the given connection.
	// Returns a provider-specific query ID for correlating responses.
	RegisterKVQuery(ctx sdk.Context, connectionID, store string, key []byte) (string, error)
}
