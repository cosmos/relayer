package provider

import (
	"github.com/avast/retry-go"
	clienttypes "github.com/cosmos/ibc-go/v2/modules/core/02-client/types"
	ibcexported "github.com/cosmos/ibc-go/v2/modules/core/exported"
	"time"
)

var (
	RtyAttNum = uint(5)
	RtyAtt    = retry.Attempts(RtyAttNum)
	RtyDel    = retry.Delay(time.Millisecond * 400)
	RtyErr    = retry.LastErrorOnly(true)
)

// MustGetHeight takes the height inteface and returns the actual height
func MustGetHeight(h ibcexported.Height) clienttypes.Height {
	height, ok := h.(clienttypes.Height)
	if !ok {
		panic("height is not an instance of height!")
	}
	return height
}
