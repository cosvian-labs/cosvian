package keeper

import (
	"context"
	"fmt"
	"strings"

	"cosmossdk.io/math"
	"bitora/x/token/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	errorsmod "cosmossdk.io/errors"
)


func (k msgServer) CreateToken(ctx context.Context,  msg *types.MsgCreateToken) (*types.MsgCreateTokenResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	
	// Validate message
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	
	// Parse creator address
	creator, err := k.addressCodec.StringToBytes(msg.Creator)
	if err != nil {
		return nil, errorsmod.Wrap(err, "invalid creator address")
	}
	
	// Get module parameters
	params, err := k.Params.Get(ctx)
	if err != nil {
		return nil, errorsmod.Wrap(err, "failed to get module params")
	}
	
	// 1. Validate symbol uniqueness (lowercase, global)
	lowerSymbol := strings.ToLower(msg.Symbol)
	if err := k.validateSymbolUniqueness(ctx, lowerSymbol, params.ReservedSymbols); err != nil {
		return nil, err
	}
	
	// 2. Check anti-spam limits (max tokens per creator)
	if err := k.validateCreatorLimits(ctx, creator, params.MaxTokensPerCreator); err != nil {
		return nil, err
	}
	
	// 3. Charge creation fee via existing x/token/fees.go logic
	if err := k.ChargeAndSplitFee(sdkCtx, creator, math.LegacyNewDecFromInt(params.CreationFeeAmount)); err != nil {
		return nil, errorsmod.Wrap(err, "failed to charge creation fee")
	}
	
	// 4. Generate tokenID using configurable prefix + sequence
	tokenID, err := k.generateTokenID(ctx, params.TokenPrefix)
	if err != nil {
		return nil, errorsmod.Wrap(err, "failed to generate token ID")
	}
	
	// 5. Store TokenMetadata in KVStore
	metadata := types.NewTokenMetadata(
		tokenID,
		msg.Name,
		msg.Symbol,
		msg.Decimals,
		math.NewIntFromUint64(msg.MaxSupply),
		math.NewIntFromUint64(msg.InitialSupply),
		msg.Mintable,
		creator,
		msg.PosCompatible,
		msg.IconUri,
		msg.Description,
		sdkCtx.BlockTime(),
	)
	
	if err := k.storeTokenMetadata(ctx, tokenID, lowerSymbol, creator, metadata); err != nil {
		return nil, errorsmod.Wrap(err, "failed to store token metadata")
	}
	
	// 6. Call k.MintToken(ctx, msg.Creator, tokenID, initialSupply)
	// TODO: Implement this based on existing MintToken logic
	
	// 7. If POSCompatible, register tokenID in x/tokenregistry
	if msg.PosCompatible {
		// TODO: Integrate with x/tokenregistry
	}
	
	// 8. Emit EventTokenCreated
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"token_created",
			sdk.NewAttribute("token_id", tokenID),
			sdk.NewAttribute("creator", msg.Creator),
			sdk.NewAttribute("symbol", msg.Symbol),
			sdk.NewAttribute("initial_supply", fmt.Sprintf("%d", msg.InitialSupply)),
		),
	)

	return &types.MsgCreateTokenResponse{}, nil
}

// validateSymbolUniqueness checks if symbol is unique and not reserved
func (k msgServer) validateSymbolUniqueness(ctx context.Context, lowerSymbol string, reservedSymbols []string) error {
	// Check if symbol is reserved
	for _, reserved := range reservedSymbols {
		if lowerSymbol == strings.ToLower(reserved) {
			return types.ErrReservedSymbol
		}
	}
	
	// Check if symbol already exists
	// TODO: Implement symbol index lookup
	
	return nil
}

// validateCreatorLimits checks anti-spam limits
func (k msgServer) validateCreatorLimits(ctx context.Context, creator sdk.AccAddress, maxTokens uint64) error {
	// TODO: Implement creator token count checking
	return nil
}

// generateTokenID creates a unique token ID with prefix + sequence
func (k msgServer) generateTokenID(ctx context.Context, prefix string) (string, error) {
	// TODO: Implement sequence-based token ID generation
	// For now, return a placeholder
	return prefix + "1", nil
}

// storeTokenMetadata stores metadata and creates indexes
func (k msgServer) storeTokenMetadata(ctx context.Context, tokenID, lowerSymbol string, creator sdk.AccAddress, metadata types.TokenMetadata) error {
	// TODO: Implement KV store operations for metadata and indexes
	return nil
}
