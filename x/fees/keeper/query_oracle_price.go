package keeper

import (
    "context"
    "time"

    "bitora/x/fees/types"

    sdk "github.com/cosmos/cosmos-sdk/types"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

func (q queryServer) OraclePrice(ctx context.Context, req *types.QueryOraclePriceRequest) (*types.QueryOraclePriceResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

    // Get price from oracle keeper (BTO per USD)
    price, err := q.k.oracleKeeper.GetExchangeRate(sdkCtx, "BTO")
    if err != nil {
        return nil, status.Error(codes.Internal, "failed to get BTO price from oracle")
    }

	// Create mock price data for testing
    priceData := types.PriceData{
        Price:      price,
        TwapPrice:  price, // Use spot price as TWAP for now
        Timestamp:  sdkCtx.BlockTime(),
        Window:     5 * time.Minute, // Mock window
        Freshness:  0,
        IsStale:    false,
        IsFallback: false,
    }

	return &types.QueryOraclePriceResponse{
		PriceData: priceData,
	}, nil
}
