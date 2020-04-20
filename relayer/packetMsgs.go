package relayer

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// MsgOpaquePacket sends an outgoing IBC packet
type MsgOpaquePacket struct {
	Data   []byte         `json:"data" yaml:"data"`
	Sender sdk.AccAddress `json:"sender" yaml:"sender"` // the sender address
}

var _ sdk.Msg = MsgOpaquePacket{}

// NewMsgOpaquePacket returns a new send request
func NewMsgOpaquePacket(packetData []byte, sender sdk.AccAddress) MsgOpaquePacket {
	return MsgOpaquePacket{
		Data:   packetData,
		Sender: sender,
	}
}

// Route implements sdk.Msg
func (msg MsgOpaquePacket) Route() string {
	// FIXME: Do we need this if we are only sending?
	return "swingset"
}

// ValidateBasic implements sdk.Msg
func (msg MsgOpaquePacket) ValidateBasic() error {
	if msg.Sender.Empty() {
		return sdkerrors.ErrInvalidAddress
	}

	return nil
}

// GetSignBytes implements sdk.Msg
func (msg MsgOpaquePacket) GetSignBytes() []byte {
	return msg.Data
}

// GetSigners implements sdk.Msg
func (msg MsgOpaquePacket) GetSigners() []sdk.AccAddress {
	return []sdk.AccAddress{msg.Sender}
}

// Type implements sdk.Msg
func (msg MsgOpaquePacket) Type() string {
	return "ibc/opaque"
}
