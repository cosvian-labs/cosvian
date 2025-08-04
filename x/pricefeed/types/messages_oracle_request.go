package types

func NewMsgSendOracleRequest(
	creator string,
	port string,
	channelID string,
	timeoutTimestamp uint64,
	oracleScriptId uint64,
	calldata string,
	symbols string,
	askCount uint64,
	minCount uint64,
	feeLimit string,
	prepareGas uint64,
	executeGas uint64,
	clientId string,
) *MsgSendOracleRequest {
	return &MsgSendOracleRequest{
		Creator:          creator,
		Port:             port,
		ChannelID:        channelID,
		TimeoutTimestamp: timeoutTimestamp,
		OracleScriptId:   oracleScriptId,
		Calldata:         calldata,
		Symbols:          symbols,
		AskCount:         askCount,
		MinCount:         minCount,
		FeeLimit:         feeLimit,
		PrepareGas:       prepareGas,
		ExecuteGas:       executeGas,
		ClientId:         clientId,
	}
}
