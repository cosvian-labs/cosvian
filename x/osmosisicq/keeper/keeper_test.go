//go:build osmosis_removed

package keeper_test

import (
	"context"
	"testing"

	"cosmossdk.io/core/address"
	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	"cosvian/x/osmosisicq/keeper"
	module "cosvian/x/osmosisicq/module"
	"cosvian/x/osmosisicq/types"
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

	authority := authtypes.NewModuleAddress(types.GovModuleName)

	// Provide a no-op oracle keeper for tests
	var noopOracle types.OracleKeeper = mockOracleKeeper{}

	k := keeper.NewKeeper(
		storeService,
		encCfg.Codec,
		addressCodec,
		authority,
		noopOracle,
		nil,
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
func TestScheduleQueriesIfDue_NoInterval(t *testing.T) {
	f := initFixture(t)
	// set interval to 0
	params, err := f.keeper.Params.Get(f.ctx)
	require.NoError(t, err)
	params.UpdateIntervalSeconds = 0
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))
	// should be no-op
	// use the original context used to construct the keeper (same store mounts)
	sdkCtx := f.ctx.(sdk.Context)
	require.NoError(t, f.keeper.ScheduleQueriesIfDue(sdkCtx))
}

func TestScheduleQueriesIfDue_IntervalSetsLastUpdate(t *testing.T) {
	f := initFixture(t)
	params, err := f.keeper.Params.Get(f.ctx)
	require.NoError(t, err)
	params.UpdateIntervalSeconds = 1
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	sdkCtx := f.ctx.(sdk.Context)

	// capture events before running
	preEvents := sdkCtx.EventManager().Events()

	// First call should set last update
	require.NoError(t, f.keeper.ScheduleQueriesIfDue(sdkCtx))
	// NextUpdate should be set > now
	next, err := f.keeper.NextUpdate.Get(sdkCtx)
	require.NoError(t, err)
	require.Greater(t, next, sdkCtx.BlockTime().Unix()-1)
	// Ensure events were emitted (register + schedule)
	postEvents := sdkCtx.EventManager().Events()
	require.GreaterOrEqual(t, len(postEvents), len(preEvents)+1)
	// sanity check: at least one event has the expected type
	found := false
	for _, ev := range postEvents {
		if ev.Type == "osmosisicq_schedule" || ev.Type == "osmosisicq_register" {
			found = true
			break
		}
	}
	require.True(t, found)

	// Calling again immediately should be a no-op and still succeed
	require.NoError(t, f.keeper.ScheduleQueriesIfDue(sdkCtx))
}

// mockOracleKeeper implements types.OracleKeeper for tests
type mockOracleKeeper struct{}

func (mockOracleKeeper) SetCSVPerUSD(ctx sdk.Context, price sdkmath.LegacyDec) error { return nil }
func (mockOracleKeeper) GetCSVPerUSD(ctx sdk.Context) sdkmath.LegacyDec {
	return sdkmath.LegacyZeroDec()
}

// mockOracleWithLGP returns a fixed Last Good Price
type mockOracleWithLGP struct{ lgp sdkmath.LegacyDec }

func (m mockOracleWithLGP) SetCSVPerUSD(ctx sdk.Context, price sdkmath.LegacyDec) error { return nil }
func (m mockOracleWithLGP) GetCSVPerUSD(ctx sdk.Context) sdkmath.LegacyDec              { return m.lgp }

// mock ICQ client for tests
type mockICQClient struct{}

func (mockICQClient) RegisterKVQuery(ctx sdk.Context, connectionID, store string, key []byte) (string, error) {
	return "qid-" + store, nil
}

func TestValidateAndPersistPrice_LowLiquidity(t *testing.T) {
	f := initFixture(t)
	sdkCtx := f.ctx.(sdk.Context)
	params, _ := f.keeper.Params.Get(f.ctx)
	params.MinLiquidity = "100"
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	// liquidity below threshold
	err := f.keeper.ValidateAndPersistPrice(sdkCtx, sdkmath.LegacyNewDec(10), sdkmath.LegacyNewDec(50))
	require.NoError(t, err)
}

func TestValidateAndPersistPrice_DeviationDrop(t *testing.T) {
	// re-init fixture with custom oracle mock (LGP=100)
	encCfg := moduletestutil.MakeTestEncodingConfig(module.AppModule{})
	addressCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	storeService := runtime.NewKVStoreService(storeKey)
	ctx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test")).Ctx
	authority := authtypes.NewModuleAddress(types.GovModuleName)
	k := keeper.NewKeeper(storeService, encCfg.Codec, addressCodec, authority, mockOracleWithLGP{lgp: sdkmath.LegacyNewDec(100)}, nil)
	require.NoError(t, k.Params.Set(ctx, types.DefaultParams()))
	// allow small deviation
	p, _ := k.Params.Get(ctx)
	p.MaxDeviation = "0.05" // 5%
	require.NoError(t, k.Params.Set(ctx, p))

	sdkCtx := ctx
	// price deviates by 20% -> should drop
	err := k.ValidateAndPersistPrice(sdkCtx, sdkmath.LegacyNewDec(120), sdkmath.LegacyNewDec(1000))
	require.NoError(t, err)
}

func TestValidateAndPersistPrice_Accept(t *testing.T) {
	encCfg := moduletestutil.MakeTestEncodingConfig(module.AppModule{})
	addressCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	storeService := runtime.NewKVStoreService(storeKey)
	ctx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test")).Ctx
	authority := authtypes.NewModuleAddress(types.GovModuleName)
	k := keeper.NewKeeper(storeService, encCfg.Codec, addressCodec, authority, mockOracleWithLGP{lgp: sdkmath.LegacyNewDec(100)}, nil)
	require.NoError(t, k.Params.Set(ctx, types.DefaultParams()))
	// allow 50% deviation
	p, _ := k.Params.Get(ctx)
	p.MaxDeviation = "0.5"
	p.MinLiquidity = "10"
	require.NoError(t, k.Params.Set(ctx, p))

	sdkCtx := ctx
	// price deviates by 4% -> accept and set LastResultTime
	err := k.ValidateAndPersistPrice(sdkCtx, sdkmath.LegacyNewDec(104), sdkmath.LegacyNewDec(100))
	require.NoError(t, err)
	// LastResultTime should be set
	_, getErr := k.LastResultTime.Get(sdkCtx)
	require.NoError(t, getErr)
}

func TestScheduleRegistersTwapQuery_WhenICQPresent(t *testing.T) {
	encCfg := moduletestutil.MakeTestEncodingConfig(module.AppModule{})
	addressCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	storeService := runtime.NewKVStoreService(storeKey)
	ctx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test")).Ctx
	authority := authtypes.NewModuleAddress(types.GovModuleName)
	k := keeper.NewKeeper(storeService, encCfg.Codec, addressCodec, authority, mockOracleKeeper{}, mockICQClient{})

	// set params with connection and twap enabled
	p := types.DefaultParams()
	p.ConnectionId = "conn-1"
	p.UpdateIntervalSeconds = 1
	p.UseTwap = true
	p.TwapWindowSeconds = 300
	require.NoError(t, k.Params.Set(ctx, p))

	sdkCtx := ctx
	require.NoError(t, k.ScheduleQueriesIfDue(sdkCtx))

	// expect a twap query id persisted
	qid, err := k.QueryIDs.Get(sdkCtx, "twap")
	require.NoError(t, err)
	require.Equal(t, "qid-twap", qid)
}

func TestScheduleRegistersSpotQuery_WhenICQPresent(t *testing.T) {
	encCfg := moduletestutil.MakeTestEncodingConfig(module.AppModule{})
	addressCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	storeService := runtime.NewKVStoreService(storeKey)
	ctx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test")).Ctx
	authority := authtypes.NewModuleAddress(types.GovModuleName)
	k := keeper.NewKeeper(storeService, encCfg.Codec, addressCodec, authority, mockOracleKeeper{}, mockICQClient{})

	// set params with connection and spot (no twap)
	p := types.DefaultParams()
	p.ConnectionId = "conn-1"
	p.UpdateIntervalSeconds = 1
	p.UseTwap = false
	p.PoolId = 42
	require.NoError(t, k.Params.Set(ctx, p))

	sdkCtx := ctx
	require.NoError(t, k.ScheduleQueriesIfDue(sdkCtx))

	// expect a spot query id persisted
	qid, err := k.QueryIDs.Get(sdkCtx, "spot")
	require.NoError(t, err)
	require.Equal(t, "qid-gamm", qid)
}

// capturing oracle mock for persistence assertions
type mockCaptureOracle struct {
	last sdkmath.LegacyDec
	set  bool
}

func (m *mockCaptureOracle) SetCSVPerUSD(ctx sdk.Context, price sdkmath.LegacyDec) error {
	m.last = price
	m.set = true
	return nil
}
func (m *mockCaptureOracle) GetCSVPerUSD(ctx sdk.Context) sdkmath.LegacyDec {
	if !m.set {
		return sdkmath.LegacyZeroDec()
	}
	return m.last
}

func TestHandleSpot_NormalizesCSVUSD(t *testing.T) {
	encCfg := moduletestutil.MakeTestEncodingConfig(module.AppModule{})
	addressCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	storeService := runtime.NewKVStoreService(storeKey)
	ctx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test")).Ctx
	authority := authtypes.NewModuleAddress(types.GovModuleName)
	cap := &mockCaptureOracle{}
	k := keeper.NewKeeper(storeService, encCfg.Codec, addressCodec, authority, cap, nil)

	// Params: base=ucsv, quote=uusdc -> given price is USDC per CSV; we invert to CSV per USD
	p := types.DefaultParams()
	p.BaseDenom = "ucsv"
	p.QuoteDenom = "uusdc"
	p.MinLiquidity = "1"
	require.NoError(t, k.Params.Set(ctx, p))

	sdkCtx := ctx
	// price 2.0 USDC per CSV -> CSV per USD = 0.5
	err := k.HandleSpotResult(sdkCtx, sdkmath.LegacyNewDec(2), sdkmath.LegacyNewDec(100))
	require.NoError(t, err)
	require.True(t, cap.last.Equal(sdkmath.LegacyMustNewDecFromStr("0.5")))
}

func TestOnKVResult_StubJSON_Twap(t *testing.T) {
	encCfg := moduletestutil.MakeTestEncodingConfig(module.AppModule{})
	addressCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	storeService := runtime.NewKVStoreService(storeKey)
	ctx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test")).Ctx
	authority := authtypes.NewModuleAddress(types.GovModuleName)
	cap := &mockCaptureOracle{}
	k := keeper.NewKeeper(storeService, encCfg.Codec, addressCodec, authority, cap, nil)

	// Params assume ucsv/uusdc inversion path
	p := types.DefaultParams()
	p.BaseDenom = "ucsv"
	p.QuoteDenom = "uusdc"
	p.MinLiquidity = "1"
	require.NoError(t, k.Params.Set(ctx, p))

	// JSON payload where price is 2 USDC per CSV, liquidity 100
	payload := []byte(`{"price":"2","liquidity":"100"}`)
	err := k.OnKVResult(ctx, "twap", []byte("unused"), payload)
	require.NoError(t, err)
	// Expect inverted 0.5 stored
	require.True(t, cap.last.Equal(sdkmath.LegacyMustNewDecFromStr("0.5")))
}

func TestOnKVResult_StubDelimited_Spot(t *testing.T) {
	encCfg := moduletestutil.MakeTestEncodingConfig(module.AppModule{})
	addressCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	storeService := runtime.NewKVStoreService(storeKey)
	ctx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test")).Ctx
	authority := authtypes.NewModuleAddress(types.GovModuleName)
	cap := &mockCaptureOracle{}
	k := keeper.NewKeeper(storeService, encCfg.Codec, addressCodec, authority, cap, nil)

	p := types.DefaultParams()
	p.BaseDenom = "uusdc"
	p.QuoteDenom = "ucsv"
	p.MinLiquidity = "1"
	require.NoError(t, k.Params.Set(ctx, p))

	// Delimited form price|liquidity represents CSV per USD directly (no inversion for this pair)
	payload := []byte("1.25|1000")
	err := k.OnKVResult(ctx, "gamm", []byte("pool/42"), payload)
	require.NoError(t, err)
	require.True(t, cap.last.Equal(sdkmath.LegacyMustNewDecFromStr("1.25")))
}
