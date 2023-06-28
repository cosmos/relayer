package icon

import (
	"encoding/json"

	"github.com/cosmos/relayer/v2/relayer/provider"
)

const defaultStepLimit = 13610920010

type IconMessage struct {
	Params interface{}
	Method string
}

func (im *IconMessage) Type() string {
	return im.Method
}

func (im *IconMessage) MsgBytes() ([]byte, error) {
	return json.Marshal(im.Params)
}

func (icp *IconProvider) NewIconMessage(msg interface{}, method string) provider.RelayerMessage {

	im := &IconMessage{
		Params: msg,
		Method: method,
	}

	// icp.log.Debug("Icon Message ", zap.String("method", method), zap.Any("message ", msg))

	return im
}
