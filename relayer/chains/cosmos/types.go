package cosmos

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
)

// ibcMessage is the type used for parsing all possible properties of IBC messages
type ibcMessage struct {
	action string
	info   ibcMessageInfo
}

type ibcMessageInfo interface {
	parseAttrs(log *zap.Logger, attrs []sdk.Attribute)
}

// alias types to the provider types, used for adding parser methods
type packetInfo provider.PacketInfo

type channelInfo provider.ChannelInfo

type connectionInfo provider.ConnectionInfo

// clientInfo contains the consensus height of the counterparty chain for a client.
type clientInfo struct {
	clientID        string
	consensusHeight clienttypes.Height
	header          []byte
}

func (clientInfo) ibcMessageInfoIndicator() {}

func (c clientInfo) ClientState() provider.ClientState {
	return provider.ClientState{
		ClientID:        c.clientID,
		ConsensusHeight: c.consensusHeight,
	}
}

// latestClientState is a map of clientID to the latest clientInfo for that client.
type latestClientState map[string]provider.ClientState

func (l latestClientState) update(clientInfo clientInfo) {
	existingClientInfo, ok := l[clientInfo.clientID]
	if ok && clientInfo.consensusHeight.LT(existingClientInfo.ConsensusHeight) {
		// height is less than latest, so no-op
		return
	}

	// update latest if no existing state or provided consensus height is newer
	l[clientInfo.clientID] = clientInfo.ClientState()
}
