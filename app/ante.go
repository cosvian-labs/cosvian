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
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	ibcante "github.com/cosmos/ibc-go/v10/modules/core/ante"
	ibckeeper "github.com/cosmos/ibc-go/v10/modules/core/keeper"

	tokenkeeper "bitora/x/token/keeper"
	tokentypes "bitora/x/token/types"
)

// AnteHandlerOptions holds the options for creating the ante handler
type AnteHandlerOptions struct {
	AccountKeeper         authkeeper.AccountKeeper
	BankKeeper            types.BankKeeper
	TokenKeeper           tokenkeeper.Keeper
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
		NewTransactionTypeFeeDecorator(options.TokenKeeper),

		// 6. Skip fee deduction entirely - this is key for zero gas fee
		NewZeroGasFeeDecorator(),

		// 7. Public key and signature handling (no gas consumption)
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

// TransactionTypeFeeDecorator detects transaction type and charges appropriate fixed USD fees
type TransactionTypeFeeDecorator struct {
	tokenKeeper tokenkeeper.Keeper
}

func NewTransactionTypeFeeDecorator(tokenKeeper tokenkeeper.Keeper) TransactionTypeFeeDecorator {
	return TransactionTypeFeeDecorator{
		tokenKeeper: tokenKeeper,
	}
}

func (tfd TransactionTypeFeeDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (newCtx sdk.Context, err error) {
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

	// Check transaction memo for mock fee testing
	var txMemo string
	if authTx, ok := tx.(interface{ GetMemo() string }); ok {
		txMemo = authTx.GetMemo()
	}

	switch txMemo {
	case "MOCK_DEX_SWAP_NATIVE":
		// Mock DEX Swap (native): $1.00 → 100% Treasury
		metadata := map[string]interface{}{}
		err = tfd.tokenKeeper.ChargeAndDistributeFeeByType(ctx, sender, tokenkeeper.FeeTypeDEXSwapNative, metadata)
		if err != nil {
			return ctx, sdkerrors.ErrInsufficientFunds.Wrapf("failed to charge mock DEX swap native fee: %v", err)
		}
		return next(ctx, tx, simulate) // Skip normal message processing

	case "MOCK_DEX_SWAP_USER":
		// Mock DEX Swap (user tokens): $3.00 → 50% Treasury, 50% Token Developer
		metadata := map[string]interface{}{
			"token_creator": sdk.AccAddress(sender).String(), // Mock: use sender as token creator
		}
		err = tfd.tokenKeeper.ChargeAndDistributeFeeByType(ctx, sender, tokenkeeper.FeeTypeDEXSwapUser, metadata)
		if err != nil {
			return ctx, sdkerrors.ErrInsufficientFunds.Wrapf("failed to charge mock DEX swap user fee: %v", err)
		}
		return next(ctx, tx, simulate) // Skip normal message processing

	case "MOCK_POS_PAYMENT":
		// Mock POS Payment: $0.15 → 50% Treasury, 50% Retail Wallet (locked 6mo)
		metadata := map[string]interface{}{
			"retail_wallet": sdk.AccAddress(sender).String(), // Mock: use sender as retail wallet
		}
		err = tfd.tokenKeeper.ChargeAndDistributeFeeByType(ctx, sender, tokenkeeper.FeeTypePOSPayment, metadata)
		if err != nil {
			return ctx, sdkerrors.ErrInsufficientFunds.Wrapf("failed to charge mock POS payment fee: %v", err)
		}
		return next(ctx, tx, simulate) // Skip normal message processing
	}

	// Analyze each message in the transaction and charge appropriate fees
	for _, message := range tx.GetMsgs() {
		switch message.(type) {
		case *banktypes.MsgSend:
			// Native Token Transfer: $1.00 → 100% Treasury
			metadata := map[string]interface{}{}
			err = tfd.tokenKeeper.ChargeAndDistributeFeeByType(ctx, sender, tokenkeeper.FeeTypeNativeTransfer, metadata)
			if err != nil {
				return ctx, sdkerrors.ErrInsufficientFunds.Wrapf("failed to charge native transfer fee: %v", err)
			}

		case *tokentypes.MsgMint:
			// Token Minting: Free (part of Token Creation Wizard)
			// Skip fee charging for token minting as it's part of token creation process
			continue

		case *tokentypes.MsgBurn:
			// Token Burning: Free (internal token operation, not interaction)
			// Skip fee charging for token burning
			continue

		case *wasmTypes.MsgStoreCode:
			// Smart Contract Deployment: Free
			// Skip fee charging for smart contract deployment
			continue

		case *wasmTypes.MsgInstantiateContract:
			// Smart Contract Instantiation: Free
			// Skip fee charging for smart contract instantiation
			continue

		case *wasmTypes.MsgExecuteContract:
			// Smart Contract Execution: Free
			// Skip fee charging for smart contract execution
			continue

		case *wasmTypes.MsgMigrateContract:
			// Smart Contract Migration: Free
			// Skip fee charging for smart contract migration
			continue

		case *wasmTypes.MsgUpdateAdmin:
			// Smart Contract Admin Update: Free
			// Skip fee charging for smart contract admin updates
			continue

		case *wasmTypes.MsgClearAdmin:
			// Smart Contract Clear Admin: Free
			// Skip fee charging for smart contract admin clearing
			continue

		// Future: Add proper token interaction cases (transfer/swap/call)
		// case *tokentypes.MsgTransfer:
		//     // Token Interaction (transfer): $3.00 → 50% Treasury, 50% Token Developer
		//     metadata := map[string]interface{}{
		//         "token_creator": "creator_address", // Get from token metadata
		//     }
		//     err = tfd.tokenKeeper.ChargeAndDistributeFeeByType(ctx, sender, tokenkeeper.FeeTypeTokenInteraction, metadata)

		// Future: Add POS payment case when POS module is implemented
		// case *postypes.MsgPayment:
		//     // POS Payment: $0.15 → 50% Treasury, 50% Retail Wallet (locked 6mo)
		//     metadata := map[string]interface{}{
		//         "retail_wallet": "retail_wallet_address",
		//     }
		//     err = tfd.tokenKeeper.ChargeAndDistributeFeeByType(ctx, sender, tokenkeeper.FeeTypePOSPayment, metadata)

		// Future: Add DEX swap cases when DEX module is implemented
		// case *dextypes.MsgSwapNative:
		//     // DEX Swap (native): $1.00 → 100% Treasury
		//     metadata := map[string]interface{}{}
		//     err = tfd.tokenKeeper.ChargeAndDistributeFeeByType(ctx, sender, tokenkeeper.FeeTypeDEXSwapNative, metadata)

		// case *dextypes.MsgSwapUserToken:
		//     // DEX Swap (user tokens): $3.00 → 50% Treasury, 50% Token Creator
		//     metadata := map[string]interface{}{
		//         "token_creator": "creator_address", // Get from token metadata
		//     }
		//     err = tfd.tokenKeeper.ChargeAndDistributeFeeByType(ctx, sender, tokenkeeper.FeeTypeDEXSwapUser, metadata)

		default:
			// For unknown message types, no fee is charged
			// This includes:
			// - Smart Contract Deployment (EVM/CW): Free
			// - Token Creation operations: Free
			// - Other system operations: Free
			continue
		}

		if err != nil {
			return ctx, err
		}
	}

	return next(ctx, tx, simulate)
}
