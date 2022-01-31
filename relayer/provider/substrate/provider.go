package substrate

import (
	clienttypes "github.com/cosmos/ibc-go/v2/modules/core/02-client/types"
	"github.com/cosmos/relayer/relayer/provider"
)

var (
	_ provider.ChainProvider  = (*SubstrateProvider)(nil)
	_ provider.ProviderConfig = (*SubstrateProviderConfig)(nil)
	_ provider.RelayPacket    = (*SubstrateRelayPacket)(nil)
	_ provider.RelayerMessage = (*SubstrateRelayerMessage)(nil)
)

type SubstrateProvider struct {
	// TODO: add properties here that are needed to implement interface definition
}

type SubstrateProviderConfig struct {
	// TODO: config stuffs for substrate provider
}

func (spc *SubstrateProviderConfig) NewProvider(homepath string, debug bool) (provider.ChainProvider, error) {
	return nil, nil
}

func (spc *SubstrateProviderConfig) Validate() error {
	return nil
}

type SubstrateRelayPacket struct {
	// TODO: things for relay Extrensics
}

func (srp *SubstrateRelayPacket) Msg(src provider.ChainProvider, srcPortId, srcChanId, dstPortId, dstChanId string) (provider.RelayerMessage, error) {
	return nil, nil
}

func (srp *SubstrateRelayPacket) FetchCommitResponse(dst provider.ChainProvider, queryHeight uint64, dstChanId, dstPortId string) error {
	return nil
}

func (srp *SubstrateRelayPacket) Data() []byte {
	return nil
}

func (srp *SubstrateRelayPacket) Seq() uint64 {
	return 0
}

func (srp *SubstrateRelayPacket) Timeout() clienttypes.Height {
	return clienttypes.Height{}
}

func (srp *SubstrateRelayPacket) TimeoutStamp() uint64 {
	return 0
}

type SubstrateRelayerMessage struct {
	// Substrate relayer message stuff
}

func (srm *SubstrateRelayerMessage) Type() string {
	return ""
}

func (srm *SubstrateRelayerMessage) MsgBytes() ([]byte, error) {
	return nil, nil
}
