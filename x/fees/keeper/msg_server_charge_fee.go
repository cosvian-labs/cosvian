package keeper

import (
	"context"
	"encoding/json"

	"cosvian/x/fees/types"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

func (k msgServer) ChargeFee(ctx context.Context, msg *types.MsgChargeFee) (*types.MsgChargeFeeResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Validate creator address
	creatorAddr, err := k.addressCodec.StringToBytes(msg.Creator)
	if err != nil {
		return nil, errorsmod.Wrap(err, "invalid creator address")
	}

	// Get fee parameters
	params, err := k.Params.Get(sdkCtx)
	if err != nil {
		return nil, errorsmod.Wrap(err, "failed to get fee parameters")
	}

	// Get fee table entry for the category
	var feeEntry types.FeeConfig
	switch msg.Category {
	case "pos_payment":
		feeEntry = params.FeeTableUsd.PosPayment
	case "token_interaction":
		feeEntry = params.FeeTableUsd.TokenInteraction
	case "native_transfer":
		feeEntry = params.FeeTableUsd.NativeTransfer
	case "dex_native":
		feeEntry = params.FeeTableUsd.DexNative
	case "dex_user":
		feeEntry = params.FeeTableUsd.DexUser
	case "deploy":
		feeEntry = params.FeeTableUsd.Deploy
	case "wizard":
		feeEntry = params.FeeTableUsd.Wizard
	default:
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "fee category '%s' not found", msg.Category)
	}

	// Check if transaction is free (deploy or wizard)
	if msg.Category == "deploy" || msg.Category == "wizard" {
		// Free transaction - no gas charge
		return &types.MsgChargeFeeResponse{
			GasUsed:    "0",
			FeeCharged: "0",
		}, nil
	}

	// Calculate fee in USD (UsdAmount is already in LegacyDec format)
	feeUSD := feeEntry.UsdAmount

	// Convert USD to CSV using the module's oracle adapter (with fallback)
	btoAmount, _, err := k.ConvertUSDToBTO(sdkCtx, feeUSD)
	if err != nil {
		return nil, errorsmod.Wrap(err, "failed to convert USD to CSV")
	}

	// Scale CSV to ucsv (assume 6 decimals)
	feeucsv := btoAmount.MulInt64(1_000_000).TruncateInt()

	// Check if user has sufficient balance
	userBalance := k.bankKeeper.SpendableCoins(sdkCtx, creatorAddr)
	ucsvBalance := userBalance.AmountOf("ucsv")
	if ucsvBalance.LT(feeucsv) {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInsufficientFunds, "insufficient balance: required %s ucsv, available %s ucsv",
			feeucsv.String(), ucsvBalance.String())
	}

	// Build metadata map from msg.Metadata for dynamic recipients
	md := make(map[string]interface{})
	if msg.Metadata != "" {
		_ = json.Unmarshal([]byte(msg.Metadata), &md)
	}

	// Delegate distribution to keeper with dynamic logic
	feeCoin := sdk.NewCoin("ucsv", feeucsv)
	if err := k.DistributeFee(sdkCtx, creatorAddr, msg.Category, feeCoin, md); err != nil {
		return nil, errorsmod.Wrap(err, "fee distribution failed")
	}

	// Set gas used to equal fee amount (CSV-equivalent shown via ucsv)
	sdkCtx.GasMeter().ConsumeGas(feeucsv.Uint64(), "fee charge")

	// Emit standardized event (no legacy FeeCharged)
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(types.EventTypeFeeCharged,
			sdk.NewAttribute("category", msg.Category),
			// alias for backward compatibility
			sdk.NewAttribute("fee_type", msg.Category),
			sdk.NewAttribute("usd_amount", feeUSD.String()),
			sdk.NewAttribute("bto_amount", btoAmount.String()),
			sdk.NewAttribute("provided_fee", sdk.NewCoins(sdk.NewCoin("ucsv", feeucsv)).String()),
			sdk.NewAttribute("system_exempt", "false"),
		),
	)

	return &types.MsgChargeFeeResponse{
		GasUsed:    feeucsv.String(),
		FeeCharged: feeucsv.String(),
	}, nil
}
