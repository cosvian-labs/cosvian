package types

import (
	"context"

	"cosmossdk.io/core/address"
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// AuthKeeper defines the expected interface for the Auth module.
type AuthKeeper interface {
	AddressCodec() address.Codec
	GetAccount(context.Context, sdk.AccAddress) sdk.AccountI // only used for simulation
	// Methods imported from account should be defined here
}

// BankKeeper defines the expected interface for the Bank module.
type BankKeeper interface {
	SpendableCoins(context.Context, sdk.AccAddress) sdk.Coins
	// Methods imported from bank should be defined here
}

// ParamSubspace defines the expected Subspace interface for parameters.
type ParamSubspace interface {
	Get(context.Context, []byte, interface{})
	Set(context.Context, []byte, interface{})
}

// OracleKeeper defines the expected interface for the Oracle module used by osmosisicq.
// We only need to set the canonical CSV per USD price.
type OracleKeeper interface {
	SetBTOPerUSD(ctx sdk.Context, price sdkmath.LegacyDec) error
	GetBTOPerUSD(ctx sdk.Context) sdkmath.LegacyDec
}
