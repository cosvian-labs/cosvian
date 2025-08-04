package keeper

import (
	"context"
	"errors"

	"bitora/x/pricefeed/types"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
)

// TransmitOracleRequestPacket transmits the packet over IBC with the specified source port and source channel
func (k Keeper) TransmitOracleRequestPacket(
	ctx context.Context,
	packetData types.OracleRequestPacketData,
	sourcePort,
	sourceChannel string,
	timeoutHeight clienttypes.Height,
	timeoutTimestamp uint64,
) (uint64, error) {
	packetBytes, err := packetData.GetBytes()
	if err != nil {
		return 0, errorsmod.Wrapf(sdkerrors.ErrJSONMarshal, "cannot marshal the packet: %s", err)
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return k.ibcKeeperFn().ChannelKeeper.SendPacket(sdkCtx, sourcePort, sourceChannel, timeoutHeight, timeoutTimestamp, packetBytes)
}

// OnRecvOracleRequestPacket processes packet reception
func (k Keeper) OnRecvOracleRequestPacket(ctx context.Context, packet channeltypes.Packet, data types.OracleRequestPacketData) (packetAck types.OracleRequestPacketAck, err error) {
	// validate packet data upon receiving

	// This function should only be called on Band Protocol chain
	// For now, we just acknowledge receipt and let Band Protocol handle the oracle request
	
	return packetAck, nil
}

// OnAcknowledgementOracleRequestPacket responds to the success or failure of a packet
// acknowledgement written on the receiving chain.
func (k Keeper) OnAcknowledgementOracleRequestPacket(ctx context.Context, packet channeltypes.Packet, data types.OracleRequestPacketData, ack channeltypes.Acknowledgement) error {
	switch dispatchedAck := ack.Response.(type) {
	case *channeltypes.Acknowledgement_Error:

		// TODO: failed acknowledgement logic
		_ = dispatchedAck.Error

		return nil
	case *channeltypes.Acknowledgement_Result:
		// Decode the packet acknowledgment
		var packetAck types.OracleRequestPacketAck

		if err := k.cdc.UnmarshalJSON(dispatchedAck.Result, &packetAck); err != nil {
			// The counter-party module doesn't implement the correct acknowledgment format
			return errors.New("cannot unmarshal acknowledgment")
		}

		// TODO: successful acknowledgement logic

		return nil
	default:
		// The counter-party module doesn't implement the correct acknowledgment format
		return errors.New("invalid acknowledgment format")
	}
}

// OnTimeoutOracleRequestPacket responds to the case where a packet has not been transmitted because of a timeout
func (k Keeper) OnTimeoutOracleRequestPacket(ctx context.Context, packet channeltypes.Packet, data types.OracleRequestPacketData) error {

	// TODO: packet timeout logic

	return nil
}
