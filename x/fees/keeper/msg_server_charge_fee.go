package keeper

import (
	"context"
	"encoding/json"

	"bitora/x/fees/types"

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

	// Convert USD to BTO using the module's oracle adapter (with fallback)
	btoAmount, _, err := k.ConvertUSDToBTO(sdkCtx, feeUSD)
	if err != nil {
		return nil, errorsmod.Wrap(err, "failed to convert USD to BTO")
	}

	// Scale BTO to ubto (assume 6 decimals)
	feeUbto := btoAmount.MulInt64(1_000_000).TruncateInt()

	// Check if user has sufficient balance
	userBalance := k.bankKeeper.SpendableCoins(sdkCtx, creatorAddr)
	ubtoBalance := userBalance.AmountOf("ubto")
	if ubtoBalance.LT(feeUbto) {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInsufficientFunds, "insufficient balance: required %s ubto, available %s ubto",
			feeUbto.String(), ubtoBalance.String())
	}

	// Build metadata map from msg.Metadata for dynamic recipients
	md := make(map[string]interface{})
	if msg.Metadata != "" {
		_ = json.Unmarshal([]byte(msg.Metadata), &md)
	}

	// Delegate distribution to keeper with dynamic logic
	feeCoin := sdk.NewCoin("ubto", feeUbto)
	if err := k.DistributeFee(sdkCtx, creatorAddr, msg.Category, feeCoin, md); err != nil {
		return nil, errorsmod.Wrap(err, "fee distribution failed")
	}

	// Set gas used to equal fee amount (BTO-equivalent shown via ubto)
	sdkCtx.GasMeter().ConsumeGas(feeUbto.Uint64(), "fee charge")

	// Emit standardized event (no legacy FeeCharged)
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(types.EventTypeFeeCharged,
			sdk.NewAttribute("category", msg.Category),
			// alias for backward compatibility
			sdk.NewAttribute("fee_type", msg.Category),
			sdk.NewAttribute("usd_amount", feeUSD.String()),
			sdk.NewAttribute("bto_amount", btoAmount.String()),
			sdk.NewAttribute("provided_fee", sdk.NewCoins(sdk.NewCoin("ubto", feeUbto)).String()),
			sdk.NewAttribute("system_exempt", "false"),
		),
	)

	return &types.MsgChargeFeeResponse{
		GasUsed:    feeUbto.String(),
		FeeCharged: feeUbto.String(),
	}, nil
}
