package keeper

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"bitora/x/fees/types"
)

// FeeAnteHandler validates transaction fees against the fee table
type FeeAnteHandler struct {
	keeper        Keeper
	feeCalculator *FeeCalculator
	bankKeeper    types.BankKeeper
}

// NewFeeAnteHandler creates a new fee ante handler
func NewFeeAnteHandler(keeper Keeper, feeCalculator *FeeCalculator, bankKeeper types.BankKeeper) *FeeAnteHandler {
	return &FeeAnteHandler{
		keeper:        keeper,
		feeCalculator: feeCalculator,
		bankKeeper:    bankKeeper,
	}
}

// AnteHandle validates fees according to the fee table mechanism
func (fah *FeeAnteHandler) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	// Skip fee validation in simulation mode
	if simulate {
		return next(ctx, tx, simulate)
	}

	// Get transaction details
	feeTx, ok := tx.(sdk.FeeTx)
	if !ok {
		return ctx, sdkerrors.ErrTxDecode.Wrap("Tx must be a FeeTx")
	}

	msgs := tx.GetMsgs()
	// Get memo from transaction - use empty string if not available
	var memo string
	if txWithMemo, ok := tx.(interface{ GetMemo() string }); ok {
		memo = txWithMemo.GetMemo()
	}
	providedFee := feeTx.GetFee()
	gasWanted := feeTx.GetGas()

	// Parse fee metadata from memo if present
	feeMetadata, err := fah.parseFeeMetadata(memo)
	if err != nil {
		return ctx, sdkerrors.ErrInvalidRequest.Wrapf("invalid transaction metadata: %v", err)
	}

	// Estimate the required fee
	estimate, err := fah.feeCalculator.EstimateFee(ctx, msgs, memo, gasWanted)
	if err != nil {
		return ctx, sdkerrors.ErrInvalidRequest.Wrapf("fee estimation failed: %v", err)
	}

	// Get BTO denom from params
	params, err := fah.keeper.Params.Get(ctx)
	if err != nil {
		return ctx, sdkerrors.ErrInvalidRequest.Wrapf("failed to get params: %v", err)
	}

	btoDenom := "ubto" // Default BTO denom - should be configurable

	// Validate the provided fee against the estimate
	tolerance := math.LegacyNewDecWithPrec(5, 2) // 5% tolerance
	err = fah.feeCalculator.ValidateFeeAgainstEstimate(providedFee, estimate, btoDenom, tolerance)
	if err != nil {
		return ctx, sdkerrors.ErrInsufficientFee.Wrapf("fee validation failed: %v", err)
	}

	// For free tier transactions, ensure gas limits are respected
	if estimate.IsFree {
		maxGas := fah.feeCalculator.getMaxGasForFreeCategory(estimate.Category, params.GuardRails)
		if gasWanted > maxGas {
			return ctx, sdkerrors.ErrOutOfGas.Wrapf(
				"gas wanted (%d) exceeds free tier limit (%d) for category %s",
				gasWanted, maxGas, estimate.Category)
		}
	}

	// Emit fee charged event
	fah.emitFeeChargedEvent(ctx, estimate, providedFee, feeMetadata)

	// Store fee metadata in the underlying stdlib context for later use
	ctx = ctx.WithContext(context.WithValue(ctx.Context(), types.FeeEstimateContextKey, estimate))
	ctx = ctx.WithContext(context.WithValue(ctx.Context(), types.FeeMetadataContextKey, feeMetadata))

	return next(ctx, tx, simulate)
}

// FeeMetadata contains additional fee information from transaction memo
type FeeMetadata struct {
	Category   string `json:"category,omitempty"`
	UserAgent  string `json:"user_agent,omitempty"`
	AppVersion string `json:"app_version,omitempty"`
	TxHash     string `json:"tx_hash,omitempty"`
	Timestamp  int64  `json:"timestamp,omitempty"`
}

// parseFeeMetadata extracts fee metadata from transaction memo
func (fah *FeeAnteHandler) parseFeeMetadata(memo string) (*FeeMetadata, error) {
	if memo == "" {
		return &FeeMetadata{}, nil
	}

	// Look for JSON metadata in memo
	if strings.HasPrefix(memo, "{") && strings.HasSuffix(memo, "}") {
		var metadata FeeMetadata
		err := json.Unmarshal([]byte(memo), &metadata)
		if err != nil {
			// If JSON parsing fails, treat as regular memo
			return &FeeMetadata{}, nil
		}
		return &metadata, nil
	}

	// Parse simple key-value pairs from memo
	metadata := &FeeMetadata{}
	parts := strings.Split(memo, ";")
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(kv[0]))
		value := strings.TrimSpace(kv[1])

		switch key {
		case "category", "cat":
			metadata.Category = value
		case "user_agent", "ua":
			metadata.UserAgent = value
		case "app_version", "version":
			metadata.AppVersion = value
		case "tx_hash", "hash":
			metadata.TxHash = value
		}
	}

	return metadata, nil
}

// emitFeeChargedEvent emits an event when a fee is charged
func (fah *FeeAnteHandler) emitFeeChargedEvent(ctx sdk.Context, estimate *FeeEstimate, providedFee sdk.Coins, metadata *FeeMetadata) {
	attributes := []sdk.Attribute{
		sdk.NewAttribute(types.AttributeKeyCategory, string(estimate.Category)),
		sdk.NewAttribute(types.AttributeKeyUsdAmount, estimate.USDAmount.String()),
		sdk.NewAttribute(types.AttributeKeyBtoAmount, estimate.BTOAmount.String()),
		sdk.NewAttribute("gas_wanted", fmt.Sprintf("%d", estimate.GasWanted)),
		sdk.NewAttribute("gas_price", estimate.GasPrice.String()),
		sdk.NewAttribute("provided_fee", providedFee.String()),
		sdk.NewAttribute("is_free", fmt.Sprintf("%t", estimate.IsFree)),
	}

	// Add metadata attributes if available
	if metadata.UserAgent != "" {
		attributes = append(attributes, sdk.NewAttribute("user_agent", metadata.UserAgent))
	}
	if metadata.AppVersion != "" {
		attributes = append(attributes, sdk.NewAttribute("app_version", metadata.AppVersion))
	}
	if metadata.TxHash != "" {
		attributes = append(attributes, sdk.NewAttribute(types.AttributeKeyTxHash, metadata.TxHash))
	}

	// Add oracle price data if available
	if estimate.PriceData != nil {
		attributes = append(attributes,
			sdk.NewAttribute("oracle_price", estimate.PriceData.Price.String()),
			sdk.NewAttribute("oracle_twap_price", estimate.PriceData.TwapPrice.String()),
			sdk.NewAttribute("oracle_is_stale", fmt.Sprintf("%t", estimate.PriceData.IsStale)),
			sdk.NewAttribute("oracle_is_fallback", fmt.Sprintf("%t", estimate.PriceData.IsFallback)),
		)
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeFeeCharged,
			attributes...,
		),
	)
}

// DeductFeeDecorator deducts fees from the fee payer account
type DeductFeeDecorator struct {
	accountKeeper types.AuthKeeper
	bankKeeper    types.BankKeeper
	feeKeeper     Keeper
}

// NewDeductFeeDecorator creates a new fee deduction decorator
func NewDeductFeeDecorator(ak types.AuthKeeper, bk types.BankKeeper, fk Keeper) DeductFeeDecorator {
	return DeductFeeDecorator{
		accountKeeper: ak,
		bankKeeper:    bk,
		feeKeeper:     fk,
	}
}

// AnteHandle deducts fees from the fee payer account and distributes them
func (dfd DeductFeeDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	feeTx, ok := tx.(sdk.FeeTx)
	if !ok {
		return ctx, sdkerrors.ErrTxDecode.Wrap("Tx must be a FeeTx")
	}

	if !simulate {
		fee := feeTx.GetFee()
		if !fee.IsZero() {
			err := dfd.deductFees(ctx, feeTx, fee)
			if err != nil {
				return ctx, err
			}

			// Distribute fees according to the fee table splits
			err = dfd.distributeFees(ctx, fee)
			if err != nil {
				return ctx, err
			}
		}
	}

	return next(ctx, tx, simulate)
}

// deductFees deducts fees from the fee payer account
func (dfd DeductFeeDecorator) deductFees(ctx sdk.Context, feeTx sdk.FeeTx, fee sdk.Coins) error {
	feePayer := feeTx.FeePayer()
	feeGranter := feeTx.FeeGranter()

	// Use fee granter if specified, otherwise use fee payer
	deductFeesFrom := feePayer
	if feeGranter != nil {
		deductFeesFrom = feeGranter
	}

	// Ensure the account exists
	deductFeesFromAcc := dfd.accountKeeper.GetAccount(ctx, deductFeesFrom)
	if deductFeesFromAcc == nil {
		return sdkerrors.ErrUnknownAddress.Wrapf("fee payer address: %s does not exist", deductFeesFrom)
	}

	// Deduct fees from the account
	err := dfd.bankKeeper.SendCoinsFromAccountToModule(ctx, deductFeesFrom, types.ModuleName, fee)
	if err != nil {
		return sdkerrors.ErrInsufficientFunds.Wrapf("failed to deduct fees: %v", err)
	}

	return nil
}

// distributeFees distributes collected fees according to the fee table splits
func (dfd DeductFeeDecorator) distributeFees(ctx sdk.Context, fee sdk.Coins) error {
	// Get fee estimate from the underlying stdlib context to determine the split
	estimate, ok := ctx.Context().Value(types.FeeEstimateContextKey).(*FeeEstimate)
	if !ok {
		// If no estimate available, send all fees to treasury (default behavior)
		return dfd.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, "treasury", fee)
	}

	// Get the fee configuration for this category
	params, err := dfd.feeKeeper.Params.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to get params: %w", err)
	}

	var feeConfig types.FeeConfig
	switch estimate.Category {
	case CategoryPOS:
		feeConfig = params.FeeTableUsd.PosPayment
	case CategoryTokenInteraction:
		feeConfig = params.FeeTableUsd.TokenInteraction
	case CategoryNativeTransfer:
		feeConfig = params.FeeTableUsd.NativeTransfer
	case CategoryDEXNative:
		feeConfig = params.FeeTableUsd.DexNative
	case CategoryDEXUser:
		feeConfig = params.FeeTableUsd.DexUser
	case CategoryDeploy:
		feeConfig = params.FeeTableUsd.Deploy
	case CategoryWizard:
		feeConfig = params.FeeTableUsd.Wizard
	default:
		feeConfig = params.FeeTableUsd.NativeTransfer
	}

    // Distribute fees according to the split configuration
    for _, coin := range fee {
        amount := coin.Amount

		// Calculate split amounts
		treasuryAmount := feeConfig.Split.Treasury.MulInt(amount).TruncateInt()
		retailWalletAmount := feeConfig.Split.RetailWallet.MulInt(amount).TruncateInt()
		tokenDevAmount := feeConfig.Split.TokenDev.MulInt(amount).TruncateInt()
		tokenCreatorAmount := feeConfig.Split.TokenCreator.MulInt(amount).TruncateInt()

		// Ensure total doesn't exceed original amount due to rounding
		total := treasuryAmount.Add(retailWalletAmount).Add(tokenDevAmount).Add(tokenCreatorAmount)
		if total.GT(amount) {
			// Adjust treasury amount to account for rounding
			treasuryAmount = amount.Sub(retailWalletAmount).Sub(tokenDevAmount).Sub(tokenCreatorAmount)
		}

        // Resolve recipient addresses from params; fallback invalid/empty to treasury
        retailAddrStr := params.RetailWallet
        devAddrStr := params.TokenDevWallet
        creatorAddrStr := params.TokenCreatorWallet

        // treasuryAddr is not needed here since we send to module for treasury
        retailAddr, _ := dfd.feeKeeper.addressCodec.StringToBytes(retailAddrStr)
        devAddr, _ := dfd.feeKeeper.addressCodec.StringToBytes(devAddrStr)
        creatorAddr, _ := dfd.feeKeeper.addressCodec.StringToBytes(creatorAddrStr)

        // Any invalid recipient shares are re-routed to treasury
        remap := math.ZeroInt()
        if len(retailAddr) == 0 {
            remap = remap.Add(retailWalletAmount)
            retailWalletAmount = math.ZeroInt()
        }
        if len(devAddr) == 0 {
            remap = remap.Add(tokenDevAmount)
            tokenDevAmount = math.ZeroInt()
        }
        if len(creatorAddr) == 0 {
            remap = remap.Add(tokenCreatorAmount)
            tokenCreatorAmount = math.ZeroInt()
        }
        if !remap.IsZero() {
            treasuryAmount = treasuryAmount.Add(remap)
        }

        // Send to treasury module account
        if !treasuryAmount.IsZero() {
            treasuryCoins := sdk.NewCoins(sdk.NewCoin(coin.Denom, treasuryAmount))
            if err := dfd.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, "treasury", treasuryCoins); err != nil {
                return fmt.Errorf("failed to send fees to treasury: %w", err)
            }
        }

        // Send to retail wallet account (if configured)
        if !retailWalletAmount.IsZero() && len(retailAddr) > 0 {
            retailCoins := sdk.NewCoins(sdk.NewCoin(coin.Denom, retailWalletAmount))
            if err := dfd.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, retailAddr, retailCoins); err != nil {
                return fmt.Errorf("failed to send fees to retail wallet: %w", err)
            }
        }

        // Send to token developer account (if configured)
        if !tokenDevAmount.IsZero() && len(devAddr) > 0 {
            devCoins := sdk.NewCoins(sdk.NewCoin(coin.Denom, tokenDevAmount))
            if err := dfd.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, devAddr, devCoins); err != nil {
                return fmt.Errorf("failed to send fees to token developer: %w", err)
            }
        }

        // Send to token creator account (if configured)
        if !tokenCreatorAmount.IsZero() && len(creatorAddr) > 0 {
            creatorCoins := sdk.NewCoins(sdk.NewCoin(coin.Denom, tokenCreatorAmount))
            if err := dfd.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, creatorAddr, creatorCoins); err != nil {
                return fmt.Errorf("failed to send fees to token creator: %w", err)
            }
        }
    }

	return nil
}
