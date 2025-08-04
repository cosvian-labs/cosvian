package types

// IBC events
const (
	EventTypeTimeout              = "timeout"
	EventTypeOracleRequestPacket  = "oracleRequest_packet"
	EventTypeOracleResponsePacket = "oracleResponse_packet"
	// this line is used by starport scaffolding # ibc/packet/event

	AttributeKeyAckSuccess = "success"
	AttributeKeyAck        = "acknowledgement"
	AttributeKeyAckError   = "error"
)
