package keeper_test

import (
	"context"
	"testing"

	"cosmossdk.io/core/address"
	storetypes "cosmossdk.io/store/types"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"bitora/x/token/keeper"
	module "bitora/x/token/module"
	"bitora/x/token/types"
)

// MockBankKeeper untuk testing
type mockBankKeeper struct{}

// GetBalance implements types.BankKeeper.
func (m mockBankKeeper) GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin {
	panic("unimplemented")
}

// GetSupply implements types.BankKeeper.
func (m mockBankKeeper) GetSupply(ctx context.Context, denom string) sdk.Coin {
	panic("unimplemented")
}

func (m mockBankKeeper) SpendableCoins(ctx context.Context, addr sdk.AccAddress) sdk.Coins {
	return sdk.NewCoins()
}

func (m mockBankKeeper) MintCoins(ctx context.Context, moduleName string, amt sdk.Coins) error {
	return nil
}

func (m mockBankKeeper) BurnCoins(ctx context.Context, moduleName string, amt sdk.Coins) error {
	return nil
}

func (m mockBankKeeper) SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error {
	return nil
}

func (m mockBankKeeper) SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error {
	return nil
}

type fixture struct {
	ctx          context.Context
	keeper       keeper.Keeper
	addressCodec address.Codec
}

func initFixture(t *testing.T) *fixture {
	t.Helper()

	encCfg := moduletestutil.MakeTestEncodingConfig(module.AppModule{})
	addressCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)

	storeService := runtime.NewKVStoreService(storeKey)
	ctx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test")).Ctx

	authority := authtypes.NewModuleAddress(types.GovModuleName)
	bankKeeper := mockBankKeeper{}

	k := keeper.NewKeeper(
		storeService,
		encCfg.Codec,
		addressCodec,
		authority.Bytes(),
		bankKeeper,
	)

	// Initialize params
	if err := k.Params.Set(ctx, types.DefaultParams()); err != nil {
		t.Fatalf("failed to set params: %v", err)
	}

	return &fixture{
		ctx:          ctx,
		keeper:       k,
		addressCodec: addressCodec,
	}
}
