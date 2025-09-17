package icqwire

import (
	"encoding/json"

	abcitypes "github.com/cometbft/cometbft/abci/types"
)

// InterchainQueryPacketData mirrors the async-icq packet data schema.
// It is JSON-encoded with a single base64-encoded field `data`.
type InterchainQueryPacketData struct {
    Data []byte `json:"data"`
}

// GetBytes marshals the packet data to JSON bytes.
func (p InterchainQueryPacketData) GetBytes() []byte {
    bz, _ := json.Marshal(p)
    return bz
}

// InterchainQueryPacketAck mirrors the async-icq acknowledgement schema.
// It is JSON-encoded with a single base64-encoded field `data`.
type InterchainQueryPacketAck struct {
    Data []byte `json:"data"`
}

// SerializeCosmosQuery encodes a set of ABCI RequestQuery items as JSON.
func SerializeCosmosQuery(reqs []abcitypes.RequestQuery) ([]byte, error) {
    return json.Marshal(reqs)
}

// DeserializeCosmosResponse decodes JSON into a slice of ABCI ResponseQuery.
func DeserializeCosmosResponse(bz []byte) ([]abcitypes.ResponseQuery, error) {
    var resps []abcitypes.ResponseQuery
    if len(bz) == 0 {
        return resps, nil
    }
    if err := json.Unmarshal(bz, &resps); err != nil {
        return nil, err
    }
    return resps, nil
}
