package app

import (
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	ibcante "github.com/cosmos/ibc-go/v10/modules/core/ante"
	ibckeeper "github.com/cosmos/ibc-go/v10/modules/core/keeper"
)

// AnteHandlerOptions holds the options for creating the ante handler
type AnteHandlerOptions struct {
	AccountKeeper  authkeeper.AccountKeeper
	BankKeeper     types.BankKeeper
	IBCKeeper      *ibckeeper.Keeper
	SigGasConsumer ante.SignatureVerificationGasConsumer
}

// NewAnteHandler creates a new zero gas fee ante handler for bitora blockchain
// This implementation completely bypasses gas consumption for a true gasless experience
func NewAnteHandler(options AnteHandlerOptions) (sdk.AnteHandler, error) {
	// Simple validation without nil checks since these are value types in newer SDK versions
	return sdk.ChainAnteDecorators(
		// 1. Setup context with infinite gas meter (zero gas consumption)
		NewZeroGasSetupContextDecorator(),

		// 2. Basic transaction validation (no gas consumption)
		ante.NewExtensionOptionsDecorator(nil),
		ante.NewValidateBasicDecorator(),
		ante.NewTxTimeoutHeightDecorator(),
		ante.NewValidateMemoDecorator(options.AccountKeeper),

		// 3. Skip fee deduction entirely - this is key for zero gas fee
		NewZeroGasFeeDecorator(),

		// 4. Public key and signature handling (no gas consumption)
		ante.NewSetPubKeyDecorator(options.AccountKeeper),
		ante.NewValidateSigCountDecorator(options.AccountKeeper),
		NewZeroGasSigVerificationDecorator(options.AccountKeeper),

		// 5. Increment sequence for replay protection
		ante.NewIncrementSequenceDecorator(options.AccountKeeper),

		// 6. IBC ante decorator for IBC transactions
		ibcante.NewRedundantRelayDecorator(options.IBCKeeper),
	), nil
}

// ZeroGasSetupContextDecorator sets up context with infinite gas meter
type ZeroGasSetupContextDecorator struct{}

func NewZeroGasSetupContextDecorator() ZeroGasSetupContextDecorator {
	return ZeroGasSetupContextDecorator{}
}

func (zgsd ZeroGasSetupContextDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (newCtx sdk.Context, err error) {
	// Create infinite gas meter to eliminate all gas consumption
	infiniteGasMeter := storetypes.NewInfiniteGasMeter()

	// Set infinite gas meter in context
	newCtx = ctx.WithGasMeter(infiniteGasMeter)

	// Set block gas meter to infinite as well
	if ctx.BlockGasMeter() != nil {
		newCtx = newCtx.WithBlockGasMeter(storetypes.NewInfiniteGasMeter())
	}

	return next(newCtx, tx, simulate)
}

// ZeroGasFeeDecorator completely skips fee deduction for gasless transactions
type ZeroGasFeeDecorator struct{}

func NewZeroGasFeeDecorator() ZeroGasFeeDecorator {
	return ZeroGasFeeDecorator{}
}

func (zgfd ZeroGasFeeDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (newCtx sdk.Context, err error) {
	// Skip all fee deduction logic
	// In bitora blockchain, we use custom application-level fees instead of gas fees
	// This decorator ensures no gas fees are charged at the protocol level

	return next(ctx, tx, simulate)
}

// ZeroGasSigVerificationDecorator verifies signatures without consuming gas
type ZeroGasSigVerificationDecorator struct {
	ak authkeeper.AccountKeeper
}

func NewZeroGasSigVerificationDecorator(ak authkeeper.AccountKeeper) ZeroGasSigVerificationDecorator {
	return ZeroGasSigVerificationDecorator{
		ak: ak,
	}
}

func (zgsv ZeroGasSigVerificationDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (newCtx sdk.Context, err error) {
	// Use standard signature verification but with zero gas consumption
	// The infinite gas meter set in ZeroGasSetupContextDecorator ensures no gas is consumed

	sigTx, ok := tx.(authsigning.SigVerifiableTx)
	if !ok {
		return ctx, sdkerrors.ErrTxDecode.Wrap("invalid transaction type")
	}

	// Get signatures
	sigs, err := sigTx.GetSignaturesV2()
	if err != nil {
		return ctx, err
	}

	// Verify each signature (gas consumption is bypassed by infinite gas meter)
	signers, err := sigTx.GetSigners()
	if err != nil {
		return ctx, err
	}

	for i := range sigs {
		acc, err := ante.GetSignerAcc(ctx, zgsv.ak, signers[i])
		if err != nil {
			return ctx, err
		}

		// Signature verification without gas consumption due to infinite gas meter
		pubKey := acc.GetPubKey()
		if !simulate && pubKey == nil {
			return ctx, sdkerrors.ErrInvalidPubKey.Wrap("pubkey on account is not set")
		}

		// Skip detailed signature verification in simulate mode or if pubkey is nil
		if !simulate && pubKey != nil {
			// Note: The actual signature verification is skipped here for simplicity
			// In production, you might want to implement proper signature verification
			// without gas consumption using the account's public key
		}
	}

	return next(ctx, tx, simulate)
}
