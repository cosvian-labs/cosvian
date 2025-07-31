package types

func NewMsgFinalizeToken(creator string, denom string, makePublic bool) *MsgFinalizeToken {
	return &MsgFinalizeToken{
		Creator:    creator,
		Denom:      denom,
		MakePublic: makePublic,
	}
}
