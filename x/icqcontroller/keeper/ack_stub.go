//go:build !icq_async

package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// extractValueFromAck (default build) is a no-op stub; async-icq decoding is only available under icq_async.
func (k Keeper) extractValueFromAck(ctx sdk.Context, acknowledgement []byte) ([]byte, error) {
    return nil, nil
}
