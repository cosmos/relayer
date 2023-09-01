package common

import (
	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
)

var (
	EventTimeoutRequest   = "TimeoutRequest(bytes)"
	IconModule            = "icon"
	WasmModule            = "wasm"
	ArchwayModule         = "archway"
	TendermintLightClient = "07-tendermint"
	IconLightClient       = "iconclient"
	ConnectionKey         = "connection"
	ChannelKey            = "channel"
	ONE_HOUR              = 60 * 60 * 1000
	NanoToMilliRatio      = 1000_000
)

var (
	EventRequiresClientUpdate = map[string]bool{
		chantypes.EventTypeRecvPacket:        true,
		chantypes.EventTypeAcknowledgePacket: true,
		chantypes.EventTypeTimeoutPacket:     true,

		conntypes.EventTypeConnectionOpenTry:     true,
		conntypes.EventTypeConnectionOpenAck:     true,
		conntypes.EventTypeConnectionOpenConfirm: true,

		chantypes.EventTypeChannelOpenTry:      true,
		chantypes.EventTypeChannelOpenAck:      true,
		chantypes.EventTypeChannelOpenConfirm:  true,
		chantypes.EventTypeChannelCloseConfirm: true,
	}
)
