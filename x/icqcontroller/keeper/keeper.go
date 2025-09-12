package keeper

import (
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	corestore "cosmossdk.io/core/store"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	capabilitykeeper "github.com/cosmos/ibc-go/modules/capability/keeper"
	ibckeeper "github.com/cosmos/ibc-go/v10/modules/core/keeper"

	"bitora/x/icqcontroller/types"
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
	// ActiveChannel maps a connection-id to the open channel-id used for ICQ
	ActiveChannel collections.Map[string, string]

	ibcKeeperFn func() *ibckeeper.Keeper
	// scopedKeeper (optional) allows this module to own the IBC port capability so
	// channel handshakes can succeed. When unset, genesis will skip capability ops.
	scopedKeeper *capabilitykeeper.ScopedKeeper
}

func NewKeeper(
	storeService corestore.KVStoreService,
	cdc codec.Codec,
	addressCodec address.Codec,
	authority []byte,
	ibcKeeperFn func() *ibckeeper.Keeper,
	scopedKeeper *capabilitykeeper.ScopedKeeper,

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

		ibcKeeperFn: ibcKeeperFn,
		scopedKeeper: scopedKeeper,
	Port:          collections.NewItem(sb, types.PortKey, "port", collections.StringValue),
	Params:        collections.NewItem(sb, types.ParamsKey, "params", codec.CollValue[types.Params](cdc)),
	ActiveChannel: collections.NewMap(sb, collections.NewPrefix("ac_icqcontroller"), "active_channel", collections.StringKey, collections.StringValue),
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

// IBCKeeper returns the injected IBC keeper instance (if available).
func (k Keeper) IBCKeeper() *ibckeeper.Keeper { return k.ibcKeeperFn() }
// SetActiveChannel records the active channel-id for a given connection-id.
func (k Keeper) SetActiveChannel(ctx sdk.Context, connectionID, channelID string) error {
	return k.ActiveChannel.Set(ctx, connectionID, channelID)
}

// RemoveActiveChannel removes any mapping for the given connection-id.
func (k Keeper) RemoveActiveChannel(ctx sdk.Context, connectionID string) error {
	return k.ActiveChannel.Remove(ctx, connectionID)
}

// GetChannelForConnection returns the channel-id bound to the given connection-id, if any.
func (k Keeper) GetChannelForConnection(ctx sdk.Context, connectionID string) (string, bool) {
	ch, err := k.ActiveChannel.Get(ctx, connectionID)
	if err != nil {
		return "", false
	}
	return ch, true
}
