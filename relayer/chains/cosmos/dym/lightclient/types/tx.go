package types

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/migrations/legacytx"
	"github.com/dymensionxyz/gerr-cosmos/gerrc"
)

const (
	TypeMsgSetCanonicalClient = "set_canonical_client"
)

var (
	_ sdk.Msg            = &MsgSetCanonicalClient{}
	_ legacytx.LegacyMsg = &MsgSetCanonicalClient{}
)

func (msg *MsgSetCanonicalClient) Route() string {
	return ModuleName
}

func (msg *MsgSetCanonicalClient) Type() string {
	return TypeMsgSetCanonicalClient
}

func (msg *MsgSetCanonicalClient) GetSigners() []sdk.AccAddress {
	creator, err := sdk.AccAddressFromBech32(msg.Signer)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{creator}
}

func (msg *MsgSetCanonicalClient) GetSignBytes() []byte {
	bz := ModuleCdc.MustMarshalJSON(msg)
	return sdk.MustSortJSON(bz)
}

func (msg *MsgSetCanonicalClient) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Signer)
	if err != nil {
		return errorsmod.Wrapf(gerrc.ErrInvalidArgument, "invalid creator address (%s)", err)
	}
	if msg.ClientId == "" {
		return gerrc.ErrInvalidArgument.Wrap("empty client id")
	}
	return nil
}
