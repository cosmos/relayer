package icon

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"cosmossdk.io/errors"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/gorilla/websocket"
	"github.com/icon-project/ibc-relayer/relayer/chains/icon/types"
	"go.uber.org/zap"
)

const (
	ENDPOINT              = "https://ctz.solidwallet.io/api/v3"
	WSS_ENDPOINT          = "wss://ctz.solidwallet.io/api/v3/icon_dex/event"
	SEND_PACKET_SIGNATURE = "SendPacket(bytes)"
	CONTRACT_ADDRESS      = "cxf17ab3c6daa47e915eab4292fbf3094067e9a026"
)

func (ip *IconProvider) FetchEvent(height int) {

	blockReq := &types.BlockRequest{
		EventFilters: []*types.EventFilter{{
			Addr:      types.Address(CONTRACT_ADDRESS),
			Signature: SEND_PACKET_SIGNATURE,
			// Indexed:   []*string{&dstAddr},
		}},
		Height: types.NewHexInt(int64(height)),
	}
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	l := zap.Logger{}

	client := NewClient(WSS_ENDPOINT, l)
	h, s := height, 0

	go func() {
		err := client.MonitorBlock(ctx, blockReq, func(conn *websocket.Conn, v *types.BlockNotification) error {
			_h, _ := v.Height.Int()
			if _h != h {
				err := fmt.Errorf("invalid block height: %d, expected: %d", _h, h+1)
				l.Warn(err.Error())
				return err
			}
			h++
			s++
			return nil
		},
			func(conn *websocket.Conn) {
				l.Info("Connected")
			},
			func(conn *websocket.Conn, err error) {
				l.Info("Disconnected")
				_ = conn.Close()
			})
		if err.Error() == "context deadline exceeded" {
			return
		}
	}()

}

type Packet struct {
	Sequence           uint
	SourcePort         string
	SourceChannel      string
	DestinationPort    string
	DestinationChannel string
	Data               []byte
	Height             Height
	Timestamp          uint
}

type Height struct {
	RevisionNumber uint
	RevisionHeight uint
}

func parseSendPacket(str string) (*Packet, error) {
	p := Packet{}
	e := rlpDecodeHex(str, &p)
	if e != nil {
		return nil, e
	}
	return &p, nil
}

func rlpDecodeHex(str string, out interface{}) error {
	str = strings.TrimPrefix(str, "0x")
	input, err := hex.DecodeString(str)
	if err != nil {
		return errors.Wrap(err, "hex.DecodeString ")
	}
	err = rlp.Decode(bytes.NewReader(input), out)
	if err != nil {
		return errors.Wrap(err, "rlp.Decode ")
	}
	return nil
}
