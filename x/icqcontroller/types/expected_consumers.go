package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// KVResultConsumer is an optional interface other modules can implement
// to receive ICQ KV query results delivered by the controller.
// For example, x/osmosisicq/keeper.Keeper implements this signature.
type KVResultConsumer interface {
    OnKVResult(ctx sdk.Context, store string, key, value []byte) error
}
