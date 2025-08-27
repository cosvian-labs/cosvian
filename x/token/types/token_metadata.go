package types

import (
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TokenMetadata defines the metadata for a created token
type TokenMetadata struct {
	TokenID       string         `json:"token_id"`
	Name          string         `json:"name"`
	Symbol        string         `json:"symbol"`
	Decimals      uint32         `json:"decimals"`
	MaxSupply     math.Int       `json:"max_supply"`
	CurrentSupply math.Int       `json:"current_supply"`
	Mintable      bool           `json:"mintable"`
	Creator       sdk.AccAddress `json:"creator"`
	POSCompatible bool           `json:"pos_compatible"`
	IconURI       string         `json:"icon_uri"`
	Description   string         `json:"description"`
	CreatedAt     time.Time      `json:"created_at"`
}

// NewTokenMetadata creates a new TokenMetadata instance
func NewTokenMetadata(
	tokenID string,
	name string,
	symbol string,
	decimals uint32,
	maxSupply math.Int,
	currentSupply math.Int,
	mintable bool,
	creator sdk.AccAddress,
	posCompatible bool,
	iconURI string,
	description string,
	createdAt time.Time,
) TokenMetadata {
	return TokenMetadata{
		TokenID:       tokenID,
		Name:          name,
		Symbol:        symbol,
		Decimals:      decimals,
		MaxSupply:     maxSupply,
		CurrentSupply: currentSupply,
		Mintable:      mintable,
		Creator:       creator,
		POSCompatible: posCompatible,
		IconURI:       iconURI,
		Description:   description,
		CreatedAt:     createdAt,
	}
}

// Validate performs basic validation of TokenMetadata
func (tm TokenMetadata) Validate() error {
	if tm.TokenID == "" {
		return ErrInvalidTokenName
	}
	if tm.Name == "" {
		return ErrInvalidTokenName
	}
	if tm.Symbol == "" {
		return ErrInvalidTokenSymbol
	}
	if tm.Decimals > 18 {
		return ErrInvalidDecimals
	}
	if tm.MaxSupply.IsZero() {
		return ErrInvalidMaxSupply
	}
	if tm.CurrentSupply.GT(tm.MaxSupply) {
		return ErrInitialSupplyExceedsMax
	}
	if tm.Creator.Empty() {
		return ErrInvalidCreator
	}

	return nil
}
