package keeper_test

import (
	"testing"

	"cosmossdk.io/core/address"
	corestore "cosmossdk.io/core/store"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	ibckeeper "github.com/cosmos/ibc-go/v10/modules/core/keeper"
	"github.com/stretchr/testify/require"

	icqkeeper "bitora/x/icqcontroller/keeper"
	icqmodule "bitora/x/icqcontroller/module"
	icqtypes "bitora/x/icqcontroller/types"
)

type fixture struct {
	ctx          sdk.Context
	keeper       icqkeeper.Keeper
	addressCodec address.Codec
	cdc          codec.Codec
	storeService corestore.KVStoreService
	authority    []byte
}

func initFixture(t *testing.T) *fixture {
	t.Helper()
	encCfg := moduletestutil.MakeTestEncodingConfig(icqmodule.AppModule{})
	addressCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	storeKey := storetypes.NewKVStoreKey(icqtypes.StoreKey)
	storeService := runtime.NewKVStoreService(storeKey)
	ctx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test")).Ctx
	authority := authtypes.NewModuleAddress(icqtypes.GovModuleName)

	k := icqkeeper.NewKeeper(storeService, encCfg.Codec, addressCodec, authority, func() *ibckeeper.Keeper { return nil }, nil, nil)

	// ensure default port id is present for any caller that reads it
	_ = k.Port.Set(ctx, icqtypes.PortID)
	return &fixture{ctx: ctx, keeper: k, addressCodec: addressCodec, cdc: encCfg.Codec, storeService: storeService, authority: authority}
}

func TestPending_SetGetRemove(t *testing.T) {
	f := initFixture(t)
	ch := "channel-0"
	seq := uint64(42)
	store := "twap"
	key := []byte("foo")
	conn := "connection-0"

	require.NoError(t, f.keeper.SetPending(f.ctx, ch, seq, store, key, conn))

	s, k, c, ok := f.keeper.GetPending(f.ctx, ch, seq)
	require.True(t, ok)
	require.Equal(t, store, s)
	require.Equal(t, key, k)
	require.Equal(t, conn, c)

	require.NoError(t, f.keeper.RemovePending(f.ctx, ch, seq))
	_, _, _, ok = f.keeper.GetPending(f.ctx, ch, seq)
	require.False(t, ok)
}

func TestHandlePacketTimeout_CleansAndEmits(t *testing.T) {
	f := initFixture(t)
	ch := "channel-1"
	seq := uint64(7)
	store := "twap"
	key := []byte("bar")
	conn := "connection-1"

	require.NoError(t, f.keeper.SetPending(f.ctx, ch, seq, store, key, conn))
	// call timeout
	require.NoError(t, f.keeper.HandlePacketTimeout(f.ctx, icqtypes.PortID, ch, seq))
	_, _, _, ok := f.keeper.GetPending(f.ctx, ch, seq)
	require.False(t, ok)
}

type mockConsumer struct {
	called bool
	gotStore string
	gotKey []byte
	gotVal []byte
}

func (m *mockConsumer) OnKVResult(ctx sdk.Context, store string, key, value []byte) error {
	m.called = true
	m.gotStore = store
	m.gotKey = append([]byte(nil), key...)
	m.gotVal = append([]byte(nil), value...)
	return nil
}

// Test ack path by injecting a test ack extractor to return a fixed value, and wiring a mock consumer.
func TestHandlePacketAcknowledgement_ForwardsToConsumer_WithSeam(t *testing.T) {
	f := initFixture(t)

	// Build a keeper instance with a consumer wired using same deps.
	mc := &mockConsumer{}
	k := icqkeeper.NewKeeper(f.storeService, f.cdc, f.addressCodec, f.authority, func() *ibckeeper.Keeper { return nil }, nil, mc)

	// Prepare pending mapping
	ch := "channel-seam"
	seq := uint64(11)
	store := "twap"
	key := []byte("seam-key")
	conn := "connection-seam"
	require.NoError(t, k.SetPending(f.ctx, ch, seq, store, key, conn))

	// Inject seam to decode ack into a fixed value
	icqkeeper.SetTestAckExtractor(func(ctx sdk.Context, acknowledgement []byte) ([]byte, error) {
		return []byte("VAL"), nil
	})
	defer icqkeeper.SetTestAckExtractor(nil)

	// Build a minimal successful IBC ack JSON
	ackJSON := []byte(`{"result":"e30="}`) // base64({}) result to satisfy JSON structure; seam ignores content

	// Call handler
	err := k.HandlePacketAcknowledgement(f.ctx, icqtypes.PortID, ch, seq, ackJSON)
	require.NoError(t, err)

	// Verify consumer called and mapping cleared
	require.True(t, mc.called)
	require.Equal(t, store, mc.gotStore)
	require.Equal(t, key, mc.gotKey)
	require.Equal(t, []byte("VAL"), mc.gotVal)
	_, _, _, ok := k.GetPending(f.ctx, ch, seq)
	require.False(t, ok)
}
