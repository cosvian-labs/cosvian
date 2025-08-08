package types

import (
    sdk "github.com/cosmos/cosmos-sdk/types"
)

func NewMsgCreateToken(creator string, name string, symbol string, decimals uint32, initialSupply uint64, maxSupply uint64, mintable bool, posCompatible bool, iconUri string, description string) *MsgCreateToken {
  return &MsgCreateToken{
		Creator: creator,
    Name: name,
    Symbol: symbol,
    Decimals: decimals,
    InitialSupply: initialSupply,
    MaxSupply: maxSupply,
    Mintable: mintable,
    PosCompatible: posCompatible,
    IconUri: iconUri,
    Description: description,
	}
}

func (msg *MsgCreateToken) ValidateBasic() error {
    _, err := sdk.AccAddressFromBech32(msg.Creator)
    if err != nil {
        return ErrInvalidCreator
    }
    
    // Validate required fields
    if msg.Name == "" {
        return ErrInvalidTokenName
    }
    if msg.Symbol == "" {
        return ErrInvalidTokenSymbol
    }
    if msg.Decimals > 18 {
        return ErrInvalidDecimals
    }
    if msg.InitialSupply == 0 {
        return ErrInvalidInitialSupply
    }
    if msg.MaxSupply == 0 {
        return ErrInvalidMaxSupply
    }
    if msg.InitialSupply > msg.MaxSupply {
        return ErrInitialSupplyExceedsMax
    }
    // If not mintable, initial supply must equal max supply
    if !msg.Mintable && msg.InitialSupply != msg.MaxSupply {
        return ErrNonMintableSupplyMismatch
    }
    
    return nil
}