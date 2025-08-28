package keeper

import (
	"context"

	"bitora/x/fees/types"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
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
		return nil, errorsmod.Wrapf(err, "fee category '%s' not found", msg.Category)
	}

	// Check if transaction is free (deploy or wizard)
	if msg.Category == "deploy" || msg.Category == "wizard" {
		// Free transaction - no gas charge
		return &types.MsgChargeFeeResponse{
			GasUsed:    "0",
			FeeCharged: "0",
		}, nil
	}

	// Get BTO price from oracle
	btoPriceResult, err := k.oracleKeeper.GetLatestPrice(sdkCtx, "BTO/USD")
	if err != nil {
		return nil, errorsmod.Wrap(err, "failed to get BTO price from oracle")
	}
	
	// Parse price from string to LegacyDec
	btoPrice, err := math.LegacyNewDecFromStr(btoPriceResult.Price)
	if err != nil {
		return nil, errorsmod.Wrap(err, "failed to parse BTO price")
	}
	
	if btoPrice.IsZero() {
		return nil, errorsmod.Wrap(err, "BTO price not available")
	}

	// Calculate fee in USD (UsdAmount is already in LegacyDec format)
	feeUSD := feeEntry.UsdAmount

	// Calculate fee in BTO
	feeBTO := feeUSD.Quo(btoPrice)

	// Convert to ubto (micro BTO)
	feeUbto := feeBTO.Mul(math.LegacyNewDec(1e18)).TruncateInt()

	// Check if user has sufficient balance
	userBalance := k.bankKeeper.SpendableCoins(sdkCtx, creatorAddr)
	ubtoBalance := userBalance.AmountOf("ubto")
	if ubtoBalance.LT(feeUbto) {
		return nil, errorsmod.Wrapf(err, "insufficient balance: required %s ubto, available %s ubto",
			feeUbto.String(), ubtoBalance.String())
	}

	// Calculate distribution amounts
	treasuryAmount := feeEntry.Split.Treasury.MulInt(feeUbto).TruncateInt()
	retailWalletAmount := feeEntry.Split.RetailWallet.MulInt(feeUbto).TruncateInt()
	tokenDevAmount := feeEntry.Split.TokenDev.MulInt(feeUbto).TruncateInt()
	tokenCreatorAmount := feeEntry.Split.TokenCreator.MulInt(feeUbto).TruncateInt()

	// Get recipient addresses from params
	treasuryAddr, _ := k.addressCodec.StringToBytes(params.TreasuryWallet)
	retailWalletAddr, _ := k.addressCodec.StringToBytes(params.RetailWallet)
	tokenDevAddr, _ := k.addressCodec.StringToBytes(params.TokenDevWallet)
	tokenCreatorAddr, _ := k.addressCodec.StringToBytes(params.TokenCreatorWallet)

	// Distribute fees
	if treasuryAmount.GT(math.ZeroInt()) && len(treasuryAddr) > 0 {
		err = k.bankKeeper.SendCoins(sdkCtx, creatorAddr, treasuryAddr,
			sdk.NewCoins(sdk.NewCoin("ubto", treasuryAmount)))
		if err != nil {
			return nil, errorsmod.Wrap(err, "failed to send treasury fee")
		}
	}

	if retailWalletAmount.GT(math.ZeroInt()) && len(retailWalletAddr) > 0 {
		err = k.bankKeeper.SendCoins(sdkCtx, creatorAddr, retailWalletAddr,
			sdk.NewCoins(sdk.NewCoin("ubto", retailWalletAmount)))
		if err != nil {
			return nil, errorsmod.Wrap(err, "failed to send retail wallet fee")
		}
	}

	if tokenDevAmount.GT(math.ZeroInt()) && len(tokenDevAddr) > 0 {
		err = k.bankKeeper.SendCoins(sdkCtx, creatorAddr, tokenDevAddr,
			sdk.NewCoins(sdk.NewCoin("ubto", tokenDevAmount)))
		if err != nil {
			return nil, errorsmod.Wrap(err, "failed to send token dev fee")
		}
	}

	if tokenCreatorAmount.GT(math.ZeroInt()) && len(tokenCreatorAddr) > 0 {
		err = k.bankKeeper.SendCoins(sdkCtx, creatorAddr, tokenCreatorAddr,
			sdk.NewCoins(sdk.NewCoin("ubto", tokenCreatorAmount)))
		if err != nil {
			return nil, errorsmod.Wrap(err, "failed to send token creator fee")
		}
	}

	// Set gas used to equal fee amount (this is the key innovation!)
	sdkCtx.GasMeter().ConsumeGas(feeUbto.Uint64(), "fee charge")

	// Emit event
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent("FeeCharged",
			sdk.NewAttribute("category", msg.Category),
			sdk.NewAttribute("creator", msg.Creator),
			sdk.NewAttribute("fee_usd", feeUSD.String()),
			sdk.NewAttribute("fee_bto", feeBTO.String()),
			sdk.NewAttribute("fee_ubto", feeUbto.String()),
			sdk.NewAttribute("gas_used", feeUbto.String()),
		),
	)

	return &types.MsgChargeFeeResponse{
		GasUsed:    feeUbto.String(),
		FeeCharged: feeUbto.String(),
	}, nil
}
