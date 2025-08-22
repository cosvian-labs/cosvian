package keeper

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"bitora/x/token/types"

	errorsmod "cosmossdk.io/errors"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/types/query"
)

var _ types.QueryServer = queryServer{}

// NewQueryServerImpl returns an implementation of the QueryServer interface
// for the provided Keeper.
func NewQueryServerImpl(k Keeper) types.QueryServer {
	return queryServer{k}
}

type queryServer struct {
	k Keeper
}

// Params already implemented elsewhere if needed (left untouched)

// TokenById returns metadata by token id
func (q queryServer) TokenById(ctx context.Context, req *types.QueryTokenByIdRequest) (*types.QueryTokenByIdResponse, error) {
	if req == nil || req.TokenId == "" {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "token_id required")
	}
	store := q.k.storeService.OpenKVStore(ctx)
	key := append([]byte("tm_"), []byte(req.TokenId)...)
	bz, err := store.Get(key)
	if err != nil {
		return nil, errorsmod.Wrap(err, "store get failed")
	}
	if bz == nil {
		return nil, errorsmod.Wrap(types.ErrTokenNotFound, "not found")
	}
	var meta types.TokenMetadata
	if err := json.Unmarshal(bz, &meta); err != nil {
		return nil, errorsmod.Wrap(err, "unmarshal")
	}
	creatorStr, _ := q.k.addressCodec.BytesToString(meta.Creator)
	return &types.QueryTokenByIdResponse{
		TokenId:       meta.TokenID,
		Name:          meta.Name,
		Symbol:        meta.Symbol,
		Decimals:      meta.Decimals,
		MaxSupply:     meta.MaxSupply.String(),
		CurrentSupply: meta.CurrentSupply.String(),
		Mintable:      meta.Mintable,
		PosCompatible: meta.POSCompatible,
		Creator:       creatorStr,
		IconUri:       meta.IconURI,
		Description:   meta.Description,
		Denom:         "u" + strings.ToLower(meta.Symbol),
		CreatedAt:     meta.CreatedAt.Format(time.RFC3339),
	}, nil
}

// TokenBySymbol returns metadata by symbol
func (q queryServer) TokenBySymbol(ctx context.Context, req *types.QueryTokenBySymbolRequest) (*types.QueryTokenBySymbolResponse, error) {
	if req == nil || req.Symbol == "" {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "symbol required")
	}
	store := q.k.storeService.OpenKVStore(ctx)
	symKey := append([]byte("si_"), []byte(strings.ToLower(req.Symbol))...)
	idBz, err := store.Get(symKey)
	if err != nil {
		return nil, errorsmod.Wrap(err, "symbol lookup failed")
	}
	if idBz == nil {
		return nil, errorsmod.Wrap(types.ErrTokenNotFound, "symbol not found")
	}
	metaKey := append([]byte("tm_"), idBz...)
	bz, err := store.Get(metaKey)
	if err != nil {
		return nil, errorsmod.Wrap(err, "metadata load failed")
	}
	if bz == nil {
		return nil, errorsmod.Wrap(types.ErrTokenNotFound, "metadata missing")
	}
	var meta types.TokenMetadata
	if err := json.Unmarshal(bz, &meta); err != nil {
		return nil, errorsmod.Wrap(err, "unmarshal")
	}
	creatorStr, _ := q.k.addressCodec.BytesToString(meta.Creator)
	return &types.QueryTokenBySymbolResponse{
		TokenId:       meta.TokenID,
		Name:          meta.Name,
		Symbol:        meta.Symbol,
		Decimals:      meta.Decimals,
		MaxSupply:     meta.MaxSupply.String(),
		CurrentSupply: meta.CurrentSupply.String(),
		Mintable:      meta.Mintable,
		PosCompatible: meta.POSCompatible,
		Creator:       creatorStr,
		IconUri:       meta.IconURI,
		Description:   meta.Description,
		Denom:         "u" + strings.ToLower(meta.Symbol),
		CreatedAt:     meta.CreatedAt.Format(time.RFC3339),
	}, nil
}

// helper to compute end range for prefix scan
func prefixRange(prefix []byte) (start, end []byte) {
	start = prefix
	end = make([]byte, len(prefix))
	copy(end, prefix)
	for i := len(end) - 1; i >= 0; i-- {
		if end[i] != 0xFF {
			end[i]++
			end = end[:i+1]
			return
		}
	}
	// all 0xFF => no upper bound
	end = nil
	return
}

// TokensByCreator lists tokens created by address
func (q queryServer) TokensByCreator(ctx context.Context, req *types.QueryTokensByCreatorRequest) (*types.QueryTokensByCreatorResponse, error) {
	if req == nil || req.Creator == "" {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "creator required")
	}
	addr, err := q.k.addressCodec.StringToBytes(req.Creator)
	if err != nil {
		return nil, errorsmod.Wrap(err, "invalid creator")
	}
	store := q.k.storeService.OpenKVStore(ctx)
	creatorPrefix := append([]byte("ci_"), addr...)
	start, end := prefixRange(creatorPrefix)
	iter, err := store.Iterator(start, end)
	if err != nil {
		return nil, errorsmod.Wrap(err, "iterator")
	}
	defer iter.Close()
	// Manual pagination variables
	limit := uint64(0)
	offset := uint64(0)
	if req.Pagination != nil {
		limit = req.Pagination.Limit
		offset = req.Pagination.Offset
	}
	var list []*types.QueryTokenByIdResponse
	var count uint64
	for ; iter.Valid(); iter.Next() {
		if !strings.HasPrefix(string(iter.Key()), string(creatorPrefix)) { // safety
			break
		}
		if count < offset { // skip until offset
			count++
			continue
		}
		if limit > 0 && uint64(len(list)) >= limit {
			break
		}
		// key = ci_ + addr + tokenID => extract tokenID suffix
		tokenID := string(iter.Key()[len(creatorPrefix):])
		resp, err := q.TokenById(ctx, &types.QueryTokenByIdRequest{TokenId: tokenID})
		if err != nil {
			return nil, err
		}
		list = append(list, resp)
	}
	var nextKey []byte
	if iter.Valid() { // has more
		nextKey = iter.Key()
	}
	pageRes := &query.PageResponse{NextKey: nextKey, Total: 0}
	return &types.QueryTokensByCreatorResponse{Tokens: list, Pagination: pageRes}, nil
}

// AllTokens lists all tokens
func (q queryServer) AllTokens(ctx context.Context, req *types.QueryAllTokensRequest) (*types.QueryAllTokensResponse, error) {
	if req == nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "request required")
	}
	store := q.k.storeService.OpenKVStore(ctx)
	metaPrefix := []byte("tm_")
	start, end := prefixRange(metaPrefix)
	iter, err := store.Iterator(start, end)
	if err != nil {
		return nil, errorsmod.Wrap(err, "iterator")
	}
	defer iter.Close()
	limit := uint64(0)
	offset := uint64(0)
	if req.Pagination != nil {
		limit = req.Pagination.Limit
		offset = req.Pagination.Offset
	}
	var list []*types.QueryTokenByIdResponse
	var count uint64
	for ; iter.Valid(); iter.Next() {
		if !strings.HasPrefix(string(iter.Key()), string(metaPrefix)) {
			break
		}
		if count < offset {
			count++
			continue
		}
		if limit > 0 && uint64(len(list)) >= limit {
			break
		}
		var meta types.TokenMetadata
		if err := json.Unmarshal(iter.Value(), &meta); err != nil {
			return nil, errorsmod.Wrap(err, "unmarshal")
		}
		creatorStr, _ := q.k.addressCodec.BytesToString(meta.Creator)
		list = append(list, &types.QueryTokenByIdResponse{
			TokenId:       meta.TokenID,
			Name:          meta.Name,
			Symbol:        meta.Symbol,
			Decimals:      meta.Decimals,
			MaxSupply:     meta.MaxSupply.String(),
			CurrentSupply: meta.CurrentSupply.String(),
			Mintable:      meta.Mintable,
			PosCompatible: meta.POSCompatible,
			Creator:       creatorStr,
			IconUri:       meta.IconURI,
			Description:   meta.Description,
			Denom:         "u" + strings.ToLower(meta.Symbol),
			CreatedAt:     meta.CreatedAt.Format(time.RFC3339),
		})
	}
	var nextKey []byte
	if iter.Valid() {
		nextKey = iter.Key()
	}
	pageRes := &query.PageResponse{NextKey: nextKey, Total: 0}
	return &types.QueryAllTokensResponse{Tokens: list, Pagination: pageRes}, nil
}

// TokenSupply returns supply for denom
func (q queryServer) TokenSupply(ctx context.Context, req *types.QueryTokenSupplyRequest) (*types.QueryTokenSupplyResponse, error) {
	if req == nil || req.Denom == "" {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "denom required")
	}
	supply := q.k.bankKeeper.GetSupply(ctx, req.Denom)
	return &types.QueryTokenSupplyResponse{Denom: req.Denom, Amount: supply.Amount.String()}, nil
}

// TokenBalance returns balance for address & denom
func (q queryServer) TokenBalance(ctx context.Context, req *types.QueryTokenBalanceRequest) (*types.QueryTokenBalanceResponse, error) {
	if req == nil || req.Address == "" || req.Denom == "" {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "address & denom required")
	}
	addr, err := q.k.addressCodec.StringToBytes(req.Address)
	if err != nil {
		return nil, errorsmod.Wrap(err, "invalid address")
	}
	bal := q.k.bankKeeper.GetBalance(ctx, addr, req.Denom)
	return &types.QueryTokenBalanceResponse{Address: req.Address, Denom: req.Denom, Balance: bal.Amount.String()}, nil
}
