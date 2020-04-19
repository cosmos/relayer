package relayer

import (
	"github.com/cosmos/cosmos-sdk/codec"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	chanTypes "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/types"
)

var ModuleCdc *codec.Codec

func RegisterCodec(cdc *codec.Codec) {
	if ModuleCdc != nil {
		return
	}
	cdc.RegisterConcrete(MsgSendPacket{}, "SendPacket", nil)
	ModuleCdc = cdc
}

// MsgSendPacket sends an outgoing IBC packet
type MsgSendPacket struct {
	Packet chanTypes.Packet `json:"packet" yaml:"packet"`
	Signer sdk.AccAddress   `json:"signer" yaml:"signer"`
}

var _ sdk.Msg = MsgSendPacket{}

// NewMsgSendPacket returns a new send request
func NewMsgSendPacket(packet chanTypes.Packet, signer sdk.AccAddress) MsgSendPacket {
	return MsgSendPacket{
		Packet: packet,
		Signer: signer,
	}
}

// Route implements sdk.Msg
func (msg MsgSendPacket) Route() string {
	// FIXME: Do we need this if we are only sending?
	return "swingset"
}

// ValidateBasic implements sdk.Msg
func (msg MsgSendPacket) ValidateBasic() error {
	if msg.Signer.Empty() {
		return sdkerrors.ErrInvalidAddress
	}

	return msg.Packet.ValidateBasic()
}

// GetSignBytes implements sdk.Msg
func (msg MsgSendPacket) GetSignBytes() []byte {
	// FIXME: What do we need here?
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(msg))
}

// GetSigners implements sdk.Msg
func (msg MsgSendPacket) GetSigners() []sdk.AccAddress {
	return []sdk.AccAddress{msg.Signer}
}

// Type implements sdk.Msg
func (msg MsgSendPacket) Type() string {
	return "sendpacket"
}
