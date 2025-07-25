package keeper

import (
	"context"

	"bitora/x/token/types"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k msgServer) Burn(goCtx context.Context, msg *types.MsgBurn) (*types.MsgBurnResponse, error) {
	// Parse sender address
	from, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		return nil, errorsmod.Wrap(err, "invalid sender address")
	}

	// Create coin for burn
	coin := sdk.NewCoin("ubto", sdkmath.NewIntFromUint64(msg.Amount))

	// Transfer coins from user account to module account
	err = k.bankKeeper.SendCoinsFromAccountToModule(goCtx, from, types.ModuleName, sdk.NewCoins(coin))
	if err != nil {
		return nil, err
	}

	// Burn coins from module account
	err = k.bankKeeper.BurnCoins(goCtx, types.ModuleName, sdk.NewCoins(coin))
	if err != nil {
		return nil, err
	}

	return &types.MsgBurnResponse{}, nil
}
