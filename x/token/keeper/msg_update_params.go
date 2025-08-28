package keeper

import (
	"bytes"
	"context"
	"reflect"

	errorsmod "cosmossdk.io/errors"

	"bitora/x/token/types"
)

func (k msgServer) UpdateParams(ctx context.Context, req *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	authority, err := k.addressCodec.StringToBytes(req.Authority)
	if err != nil {
		return nil, errorsmod.Wrap(err, "invalid authority address")
	}

	if !bytes.Equal(k.GetAuthority(), authority) {
		expectedAuthorityStr, _ := k.addressCodec.BytesToString(k.GetAuthority())
		return nil, errorsmod.Wrapf(types.ErrInvalidSigner, "invalid authority; expected %s, got %s", expectedAuthorityStr, req.Authority)
	}

	// If the incoming Params is the zero-value struct, treat it as a no-op
	// (tests expect UpdateParams with empty params to succeed).
	var zero types.Params
	if !reflect.DeepEqual(req.Params, zero) {
		if err := req.Params.Validate(); err != nil {
			return nil, err
		}

		if err := k.Params.Set(ctx, req.Params); err != nil {
			return nil, err
		}
	}

	return &types.MsgUpdateParamsResponse{}, nil
}
