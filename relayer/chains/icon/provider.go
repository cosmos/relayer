package icon

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cosmos/gogoproto/proto"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	commitmenttypes "github.com/cosmos/ibc-go/v7/modules/core/23-commitment/types"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/exported"
)

var (
	_ provider.ChainProvider  = &IconProvider{}
	_ provider.KeyProvider    = &IconProvider{}
	_ provider.ProviderConfig = &IconProviderConfig{}
)

type IconProviderConfig struct {
	Key               string `json:"key" yaml:"key"`
	ChainName         string `json:"-" yaml:"-"`
	ChainID           string `json:"chain-id" yaml:"chain-id"`
	RPCAddr           string `json:"rpc-addr" yaml:"rpc-addr"`
	Timeout           string `json:"timeout" yaml:"timeout"`
	IbcHostAddress    string `json:"ibc_host_address,omitempty"`
	IbcHandlerAddress string `json:"ibc_handler_address,omitempty"`
}

func (pp IconProviderConfig) Validate() error {
	if _, err := time.ParseDuration(pp.Timeout); err != nil {
		return fmt.Errorf("invalid Timeout: %w", err)
	}
	return nil
}

// NewProvider should provide a new Icon provider
func (pp IconProviderConfig) NewProvider(log *zap.Logger, homepath string, debug bool, chainName string) (provider.ChainProvider, error) {
	if err := pp.Validate(); err != nil {
		return nil, err
	}

	return &IconProvider{
		log:    log.With(zap.String("sys", "chain_client")),
		client: NewClient(pp.getRPCAddr(), *log),
		PCfg:   pp,
	}, nil
}

func (pp IconProviderConfig) getRPCAddr() string {
	return pp.RPCAddr
}

func ChainClientConfig(pcfg *IconProviderConfig) {
}

type IconProvider struct {
	log     *zap.Logger
	PCfg    IconProviderConfig
	txMu    sync.Mutex
	client  *Client
	metrics *processor.PrometheusMetrics
}

type IconIBCHeader struct {

	//data to add
}

func (h IconIBCHeader) Height() uint64 {
	return 0
}

func (h IconIBCHeader) ConsensusState() ibcexported.ConsensusState {
	return nil
}

//ChainProvider Methods

func (icp *IconProvider) Init(ctx context.Context) error {
	return nil
}

func (icp *IconProvider) NewClientState(dstChainID string, dstIBCHeader provider.IBCHeader, dstTrustingPeriod, dstUbdPeriod time.Duration, allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour bool) (ibcexported.ClientState, error) {
	return nil, nil
}

func (icp *IconProvider) MsgCreateClient(clientState ibcexported.ClientState, consensusState ibcexported.ConsensusState) (provider.RelayerMessage, error) {
	return nil, nil
}

func (icp *IconProvider) MsgUpgradeClient(srcClientId string, consRes *clienttypes.QueryConsensusStateResponse, clientRes *clienttypes.QueryClientStateResponse) (provider.RelayerMessage, error) {
	return nil, nil
}

func (icp *IconProvider) ValidatePacket(msgTransfer provider.PacketInfo, latestBlock provider.LatestBlock) error {
	return nil
}

func (icp *IconProvider) PacketCommitment(ctx context.Context, msgTransfer provider.PacketInfo, height uint64) (provider.PacketProof, error) {
	return provider.PacketProof{}, nil
}

func (icp *IconProvider) PacketAcknowledgement(ctx context.Context, msgRecvPacket provider.PacketInfo, height uint64) (provider.PacketProof, error) {
	return provider.PacketProof{}, nil

}

func (icp *IconProvider) PacketReceipt(ctx context.Context, msgTransfer provider.PacketInfo, height uint64) (provider.PacketProof, error) {
	return provider.PacketProof{}, nil

}

func (icp *IconProvider) NextSeqRecv(ctx context.Context, msgTransfer provider.PacketInfo, height uint64) (provider.PacketProof, error) {
	return provider.PacketProof{}, nil

}

func (icp *IconProvider) MsgTransfer(dstAddr string, amount sdk.Coin, info provider.PacketInfo) (provider.RelayerMessage, error) {
	return nil, nil
}

func (icp *IconProvider) MsgRecvPacket(msgTransfer provider.PacketInfo, proof provider.PacketProof) (provider.RelayerMessage, error) {
	return nil, nil
}

func (icp *IconProvider) MsgAcknowledgement(msgRecvPacket provider.PacketInfo, proofAcked provider.PacketProof) (provider.RelayerMessage, error) {
	return nil, nil
}

func (icp *IconProvider) MsgTimeout(msgTransfer provider.PacketInfo, proofUnreceived provider.PacketProof) (provider.RelayerMessage, error) {
	return nil, nil
}

func (icp *IconProvider) MsgTimeoutOnClose(msgTransfer provider.PacketInfo, proofUnreceived provider.PacketProof) (provider.RelayerMessage, error) {
	return nil, nil
}

func (icp *IconProvider) ConnectionHandshakeProof(ctx context.Context, msgOpenInit provider.ConnectionInfo, height uint64) (provider.ConnectionProof, error) {
	return provider.ConnectionProof{}, nil

}

func (icp *IconProvider) ConnectionProof(ctx context.Context, msgOpenAck provider.ConnectionInfo, height uint64) (provider.ConnectionProof, error) {
	return provider.ConnectionProof{}, nil
}

func (icp *IconProvider) MsgConnectionOpenInit(info provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	return nil, nil
}

func (icp *IconProvider) MsgConnectionOpenTry(msgOpenInit provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	return nil, nil
}

func (icp *IconProvider) MsgConnectionOpenAck(msgOpenTry provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	return nil, nil
}

func (icp *IconProvider) MsgConnectionOpenConfirm(msgOpenAck provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	return nil, nil
}

func (icp *IconProvider) ChannelProof(ctx context.Context, msg provider.ChannelInfo, height uint64) (provider.ChannelProof, error) {
	return provider.ChannelProof{}, nil
}

func (icp *IconProvider) MsgChannelOpenInit(info provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	return nil, nil
}

func (icp *IconProvider) MsgChannelOpenTry(msgOpenInit provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	return nil, nil
}

func (icp *IconProvider) MsgChannelOpenAck(msgOpenTry provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	return nil, nil
}

func (icp *IconProvider) MsgChannelOpenConfirm(msgOpenAck provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	return nil, nil
}

func (icp *IconProvider) MsgChannelCloseInit(info provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	return nil, nil
}

func (icp *IconProvider) MsgChannelCloseConfirm(msgCloseInit provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	return nil, nil
}

func (icp *IconProvider) MsgUpdateClientHeader(latestHeader provider.IBCHeader, trustedHeight clienttypes.Height, trustedHeader provider.IBCHeader) (ibcexported.ClientMessage, error) {
	return nil, nil

}

func (icp *IconProvider) MsgUpdateClient(clientID string, counterpartyHeader ibcexported.ClientMessage) (provider.RelayerMessage, error) {
	return nil, nil
}

func (icp *IconProvider) QueryICQWithProof(ctx context.Context, msgType string, request []byte, height uint64) (provider.ICQProof, error) {
	return provider.ICQProof{}, nil
}

func (icp *IconProvider) MsgSubmitQueryResponse(chainID string, queryID provider.ClientICQQueryID, proof provider.ICQProof) (provider.RelayerMessage, error) {
	return nil, nil
}

func (icp *IconProvider) RelayPacketFromSequence(ctx context.Context, src provider.ChainProvider, srch, dsth, seq uint64, srcChanID, srcPortID string, order chantypes.Order) (provider.RelayerMessage, provider.RelayerMessage, error) {
	return nil, nil, nil
}

func (icp *IconProvider) AcknowledgementFromSequence(ctx context.Context, dst provider.ChainProvider, dsth, seq uint64, dstChanID, dstPortID, srcChanID, srcPortID string) (provider.RelayerMessage, error) {
	return nil, nil
}

func (icp *IconProvider) SendMessage(ctx context.Context, msg provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {
	return nil, false, nil
}

func (icp *IconProvider) SendMessages(ctx context.Context, msgs []provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {
	return nil, false, nil
}

func (icp *IconProvider) ChainName() string {
	return "icon"
}

func (icp *IconProvider) ChainId() string {
	return ""
}

func (icp *IconProvider) Type() string {
	return ""
}

func (icp *IconProvider) ProviderConfig() provider.ProviderConfig {
	return icp.PCfg
}

func (icp *IconProvider) CommitmentPrefix() commitmenttypes.MerklePrefix {
	return commitmenttypes.MerklePrefix{}
}

func (icp *IconProvider) Key() string {
	return ""
}

func (icp *IconProvider) Address() (string, error) {
	return "", nil
}

func (icp *IconProvider) Timeout() string {
	return ""
}

func (icp *IconProvider) TrustingPeriod(ctx context.Context) (time.Duration, error) {
	return 0, nil
}

func (icp *IconProvider) WaitForNBlocks(ctx context.Context, n int64) error {
	return nil
}

func (icp *IconProvider) Sprint(toPrint proto.Message) (string, error) {
	return "", nil
}
