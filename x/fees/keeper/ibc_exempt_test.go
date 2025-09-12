package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"

	"bitora/x/fees/keeper"
	"bitora/x/fees/types"
)

// Test that IBC control-plane msgs are classified as system (exempt) while ICS20 transfers are not.
func TestIBCSystemExemptClassification(t *testing.T) {
	f := initFixture(t)

	params := types.DefaultParams()
	params.FeeMode = types.FeeModeHybrid
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	oracle := keeper.NewOracleAdapter(f.keeper, nil)
	calc := keeper.NewFeeCalculator(f.keeper, oracle)

	// 1) Core IBC client update should be CategorySystem
	signer, _ := f.addressCodec.BytesToString([]byte{1, 2, 3, 4})
	msgs := []sdk.Msg{&clienttypes.MsgUpdateClient{ClientId: "07-tendermint-0", ClientMessage: nil, Signer: signer}}
	est, err := calc.EstimateFee(sdk.UnwrapSDKContext(f.ctx), msgs, "", 100000)
	require.NoError(t, err)
	require.Equal(t, keeper.CategorySystem, est.Category)
	require.True(t, est.IsFree)
	require.True(t, est.USDAmount.IsZero())

	// 2) Core IBC acknowledgement should be CategorySystem
	ackMsg := &channeltypes.MsgAcknowledgement{Packet: channeltypes.Packet{}, Acknowledgement: []byte{0x01}, ProofAcked: nil, ProofHeight: clienttypes.Height{}, Signer: signer}
	est2, err := calc.EstimateFee(sdk.UnwrapSDKContext(f.ctx), []sdk.Msg{ackMsg}, "", 50000)
	require.NoError(t, err)
	require.Equal(t, keeper.CategorySystem, est2.Category)
	require.True(t, est2.IsFree)

	_ = transfertypes.ModuleName // keep import for future test extensions
}
