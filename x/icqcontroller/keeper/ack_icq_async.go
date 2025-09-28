//go:build icq_async

package keeper

import (
	"encoding/json"

	"cosvian/x/icqcontroller/icqwire"

	abcitypes "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
)

// extractValueFromAck (icq_async) decodes the ICS-04 JSON ack into async-icq CosmosResponse
// and returns the first response value bytes.
func (k Keeper) extractValueFromAck(ctx sdk.Context, acknowledgement []byte) ([]byte, error) {
	// Parse IBC ack wrapper
	var ack channeltypes.Acknowledgement
	if err := json.Unmarshal(acknowledgement, &ack); err != nil {
		return nil, err
	}
	if !ack.Success() {
		// error ack: nothing to deliver
		return nil, nil
	}
	// inner bytes are InterchainQueryPacketAck (JSON-encoded)
	var inner icqwire.InterchainQueryPacketAck
	if err := json.Unmarshal(ack.GetResult(), &inner); err != nil {
		return nil, err
	}
	// decode CosmosResponse
	resps, err := icqwire.DeserializeCosmosResponse(inner.Data)
	if err != nil {
		return nil, err
	}
	if len(resps) == 0 {
		return nil, nil
	}
	// Return raw Value from the first ResponseQuery
	// ResponseQuery has fields: Code, Log, Info, Index, Key, Value, ProofOps, Height, Codespace
	var v []byte
	for _, r := range resps {
		// pick first item that has a value
		if len(r.Value) > 0 {
			v = r.Value
			break
		}
	}
	return v, nil
}

// helper to make ResponseQuery zero value available if needed
var _ = abcitypes.ResponseQuery{}

// ExtractValueFromAckForTest exposes extractValueFromAck for tests under icq_async.
func (k Keeper) ExtractValueFromAckForTest(ctx sdk.Context, acknowledgement []byte) ([]byte, error) {
	return k.extractValueFromAck(ctx, acknowledgement)
}
