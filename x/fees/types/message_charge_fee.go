package types

func NewMsgChargeFee(creator string, category string, amount string, metadata string) *MsgChargeFee {
	return &MsgChargeFee{
		Creator:  creator,
		Category: category,
		Amount:   amount,
		Metadata: metadata,
	}
}
