package keeper

import (
	"context"
	"errors"
	"fmt"

	"bitora/x/icqcontroller/types"

	"cosmossdk.io/collections"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func (k Keeper) InitGenesis(ctx context.Context, genState types.GenesisState) error {
	if err := k.Port.Set(ctx, genState.PortId); err != nil {
		return err
	}

	// Ensure IBC port capability exists under this module's scope so channel handshake can succeed.
	// Not all environments will have capability keeper wired; skip if unavailable.
	if k.scopedKeeper != nil {
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		name := fmt.Sprintf("ports/%s", genState.PortId)
		if _, ok := k.scopedKeeper.GetCapability(sdkCtx, name); !ok {
			if _, err := k.scopedKeeper.NewCapability(sdkCtx, name); err != nil {
				// best-effort: ignore errors if capability already exists globally
			}
		}
	}

	return k.Params.Set(ctx, genState.Params)
}

// ExportGenesis returns the module's exported genesis.
func (k Keeper) ExportGenesis(ctx context.Context) (*types.GenesisState, error) {
	var err error

	genesis := types.DefaultGenesis()
	genesis.Params, err = k.Params.Get(ctx)
	if err != nil {
		return nil, err
	}
	genesis.PortId, err = k.Port.Get(ctx)
	if err != nil && !errors.Is(err, collections.ErrNotFound) {
		return nil, err
	}

	return genesis, nil
}
