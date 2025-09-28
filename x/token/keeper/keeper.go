package keeper

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	corestore "cosmossdk.io/core/store"
	"github.com/cosmos/cosmos-sdk/codec"

	"cosvian/x/token/types"

	feesTypes "cosvian/x/fees/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type Keeper struct {
	storeService corestore.KVStoreService
	cdc          codec.Codec
	addressCodec address.Codec
	// Address capable of executing a MsgUpdateParams message.
	// Typically, this should be the x/gov module account.
	authority []byte

	bankKeeper types.BankKeeper
	feesKeeper feesTypes.FeesKeeper

	Schema collections.Schema
	Params collections.Item[types.Params]
}

func NewKeeper(
	storeService corestore.KVStoreService,
	cdc codec.Codec,
	addressCodec address.Codec,
	authority []byte,
	bankKeeper types.BankKeeper,
	feesKeeper feesTypes.FeesKeeper,

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
		bankKeeper:   bankKeeper,
		feesKeeper:   feesKeeper,

		Params: collections.NewItem(sb, types.ParamsKey, "params", codec.CollValue[types.Params](cdc)),
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

// GetTokenMetadata retrieves token metadata by token ID
func (k Keeper) GetTokenMetadata(ctx context.Context, tokenID string) (*types.TokenMetadata, error) {
	store := k.storeService.OpenKVStore(ctx)
	key := append([]byte("tm_"), []byte(tokenID)...)
	bz, err := store.Get(key)
	if err != nil {
		return nil, err
	}
	if bz == nil {
		return nil, types.ErrTokenNotFound
	}
	var meta types.TokenMetadata
	if err := json.Unmarshal(bz, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// GetTokenCreator returns the creator address for a given tokenID
func (k Keeper) GetTokenCreator(ctx context.Context, tokenID string) (sdk.AccAddress, error) {
	meta, err := k.GetTokenMetadata(ctx, tokenID)
	if err != nil {
		return nil, err
	}
	return meta.Creator, nil
}

// GetTokenCreatorBySymbol returns the creator address for a given token symbol (case-insensitive)
func (k Keeper) GetTokenCreatorBySymbol(ctx context.Context, symbol string) (sdk.AccAddress, error) {
	store := k.storeService.OpenKVStore(ctx)
	sym := strings.ToLower(symbol)
	key := append([]byte("si_"), []byte(sym)...)
	tokenID, err := store.Get(key)
	if err != nil {
		return nil, err
	}
	if tokenID == nil {
		return nil, types.ErrTokenNotFound
	}
	meta, err := k.GetTokenMetadata(ctx, string(tokenID))
	if err != nil {
		return nil, err
	}
	return meta.Creator, nil
}
