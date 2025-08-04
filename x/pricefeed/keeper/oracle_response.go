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

// TransmitOracleResponsePacket transmits the packet over IBC with the specified source port and source channel
func (k Keeper) TransmitOracleResponsePacket(
	ctx context.Context,
	packetData types.OracleResponsePacketData,
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

// OnRecvOracleResponsePacket processes packet reception
func (k Keeper) OnRecvOracleResponsePacket(ctx context.Context, packet channeltypes.Packet, data types.OracleResponsePacketData) (packetAck types.OracleResponsePacketAck, err error) {
	// validate packet data upon receiving
	if data.Error != "" {
		// Handle error case
		return packetAck, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "oracle response error: %s", data.Error)
	}

	// Process price data from Band Protocol using our integration helper
	err = k.ProcessPriceResponse(ctx, data)
	if err != nil {
		return packetAck, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "failed to process price response: %s", err.Error())
	}

	return packetAck, nil
}

// OnAcknowledgementOracleResponsePacket responds to the success or failure of a packet
// acknowledgement written on the receiving chain.
func (k Keeper) OnAcknowledgementOracleResponsePacket(ctx context.Context, packet channeltypes.Packet, data types.OracleResponsePacketData, ack channeltypes.Acknowledgement) error {
	switch dispatchedAck := ack.Response.(type) {
	case *channeltypes.Acknowledgement_Error:

		// TODO: failed acknowledgement logic
		_ = dispatchedAck.Error

		return nil
	case *channeltypes.Acknowledgement_Result:
		// Decode the packet acknowledgment
		var packetAck types.OracleResponsePacketAck

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

// OnTimeoutOracleResponsePacket responds to the case where a packet has not been transmitted because of a timeout
func (k Keeper) OnTimeoutOracleResponsePacket(ctx context.Context, packet channeltypes.Packet, data types.OracleResponsePacketData) error {

	// TODO: packet timeout logic

	return nil
}
