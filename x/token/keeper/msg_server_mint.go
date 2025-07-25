package keeper

import (
	"context"

	"bitora/x/token/types"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k msgServer) Mint(goCtx context.Context, msg *types.MsgMint) (*types.MsgMintResponse, error) {
	// Validate authority address
	if _, err := k.addressCodec.StringToBytes(msg.Creator); err != nil {
		return nil, errorsmod.Wrap(err, "invalid authority address")
	}

	// Parse recipient address
	addr, err := sdk.AccAddressFromBech32(msg.To)
	if err != nil {
		return nil, err
	}

	// Create coin for mint
	coin := sdk.NewCoin("ubto", sdkmath.NewIntFromUint64(msg.Amount))
	
	// Mint coins to module account using bankKeeper from Keeper
	err = k.bankKeeper.MintCoins(goCtx, types.ModuleName, sdk.NewCoins(coin))
	if err != nil {
		return nil, err
	}

	// Transfer coins from module account to recipient
	err = k.bankKeeper.SendCoinsFromModuleToAccount(goCtx, types.ModuleName, addr, sdk.NewCoins(coin))
	if err != nil {
		return nil, err
	}

	return &types.MsgMintResponse{}, nil
}
