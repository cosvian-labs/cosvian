package keeper

import (
	"context"

	"cosvian/x/oracle/types"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	errorsmod "cosmossdk.io/errors"
)

func (k msgServer) SetPrice(ctx context.Context, msg *types.MsgSetPrice) (*types.MsgSetPriceResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Validate creator address
	_, err := k.addressCodec.StringToBytes(msg.Creator)
	if err != nil {
		return nil, errorsmod.Wrap(err, "invalid creator address")
	}

	// Validate denom/symbol: currently only supports CSV priced in USD
	if msg.Denom != "CSV" && msg.Denom != "ucsv" {
		return nil, errorsmod.Wrapf(types.ErrInvalidDenom, "unsupported denom: %s", msg.Denom)
	}

	// Parse price (CSV per 1 USD)
	price, err := math.LegacyNewDecFromStr(msg.Price)
	if err != nil {
		return nil, errorsmod.Wrap(err, "invalid price format")
	}

	// Validate price is positive
	if price.IsNegative() || price.IsZero() {
		return nil, errorsmod.Wrap(err, "price must be positive")
	}

	// Set the price
	err = k.SetBTOPerUSD(sdkCtx, price)
	if err != nil {
		return nil, errorsmod.Wrap(err, "failed to set price")
	}

	// Emit event
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent("PriceSet",
			sdk.NewAttribute("creator", msg.Creator),
			sdk.NewAttribute("symbol", "CSV"),
			sdk.NewAttribute("price", price.String()),
		),
	)

	return &types.MsgSetPriceResponse{}, nil
}
