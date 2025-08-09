package keeper

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"bitora/x/token/types"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
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
	
	// Check for reserved symbols
	lowerSymbol := strings.ToLower(msg.Symbol)
	for _, reserved := range params.ReservedSymbols {
		if lowerSymbol == strings.ToLower(reserved) {
			return nil, types.ErrReservedSymbol
		}
	}
	
	// Check symbol uniqueness
	if err := k.validateSymbolUniqueness(ctx, lowerSymbol); err != nil {
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
	
	// 4. Generate tokenID using fixed format "token_XXX"
	tokenID, err := k.generateTokenID(ctx)
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
	
	// 6. Mint initial supply to creator
	if msg.InitialSupply > 0 {
		if err := k.mintInitialSupply(ctx, creator, tokenID, msg.InitialSupply, msg.Symbol); err != nil {
			return nil, errorsmod.Wrap(err, "failed to mint initial supply")
		}
	}
	
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
func (k msgServer) validateSymbolUniqueness(ctx context.Context, symbol string) error {
	lowerSymbol := strings.ToLower(symbol)
	
	// Check if symbol already exists using store service
	store := k.storeService.OpenKVStore(ctx)
	
	// Create symbol index key: SymbolIndexPrefix + symbol
	prefixBytes := []byte("si_")  // from SymbolIndexPrefix
	symbolKey := append(prefixBytes, []byte(lowerSymbol)...)
	
	// Check if key exists
	has, err := store.Has(symbolKey)
	if err != nil {
		return err
	}
	if has {
		return types.ErrSymbolAlreadyExists
	}
	
	return nil
}// validateCreatorLimits checks anti-spam limits
func (k msgServer) validateCreatorLimits(ctx context.Context, creator sdk.AccAddress, maxTokens uint64) error {
	store := k.storeService.OpenKVStore(ctx)
	
	// Get current creator token count
	prefixBytes := []byte("cc_")  // from CreatorCountPrefix
	countKey := append(prefixBytes, creator...)
	countBytes, err := store.Get(countKey)
	
	var currentCount uint64 = 0
	if err == nil && countBytes != nil && len(countBytes) == 8 {
		currentCount = sdk.BigEndianToUint64(countBytes)
	}
	
	// Check if creator has reached the limit
	if currentCount >= maxTokens {
		return types.ErrMaxTokensPerCreatorExceeded
	}
	
	return nil
}

// generateTokenID creates a unique token ID using fixed format "token_XXX"
func (k msgServer) generateTokenID(ctx context.Context) (string, error) {
	// Get current sequence number
	sequence, err := k.getNextTokenSequence(ctx)
	if err != nil {
		return "", err
	}
	
	// Fixed format: "token_001", "token_002", etc. (not configurable)
	return fmt.Sprintf("token_%03d", sequence), nil
}

// getNextTokenSequence gets and increments the token sequence counter
func (k msgServer) getNextTokenSequence(ctx context.Context) (uint64, error) {
	store := k.storeService.OpenKVStore(ctx)
	
	// Get current sequence from KVStore
	sequenceKey := []byte("ts")  // from TokenSequenceKey
	sequenceBytes, err := store.Get(sequenceKey)
	
	var sequence uint64 = 1 // Start from 1 for first token
	
	if err == nil && sequenceBytes != nil && len(sequenceBytes) == 8 {
		// Parse existing sequence and increment
		sequence = sdk.BigEndianToUint64(sequenceBytes) + 1
	}
	
	// Store updated sequence
	newSequenceBytes := sdk.Uint64ToBigEndian(sequence)
	if err := store.Set(sequenceKey, newSequenceBytes); err != nil {
		return 0, err
	}
	
	return sequence, nil
}

// storeTokenMetadata stores metadata and creates indexes
func (k msgServer) storeTokenMetadata(ctx context.Context, tokenID, lowerSymbol string, creator sdk.AccAddress, metadata types.TokenMetadata) error {
	store := k.storeService.OpenKVStore(ctx)
	
	// 1. Store main metadata: TokenMetadataPrefix + tokenID -> TokenMetadata
	metadataPrefixBytes := []byte("tm_")  // from TokenMetadataPrefix
	metadataKey := append(metadataPrefixBytes, []byte(tokenID)...)
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	if err := store.Set(metadataKey, metadataBytes); err != nil {
		return err
	}
	
	// 2. Store symbol index: SymbolIndexPrefix + symbol -> tokenID
	symbolPrefixBytes := []byte("si_")  // from SymbolIndexPrefix
	symbolKey := append(symbolPrefixBytes, []byte(lowerSymbol)...)
	if err := store.Set(symbolKey, []byte(tokenID)); err != nil {
		return err
	}
	
	// 3. Store creator index: CreatorIndexPrefix + creator + tokenID -> tokenID  
	creatorPrefixBytes := []byte("ci_")  // from CreatorIndexPrefix
	creatorKey := append(creatorPrefixBytes, creator...)
	creatorKey = append(creatorKey, []byte(tokenID)...)
	if err := store.Set(creatorKey, []byte(tokenID)); err != nil {
		return err
	}
	
	// 4. Update creator token count: CreatorCountPrefix + creator -> count
	countPrefixBytes := []byte("cc_")  // from CreatorCountPrefix
	countKey := append(countPrefixBytes, creator...)
	countBytes, err := store.Get(countKey)
	
	var count uint64 = 1
	if err == nil && countBytes != nil {
		// Parse existing count and increment
		if len(countBytes) == 8 {
			count = sdk.BigEndianToUint64(countBytes) + 1
		}
	}
	
	countValue := sdk.Uint64ToBigEndian(count)
	if err := store.Set(countKey, countValue); err != nil {
		return err
	}
	
	return nil
}

// mintInitialSupply mints initial token supply to creator
func (k msgServer) mintInitialSupply(ctx context.Context, creator sdk.AccAddress, tokenID string, amount uint64, symbol string) error {
	// Create proper denom format: u + lowercase symbol
	denom := "u" + strings.ToLower(symbol)
	coin := sdk.NewCoin(denom, math.NewIntFromUint64(amount))
	
	// Mint coins to module account
	if err := k.bankKeeper.MintCoins(ctx, types.ModuleName, sdk.NewCoins(coin)); err != nil {
		return err
	}
	
	// Transfer coins from module account to creator
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, creator, sdk.NewCoins(coin)); err != nil {
		return err
	}
	
	return nil
}
