package keeper

import (
	"fmt"
	"time"

	"bitora/x/icqcontroller/icqwire"

	abcitypes "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"

	"bitora/x/icqcontroller/types"
)

// SendKVQuery builds and sends an async-icq CosmosQuery packet over the channel mapped to the connection.
// Returns a provider-specific query identifier derived from the send sequence.
func (k Keeper) SendKVQuery(ctx sdk.Context, connectionID, store string, key []byte) (string, error) {
	// Resolve source port and channel
	portID, err := k.Port.Get(ctx)
	if err != nil || portID == "" {
		portID = types.PortID
	}
	channelID, ok := k.GetChannelForConnection(ctx, connectionID)
	if !ok || channelID == "" {
		return "", fmt.Errorf("no active ICQ channel for connection %s", connectionID)
	}

	// Build an ABCI store/key query
	req := abcitypes.RequestQuery{
		Path:  fmt.Sprintf("/store/%s/key", store),
		Data:  key,
		Prove: false,
	}
	bz, err := icqwire.SerializeCosmosQuery([]abcitypes.RequestQuery{req})
	if err != nil {
		return "", err
	}
	packet := icqwire.InterchainQueryPacketData{Data: bz}

	// Use a reasonable timeout (e.g. 2 minutes from now)
	timeoutTimestamp := uint64(ctx.BlockTime().Add(2 * time.Minute).UnixNano())

	// Determine next sequence and persist pending correlation BEFORE sending
	nextSeq, found := k.ibcKeeperFn().ChannelKeeper.GetNextSequenceSend(ctx, portID, channelID)
	if !found {
		return "", fmt.Errorf("cannot get next sequence for %s/%s", portID, channelID)
	}
	if err := k.SetPending(ctx, channelID, nextSeq, store, key, connectionID); err != nil {
		return "", err
	}

	seq, err := k.ibcKeeperFn().ChannelKeeper.SendPacket(ctx, portID, channelID, clienttypes.ZeroHeight(), timeoutTimestamp, packet.GetBytes())
	if err != nil {
		// cleanup pending on failure
		_ = k.RemovePending(ctx, channelID, nextSeq)
		return "", err
	}
	// Return a simple correlation id
	return fmt.Sprintf("%s/%s#%d", portID, channelID, seq), nil
}
