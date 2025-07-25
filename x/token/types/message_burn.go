package types

func NewMsgBurn(creator string, to string, amount uint64) *MsgBurn {
	return &MsgBurn{
		Creator: creator,
		To:      to,
		Amount:  amount,
	}
}
