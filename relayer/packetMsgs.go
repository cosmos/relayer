package relayer

import (
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	chanTypes "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/types"
)

// ModuleCdc references the relayer module codec
var ModuleCdc = codec.NewProtoCodec(codectypes.NewInterfaceRegistry())

// RegisterInterfaces register the packet interfaces to protobuf any
func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgSendPacket{},
	)
}

var _ sdk.Msg = &MsgSendPacket{}

// NewMsgSendPacket returns a new send request
func NewMsgSendPacket(packet chanTypes.Packet, sender sdk.AccAddress) *MsgSendPacket {
	return &MsgSendPacket{
		Packet: packet,
		Sender: sender,
	}
}

// Route implements sdk.Msg
func (msg MsgSendPacket) Route() string {
	// FIXME: Do we need this if we are only sending?
	return "swingset"
}

// ValidateBasic implements sdk.Msg
func (msg MsgSendPacket) ValidateBasic() error {
	if msg.Sender.Empty() {
		return sdkerrors.ErrInvalidAddress
	}

	return msg.Packet.ValidateBasic()
}

// GetSignBytes implements sdk.Msg
func (msg MsgSendPacket) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(&msg))
}

// GetSigners implements sdk.Msg
func (msg MsgSendPacket) GetSigners() []sdk.AccAddress {
	return []sdk.AccAddress{msg.Sender}
}

// Type implements sdk.Msg
func (msg MsgSendPacket) Type() string {
	return "sendpacket"
}
