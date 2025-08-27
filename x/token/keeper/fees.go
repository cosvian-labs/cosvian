package keeper

import (
	"cosmossdk.io/errors"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ErrOraclePriceUnavailable oracle price unavailable error
var ErrOraclePriceUnavailable = errors.Register("token", 2001, "oracle price unavailable")

// ErrInsufficientFunds insufficient funds error
var ErrInsufficientFunds = errors.Register("token", 2002, "insufficient funds")

// InfrastructureModuleAccount module account name for infrastructure
const InfrastructureModuleAccount = "infrastructure"

// TreasuryModuleAccount module account name for treasury
const TreasuryModuleAccount = "treasury"

// Fee types for different transaction categories
const (
	FeeTypePOSPayment       = "pos_payment"       // $0.15
	FeeTypeTokenInteraction = "token_interaction" // $3.00
	FeeTypeNativeTransfer   = "native_transfer"   // $1.00
	FeeTypeDEXSwapNative    = "dex_swap_native"   // $1.00
	FeeTypeDEXSwapUser      = "dex_swap_user"     // $3.00
	FeeTypeTokenCreation    = "token_creation"    // Free
	FeeTypeContractDeploy   = "contract_deploy"   // Free
)

// ChargeAndSplitFee memotong fee dari sender dan membagi 50:50 ke treasury dan infrastructure
func (k Keeper) ChargeAndSplitFee(ctx sdk.Context, sender sdk.AccAddress, usdAmount math.LegacyDec) error {
	// Use the central fees keeper to convert USD to BTO/ubto
	feeBTO, pd, err := k.feesKeeper.ConvertUSDToBTO(ctx, usdAmount)
	if err != nil {
		return errors.Wrapf(err, "failed to convert USD to BTO")
	}
	feeCoin := sdk.NewCoin("ubto", feeBTO.TruncateInt())

	// 3. Cek apakah sender memiliki saldo yang cukup
	balance := k.bankKeeper.SpendableCoins(ctx, sender)
	if balance.AmountOf("ubto").LT(feeCoin.Amount) {
		return errors.Wrapf(ErrInsufficientFunds,
			"insufficient funds: required %s, available %s ubto",
			feeCoin.Amount.String(), balance.AmountOf("ubto").String())
	}

	// 4. Bagi fee 50:50
	split := feeCoin.Amount.Quo(math.NewInt(2))
	toTreasury := sdk.NewCoin("ubto", split)
	toInfra := sdk.NewCoin("ubto", feeCoin.Amount.Sub(split)) // Sisa untuk infra (menghindari rounding error)

	// 5. Potong fee dari sender ke treasury module account
	if err = k.bankKeeper.SendCoinsFromAccountToModule(ctx, sender, TreasuryModuleAccount, sdk.NewCoins(toTreasury)); err != nil {
		return errors.Wrapf(err, "failed to send fee to treasury")
	}

	// 6. Potong fee dari sender ke infrastructure module
	if err = k.bankKeeper.SendCoinsFromAccountToModule(ctx, sender, InfrastructureModuleAccount, sdk.NewCoins(toInfra)); err != nil {
		return errors.Wrapf(err, "failed to send fee to infrastructure")
	}

	// 7. Emit event
	ctx.EventManager().EmitEvent(
		sdk.NewEvent("FeeCharged",
			sdk.NewAttribute("sender", sender.String()),
			sdk.NewAttribute("fee_usd", usdAmount.String()),
			sdk.NewAttribute("bto_price", pd.String()),
			sdk.NewAttribute("total_fee_ubto", feeCoin.String()),
			sdk.NewAttribute("treasury_fee", toTreasury.String()),
			sdk.NewAttribute("infrastructure_fee", toInfra.String()),
		),
	)

	return nil
}

// getOracleKeeper helper function untuk mendapatkan oracle keeper
// Implementasi sementara - nanti bisa diganti dengan dependency injection yang proper
func (k Keeper) getOracleKeeper(ctx sdk.Context) oracleKeeperInterface {
	// Untuk sekarang kita return mock interface
	// Dalam implementasi sesungguhnya, oracle keeper akan diinjek via constructor
	return &mockOracleKeeper{}
}

// oracleKeeperInterface interface untuk oracle keeper
type oracleKeeperInterface interface {
	GetBTOPerUSD(ctx sdk.Context) math.LegacyDec
}

// mockOracleKeeper implementasi sementara untuk testing
type mockOracleKeeper struct{}

func (m *mockOracleKeeper) GetBTOPerUSD(ctx sdk.Context) math.LegacyDec {
	// Return default price 1 BTO = 0.5 USD (untuk testing)
	// Dalam implementasi sesungguhnya, ini akan mengambil dari Band Protocol
	return math.LegacyNewDecWithPrec(5, 1) // 0.5
}

// GetFeeInUBTO menghitung berapa ubto yang dibutuhkan untuk fee USD tertentu
func (k Keeper) GetFeeInUBTO(ctx sdk.Context, usdAmount math.LegacyDec) (sdk.Coin, error) {
	feeBTO, _, err := k.feesKeeper.ConvertUSDToBTO(ctx, usdAmount)
	if err != nil {
		return sdk.Coin{}, errors.Wrapf(err, "failed to convert USD to BTO")
	}
	return sdk.NewCoin("ubto", feeBTO.TruncateInt()), nil
}

// GetFeeByType returns the USD fee amount for a given fee type
func (k Keeper) GetFeeByType(ctx sdk.Context, feeType string) math.LegacyDec {
	// Delegate to the fees module for configured fee amounts
	return k.feesKeeper.GetFeeByType(ctx, feeType)
}

// ChargeAndDistributeFeeByType charges and distributes fee based on transaction type
func (k Keeper) ChargeAndDistributeFeeByType(
	ctx sdk.Context,
	sender sdk.AccAddress,
	feeType string,
	metadata map[string]interface{},
) error {
	// Get fee amount for this type
	feeUSD := k.GetFeeByType(ctx, feeType)

	// If fee is zero (free transactions), skip charging
	if feeUSD.IsZero() {
		ctx.EventManager().EmitEvent(
			sdk.NewEvent("FeeCharged",
				sdk.NewAttribute("sender", sender.String()),
				sdk.NewAttribute("fee_type", feeType),
				sdk.NewAttribute("fee_usd", "0"),
				sdk.NewAttribute("total_fee_ubto", "0"),
			),
		)
		return nil
	}

	// Calculate BTO amount using existing oracle logic
	oracleKeeper := k.getOracleKeeper(ctx)
	btoPrice := oracleKeeper.GetBTOPerUSD(ctx)

	if btoPrice.IsZero() {
		return errors.Wrapf(ErrOraclePriceUnavailable, "BTO price is zero or unavailable")
	}

	feeBTO := feeUSD.Quo(btoPrice).Mul(math.LegacyNewDec(1_000_000))
	feeCoin := sdk.NewCoin("ubto", feeBTO.TruncateInt())

	// Check if sender has enough balance
	balance := k.bankKeeper.SpendableCoins(ctx, sender)
	if balance.AmountOf("ubto").LT(feeCoin.Amount) {
		return errors.Wrapf(ErrInsufficientFunds,
			"insufficient funds: required %s, available %s ubto",
			feeCoin.Amount.String(), balance.AmountOf("ubto").String())
	}

	// Distribute based on fee type
	return k.distributeFeeByType(ctx, sender, feeType, feeCoin, metadata)
}

// distributeFeeByType handles different distribution logic based on fee type
func (k Keeper) distributeFeeByType(
	ctx sdk.Context,
	sender sdk.AccAddress,
	feeType string,
	feeCoin sdk.Coin,
	metadata map[string]interface{},
) error {
	switch feeType {
	case FeeTypePOSPayment:
		return k.distributePOSPaymentFee(ctx, sender, feeCoin, metadata)
	case FeeTypeTokenInteraction, FeeTypeDEXSwapUser:
		return k.distributeTokenInteractionFee(ctx, sender, feeCoin, metadata)
	case FeeTypeNativeTransfer, FeeTypeDEXSwapNative:
		return k.distributeNativeTransferFee(ctx, sender, feeCoin)
	default:
		// Default to treasury distribution
		return k.distributeNativeTransferFee(ctx, sender, feeCoin)
	}
}

// distributePOSPaymentFee handles 50% treasury, 50% retail wallet (locked 6mo)
func (k Keeper) distributePOSPaymentFee(
	ctx sdk.Context,
	sender sdk.AccAddress,
	feeCoin sdk.Coin,
	metadata map[string]interface{},
) error {
	// Split 50:50
	split := feeCoin.Amount.Quo(math.NewInt(2))
	toTreasury := sdk.NewCoin("ubto", split)
	toRetail := sdk.NewCoin("ubto", feeCoin.Amount.Sub(split))

	// Send to treasury
	err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, sender, TreasuryModuleAccount, sdk.NewCoins(toTreasury))
	if err != nil {
		return errors.Wrapf(err, "failed to send fee to treasury")
	}

	// TODO: Implement retail wallet locking mechanism
	// For now, send to infrastructure as placeholder
	err = k.bankKeeper.SendCoinsFromAccountToModule(ctx, sender, InfrastructureModuleAccount, sdk.NewCoins(toRetail))
	if err != nil {
		return errors.Wrapf(err, "failed to send fee to retail rewards")
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("FeeCharged",
			sdk.NewAttribute("sender", sender.String()),
			sdk.NewAttribute("fee_type", FeeTypePOSPayment),
			sdk.NewAttribute("fee_usd", k.GetFeeByType(ctx, FeeTypePOSPayment).String()),
			sdk.NewAttribute("total_fee_ubto", feeCoin.String()),
			sdk.NewAttribute("treasury_fee", toTreasury.String()),
			sdk.NewAttribute("retail_fee", toRetail.String()),
		),
	)

	return nil
}

// distributeTokenInteractionFee handles 50% treasury, 50% token developer
func (k Keeper) distributeTokenInteractionFee(
	ctx sdk.Context,
	sender sdk.AccAddress,
	feeCoin sdk.Coin,
	metadata map[string]interface{},
) error {
	// Split 50:50
	split := feeCoin.Amount.Quo(math.NewInt(2))
	toTreasury := sdk.NewCoin("ubto", split)
	toDeveloper := sdk.NewCoin("ubto", feeCoin.Amount.Sub(split))

	// Send to treasury
	err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, sender, TreasuryModuleAccount, sdk.NewCoins(toTreasury))
	if err != nil {
		return errors.Wrapf(err, "failed to send fee to treasury")
	}

	// TODO: Send to actual token developer wallet from metadata
	// For now, send to infrastructure as placeholder
	err = k.bankKeeper.SendCoinsFromAccountToModule(ctx, sender, InfrastructureModuleAccount, sdk.NewCoins(toDeveloper))
	if err != nil {
		return errors.Wrapf(err, "failed to send fee to developer")
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("FeeCharged",
			sdk.NewAttribute("sender", sender.String()),
			sdk.NewAttribute("fee_type", FeeTypeTokenInteraction),
			sdk.NewAttribute("fee_usd", k.GetFeeByType(ctx, FeeTypeTokenInteraction).String()),
			sdk.NewAttribute("total_fee_ubto", feeCoin.String()),
			sdk.NewAttribute("treasury_fee", toTreasury.String()),
			sdk.NewAttribute("developer_fee", toDeveloper.String()),
		),
	)

	return nil
}

// distributeNativeTransferFee handles 100% treasury
func (k Keeper) distributeNativeTransferFee(
	ctx sdk.Context,
	sender sdk.AccAddress,
	feeCoin sdk.Coin,
) error {
	// Send 100% to treasury
	err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, sender, TreasuryModuleAccount, sdk.NewCoins(feeCoin))
	if err != nil {
		return errors.Wrapf(err, "failed to send fee to treasury")
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("FeeCharged",
			sdk.NewAttribute("sender", sender.String()),
			sdk.NewAttribute("fee_type", FeeTypeNativeTransfer),
			sdk.NewAttribute("fee_usd", k.GetFeeByType(ctx, FeeTypeNativeTransfer).String()),
			sdk.NewAttribute("total_fee_ubto", feeCoin.String()),
			sdk.NewAttribute("treasury_fee", feeCoin.String()),
		),
	)

	return nil
}
