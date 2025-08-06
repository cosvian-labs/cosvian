package keeper

import (
	"context"
	"strconv"

	"bitora/x/token/types"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	errorsmod "cosmossdk.io/errors"
)

func (k msgServer) FinalizeToken(ctx context.Context, msg *types.MsgFinalizeToken) (*types.MsgFinalizeTokenResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Validate creator address
	creatorAddr, err := k.addressCodec.StringToBytes(msg.Creator)
	if err != nil {
		return nil, errorsmod.Wrap(err, "invalid creator address")
	}

	// Charge finalization fee of $3 USD
	feeUSD := math.LegacyNewDec(3) // $3 USD fee for token finalization
	err = k.ChargeAndSplitFee(sdkCtx, creatorAddr, feeUSD)
	if err != nil {
		return nil, errorsmod.Wrap(err, "failed to charge finalization fee")
	}

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
			sdk.NewAttribute("fee_charged", feeUSD.String()),
		),
	)

	return &types.MsgFinalizeTokenResponse{}, nil
}
