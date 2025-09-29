package keeper

import (
	"fmt"

	"cosvian/x/oracle/types"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	corestore "cosmossdk.io/core/store"
	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
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
	CSVPrice collections.Map[string, string] // Store CSV/USD price as string
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
		CSVPrice: collections.NewMap(sb, []byte("csv_price"), "csv_price", collections.StringKey, collections.StringValue),
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

// GetCSVPerUSD returns the current CSV/USD exchange rate
func (k Keeper) GetCSVPerUSD(ctx sdk.Context) math.LegacyDec {
	priceStr, err := k.CSVPrice.Get(ctx, "CSV_USD")
	if err != nil {
		// Return zero decimal if price not found
		return math.LegacyZeroDec()
	}

	price, err := math.LegacyNewDecFromStr(priceStr)
	if err != nil {
		// Return zero decimal if price is invalid
		return math.LegacyZeroDec()
	}

	return price
}

// SetCSVPerUSD sets the CSV/USD exchange rate
func (k Keeper) SetCSVPerUSD(ctx sdk.Context, price math.LegacyDec) error {
	return k.CSVPrice.Set(ctx, "CSV_USD", price.String())
}

// GetExchangeRate gets exchange rate for any symbol (implements BandOracleKeeper interface)
func (k Keeper) GetExchangeRate(ctx sdk.Context, symbol string) (math.LegacyDec, error) {
	if symbol == "CSV" {
		price := k.GetCSVPerUSD(ctx)
		if price.IsZero() {
			return math.LegacyZeroDec(), fmt.Errorf("CSV price not available")
		}
		return price, nil
	}

	// For other symbols, you could integrate with Band Protocol here
	// For now, return error for unsupported symbols
	return math.LegacyZeroDec(), fmt.Errorf("unsupported symbol: %s", symbol)
}
