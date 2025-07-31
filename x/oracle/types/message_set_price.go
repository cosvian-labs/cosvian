package types

func NewMsgSetPrice(creator string, denom string, price string) *MsgSetPrice {
	return &MsgSetPrice{
		Creator: creator,
		Denom:   denom,
		Price:   price,
	}
}
