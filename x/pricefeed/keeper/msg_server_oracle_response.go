package keeper

import (
	"context"
	"fmt"

	"bitora/x/pricefeed/types"

	errorsmod "cosmossdk.io/errors"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
)

func (k msgServer) SendOracleResponse(ctx context.Context, msg *types.MsgSendOracleResponse) (*types.MsgSendOracleResponseResponse, error) {
	// validate incoming message
	if _, err := k.addressCodec.StringToBytes(msg.Creator); err != nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidAddress, fmt.Sprintf("invalid address: %s", err))
	}

	if msg.Port == "" {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "invalid packet port")
	}

	if msg.ChannelID == "" {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "invalid packet channel")
	}

	if msg.TimeoutTimestamp == 0 {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "invalid packet timeout")
	}

	// TODO: logic before transmitting the packet

	// Construct the packet
	var packet types.OracleResponsePacketData

	packet.RequestId = msg.RequestId
	packet.Prices = msg.Prices
	packet.Rates = msg.Rates
	packet.Error = msg.Error

	// Transmit the packet
	_, err := k.TransmitOracleResponsePacket(
		ctx,
		packet,
		msg.Port,
		msg.ChannelID,
		clienttypes.ZeroHeight(),
		msg.TimeoutTimestamp,
	)
	if err != nil {
		return nil, err
	}

	return &types.MsgSendOracleResponseResponse{}, nil
}
