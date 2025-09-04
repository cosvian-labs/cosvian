package types

// DONTCOVER

import (
	"cosmossdk.io/errors"
)

// x/oracle module sentinel errors
var (
	ErrInvalidSigner = errors.Register(ModuleName, 1100, "expected gov account as only signer for proposal message")
	ErrInvalidDenom  = errors.Register(ModuleName, 1101, "invalid or unsupported denom")
)
