package keeper

import (
	"fmt"
	"strings"

	"cosmossdk.io/math"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"bitora/x/fees/types"
)

// FeeCalculator handles fee calculation for SDK/Wallet middleware
type FeeCalculator struct {
	keeper        Keeper
	oracleAdapter *OracleAdapter
}

// NewFeeCalculator creates a new fee calculator
func NewFeeCalculator(keeper Keeper, oracleAdapter *OracleAdapter) *FeeCalculator {
	return &FeeCalculator{
		keeper:        keeper,
		oracleAdapter: oracleAdapter,
	}
}

// TransactionCategory represents the fee category for a transaction
type TransactionCategory string

const (
	CategoryPOS              TransactionCategory = "pos"
	CategoryTokenInteraction TransactionCategory = "token_interaction"
	CategoryNativeTransfer   TransactionCategory = "native_transfer"
	CategoryDEXNative        TransactionCategory = "dex_native"
	CategoryDEXUser          TransactionCategory = "dex_user"
	CategoryDeploy           TransactionCategory = "deploy"
	CategoryWizard           TransactionCategory = "wizard"
	CategoryDefault          TransactionCategory = "default"
)

// FeeEstimate contains the calculated fee information
type FeeEstimate struct {
	Category  TransactionCategory `json:"category"`
	USDAmount math.LegacyDec      `json:"usd_amount"`
	BTOAmount math.LegacyDec      `json:"bto_amount"`
	GasWanted uint64              `json:"gas_wanted"`
	GasPrice  math.LegacyDec      `json:"gas_price"`
	IsFree    bool                `json:"is_free"`
	PriceData *types.PriceData    `json:"price_data,omitempty"`
}

// EstimateFee calculates the fee for a transaction based on its messages and memo
func (fc *FeeCalculator) EstimateFee(ctx sdk.Context, msgs []sdk.Msg, memo string, gasWanted uint64) (*FeeEstimate, error) {
	// Classify the transaction category
	category := fc.classifyTransaction(msgs, memo)

	// Get fee configuration for this category
	params, err := fc.keeper.Params.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get params: %w", err)
	}

	var feeConfig types.FeeConfig
	var isFree bool

	switch category {
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
		isFree = feeConfig.UsdAmount.IsZero()
	case CategoryWizard:
		feeConfig = params.FeeTableUsd.Wizard
		isFree = feeConfig.UsdAmount.IsZero()
	default:
		// Use native transfer as default
		feeConfig = params.FeeTableUsd.NativeTransfer
	}

	// Handle free tier
	if isFree {
		// Apply gas cap for free transactions
		maxGas := fc.getMaxGasForFreeCategory(category, params.GuardRails)
		if gasWanted > maxGas {
			return nil, fmt.Errorf("gas wanted (%d) exceeds free tier limit (%d) for category %s", gasWanted, maxGas, category)
		}

		return &FeeEstimate{
			Category:  category,
			USDAmount: math.LegacyZeroDec(),
			BTOAmount: math.LegacyZeroDec(),
			GasWanted: gasWanted,
			GasPrice:  math.LegacyZeroDec(),
			IsFree:    true,
		}, nil
	}

	// Convert USD fee to BTO
	btoAmount, priceData, err := fc.oracleAdapter.ConvertUSDToBTO(ctx, feeConfig.UsdAmount)
	if err != nil {
		return nil, fmt.Errorf("failed to convert USD to BTO: %w", err)
	}

	// Apply guard rails
	btoAmount, err = fc.applyGuardRails(btoAmount, gasWanted, params.GuardRails)
	if err != nil {
		return nil, fmt.Errorf("guard rails validation failed: %w", err)
	}

	// Calculate gas price: gasPrice = feeBTO / gasWanted
	var gasPrice math.LegacyDec
	if gasWanted > 0 {
		gasPrice = btoAmount.QuoInt64(int64(gasWanted))
	}

	return &FeeEstimate{
		Category:  category,
		USDAmount: feeConfig.UsdAmount,
		BTOAmount: btoAmount,
		GasWanted: gasWanted,
		GasPrice:  gasPrice,
		IsFree:    false,
		PriceData: priceData,
	}, nil
}

// classifyTransaction determines the fee category based on transaction messages and memo
func (fc *FeeCalculator) classifyTransaction(msgs []sdk.Msg, memo string) TransactionCategory {
	// Check memo for explicit category hints
	memoLower := strings.ToLower(memo)
	if strings.Contains(memoLower, "pos") || strings.Contains(memoLower, "payment") {
		return CategoryPOS
	}
	if strings.Contains(memoLower, "dex") {
		if strings.Contains(memoLower, "native") {
			return CategoryDEXNative
		}
		return CategoryDEXUser
	}
	if strings.Contains(memoLower, "deploy") {
		return CategoryDeploy
	}
	if strings.Contains(memoLower, "wizard") {
		return CategoryWizard
	}

	// Classify based on message types
	for _, msg := range msgs {
		switch msg := msg.(type) {
		case *banktypes.MsgSend:
			return CategoryNativeTransfer
		case *banktypes.MsgMultiSend:
			return CategoryNativeTransfer
		case *wasmtypes.MsgStoreCode:
			return CategoryDeploy
		case *wasmtypes.MsgInstantiateContract:
			return CategoryDeploy
		case *wasmtypes.MsgExecuteContract:
			// Check if it's a token interaction
			if fc.isTokenInteraction(msg) {
				return CategoryTokenInteraction
			}
			// Check if it's a DEX interaction
			if fc.isDEXInteraction(msg) {
				return CategoryDEXUser
			}
			return CategoryTokenInteraction
		case *wasmtypes.MsgMigrateContract:
			return CategoryTokenInteraction
		case *wasmtypes.MsgUpdateAdmin:
			return CategoryTokenInteraction
		}
	}

	// Default to native transfer
	return CategoryNativeTransfer
}

// isTokenInteraction checks if a wasm execute message is a token interaction
func (fc *FeeCalculator) isTokenInteraction(msg *wasmtypes.MsgExecuteContract) bool {
	// Parse the execute message to determine if it's token-related
	// This is a simplified check - in practice, you'd parse the JSON message
	msgStr := string(msg.Msg)
	tokenKeywords := []string{"transfer", "mint", "burn", "approve", "allowance", "balance"}

	for _, keyword := range tokenKeywords {
		if strings.Contains(strings.ToLower(msgStr), keyword) {
			return true
		}
	}

	return false
}

// isDEXInteraction checks if a wasm execute message is a DEX interaction
func (fc *FeeCalculator) isDEXInteraction(msg *wasmtypes.MsgExecuteContract) bool {
	// Parse the execute message to determine if it's DEX-related
	msgStr := string(msg.Msg)
	dexKeywords := []string{"swap", "provide_liquidity", "withdraw_liquidity", "create_pair"}

	for _, keyword := range dexKeywords {
		if strings.Contains(strings.ToLower(msgStr), keyword) {
			return true
		}
	}

	return false
}

// getMaxGasForFreeCategory returns the maximum gas allowed for free tier categories
func (fc *FeeCalculator) getMaxGasForFreeCategory(category TransactionCategory, guardRails types.GuardRails) uint64 {
	switch category {
	case CategoryDeploy:
		return guardRails.MaxGasDeploy
	case CategoryWizard:
		return guardRails.MaxGasWizard
	default:
		return 0 // No free tier for other categories
	}
}

// applyGuardRails applies min/max gas price limits and validates the fee
func (fc *FeeCalculator) applyGuardRails(btoAmount math.LegacyDec, gasWanted uint64, guardRails types.GuardRails) (math.LegacyDec, error) {
	if gasWanted == 0 {
		return btoAmount, nil
	}

	// Calculate effective gas price
	gasPrice := btoAmount.QuoInt64(int64(gasWanted))

	// Apply minimum gas price
	if gasPrice.LT(guardRails.MinGasPriceBto) {
		// Adjust BTO amount to meet minimum gas price
		btoAmount = guardRails.MinGasPriceBto.MulInt64(int64(gasWanted))
	}

	// Apply maximum gas price
	if gasPrice.GT(guardRails.MaxGasPriceBto) {
		// Cap BTO amount to maximum gas price
		btoAmount = guardRails.MaxGasPriceBto.MulInt64(int64(gasWanted))
	}

	return btoAmount, nil
}

// BuildFeeFromEstimate creates a fee object from the estimate for transaction building
func (fc *FeeCalculator) BuildFeeFromEstimate(estimate *FeeEstimate, denom string) sdk.Coins {
	if estimate.IsFree || estimate.BTOAmount.IsZero() {
		return sdk.NewCoins()
	}

	// Convert decimal to integer amount
	amount := estimate.BTOAmount.TruncateInt()
	// If building a fee in micro-denom (e.g., "ubto"), scale by 1e6
	if strings.HasPrefix(denom, "u") {
		amount = estimate.BTOAmount.MulInt64(1_000_000).TruncateInt()
	}
	if amount.IsZero() {
		// Ensure minimum fee of 1 unit if not free
		amount = math.OneInt()
	}

	return sdk.NewCoins(sdk.NewCoin(denom, amount))
}

// ValidateFeeAgainstEstimate checks if the provided fee matches the estimated fee
func (fc *FeeCalculator) ValidateFeeAgainstEstimate(providedFee sdk.Coins, estimate *FeeEstimate, denom string, tolerance math.LegacyDec) error {
	expectedFee := fc.BuildFeeFromEstimate(estimate, denom)

	if estimate.IsFree {
		if !providedFee.IsZero() {
			return fmt.Errorf("expected zero fee for free tier, got: %s", providedFee)
		}
		return nil
	}

	if providedFee.IsZero() && !expectedFee.IsZero() {
		return fmt.Errorf("expected non-zero fee, got zero")
	}

	// Check if provided fee is within tolerance of expected fee
	expectedAmount := expectedFee.AmountOf(denom)
	providedAmount := providedFee.AmountOf(denom)

	if expectedAmount.IsZero() {
		return nil
	}

	// Calculate percentage difference
	diff := providedAmount.Sub(expectedAmount).Abs()
	maxDiff := tolerance.MulInt(expectedAmount).TruncateInt()

	if diff.GT(maxDiff) {
		return fmt.Errorf("fee amount %s is outside tolerance range of expected %s", providedAmount, expectedAmount)
	}

	return nil
}
