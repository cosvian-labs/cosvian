package keeper

import (
	"context"
	"time"

	"bitora/x/fees/types"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (q queryServer) OraclePrice(ctx context.Context, req *types.QueryOraclePriceRequest) (*types.QueryOraclePriceResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Get price from oracle keeper
	priceResult, err := q.k.oracleKeeper.GetLatestPrice(sdkCtx, "BTO/USD")
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get BTO price from oracle")
	}

	// Parse price from string
	price, err := math.LegacyNewDecFromStr(priceResult.Price)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to parse BTO price")
	}

	// Create mock price data for testing
	priceData := types.PriceData{
		Price:      price,
		TwapPrice:  price, // Use spot price as TWAP for now
		Timestamp:  priceResult.Timestamp,
		Window:     5 * time.Minute, // Mock window
		Freshness:  time.Since(priceResult.Timestamp),
		IsStale:    false,
		IsFallback: false,
	}

	return &types.QueryOraclePriceResponse{
		PriceData: priceData,
	}, nil
}
