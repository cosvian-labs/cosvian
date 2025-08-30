package keeper

import (
    "fmt"
    "time"

    "cosmossdk.io/math"
    sdk "github.com/cosmos/cosmos-sdk/types"

    "bitora/x/fees/types"
)

// OracleAdapter handles Band Protocol integration for BTO/USD price feeds
type OracleAdapter struct {
	keeper       Keeper
	oracleKeeper types.OracleKeeper
}

// NewOracleAdapter creates a new oracle adapter
func NewOracleAdapter(keeper Keeper, oracleKeeper types.OracleKeeper) *OracleAdapter {
	return &OracleAdapter{
		keeper:       keeper,
		oracleKeeper: oracleKeeper,
	}
}

// GetBTOUSDPrice retrieves BTO/USD price with fallback logic
func (oa *OracleAdapter) GetBTOUSDPrice(ctx sdk.Context) (*types.PriceData, error) {
    params, err := oa.keeper.Params.Get(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to get params: %w", err)
    }
    oracleParams := params.OracleParams

    // Query oracle module for BTO exchange rate (BTO per USD)
    price, err := oa.oracleKeeper.GetExchangeRate(ctx, "BTO")
    if err == nil && !price.IsZero() {
        priceData := &types.PriceData{
            Price:      price,
            TwapPrice:  price,
            Timestamp:  ctx.BlockTime(),
            Window:     oracleParams.TwapWindow,
            Freshness:  0,
            IsStale:    false,
            IsFallback: false,
        }
        oa.emitOracleEvent(ctx, priceData)
        return priceData, nil
    }

    // Fallback
    return oa.getFallbackPrice(ctx, oracleParams)
}

// getBandPrice retrieves price from Band Protocol oracle
// getBandPrice not used; oracle module provides GetExchangeRate directly

// OraclePrice represents a price data point from Band Protocol
type OraclePrice struct {
	Price     math.LegacyDec `json:"price"`
	Timestamp time.Time      `json:"timestamp"`
	RequestId uint64         `json:"request_id"`
}

// isPriceValid checks if the price is valid and within deviation limits
func (oa *OracleAdapter) isPriceValid(price *OraclePrice, params types.OracleParams) bool {
	// Check if price is too old
	age := time.Since(price.Timestamp)
	if age > params.MaxPriceAge {
		return false
	}

	// Check if price is reasonable (not zero or negative)
	if price.Price.IsZero() || price.Price.IsNegative() {
		return false
	}

	return true
}

// getFallbackPrice returns a default fallback price when oracle is unavailable
func (oa *OracleAdapter) getFallbackPrice(ctx sdk.Context, params types.OracleParams) (*types.PriceData, error) {
	// Use a hardcoded fallback price for now (e.g., $1.00 for BTO)
	fallbackPrice := math.LegacyOneDec()

	priceData := &types.PriceData{
		Price:      fallbackPrice,
		TwapPrice:  fallbackPrice,
		Timestamp:  ctx.BlockTime(),
		Window:     0,
		Freshness:  0,
		IsStale:    true,
		IsFallback: true,
	}

	// Emit oracle usage event
	oa.emitOracleEvent(ctx, priceData)

	return priceData, nil
}

// emitOracleEvent emits an event for oracle price usage
func (oa *OracleAdapter) emitOracleEvent(ctx sdk.Context, priceData *types.PriceData) {
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeOracleUsed,
			sdk.NewAttribute(types.AttributeKeyPrice, priceData.Price.String()),
			sdk.NewAttribute(types.AttributeKeyTwapPrice, priceData.TwapPrice.String()),
			sdk.NewAttribute(types.AttributeKeyTimestamp, priceData.Timestamp.Format(time.RFC3339)),
			sdk.NewAttribute(types.AttributeKeyWindow, priceData.Window.String()),
			sdk.NewAttribute(types.AttributeKeyFreshness, priceData.Freshness.String()),
			sdk.NewAttribute(types.AttributeKeyIsStale, fmt.Sprintf("%t", priceData.IsStale)),
			sdk.NewAttribute(types.AttributeKeyIsFallback, fmt.Sprintf("%t", priceData.IsFallback)),
		),
	)
}

// ConvertUSDToBTO converts USD amount to BTO using current oracle price
func (oa *OracleAdapter) ConvertUSDToBTO(ctx sdk.Context, usdAmount math.LegacyDec) (math.LegacyDec, *types.PriceData, error) {
	priceData, err := oa.GetBTOUSDPrice(ctx)
	if err != nil {
		return math.LegacyZeroDec(), nil, err
	}

    // priceData.TwapPrice represents BTO per USD. For a USD amount, multiply to get BTO.
    btoAmount := usdAmount.Mul(priceData.TwapPrice)

    return btoAmount, priceData, nil
}
