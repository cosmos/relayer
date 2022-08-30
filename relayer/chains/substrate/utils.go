package substrate

import (
	chantypes "github.com/cosmos/ibc-go/v5/modules/core/04-channel/types"
)

func IntoIBCType(substrateType SubstrateEventType) string {
	switch substrateType {
	case SendPacket:
		return chantypes.EventTypeSendPacket
	default:
		return ""
	}
}
