package substrate

import (
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v2/modules/core/02-client/types"
	chantypes "github.com/cosmos/ibc-go/v2/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v2/modules/core/exported"
	"github.com/cosmos/relayer/relayer/provider"
)

func (sp *SubstrateProvider) Init() error {
	return nil
}

func (sp *SubstrateProvider) CreateClient(clientState ibcexported.ClientState, dstHeader ibcexported.Header) (provider.RelayerMessage, error) {
	return nil, nil
}

func (sp *SubstrateProvider) SubmitMisbehavior( /*TODO TBD*/ ) (provider.RelayerMessage, error) {
	return nil, nil
}

func (sp *SubstrateProvider) UpdateClient(srcClientId string, dstHeader ibcexported.Header) (provider.RelayerMessage, error) {
	return nil, nil
}

func (sp *SubstrateProvider) ConnectionOpenInit(srcClientId, dstClientId string, dstHeader ibcexported.Header) ([]provider.RelayerMessage, error) {
	return nil, nil
}

func (sp *SubstrateProvider) ConnectionOpenTry(dstQueryProvider provider.QueryProvider, dstHeader ibcexported.Header, srcClientId, dstClientId, srcConnId, dstConnId string) ([]provider.RelayerMessage, error) {
	return nil, nil
}

func (sp *SubstrateProvider) ConnectionOpenAck(dstQueryProvider provider.QueryProvider, dstHeader ibcexported.Header, srcClientId, srcConnId, dstClientId, dstConnId string) ([]provider.RelayerMessage, error) {
	return nil, nil
}

func (sp *SubstrateProvider) ConnectionOpenConfirm(dstQueryProvider provider.QueryProvider, dstHeader ibcexported.Header, dstConnId, srcClientId, srcConnId string) ([]provider.RelayerMessage, error) {
	return nil, nil
}

func (sp *SubstrateProvider) ChannelOpenInit(srcClientId, srcConnId, srcPortId, srcVersion, dstPortId string, order chantypes.Order, dstHeader ibcexported.Header) ([]provider.RelayerMessage, error) {
	return nil, nil
}

func (sp *SubstrateProvider) ChannelOpenTry(dstQueryProvider provider.QueryProvider, dstHeader ibcexported.Header, srcPortId, dstPortId, srcChanId, dstChanId, srcVersion, srcConnectionId, srcClientId string) ([]provider.RelayerMessage, error) {
	return nil, nil
}

func (sp *SubstrateProvider) ChannelOpenAck(dstQueryProvider provider.QueryProvider, dstHeader ibcexported.Header, srcClientId, srcPortId, srcChanId, dstChanId, dstPortId string) ([]provider.RelayerMessage, error) {
	return nil, nil
}

func (sp *SubstrateProvider) ChannelOpenConfirm(dstQueryProvider provider.QueryProvider, dstHeader ibcexported.Header, srcClientId, srcPortId, srcChanId, dstPortId, dstChannId string) ([]provider.RelayerMessage, error) {
	return nil, nil
}

func (sp *SubstrateProvider) ChannelCloseInit(srcPortId, srcChanId string) (provider.RelayerMessage, error) {
	return nil, nil
}

func (sp *SubstrateProvider) ChannelCloseConfirm(dstQueryProvider provider.QueryProvider, dsth int64, dstChanId, dstPortId, srcPortId, srcChanId string) (provider.RelayerMessage, error) {
	return nil, nil
}

func (sp *SubstrateProvider) MsgRelayAcknowledgement(dst provider.ChainProvider, dstChanId, dstPortId, srcChanId, srcPortId string, dsth int64, packet provider.RelayPacket) (provider.RelayerMessage, error) {
	return nil, nil
}

func (sp *SubstrateProvider) MsgTransfer(amount sdk.Coin, dstChainId, dstAddr, srcPortId, srcChanId string, timeoutHeight, timeoutTimestamp uint64) (provider.RelayerMessage, error) {
	return nil, nil
}

func (sp *SubstrateProvider) MsgRelayTimeout(dst provider.ChainProvider, dsth int64, packet provider.RelayPacket, dstChanId, dstPortId, srcChanId, srcPortId string) (provider.RelayerMessage, error) {
	return nil, nil
}

func (sp *SubstrateProvider) MsgRelayRecvPacket(dst provider.ChainProvider, dsth int64, packet provider.RelayPacket, dstChanId, dstPortId, srcChanId, srcPortId string) (provider.RelayerMessage, error) {
	return nil, nil
}

func (sp *SubstrateProvider) MsgUpgradeClient(srcClientId string, consRes *clienttypes.QueryConsensusStateResponse, clientRes *clienttypes.QueryClientStateResponse) (provider.RelayerMessage, error) {
	return nil, nil
}

func (sp *SubstrateProvider) RelayPacketFromSequence(src, dst provider.ChainProvider, srch, dsth, seq uint64, dstChanId, dstPortId, srcChanId, srcPortId, srcClientId string) (provider.RelayerMessage, provider.RelayerMessage, error) {
	return nil, nil, nil
}

func (sp *SubstrateProvider) AcknowledgementFromSequence(dst provider.ChainProvider, dsth, seq uint64, dstChanId, dstPortId, srcChanId, srcPortId string) (provider.RelayerMessage, error) {
	return nil, nil
}

func (sp *SubstrateProvider) SendMessage(msg provider.RelayerMessage) (*provider.RelayerTxResponse, bool, error) {
	return nil, false, nil
}

func (sp *SubstrateProvider) SendMessages(msgs []provider.RelayerMessage) (*provider.RelayerTxResponse, bool, error) {
	return nil, false, nil
}

func (sp *SubstrateProvider) GetLightSignedHeaderAtHeight(h int64) (ibcexported.Header, error) {
	return nil, nil
}

func (sp *SubstrateProvider) GetIBCUpdateHeader(srch int64, dst provider.ChainProvider, dstClientId string) (ibcexported.Header, error) {
	return nil, nil
}

func (sp *SubstrateProvider) ChainId() string {
	return ""
}

func (sp *SubstrateProvider) Type() string {
	return ""
}

func (sp *SubstrateProvider) ProviderConfig() provider.ProviderConfig {
	return nil
}

func (sp *SubstrateProvider) Key() string {
	return ""
}

func (sp *SubstrateProvider) Address() (string, error) {
	return "", nil
}

func (sp *SubstrateProvider) Timeout() string {
	return ""
}

func (sp *SubstrateProvider) TrustingPeriod() (time.Duration, error) {
	return 0, nil
}

func (sp *SubstrateProvider) WaitForNBlocks(n int64) error {
	return nil
}
