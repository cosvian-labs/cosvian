package types

// DONTCOVER

import (
	"cosmossdk.io/errors"
)

// x/osmosisicq module sentinel errors
var (
	ErrInvalidSigner = errors.Register(ModuleName, 1100, "expected gov account as only signer for proposal message")
	ErrNoActiveChannel = errors.Register(ModuleName, 1101, "no active icq channel for connection")
)
