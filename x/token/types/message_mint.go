package types

func NewMsgMint(creator string, to string, amount uint64) *MsgMint {
	return &MsgMint{
		Creator: creator,
		To:      to,
		Amount:  amount,
	}
}
