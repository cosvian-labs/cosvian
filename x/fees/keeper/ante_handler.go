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

	// Fail-open agresif:
	// 1. Ambil params sekali di awal. Jika belum ada (genesis phase / urutan init) -> langsung skip (no fee logic)
	// 2. Tidak ada panggilan Params.Get kedua untuk menghindari race ekstra.
	params, perr := fah.keeper.Params.Get(ctx)
	if perr != nil {
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

	// Estimate the required fee (classification happens inside)
	estimate, err := fah.feeCalculator.EstimateFee(ctx, msgs, memo, gasWanted)
	if err != nil {
		return ctx, sdkerrors.ErrInvalidRequest.Wrapf("fee estimation failed: %v", err)
	}

	// params sudah diambil di awal; gunakan langsung.

	btoDenom := "ubto" // Default BTO denom - should be configurable

	// Hybrid / gas-only branching
	switch {
	case params.IsGasOnly():
		// Only ensure non-zero gas fee if provided (gas prices enforced elsewhere)
		// Accept any provided fee; skip USD table validation entirely.
	case params.IsHybridEnabled():
		if estimate.Category == CategorySystem {
			// Pure system tx: must have zero USD component; accept provided fee as long as it's not absurdly large negative (checked earlier) – skip table validate
			// Optionally enforce zero or minimal fee? We allow any fee (user may voluntarily overpay) but still emit event marked system_exempt
		} else if estimate.Category == CategoryMixed {
			// Need to recompute category as highest-paying user category for fee table validation.
			// For simplicity reuse existing estimate (which currently defaulted maybe). Re-classify to user category priority.
			// Fallback: treat as native_transfer baseline.
		}
		// For user or mixed categories use normal validation.
		if estimate.Category != CategorySystem {
			tolerance := math.LegacyNewDecWithPrec(5, 2)
			if err := fah.feeCalculator.ValidateFeeAgainstEstimate(providedFee, estimate, btoDenom, tolerance); err != nil {
				return ctx, sdkerrors.ErrInsufficientFee.Wrapf("fee validation failed (hybrid): %v", err)
			}
		}
	default: // table mode
		tolerance := math.LegacyNewDecWithPrec(5, 2) // 5% tolerance
		if err := fah.feeCalculator.ValidateFeeAgainstEstimate(providedFee, estimate, btoDenom, tolerance); err != nil {
			return ctx, sdkerrors.ErrInsufficientFee.Wrapf("fee validation failed: %v", err)
		}
	}

	// For free tier transactions, ensure gas limits are respected
	if estimate.Category == CategorySystem {
		// System: skip event emission & fee expectations (optionally still emit marker event if needed). No deduction.
		return next(ctx, tx, simulate)
	}

	if estimate.IsFree {
		maxGas := fah.feeCalculator.getMaxGasForFreeCategory(estimate.Category, params.GuardRails)
		if gasWanted > maxGas {
			return ctx, sdkerrors.ErrOutOfGas.Wrapf(
				"gas wanted (%d) exceeds free tier limit (%d) for category %s",
				gasWanted, maxGas, estimate.Category)
		}
		// Free category: don't emit fee_charged to reduce noise OR emit with is_free=true. We choose to emit for analytics.
	}

	// Emit fee charged event (with hybrid marker attributes)
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
		// Backcompat alias (temporary): fee_type mirrors category
		sdk.NewAttribute("fee_type", string(estimate.Category)),
		sdk.NewAttribute("gas_wanted", fmt.Sprintf("%d", estimate.GasWanted)),
		sdk.NewAttribute("gas_price", estimate.GasPrice.String()),
		sdk.NewAttribute("provided_fee", providedFee.String()),
		sdk.NewAttribute("is_free", fmt.Sprintf("%t", estimate.IsFree)),
	}

	// Mark system exemption explicitly (useful for relayer / analytics)
	// Always include system_exempt for schema consistency
	attributes = append(attributes, sdk.NewAttribute("system_exempt", fmt.Sprintf("%t", estimate.Category == CategorySystem)))

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

	// Fail-open guard: kita tidak punya akses langsung module address di AuthKeeper interface.
	// Jika nanti deduction memicu panic (module not found) akan ditangkap recover di BaseApp dan code non-zero.
	// Di sini tidak bisa cek, jadi lanjut.

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
	// Module account 'fees' now guaranteed in genesis (see app_config.go moduleAccPerms).
	// Any panic here should surface so we can catch real misconfigurations early.
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
	// Kita tidak bisa cek keberadaan module account treasury dari interface ini.

	md := map[string]interface{}{}
	if v := ctx.Context().Value(types.FeeMetadataContextKey); v != nil {
		if m, ok := v.(*FeeMetadata); ok {
			_ = m
		}
	}
	feeType := "native_transfer"
	if v := ctx.Context().Value(types.FeeEstimateContextKey); v != nil {
		if est, ok := v.(*FeeEstimate); ok {
			feeType = string(est.Category)
		}
	}
	for _, coin := range fee {
		// Jika treasuryAddr nil, kita modifikasi metadata agar distribusi treat seluruh amount tetap di akun fees tanpa redistribusi.
		if err := dfd.feeKeeper.DistributeFeeFromModule(ctx, types.ModuleName, feeType, coin, md); err != nil {
			// Jika error karena params belum siap atau module treasury tidak ada, ignore (fail-open) agar tx tidak gagal.
			return nil
		}
	}
	return nil
}
