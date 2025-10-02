//go:build osmosis_removed

package keeper

import (
	"errors"
	"fmt"
	"time"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	corestore "cosmossdk.io/core/store"
	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"cosvian/x/osmosisicq/types"
)

type Keeper struct {
	storeService corestore.KVStoreService
	cdc          codec.Codec
	addressCodec address.Codec
	// Address capable of executing a MsgUpdateParams message.
	// Typically, this should be the x/gov module account.
	authority []byte

	// oracleKeeper is used to persist computed CSV/USD.
	oracleKeeper types.OracleKeeper
	// icqClient is optional and used to register KV queries when present
	icqClient types.ICQClient

	Schema         collections.Schema
	Params         collections.Item[types.Params]
	LastUpdate     collections.Item[int64]
	NextUpdate     collections.Item[int64]
	LastResultTime collections.Item[int64]
	QueryIDs       collections.Map[string, string]
}

func NewKeeper(
	storeService corestore.KVStoreService,
	cdc codec.Codec,
	addressCodec address.Codec,
	authority []byte,
	oracleKeeper types.OracleKeeper,
	icqClient types.ICQClient,

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
		icqClient:    icqClient,

		Params:         collections.NewItem(sb, types.ParamsKey, "params", codec.CollValue[types.Params](cdc)),
		LastUpdate:     collections.NewItem(sb, types.LastUpdateKey, "last_update", collections.Int64Value),
		NextUpdate:     collections.NewItem(sb, types.NextUpdateKey, "next_update", collections.Int64Value),
		LastResultTime: collections.NewItem(sb, types.LastResultTimeKey, "last_result_time", collections.Int64Value),
		QueryIDs:       collections.NewMap(sb, types.QueryIDsKey, "query_ids", collections.StringKey, collections.StringValue),
	}

	schema, err := sb.Build()
	if err != nil {
		panic(err)
	}
	k.Schema = schema

	return k
}

// ScheduleQueriesIfDue checks params.update_interval_seconds and records last update time.
// For now this is a no-op placeholder to keep wiring green.
func (k Keeper) ScheduleQueriesIfDue(ctx sdk.Context) error {
	params, err := k.Params.Get(ctx)
	if err != nil {
		return err
	}
	interval := params.UpdateIntervalSeconds
	if interval == 0 {
		return nil
	}
	now := ctx.BlockTime().Unix()
	// read next scheduled time if present
	next, err := k.NextUpdate.Get(ctx)
	if err != nil && !errors.Is(err, collections.ErrNotFound) {
		return err
	}
	if err == nil {
		if now < next {
			// not due yet
			return nil
		}
	} else {
		// fall back to last update window gate (for backward compatibility)
		if last, e2 := k.LastUpdate.Get(ctx); e2 == nil {
			if now < last+int64(interval) {
				return nil
			}
		}
	}
	// If an ICQ client is available, register or refresh the queries and persist IDs
	if k.icqClient != nil && params.ConnectionId != "" {
		// Register TWAP (primary) and always register SPOT (GAMM) as fallback when UseTwap is true.
		if params.UseTwap {
			if store, key := osmo.BuildTwapKey(params.BaseDenom, params.QuoteDenom, params.PoolId, params.TwapWindowSeconds); len(key) > 0 {
				if qid, err := k.icqClient.RegisterKVQuery(ctx, params.ConnectionId, store, key); err == nil {
					_ = k.QueryIDs.Set(ctx, "twap", qid)
				}
			}
			if store, key := osmo.BuildGammPoolKey(params.PoolId); len(key) > 0 {
				if qid, err := k.icqClient.RegisterKVQuery(ctx, params.ConnectionId, store, key); err == nil {
					_ = k.QueryIDs.Set(ctx, "spot", qid)
				}
			}
		} else {
			if store, key := osmo.BuildGammPoolKey(params.PoolId); len(key) > 0 {
				if qid, err := k.icqClient.RegisterKVQuery(ctx, params.ConnectionId, store, key); err == nil {
					_ = k.QueryIDs.Set(ctx, "spot", qid)
				}
			}
		}
	}

	// Emit an intent event with params
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"osmosisicq_register",
			sdk.NewAttribute("connection_id", params.ConnectionId),
			sdk.NewAttribute("pool_id", fmt.Sprintf("%d", params.PoolId)),
			sdk.NewAttribute("base_denom", params.BaseDenom),
			sdk.NewAttribute("quote_denom", params.QuoteDenom),
			sdk.NewAttribute("use_twap", fmt.Sprintf("%t", params.UseTwap)),
			sdk.NewAttribute("twap_window_seconds", fmt.Sprintf("%d", params.TwapWindowSeconds)),
		),
	)

	// mark the time and schedule next
	if err := k.LastUpdate.Set(ctx, now); err != nil {
		return err
	}
	if err := k.NextUpdate.Set(ctx, now+int64(interval)); err != nil {
		return err
	}
	// emit an event for visibility
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"osmosisicq_schedule",
			sdk.NewAttribute("time", time.Unix(now, 0).UTC().Format(time.RFC3339)),
		),
	)
	return nil
}

// GetAuthority returns the module's authority.
func (k Keeper) GetAuthority() []byte {
	return k.authority
}

// ValidateAndPersistPrice applies guardrails (min liquidity and max deviation vs LGP)
// and persists the price via OracleKeeper if accepted. Emits events and updates LastResultTime on success.
func (k Keeper) ValidateAndPersistPrice(ctx sdk.Context, price sdkmath.LegacyDec, liquidity sdkmath.LegacyDec) error {
	params, err := k.Params.Get(ctx)
	if err != nil {
		return err
	}

	// Min liquidity check
	minLiq, err := sdkmath.LegacyNewDecFromStr(params.MinLiquidity)
	if err != nil {
		return err
	}
	if liquidity.IsNegative() {
		liquidity = sdkmath.LegacyZeroDec()
	}
	if liquidity.LT(minLiq) {
		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				"osmosisicq_price_dropped",
				sdk.NewAttribute("reason", "low_liquidity"),
				sdk.NewAttribute("price", price.String()),
				sdk.NewAttribute("liquidity", liquidity.String()),
			),
		)
		return nil
	}

	// Deviation vs LGP check
	maxDev, err := sdkmath.LegacyNewDecFromStr(params.MaxDeviation)
	if err != nil {
		return err
	}
	lgp := k.oracleKeeper.GetCSVPerUSD(ctx)
	if !lgp.IsZero() && maxDev.IsPositive() {
		// dev = |price - lgp| / lgp
		diff := price.Sub(lgp)
		if diff.IsNegative() {
			diff = diff.Neg()
		}
		dev := sdkmath.LegacyZeroDec()
		if !lgp.IsZero() {
			dev = diff.Quo(lgp)
		}
		if dev.GT(maxDev) {
			ctx.EventManager().EmitEvent(
				sdk.NewEvent(
					"osmosisicq_price_dropped",
					sdk.NewAttribute("reason", "deviation"),
					sdk.NewAttribute("price", price.String()),
					sdk.NewAttribute("lgp", lgp.String()),
					sdk.NewAttribute("deviation", dev.String()),
				),
			)
			return nil
		}
	}

	// Accept and persist
	if err := k.oracleKeeper.SetCSVPerUSD(ctx, price); err != nil {
		return err
	}
	now := ctx.BlockTime().Unix()
	_ = k.LastResultTime.Set(ctx, now)
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"osmosisicq_price_updated",
			sdk.NewAttribute("price", price.String()),
			sdk.NewAttribute("liquidity", liquidity.String()),
		),
	)
	return nil
}

// normalizeToCSVPerUSD converts a quote/base price into canonical CSV per USD using params.BaseDenom/QuoteDenom.
// If BaseDenom=="ucsv" and QuoteDenom=="uusdc", given price is USDC per CSV, so CSV per USD = 1/price.
// If BaseDenom=="uusdc" and QuoteDenom=="ucsv", given price is CSV per USD already.
// Otherwise, assume provided price is already CSV per USD (e.g., two-hop aggregated externally).
func (k Keeper) normalizeToCSVPerUSD(ctx sdk.Context, priceQuotePerBase sdkmath.LegacyDec) sdkmath.LegacyDec {
	params, err := k.Params.Get(ctx)
	if err != nil {
		return priceQuotePerBase
	}
	base := params.BaseDenom
	quote := params.QuoteDenom
	if base == "ucsv" && quote == "uusdc" {
		if priceQuotePerBase.IsZero() {
			return sdkmath.LegacyZeroDec()
		}
		return sdkmath.LegacyOneDec().Quo(priceQuotePerBase)
	}
	if base == "uusdc" && quote == "ucsv" {
		return priceQuotePerBase
	}
	// Unknown pair mapping; treat as already normalized
	return priceQuotePerBase
}

// HandleTwapResult consumes a TWAP price (quote/base) and a liquidity indicator, applies guardrails, and persists.
func (k Keeper) HandleTwapResult(ctx sdk.Context, priceQuotePerBase, liquidity sdkmath.LegacyDec) error {
	csvPerUSD := k.normalizeToCSVPerUSD(ctx, priceQuotePerBase)
	return k.ValidateAndPersistPrice(ctx, csvPerUSD, liquidity)
}

// HandleSpotResult consumes a spot price (quote/base) and a liquidity indicator, applies guardrails, and persists.
func (k Keeper) HandleSpotResult(ctx sdk.Context, priceQuotePerBase, liquidity sdkmath.LegacyDec) error {
	csvPerUSD := k.normalizeToCSVPerUSD(ctx, priceQuotePerBase)
	return k.ValidateAndPersistPrice(ctx, csvPerUSD, liquidity)
}
