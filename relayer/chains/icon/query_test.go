package icon

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	"github.com/gogo/protobuf/proto"
	itm "github.com/icon-project/IBC-Integration/libraries/go/common/tendermint"
	"github.com/stretchr/testify/assert"
)

func TestAnyTypeConversion(t *testing.T) {
	clientStateByte := "0x0a0469636f6e1204080210031a0308e80722050880b899292a070880c0cbacf622384a40014801"
	clB, _ := hex.DecodeString(strings.TrimPrefix(clientStateByte, "0x"))
	var clientState itm.ClientState
	proto.Unmarshal(clB, &clientState)

	any, err := clienttypes.PackClientState(&clientState)
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("0x%x", any.Value), clientStateByte)

}
