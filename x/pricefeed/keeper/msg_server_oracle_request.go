package keeper

import (
	"context"
	"fmt"

	"bitora/x/pricefeed/types"

	errorsmod "cosmossdk.io/errors"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
)

func (k msgServer) SendOracleRequest(ctx context.Context, msg *types.MsgSendOracleRequest) (*types.MsgSendOracleRequestResponse, error) {
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
	// Validate oracle script ID (should be Band Protocol oracle script)
	if msg.OracleScriptId == 0 {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "oracle script ID cannot be zero")
	}

	// Set default values for Band Protocol if not provided
	if msg.AskCount == 0 {
		msg.AskCount = 4 // Default ask count
	}
	if msg.MinCount == 0 {
		msg.MinCount = 3 // Default min count
	}
	if msg.FeeLimit == "" {
		msg.FeeLimit = "100000uband" // Default fee limit
	}
	if msg.PrepareGas == 0 {
		msg.PrepareGas = 50000 // Default prepare gas
	}
	if msg.ExecuteGas == 0 {
		msg.ExecuteGas = 300000 // Default execute gas
	}

	// Construct the packet
	var packet types.OracleRequestPacketData

	packet.OracleScriptId = msg.OracleScriptId
	packet.Calldata = msg.Calldata
	packet.Symbols = msg.Symbols
	packet.AskCount = msg.AskCount
	packet.MinCount = msg.MinCount
	packet.FeeLimit = msg.FeeLimit
	packet.PrepareGas = msg.PrepareGas
	packet.ExecuteGas = msg.ExecuteGas
	packet.ClientId = msg.ClientId

	// Transmit the packet
	_, err := k.TransmitOracleRequestPacket(
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

	return &types.MsgSendOracleRequestResponse{}, nil
}
