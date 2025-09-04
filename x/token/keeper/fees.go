package keeper

import (
	"cosmossdk.io/errors"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

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
	// scale BTO to ubto (assume 6 decimals)
	feeCoin := sdk.NewCoin("ubto", feeBTO.MulInt64(1_000_000).TruncateInt())

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

// GetFeeInUBTO menghitung berapa ubto yang dibutuhkan untuk fee USD tertentu
func (k Keeper) GetFeeInUBTO(ctx sdk.Context, usdAmount math.LegacyDec) (sdk.Coin, error) {
	feeBTO, _, err := k.feesKeeper.ConvertUSDToBTO(ctx, usdAmount)
	if err != nil {
		return sdk.Coin{}, errors.Wrapf(err, "failed to convert USD to BTO")
	}
	return sdk.NewCoin("ubto", feeBTO.MulInt64(1_000_000).TruncateInt()), nil
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

	// Calculate BTO amount using central fees keeper
	feeBTO, _, err := k.feesKeeper.ConvertUSDToBTO(ctx, feeUSD)
	if err != nil {
		return errors.Wrapf(err, "failed to convert USD to BTO")
	}
	feeCoin := sdk.NewCoin("ubto", feeBTO.MulInt64(1_000_000).TruncateInt())

	// Check if sender has enough balance
	balance := k.bankKeeper.SpendableCoins(ctx, sender)
	if balance.AmountOf("ubto").LT(feeCoin.Amount) {
		return errors.Wrapf(ErrInsufficientFunds,
			"insufficient funds: required %s, available %s ubto",
			feeCoin.Amount.String(), balance.AmountOf("ubto").String())
	}

	// Delegate distribution to fees module
	return k.feesKeeper.DistributeFee(ctx, sender, feeType, feeCoin, metadata)
}
