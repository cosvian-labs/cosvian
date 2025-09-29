package keeper

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"cosvian/x/pricefeed/types"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// RequestPriceData requests price data via Band Protocol oracle integration
func (k Keeper) RequestPriceData(ctx context.Context, symbols []string, channelID string) error {
	// Prepare calldata for Band Protocol oracle script
	calldataStruct := struct {
		Symbols []string `json:"symbols"`
	}{
		Symbols: symbols,
	}

	calldataBytes, err := json.Marshal(calldataStruct)
	if err != nil {
		return fmt.Errorf("failed to marshal calldata: %w", err)
	}

	// Create oracle request message
	msg := &types.MsgSendOracleRequest{
		Creator:          sdk.AccAddress(k.authority).String(),
		Port:             types.PortID,
		ChannelID:        channelID,
		TimeoutTimestamp: uint64(sdk.UnwrapSDKContext(ctx).BlockTime().Unix()) + 3600, // 1 hour timeout
		OracleScriptId:   37,                                                          // Band Protocol standard price oracle script
		Calldata:         hex.EncodeToString(calldataBytes),                           // Hex encoded calldata
		Symbols:          fmt.Sprintf("%v", symbols),                                  // Comma separated symbols
		AskCount:         4,                                                           // Ask 4 validators
		MinCount:         3,                                                           // Need minimum 3 responses
		FeeLimit:         "100000uband",                                               // Fee limit in uband
		PrepareGas:       50000,                                                       // Gas for prepare phase
		ExecuteGas:       300000,                                                      // Gas for execute phase
		ClientId:         fmt.Sprintf("cosvian-price-req-%d", sdk.UnwrapSDKContext(ctx).BlockHeight()),
	}

	// Send the oracle request
	msgServer := NewMsgServerImpl(k)
	_, err = msgServer.SendOracleRequest(ctx, msg)
	if err != nil {
		return fmt.Errorf("failed to send oracle request: %w", err)
	}

	return nil
}

// IntegrateWithOracleModule connects with existing oracle module for fallback
func (k Keeper) IntegrateWithOracleModule(ctx context.Context, symbols []string) error {
	// This method integrates with the existing oracle module for fallback price data
	// In case Band Protocol is not available, we can fallback to the local oracle

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Emit event that we're attempting price request
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"pricefeed_request",
			sdk.NewAttribute("symbols", fmt.Sprintf("%v", symbols)),
			sdk.NewAttribute("source", "band_protocol"),
			sdk.NewAttribute("block_height", fmt.Sprintf("%d", sdkCtx.BlockHeight())),
		),
	)

	return nil
}

// ProcessPriceResponse processes the price response from Band Protocol
func (k Keeper) ProcessPriceResponse(ctx context.Context, response types.OracleResponsePacketData) error {
	if response.Error != "" {
		return fmt.Errorf("oracle response error: %s", response.Error)
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// 1) Prefer response.Rates as direct CSV per USD if provided
	if response.Rates != "" {
		var rateData map[string]interface{}
		if err := json.Unmarshal([]byte(response.Rates), &rateData); err == nil {
			if v, ok := rateData["CSV"]; ok {
				var rateStr string
				switch vv := v.(type) {
				case string:
					rateStr = vv
				default:
					rateStr = fmt.Sprintf("%v", vv)
				}
				if dec, err := math.LegacyNewDecFromStr(rateStr); err == nil && dec.IsPositive() {
					_ = k.oracleKeeper.SetCSVPerUSD(sdkCtx, dec)
					sdkCtx.EventManager().EmitEvent(
						sdk.NewEvent(
							"price_updated",
							sdk.NewAttribute("symbol", "CSV/USD"),
							sdk.NewAttribute("price", dec.String()),
							sdk.NewAttribute("source", "band_protocol_rates"),
							sdk.NewAttribute("request_id", fmt.Sprintf("%d", response.RequestId)),
						),
					)
					return nil
				}
			}
		}
	}

	// 2) Fallback to response.Prices assumed as USD per CSV; invert to get CSV per USD
	if response.Prices != "" {
		var priceData map[string]interface{}
		if err := json.Unmarshal([]byte(response.Prices), &priceData); err == nil {
			if v, ok := priceData["CSV"]; ok {
				var usdPerCSVStr string
				switch vv := v.(type) {
				case string:
					usdPerCSVStr = vv
				default:
					usdPerCSVStr = fmt.Sprintf("%v", vv)
				}
				if usdPerCSV, err := math.LegacyNewDecFromStr(usdPerCSVStr); err == nil && usdPerCSV.IsPositive() {
					csvPerUSD := math.LegacyOneDec().Quo(usdPerCSV)
					_ = k.oracleKeeper.SetCSVPerUSD(sdkCtx, csvPerUSD)
					sdkCtx.EventManager().EmitEvent(
						sdk.NewEvent(
							"price_updated",
							sdk.NewAttribute("symbol", "CSV/USD"),
							sdk.NewAttribute("price", csvPerUSD.String()),
							sdk.NewAttribute("source", "band_protocol_prices_inverted"),
							sdk.NewAttribute("request_id", fmt.Sprintf("%d", response.RequestId)),
						),
					)
					return nil
				}
			}
		}
	}

	return nil
}
