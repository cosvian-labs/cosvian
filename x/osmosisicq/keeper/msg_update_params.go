package keeper

import (
	"bytes"
	"context"
	"fmt"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"bitora/x/osmosisicq/types"
)

func (k msgServer) UpdateParams(ctx context.Context, req *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	// unwrap early for logging & events (panics if not sdk.Context which should not happen in Msg server)
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.Logger().Info("osmosisicq: MsgUpdateParams received", "height", sdkCtx.BlockHeight(),
		"authority", req.Authority,
		"connection_id", req.Params.ConnectionId,
		"pool_id", req.Params.PoolId,
		"base_denom", req.Params.BaseDenom,
		"quote_denom", req.Params.QuoteDenom,
		"use_twap", req.Params.UseTwap,
		"twap_window", req.Params.TwapWindowSeconds,
		"update_interval", req.Params.UpdateIntervalSeconds,
	)

	authority, err := k.addressCodec.StringToBytes(req.Authority)
	if err != nil {
		sdkCtx.Logger().Error("osmosisicq: MsgUpdateParams invalid authority format", "err", err)
		return nil, errorsmod.Wrap(err, "invalid authority address")
	}

	if !bytes.Equal(k.GetAuthority(), authority) {
		expectedAuthorityStr, _ := k.addressCodec.BytesToString(k.GetAuthority())
		sdkCtx.Logger().Error("osmosisicq: MsgUpdateParams authority mismatch", "expected", expectedAuthorityStr, "got", req.Authority)
		return nil, errorsmod.Wrapf(types.ErrInvalidSigner, "invalid authority; expected %s, got %s", expectedAuthorityStr, req.Authority)
	}

	if err := req.Params.Validate(); err != nil {
		sdkCtx.Logger().Error("osmosisicq: MsgUpdateParams params validation failed", "err", err)
		return nil, err
	}

	if err := k.Params.Set(ctx, req.Params); err != nil {
		sdkCtx.Logger().Error("osmosisicq: MsgUpdateParams params set failed", "err", err)
		return nil, err
	}

	// Emit explicit update event with full param context for downstream indexing.
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"osmosisicq_update_params",
			sdk.NewAttribute("authority", req.Authority),
			sdk.NewAttribute("connection_id", req.Params.ConnectionId),
			sdk.NewAttribute("pool_id", fmt.Sprintf("%d", req.Params.PoolId)),
			sdk.NewAttribute("base_denom", req.Params.BaseDenom),
			sdk.NewAttribute("quote_denom", req.Params.QuoteDenom),
			sdk.NewAttribute("use_twap", fmt.Sprintf("%t", req.Params.UseTwap)),
			sdk.NewAttribute("twap_window_seconds", fmt.Sprintf("%d", req.Params.TwapWindowSeconds)),
			sdk.NewAttribute("update_interval_seconds", fmt.Sprintf("%d", req.Params.UpdateIntervalSeconds)),
		),
	)

	sdkCtx.Logger().Info("osmosisicq: MsgUpdateParams applied", "height", sdkCtx.BlockHeight())
	return &types.MsgUpdateParamsResponse{}, nil
}
