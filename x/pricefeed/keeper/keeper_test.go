package keeper_test

import (
	"context"
	"testing"

	"cosmossdk.io/core/address"
	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	ibckeeper "github.com/cosmos/ibc-go/v10/modules/core/keeper"
	ibctypes "github.com/cosmos/ibc-go/v10/modules/core/types"

	feesTypes "bitora/x/fees/types"
	"bitora/x/pricefeed/keeper"
	module "bitora/x/pricefeed/module"
	"bitora/x/pricefeed/types"

	"cosmossdk.io/math"
)

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

	authority := authtypes.NewModuleAddress(govtypes.ModuleName)
	mockUpgradeKeeper := newMockUpgradeKeeper()

	k := keeper.NewKeeper(
		storeService,
		encCfg.Codec,
		addressCodec,
		authority,
		func() *ibckeeper.Keeper {
			return ibckeeper.NewKeeper(encCfg.Codec, storeService, newMockParams(), mockUpgradeKeeper, authority.String())
		},
		(feesTypes.FeesKeeper)(nil),
		mockOracleKeeper{},
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

type mockUpgradeKeeper struct {
	clienttypes.UpgradeKeeper

	initialized bool
}

func (m mockUpgradeKeeper) GetUpgradePlan(ctx context.Context) (upgradetypes.Plan, error) {
	return upgradetypes.Plan{}, nil
}

func newMockUpgradeKeeper() *mockUpgradeKeeper {
	return &mockUpgradeKeeper{initialized: true}
}

type mockParams struct {
	ibctypes.ParamSubspace

	initialized bool
}

func newMockParams() *mockParams {
	return &mockParams{initialized: true}
}

func (mockParams) GetParamSet(ctx sdk.Context, ps paramtypes.ParamSet) {
}

// mockOracleKeeper implements types.OracleKeeper for tests
type mockOracleKeeper struct{}
type spyOracleKeeper struct{ called bool; val math.LegacyDec }
func (mockOracleKeeper) SetBTOPerUSD(ctx sdk.Context, price math.LegacyDec) error { return nil }
func (s *spyOracleKeeper) SetBTOPerUSD(ctx sdk.Context, price math.LegacyDec) error { s.called = true; s.val = price; return nil }

func TestProcessPriceResponse_Rates(t *testing.T) {
	fx := initFixture(t)
	msg := types.OracleResponsePacketData{
		RequestId: 1,
		Prices:    "",
		Rates:     `{"BTO":"0.25"}`,
		Error:     "",
	}
	if err := fx.keeper.ProcessPriceResponse(fx.ctx, msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProcessPriceResponse_PricesInvert(t *testing.T) {
	fx := initFixture(t)
	msg := types.OracleResponsePacketData{
		RequestId: 2,
		Prices:    `{"BTO":"4"}`, // 4 USD per BTO -> BTO/USD = 0.25
		Rates:     "",
		Error:     "",
	}
	if err := fx.keeper.ProcessPriceResponse(fx.ctx, msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOnRecvOracleResponse_StoresPrice(t *testing.T) {
	// Build a fixture but replace oracle keeper with spy
	encCfg := moduletestutil.MakeTestEncodingConfig(module.AppModule{})
	addressCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	storeService := runtime.NewKVStoreService(storeKey)
	ctx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test")).Ctx
	authority := authtypes.NewModuleAddress(govtypes.ModuleName)
	mockUpgradeKeeper := newMockUpgradeKeeper()
	spy := &spyOracleKeeper{}

	k := keeper.NewKeeper(
		storeService,
		encCfg.Codec,
		addressCodec,
		authority,
		func() *ibckeeper.Keeper { return ibckeeper.NewKeeper(encCfg.Codec, storeService, newMockParams(), mockUpgradeKeeper, authority.String()) },
		(feesTypes.FeesKeeper)(nil),
		spy,
	)
	if err := k.Params.Set(ctx, types.DefaultParams()); err != nil { t.Fatal(err) }

	// Build a fake packet and data
	pkt := channeltypes.Packet{}
	data := types.OracleResponsePacketData{RequestId: 10, Rates: `{"BTO":"0.5"}`}
	_, err := k.OnRecvOracleResponsePacket(ctx, pkt, data)
	if err != nil { t.Fatalf("unexpected: %v", err) }
	if !spy.called { t.Fatalf("expected oracle keeper to be called") }
}
