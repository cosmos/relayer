package icon

import (
	"encoding/json"

	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
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
	return json.Marshal(im)
}

func (icp *IconProvider) NewIconMessage(msg interface{}, method string) provider.RelayerMessage {

	im := &IconMessage{
		Params: msg,
		Method: method,
	}

	icp.log.Debug("Icon Message ", zap.String("Method name", method), zap.Any("Value ", msg))

	return im
}
