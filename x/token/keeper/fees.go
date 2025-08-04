package keeper

import (
	"cosmossdk.io/errors"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ErrOraclePriceUnavailable oracle price unavailable error
var ErrOraclePriceUnavailable = errors.Register("token", 1101, "oracle price unavailable")

// ErrInsufficientFunds insufficient funds error
var ErrInsufficientFunds = errors.Register("token", 1102, "insufficient funds")

// InfrastructureModuleAccount module account name for infrastructure
const InfrastructureModuleAccount = "infrastructure"

// TreasuryModuleAccount module account name for treasury
const TreasuryModuleAccount = "treasury"

// ChargeAndSplitFee memotong fee dari sender dan membagi 50:50 ke treasury dan infrastructure
func (k Keeper) ChargeAndSplitFee(ctx sdk.Context, sender sdk.AccAddress, usdAmount math.LegacyDec) error {
	// 1. Ambil harga BTO/USD dari oracle keeper
	// Menggunakan oracle module yang sudah ada di aplikasi
	oracleKeeper := k.getOracleKeeper(ctx)
	btoPrice := oracleKeeper.GetBTOPerUSD(ctx)

	if btoPrice.IsZero() {
		return errors.Wrapf(ErrOraclePriceUnavailable, "BTO price is zero or unavailable")
	}

	// 2. Hitung fee dalam ubto (BTO tokens dengan 6 decimal places)
	// Formula: usdAmount / btoPrice * 1e6 = ubto amount
	feeBTO := usdAmount.Quo(btoPrice).Mul(math.LegacyNewDec(1_000_000))
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
	err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, sender, TreasuryModuleAccount, sdk.NewCoins(toTreasury))
	if err != nil {
		return errors.Wrapf(err, "failed to send fee to treasury")
	}

	// 6. Potong fee dari sender ke infrastructure module
	err = k.bankKeeper.SendCoinsFromAccountToModule(ctx, sender, InfrastructureModuleAccount, sdk.NewCoins(toInfra))
	if err != nil {
		return errors.Wrapf(err, "failed to send fee to infrastructure")
	}

	// 7. Emit event
	ctx.EventManager().EmitEvent(
		sdk.NewEvent("FeeCharged",
			sdk.NewAttribute("sender", sender.String()),
			sdk.NewAttribute("fee_usd", usdAmount.String()),
			sdk.NewAttribute("bto_price", btoPrice.String()),
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
	// Ambil harga BTO/USD dari oracle
	oracleKeeper := k.getOracleKeeper(ctx)
	btoPrice := oracleKeeper.GetBTOPerUSD(ctx)

	if btoPrice.IsZero() {
		return sdk.Coin{}, errors.Wrapf(ErrOraclePriceUnavailable, "BTO price unavailable")
	}

	// Hitung fee dalam ubto
	feeBTO := usdAmount.Quo(btoPrice).Mul(math.LegacyNewDec(1_000_000))
	return sdk.NewCoin("ubto", feeBTO.TruncateInt()), nil
}
