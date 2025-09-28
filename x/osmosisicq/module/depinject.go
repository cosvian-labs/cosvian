package osmosisicq

import (
	"sync"

	"cosmossdk.io/core/address"
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/core/store"
	"cosmossdk.io/depinject"
	"cosmossdk.io/depinject/appconfig"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	icqctrltypes "cosvian/x/icqcontroller/types"
	"cosvian/x/osmosisicq/keeper"
	"cosvian/x/osmosisicq/types"
)

var _ depinject.OnePerModuleType = AppModule{}

// IsOnePerModuleType implements the depinject.OnePerModuleType interface.
func (AppModule) IsOnePerModuleType() {}

func init() {
	registerOnce.Do(func() {
		appconfig.Register(
			&types.Module{},
			appconfig.Provide(ProvideModule),
		)
	})
}

// ensure registration happens only once even if this package is imported multiple times
var registerOnce sync.Once

type ModuleInputs struct {
	depinject.In

	Config       *types.Module
	StoreService store.KVStoreService
	Cdc          codec.Codec
	AddressCodec address.Codec

	AuthKeeper   types.AuthKeeper
	BankKeeper   types.BankKeeper
	OracleKeeper types.OracleKeeper
	// Optional ICQ client implementation
	ICQClient types.ICQClient `optional:"true"`
}

type ModuleOutputs struct {
	depinject.Out

	Module appmodule.AppModule
	// Expose keeper explicitly so the application can inject &app.OsmosisicqKeeper (for debugging / cross-module usage)
	OsmosisicqKeeper keeper.Keeper
	// also export keeper as an ICQ KV consumer for other modules (e.g., icqcontroller)
	KVConsumer icqctrltypes.KVResultConsumer
}

func ProvideModule(in ModuleInputs) ModuleOutputs {
	// default to governance authority if not provided
	authority := authtypes.NewModuleAddress(types.GovModuleName)
	if in.Config.Authority != "" {
		authority = authtypes.NewModuleAddressOrBech32Address(in.Config.Authority)
	}
	// Instantiate a minimal registration-only ICQ client if none provided.
	icqClient := in.ICQClient
	if icqClient == nil {
		icqClient = &regOnlyClient{}
	}
	k := keeper.NewKeeper(
		in.StoreService,
		in.Cdc,
		in.AddressCodec,
		authority,
		in.OracleKeeper,
		icqClient,
	)
	m := NewAppModule(in.Cdc, k, in.AuthKeeper, in.BankKeeper)

	// Return a thin adapter for KVConsumer to avoid duplicate concrete-type registration.
	return ModuleOutputs{Module: m, OsmosisicqKeeper: k, KVConsumer: kvConsumerAdapter{k: k}}
}

// kvConsumerAdapter forwards KV results to the keeper while presenting a distinct
// concrete type for DI to bind to the KVResultConsumer interface.
type kvConsumerAdapter struct{ k keeper.Keeper }

func (a kvConsumerAdapter) OnKVResult(ctx sdk.Context, store string, key, value []byte) error {
	return a.k.OnKVResult(ctx, store, key, value)
}
