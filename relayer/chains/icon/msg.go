package icon

import (
	"github.com/cosmos/relayer/v2/relayer/provider"
)

const defaultStepLimit = 13610920010

type IconMessage struct {
	Params interface{}
	Method string
}

func (im *IconMessage) Type() string {
	return "icon"
}

func (im *IconMessage) MsgBytes() ([]byte, error) {
	b := []byte("abc")
	return b, nil
}

func NewIconMessage(msg interface{}, method string) provider.RelayerMessage {
	return &IconMessage{
		Params: msg,
		Method: method,
	}
}
