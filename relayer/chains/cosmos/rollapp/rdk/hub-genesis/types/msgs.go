package types

import (
	"errors"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/dymensionxyz/gerr-cosmos/gerrc"
)

var _ sdk.Msg = (*MsgSendTransfer)(nil)

func (m *MsgSendTransfer) GetSigners() []sdk.AccAddress {
	return []sdk.AccAddress{sdk.MustAccAddressFromBech32(m.Signer)}
}

func (m *MsgSendTransfer) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(m.Signer)
	if err != nil {
		return errorsmod.Wrap(errors.Join(gerrc.ErrInvalidArgument, err), "get relayer addr from bech32")
	}
	if m.ChannelId == "" {
		return errorsmod.Wrap(gerrc.ErrInvalidArgument, "channel id is empty")
	}
	return nil
}
