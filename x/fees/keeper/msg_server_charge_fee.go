package keeper

import (
	"context"

	"bitora/x/fees/types"

	errorsmod "cosmossdk.io/errors"
)

func (k msgServer) ChargeFee(ctx context.Context, msg *types.MsgChargeFee) (*types.MsgChargeFeeResponse, error) {
	if _, err := k.addressCodec.StringToBytes(msg.Creator); err != nil {
		return nil, errorsmod.Wrap(err, "invalid authority address")
	}

	// TODO: Handle the message

	return &types.MsgChargeFeeResponse{}, nil
}
