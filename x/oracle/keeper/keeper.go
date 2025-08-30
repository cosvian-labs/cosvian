package keeper

import (
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	corestore "cosmossdk.io/core/store"
	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"bitora/x/oracle/types"
)

type Keeper struct {
	storeService corestore.KVStoreService
	cdc          codec.Codec
	addressCodec address.Codec
	// Address capable of executing a MsgUpdateParams message.
	// Typically, this should be the x/gov module account.
	authority []byte

	Schema   collections.Schema
	Params   collections.Item[types.Params]
	BTOPrice collections.Map[string, string] // Store BTO/USD price as string
}

func NewKeeper(
	storeService corestore.KVStoreService,
	cdc codec.Codec,
	addressCodec address.Codec,
	authority []byte,
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

		Params:   collections.NewItem(sb, types.ParamsKey, "params", codec.CollValue[types.Params](cdc)),
		BTOPrice: collections.NewMap(sb, []byte("bto_price"), "bto_price", collections.StringKey, collections.StringValue),
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

// GetBTOPerUSD returns the current BTO/USD exchange rate
func (k Keeper) GetBTOPerUSD(ctx sdk.Context) math.LegacyDec {
	// MOCK: Return hardcoded price for testing
	return math.LegacyMustNewDecFromStr("0.1")

	// Original implementation:
	// priceStr, err := k.BTOPrice.Get(ctx, "BTO_USD")
	// if err != nil {
	// 	// Return zero decimal if price not found
	// 	return math.LegacyZeroDec()
	// }
	//
	// price, err := math.LegacyNewDecFromStr(priceStr)
	// if err != nil {
	// 	// Return zero decimal if price is invalid
	// 	return math.LegacyZeroDec()
	// }
	//
	// return price
}

// SetBTOPerUSD sets the BTO/USD exchange rate
func (k Keeper) SetBTOPerUSD(ctx sdk.Context, price math.LegacyDec) error {
	return k.BTOPrice.Set(ctx, "BTO_USD", price.String())
}

// GetExchangeRate gets exchange rate for any symbol (implements BandOracleKeeper interface)
func (k Keeper) GetExchangeRate(ctx sdk.Context, symbol string) (math.LegacyDec, error) {
	if symbol == "BTO" {
		price := k.GetBTOPerUSD(ctx)
		if price.IsZero() {
			return math.LegacyZeroDec(), fmt.Errorf("BTO price not available")
		}
		return price, nil
	}

	// For other symbols, you could integrate with Band Protocol here
	// For now, return error for unsupported symbols
	return math.LegacyZeroDec(), fmt.Errorf("unsupported symbol: %s", symbol)
}
