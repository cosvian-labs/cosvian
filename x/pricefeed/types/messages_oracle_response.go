package types

func NewMsgSendOracleResponse(
	creator string,
	port string,
	channelID string,
	timeoutTimestamp uint64,
	requestId uint64,
	prices string,
	rates string,
	error string,
) *MsgSendOracleResponse {
	return &MsgSendOracleResponse{
		Creator:          creator,
		Port:             port,
		ChannelID:        channelID,
		TimeoutTimestamp: timeoutTimestamp,
		RequestId:        requestId,
		Prices:           prices,
		Rates:            rates,
		Error:            error,
	}
}
