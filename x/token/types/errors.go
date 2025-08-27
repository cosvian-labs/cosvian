package types

// DONTCOVER

import (
	"cosmossdk.io/errors"
)

// x/token module sentinel errors
var (
	ErrInvalidSigner = errors.Register(ModuleName, 1100, "expected gov account as only signer for proposal message")

	// Token creation errors
	ErrInvalidCreator              = errors.Register(ModuleName, 1101, "invalid creator address")
	ErrInvalidTokenName            = errors.Register(ModuleName, 1102, "invalid token name")
	ErrInvalidTokenSymbol          = errors.Register(ModuleName, 1103, "invalid token symbol")
	ErrInvalidDecimals             = errors.Register(ModuleName, 1104, "invalid decimals")
	ErrInvalidInitialSupply        = errors.Register(ModuleName, 1105, "invalid initial supply")
	ErrInvalidMaxSupply            = errors.Register(ModuleName, 1106, "invalid max supply")
	ErrInitialSupplyExceedsMax     = errors.Register(ModuleName, 1107, "initial supply exceeds max supply")
	ErrNonMintableSupplyMismatch   = errors.Register(ModuleName, 1108, "non-mintable tokens must have initial supply equal to max supply")
	ErrSymbolAlreadyExists         = errors.Register(ModuleName, 1109, "token symbol already exists")
	ErrReservedSymbol              = errors.Register(ModuleName, 1110, "symbol is reserved")
	ErrMaxTokensPerCreatorExceeded = errors.Register(ModuleName, 1111, "max tokens per creator exceeded")
	ErrTokenNotFound               = errors.Register(ModuleName, 1112, "token not found")
	ErrUnauthorizedMinter          = errors.Register(ModuleName, 1113, "unauthorized to mint tokens")
	ErrMaxSupplyExceeded           = errors.Register(ModuleName, 1114, "minting would exceed max supply")
)
