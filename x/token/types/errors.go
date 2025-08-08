package types

// DONTCOVER

import (
	"cosmossdk.io/errors"
)

// x/token module sentinel errors
var (
	ErrInvalidSigner = errors.Register(ModuleName, 1100, "expected gov account as only signer for proposal message")
	
	// Token creation errors (starting from 1110 to avoid conflict with fees.go)
	ErrInvalidCreator               = errors.Register(ModuleName, 1110, "invalid creator address")
	ErrInvalidTokenName             = errors.Register(ModuleName, 1111, "invalid token name")
	ErrInvalidTokenSymbol           = errors.Register(ModuleName, 1112, "invalid token symbol")
	ErrInvalidDecimals              = errors.Register(ModuleName, 1113, "invalid decimals")
	ErrInvalidInitialSupply         = errors.Register(ModuleName, 1114, "invalid initial supply")
	ErrInvalidMaxSupply             = errors.Register(ModuleName, 1115, "invalid max supply")
	ErrInitialSupplyExceedsMax      = errors.Register(ModuleName, 1116, "initial supply exceeds max supply")
	ErrNonMintableSupplyMismatch    = errors.Register(ModuleName, 1117, "non-mintable tokens must have initial supply equal to max supply")
	ErrSymbolAlreadyExists          = errors.Register(ModuleName, 1118, "token symbol already exists")
	ErrReservedSymbol               = errors.Register(ModuleName, 1119, "symbol is reserved")
	ErrMaxTokensPerCreatorExceeded  = errors.Register(ModuleName, 1120, "max tokens per creator exceeded")
	ErrTokenNotFound                = errors.Register(ModuleName, 1121, "token not found")
	ErrUnauthorizedMinter           = errors.Register(ModuleName, 1122, "unauthorized to mint tokens")
	ErrMaxSupplyExceeded            = errors.Register(ModuleName, 1123, "minting would exceed max supply")
)
