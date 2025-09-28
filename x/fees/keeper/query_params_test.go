package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"cosvian/x/fees/keeper"
	"cosvian/x/fees/types"
)

func TestParamsQuery(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	response, err := qs.Params(f.ctx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	// normalize nil vs empty slices for comparison
	if params.ExemptMsgTypeUrls == nil {
		params.ExemptMsgTypeUrls = []string{}
	}
	if params.ExemptAddresses == nil {
		params.ExemptAddresses = []string{}
	}
	if response.Params.ExemptMsgTypeUrls == nil {
		response.Params.ExemptMsgTypeUrls = []string{}
	}
	if response.Params.ExemptAddresses == nil {
		response.Params.ExemptAddresses = []string{}
	}
	require.Equal(t, &types.QueryParamsResponse{Params: params}, response)
}
