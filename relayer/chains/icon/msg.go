package icon

import (
	"github.com/icon-project/ibc-relayer/relayer/provider"
)

const defaultStepLimit = 13610920010

type IconMessage struct {
	Params interface{}
	Method string
}

func (im *IconMessage) Type() string {
	return ""
}

func (im *IconMessage) MsgBytes() ([]byte, error) {
	return nil, nil
}

func NewIconMessage(msg interface{}, method string) provider.RelayerMessage {
	return &IconMessage{
		Params: msg,
		Method: method,
	}
}
