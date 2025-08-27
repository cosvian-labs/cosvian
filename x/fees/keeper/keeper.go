package keeper

import (
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	corestore "cosmossdk.io/core/store"
	"github.com/cosmos/cosmos-sdk/codec"

	math "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"bitora/x/fees/types"
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

	oracleKeeper types.OracleKeeper
}

func NewKeeper(
	storeService corestore.KVStoreService,
	cdc codec.Codec,
	addressCodec address.Codec,
	authority []byte,

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

		oracleKeeper: oracleKeeper,
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

// ConvertUSDToBTO converts a USD amount to BTO using the module's oracle adapter
func (k Keeper) ConvertUSDToBTO(ctx sdk.Context, usd math.LegacyDec) (math.LegacyDec, *types.PriceData, error) {
	oa := NewOracleAdapter(k, k.oracleKeeper)
	bto, pd, err := oa.ConvertUSDToBTO(ctx, usd)
	return bto, pd, err
}

// GetFeeByType returns the USD fee amount for a given fee type by reading module params
func (k Keeper) GetFeeByType(ctx sdk.Context, feeType string) math.LegacyDec {
	params, err := k.Params.Get(ctx)
	if err != nil {
		return math.LegacyZeroDec()
	}
	switch feeType {
	case "pos_payment":
		return params.FeeTableUsd.PosPayment.UsdAmount
	case "token_interaction":
		return params.FeeTableUsd.TokenInteraction.UsdAmount
	case "native_transfer":
		return params.FeeTableUsd.NativeTransfer.UsdAmount
	case "dex_swap_native":
		return params.FeeTableUsd.DexNative.UsdAmount
	case "dex_swap_user":
		return params.FeeTableUsd.DexUser.UsdAmount
	case "token_creation", "contract_deploy":
		return params.FeeTableUsd.Deploy.UsdAmount
	default:
		return params.FeeTableUsd.NativeTransfer.UsdAmount
	}
}
