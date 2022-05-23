package ibc

import (
	"fmt"
	"time"

	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
)

type LatestBlock struct {
	Height uint64
	Time   time.Time
}

type PacketInfo struct {
	Sequence             uint64
	SourceChannelID      string
	SourcePortID         string
	DestinationChannelID string
	DestinationPortID    string
	ConnectionID         string
	ChannelOrdering      string
	Data                 []byte
	Ack                  []byte
	TimeoutTimestamp     uint64
	TimeoutHeight        clienttypes.Height
}

type ChannelKey struct {
	ChannelID             string
	PortID                string
	CounterpartyChannelID string
	CounterpartyPortID    string
}

type ChannelInfo struct {
	ConnectionID             string
	ClientID                 string
	PortID                   string
	ChannelID                string
	CounterpartyClientID     string
	CounterpartyConnectionID string
	CounterpartyPortID       string
	CounterpartyChannelID    string
}

type ClientInfo struct {
	ClientID        string
	ConsensusHeight clienttypes.Height
}

type IBCMessageWithSequence struct {
	Sequence uint64
	Message  provider.RelayerMessage
}

type InProgressSend struct {
	SendHeight uint64
	RetryCount uint64
	Message    provider.RelayerMessage
}

type TimeoutError struct {
	msg string
}

func (t *TimeoutError) Error() string {
	return fmt.Sprintf("packet timeout error: %s", t.msg)
}

func NewTimeoutError(msg string) *TimeoutError {
	return &TimeoutError{msg}
}

type TimeoutOnCloseError struct {
	msg string
}

func (t *TimeoutOnCloseError) Error() string {
	return fmt.Sprintf("packet timeout on close error: %s", t.msg)
}

func NewTimeoutOnCloseError(msg string) *TimeoutOnCloseError {
	return &TimeoutOnCloseError{msg}
}
