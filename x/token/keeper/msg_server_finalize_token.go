package keeper

import (
	"context"
	"strconv"

	"bitora/x/token/types"

	sdk "github.com/cosmos/cosmos-sdk/types"

	errorsmod "cosmossdk.io/errors"
)

func (k msgServer) FinalizeToken(ctx context.Context, msg *types.MsgFinalizeToken) (*types.MsgFinalizeTokenResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Validate creator address
	_, err := k.addressCodec.StringToBytes(msg.Creator)
	if err != nil {
		return nil, errorsmod.Wrap(err, "invalid creator address")
	}

	// Token Creation/Finalization is FREE according to Fee.txt
	// No fee charged for finalize-token operation

	// TODO: Implement token finalization logic here
	// - Update token metadata
	// - Set token as finalized
	// - Update public/private status based on msg.MakePublic

	// Emit finalization event
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent("TokenFinalized",
			sdk.NewAttribute("creator", msg.Creator),
			sdk.NewAttribute("denom", msg.Denom),
			sdk.NewAttribute("make_public", strconv.FormatBool(msg.MakePublic)),
			sdk.NewAttribute("fee_charged", "0"), // Free token creation
		),
	)

	return &types.MsgFinalizeTokenResponse{}, nil
}
