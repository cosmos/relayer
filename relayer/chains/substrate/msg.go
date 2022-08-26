package substrate

import (
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/gogo/protobuf/proto"
)

var _ provider.RelayerMessage = &SubstrateMessage{}

type Msg interface {
	proto.Message
}

type SubstrateMessage struct {
	Msg Msg
}

func NewSubstrateMessage(msg Msg) provider.RelayerMessage {
	return SubstrateMessage{
		Msg: msg,
	}
}

func (cm SubstrateMessage) Type() string {
	return "/" + proto.MessageName(cm.Msg)
}

func (cm SubstrateMessage) MsgBytes() ([]byte, error) {
	return proto.Marshal(cm.Msg)
}
