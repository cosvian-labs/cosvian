//go:build osmosis_removed

package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"cosvian/x/osmosisicq/types"
)

var _ types.QueryServer = queryServer{}

// NewQueryServerImpl returns an implementation of the QueryServer interface
// for the provided Keeper.
func NewQueryServerImpl(k Keeper) types.QueryServer {
	return queryServer{k}
}

type queryServer struct {
	k Keeper
}

func (qs queryServer) Status(ctx context.Context, _ *types.QueryStatusRequest) (*types.QueryStatusResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Gather scheduler times
	last, _ := qs.k.LastUpdate.Get(sdkCtx)
	next, _ := qs.k.NextUpdate.Get(sdkCtx)
	lrt, _ := qs.k.LastResultTime.Get(sdkCtx)

	// Collect query IDs
	queries := map[string]string{}
	// collections.Map doesn't have a direct helper here; do a simple scan via Walk when available.
	// Since the keyspace is tiny, attempt to get known keys.
	if qid, err := qs.k.QueryIDs.Get(sdkCtx, "twap"); err == nil {
		queries["twap"] = qid
	}
	if qid, err := qs.k.QueryIDs.Get(sdkCtx, "spot"); err == nil {
		queries["spot"] = qid
	}

	return &types.QueryStatusResponse{
		LastUpdate:     last,
		NextUpdate:     next,
		LastResultTime: lrt,
		Queries:        queries,
	}, nil
}
