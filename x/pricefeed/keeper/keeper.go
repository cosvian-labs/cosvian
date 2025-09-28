package keeper

import (
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	corestore "cosmossdk.io/core/store"
	"github.com/cosmos/cosmos-sdk/codec"
	ibckeeper "github.com/cosmos/ibc-go/v10/modules/core/keeper"

	capabilitykeeper "github.com/cosmos/ibc-go/modules/capability/keeper"

	feesTypes "cosvian/x/fees/types"
	"cosvian/x/pricefeed/types"
)

type Keeper struct {
	storeService corestore.KVStoreService
	cdc          codec.Codec
	addressCodec address.Codec
	// Address capable of executing a MsgUpdateParams message.
	// Typically, this should be the x/gov module account.
	authority []byte

	Schema collections.Schema
	Params collections.Item[types.Params]

	Port collections.Item[string]

	feesKeeper feesTypes.FeesKeeper

	ibcKeeperFn  func() *ibckeeper.Keeper
	scopedKeeper *capabilitykeeper.ScopedKeeper

	// oracleKeeper is the canonical store for CSV/USD, used to expose to fees
	oracleKeeper types.OracleKeeper
}

func NewKeeper(
	storeService corestore.KVStoreService,
	cdc codec.Codec,
	addressCodec address.Codec,
	authority []byte,
	ibcKeeperFn func() *ibckeeper.Keeper,
	scopedKeeper *capabilitykeeper.ScopedKeeper,
	feesKeeper feesTypes.FeesKeeper,
	oracleKeeper types.OracleKeeper,

) Keeper {
	if _, err := addressCodec.BytesToString(authority); err != nil {
		panic(fmt.Sprintf("invalid authority address %s: %s", authority, err))
	}

	sb := collections.NewSchemaBuilder(storeService)

	k := Keeper{
		storeService: storeService,
		cdc:          cdc,
		addressCodec: addressCodec,
		authority:    authority,
		feesKeeper:   feesKeeper,
		ibcKeeperFn:  ibcKeeperFn,
		scopedKeeper: scopedKeeper,
		oracleKeeper: oracleKeeper,
		Port:         collections.NewItem(sb, types.PortKey, "port", collections.StringValue),
		Params:       collections.NewItem(sb, types.ParamsKey, "params", codec.CollValue[types.Params](cdc)),
	}

	schema, err := sb.Build()
	if err != nil {
		panic(err)
	}
	k.Schema = schema

	return k
}

// GetAuthority returns the module's authority.
func (k Keeper) GetAuthority() []byte {
	return k.authority
}
