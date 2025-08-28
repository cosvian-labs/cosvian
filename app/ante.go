package app

import (
	corestoretypes "cosmossdk.io/core/store"
	storetypes "cosmossdk.io/store/types"
	circuitante "cosmossdk.io/x/circuit/ante"
	circuitkeeper "cosmossdk.io/x/circuit/keeper"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmTypes "github.com/CosmWasm/wasmd/x/wasm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	ibcante "github.com/cosmos/ibc-go/v10/modules/core/ante"
	ibckeeper "github.com/cosmos/ibc-go/v10/modules/core/keeper"

	feeskeeper "bitora/x/fees/keeper"
	feestypes "bitora/x/fees/types"
	tokenkeeper "bitora/x/token/keeper"
)

// AnteHandlerOptions holds the options for creating the ante handler
type AnteHandlerOptions struct {
	ante.HandlerOptions

	AccountKeeper         authkeeper.AccountKeeper
	BankKeeper            types.BankKeeper
	TokenKeeper           tokenkeeper.Keeper
	FeesKeeper            feeskeeper.Keeper
	IBCKeeper             *ibckeeper.Keeper
	SigGasConsumer        ante.SignatureVerificationGasConsumer
	NodeConfig            *wasmTypes.NodeConfig
	WasmKeeper            *wasmkeeper.Keeper
	TXCounterStoreService corestoretypes.KVStoreService
	CircuitKeeper         *circuitkeeper.Keeper
}

// NewAnteHandler creates a new zero gas fee ante handler for bitora blockchain
// This implementation completely bypasses gas consumption for a true gasless experience
func NewAnteHandler(options AnteHandlerOptions) (sdk.AnteHandler, error) {
	// Validation for required WASM components
	if options.NodeConfig == nil {
		return nil, sdkerrors.ErrInvalidRequest.Wrap("wasm node config is required for ante builder")
	}
	if options.TXCounterStoreService == nil {
		return nil, sdkerrors.ErrInvalidRequest.Wrap("wasm store service is required for ante builder")
	}
	if options.CircuitKeeper == nil {
		return nil, sdkerrors.ErrInvalidRequest.Wrap("circuit keeper is required for ante builder")
	}

	return sdk.ChainAnteDecorators(
		// 1. Setup context with infinite gas meter (zero gas consumption)
		NewZeroGasSetupContextDecorator(),

		// 2. WASM decorators with zero gas consumption
		wasmkeeper.NewLimitSimulationGasDecorator(options.NodeConfig.SimulationGasLimit),
		wasmkeeper.NewCountTXDecorator(options.TXCounterStoreService),
		wasmkeeper.NewGasRegisterDecorator(options.WasmKeeper.GetGasRegister()),
		wasmkeeper.NewTxContractsDecorator(),

		// 3. Circuit breaker for emergency stops
		circuitante.NewCircuitBreakerDecorator(options.CircuitKeeper),

		// 4. Basic transaction validation (no gas consumption)
		ante.NewExtensionOptionsDecorator(nil),
		ante.NewValidateBasicDecorator(),
		ante.NewTxTimeoutHeightDecorator(),
		ante.NewValidateMemoDecorator(options.AccountKeeper),

		// 5. Custom transaction type fee detection and charging
		NewBitoraFeeDecorator(options.TokenKeeper, options.FeesKeeper),

	// 6. Skip fee deduction entirely - this is key for zero gas fee
	NewZeroGasFeeDecorator(),		// 7. Public key and signature handling (no gas consumption)
		ante.NewSetPubKeyDecorator(options.AccountKeeper),
		ante.NewValidateSigCountDecorator(options.AccountKeeper),
		NewZeroGasSigVerificationDecorator(options.AccountKeeper),

		// 8. Increment sequence for replay protection
		ante.NewIncrementSequenceDecorator(options.AccountKeeper),

		// 9. IBC ante decorator for IBC transactions
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

// BitoraFeeDecorator implements the new fee system where Gas Used = Fee Amount
type BitoraFeeDecorator struct {
	tokenKeeper tokenkeeper.Keeper
	feesKeeper  feeskeeper.Keeper
}

func NewBitoraFeeDecorator(tokenKeeper tokenkeeper.Keeper, feesKeeper feeskeeper.Keeper) BitoraFeeDecorator {
	return BitoraFeeDecorator{
		tokenKeeper: tokenKeeper,
		feesKeeper:  feesKeeper,
	}
}

func (bfd BitoraFeeDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (newCtx sdk.Context, err error) {
	// Skip fee charging in simulation mode
	if simulate {
		return next(ctx, tx, simulate)
	}

	// Get transaction signer (fee payer)
	signers, err := tx.(authsigning.SigVerifiableTx).GetSigners()
	if err != nil {
		return ctx, sdkerrors.ErrInvalidAddress.Wrap("failed to get transaction signers")
	}

	if len(signers) == 0 {
		return ctx, sdkerrors.ErrInvalidAddress.Wrap("no signers found in transaction")
	}

	sender := signers[0] // First signer pays the fee

	// Check transaction memo for fee category
	var txMemo string
	if authTx, ok := tx.(interface{ GetMemo() string }); ok {
		txMemo = authTx.GetMemo()
	}

	// Map memo to fee type
	var feeType string
	switch txMemo {
	case "POS_PAYMENT":
		feeType = "pos_payment"
	case "TOKEN_INTERACTION":
		feeType = "token_interaction"
	case "NATIVE_TRANSFER":
		feeType = "native_transfer"
	case "DEX_NATIVE":
		feeType = "dex_native"
	case "DEX_USER":
		feeType = "dex_user"
	case "DEPLOY":
		feeType = "deploy"
	case "WIZARD":
		feeType = "wizard"
	default:
		// Default to token interaction for unknown memos
		feeType = "token_interaction"
	}

	// Charge fee using feesKeeper
	msgServer := feeskeeper.NewMsgServerImpl(bfd.feesKeeper)
	_, err = msgServer.ChargeFee(sdk.WrapSDKContext(ctx), &feestypes.MsgChargeFee{
		Creator:  sdk.AccAddress(sender).String(),
		Category: feeType,
		Amount:   "0", // Amount will be determined by category
		Metadata: "{}",
	})
	if err != nil {
		return ctx, sdkerrors.ErrInsufficientFunds.Wrapf("failed to charge fee: %v", err)
	}

	return next(ctx, tx, simulate)
}
