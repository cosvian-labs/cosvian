package keeper_test

import (
	"testing"

	"cosvian/x/fees/types"

	"github.com/stretchr/testify/require"
)

func TestGenesis(t *testing.T) {
	genesisState := types.GenesisState{
		Params: types.DefaultParams(),
	}

	f := initFixture(t)
	err := f.keeper.InitGenesis(f.ctx, genesisState)
	require.NoError(t, err)
	got, err := f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)
	require.NotNil(t, got)

	// Normalize nil vs empty slices for equality
	if genesisState.Params.ExemptMsgTypeUrls == nil {
		genesisState.Params.ExemptMsgTypeUrls = []string{}
	}
	if got.Params.ExemptMsgTypeUrls == nil {
		got.Params.ExemptMsgTypeUrls = []string{}
	}
	if genesisState.Params.ExemptAddresses == nil {
		genesisState.Params.ExemptAddresses = []string{}
	}
	if got.Params.ExemptAddresses == nil {
		got.Params.ExemptAddresses = []string{}
	}
	require.EqualExportedValues(t, genesisState.Params, got.Params)
}
