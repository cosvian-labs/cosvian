//go:build osmosis_removed

package types

import (
    errorsmod "cosmossdk.io/errors"
    sdk "github.com/cosmos/cosmos-sdk/types"
    sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// Implement legacy sdk.Msg interface for MsgUpdateParams so governance proposal
// messages execute on SDK versions expecting legacy routing. The Msg service
// registration alone produced deliverTx code=10 (unrecognized by router), so we
// add the legacy methods.
// Keep this minimal; keeper logic still does authoritative checks.

var _ sdk.Msg = (*MsgUpdateParams)(nil)

func (m *MsgUpdateParams) Route() string { return ModuleName }

func (m *MsgUpdateParams) Type() string { return "update_params" }

func (m *MsgUpdateParams) ValidateBasic() error {
    if m == nil {
        return errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "nil MsgUpdateParams")
    }
    if m.Authority == "" {
        return errorsmod.Wrap(sdkerrors.ErrUnauthorized, "missing authority")
    }
    return nil
}

func (m *MsgUpdateParams) GetSigners() []sdk.AccAddress {
    addr, err := sdk.AccAddressFromBech32(m.Authority)
    if err != nil {
        return []sdk.AccAddress{}
    }
    return []sdk.AccAddress{addr}
}
