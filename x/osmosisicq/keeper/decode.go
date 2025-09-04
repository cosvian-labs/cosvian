package keeper

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// OnKVResult is a stub entrypoint for an ICQ controller to deliver raw KV results.
// It routes to TWAP or spot handlers after decoding a price and liquidity signal.
//
// This stub accepts value encodings in either JSON: {"price":"<dec>","liquidity":"<dec>"}
// or a simple text form: "<price>|<liquidity>". Real integration should decode
// Osmosis store values properly and compute price/liquidity from them.
func (k Keeper) OnKVResult(ctx sdk.Context, store string, key, value []byte) error {
    if len(value) == 0 {
        return errors.New("empty ICQ value")
    }

    var (
        price, liquidity sdkmath.LegacyDec
        err error
    )

    switch store {
    case "twap":
        p, errp := k.Params.Get(ctx)
        if errp != nil {
            return errp
        }
        price, liquidity, err = osmo.DecodeTwap(key, value, p.BaseDenom, p.QuoteDenom)
        if err != nil {
            return fmt.Errorf("decode twap: %w", err)
        }
        return k.HandleTwapResult(ctx, price, liquidity)
    case "gamm":
        p, errp := k.Params.Get(ctx)
        if errp != nil {
            return errp
        }
        price, liquidity, err = osmo.DecodeSpot(key, value, p.BaseDenom, p.QuoteDenom)
        if err != nil {
            return fmt.Errorf("decode gamm: %w", err)
        }
        return k.HandleSpotResult(ctx, price, liquidity)
    default:
        // Unknown store; ignore gracefully
        return nil
    }
}

type pl struct {
    Price     string `json:"price"`
    Liquidity string `json:"liquidity"`
}

// decodePriceLiquidity supports two stub formats:
// - JSON object: {"price":"1.23","liquidity":"1000"}
// - Delimited string: "1.23|1000"
func decodePriceLiquidity(b []byte) (sdkmath.LegacyDec, sdkmath.LegacyDec, error) {
    // Try JSON first
    var j pl
    if err := json.Unmarshal(b, &j); err == nil && (j.Price != "" || j.Liquidity != "") {
        p, err1 := sdkmath.LegacyNewDecFromStr(j.Price)
        if err1 != nil {
            return sdkmath.LegacyZeroDec(), sdkmath.LegacyZeroDec(), err1
        }
        l, err2 := sdkmath.LegacyNewDecFromStr(j.Liquidity)
        if err2 != nil {
            return sdkmath.LegacyZeroDec(), sdkmath.LegacyZeroDec(), err2
        }
        return p, l, nil
    }

    // Fallback: delimiter form
    s := string(b)
    if parts := strings.Split(s, "|"); len(parts) == 2 {
        p, err1 := sdkmath.LegacyNewDecFromStr(strings.TrimSpace(parts[0]))
        if err1 != nil {
            return sdkmath.LegacyZeroDec(), sdkmath.LegacyZeroDec(), err1
        }
        l, err2 := sdkmath.LegacyNewDecFromStr(strings.TrimSpace(parts[1]))
        if err2 != nil {
            return sdkmath.LegacyZeroDec(), sdkmath.LegacyZeroDec(), err2
        }
        return p, l, nil
    }
    return sdkmath.LegacyZeroDec(), sdkmath.LegacyZeroDec(), errors.New("unsupported stub encoding")
}
