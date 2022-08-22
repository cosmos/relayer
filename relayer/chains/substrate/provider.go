package substrate

import (
	"context"
	"fmt"
	"io"
	"path"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v5/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v5/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v5/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v5/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/chains/substrate/keystore"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
)

var (
	_ provider.ChainProvider  = &SubstrateProvider{}
	_ provider.KeyProvider    = &SubstrateProvider{}
	_ provider.ProviderConfig = &SubstrateProviderConfig{}
)

type SubstrateProviderConfig struct {
	Key            string `json:"key" yaml:"key"`
	ChainName      string `json:"-" yaml:"-"`
	ChainID        string `json:"chain-id" yaml:"chain-id"`
	RPCAddr        string `json:"rpc-addr" yaml:"rpc-addr"`
	RelayRPCAddr   string `json:"relay-rpc-addr" yaml:"relay-rpc-addr"`
	AccountPrefix  string `json:"account-prefix" yaml:"account-prefix"`
	KeyringBackend string `json:"keyring-backend" yaml:"keyring-backend"`
	KeyDirectory   string `json:"key-directory" yaml:"key-directory"`
	Debug          bool   `json:"debug" yaml:"debug"`
	Timeout        string `json:"timeout" yaml:"timeout"`
	Network        uint16 `json:"network" yaml:"network"`
}

func (spc SubstrateProviderConfig) Validate() error {
	if _, err := time.ParseDuration(spc.Timeout); err != nil {
		return fmt.Errorf("invalid Timeout: %w", err)
	}
	return nil
}

func keysDir(home, chainID string) string {
	return path.Join(home, "keys", chainID)
}

// NewProvider validates the SubstrateProviderConfig, instantiates a ChainClient and then instantiates a SubstrateProvider
func (spc SubstrateProviderConfig) NewProvider(log *zap.Logger, homepath string, debug bool, chainName string) (provider.ChainProvider, error) {
	if err := spc.Validate(); err != nil {
		return nil, err
	}

	if len(spc.KeyDirectory) == 0 {
		spc.KeyDirectory = keysDir(homepath, spc.ChainID)
	}

	sp := &SubstrateProvider{
		log:  log,
		PCfg: spc,
	}

	err := sp.Init()
	if err != nil {
		return nil, err
	}

	return sp, nil
}

func (sp *SubstrateProvider) Init() error {
	keybase, err := keystore.New(sp.PCfg.ChainID, sp.PCfg.KeyringBackend, sp.PCfg.KeyDirectory, sp.Input)
	if err != nil {
		return err
	}

	sp.Keybase = keybase
	return nil
}

type SubstrateProvider struct {
	log     *zap.Logger
	Keybase keystore.Keyring
	PCfg    SubstrateProviderConfig
	Input   io.Reader
}

func (sp *SubstrateProvider) BlockTime(ctx context.Context, height int64) (time.Time, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryTx(ctx context.Context, hashHex string) (*provider.RelayerTxResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryTxs(ctx context.Context, page, limit int, events []string) ([]*provider.RelayerTxResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryLatestHeight(ctx context.Context) (int64, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryIBCHeader(ctx context.Context, h int64) (provider.IBCHeader, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QuerySendPacket(ctx context.Context, srcChanID, srcPortID string, sequence uint64) (provider.PacketInfo, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryRecvPacket(ctx context.Context, dstChanID, dstPortID string, sequence uint64) (provider.PacketInfo, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryBalance(ctx context.Context, keyName string) (sdk.Coins, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryBalanceWithAddress(ctx context.Context, addr string) (sdk.Coins, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryUnbondingPeriod(ctx context.Context) (time.Duration, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryClientState(ctx context.Context, height int64, clientid string) (ibcexported.ClientState, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryClientStateResponse(ctx context.Context, height int64, srcClientId string) (*clienttypes.QueryClientStateResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryClientConsensusState(ctx context.Context, chainHeight int64, clientid string, clientHeight ibcexported.Height) (*clienttypes.QueryConsensusStateResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryUpgradedClient(ctx context.Context, height int64) (*clienttypes.QueryClientStateResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryUpgradedConsState(ctx context.Context, height int64) (*clienttypes.QueryConsensusStateResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryConsensusState(ctx context.Context, height int64) (ibcexported.ConsensusState, int64, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryClients(ctx context.Context) (clienttypes.IdentifiedClientStates, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryConnection(ctx context.Context, height int64, connectionid string) (*conntypes.QueryConnectionResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryConnections(ctx context.Context) (conns []*conntypes.IdentifiedConnection, err error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryConnectionsUsingClient(ctx context.Context, height int64, clientid string) (*conntypes.QueryConnectionsResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) GenerateConnHandshakeProof(ctx context.Context, height int64, clientId, connId string) (clientState ibcexported.ClientState, clientStateProof []byte, consensusProof []byte, connectionProof []byte, connectionProofHeight ibcexported.Height, err error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryChannel(ctx context.Context, height int64, channelid, portid string) (chanRes *chantypes.QueryChannelResponse, err error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryChannelClient(ctx context.Context, height int64, channelid, portid string) (*clienttypes.IdentifiedClientState, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryConnectionChannels(ctx context.Context, height int64, connectionid string) ([]*chantypes.IdentifiedChannel, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryChannels(ctx context.Context) ([]*chantypes.IdentifiedChannel, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryPacketCommitments(ctx context.Context, height uint64, channelid, portid string) (commitments *chantypes.QueryPacketCommitmentsResponse, err error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryPacketAcknowledgements(ctx context.Context, height uint64, channelid, portid string) (acknowledgements []*chantypes.PacketState, err error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryUnreceivedPackets(ctx context.Context, height uint64, channelid, portid string, seqs []uint64) ([]uint64, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryUnreceivedAcknowledgements(ctx context.Context, height uint64, channelid, portid string, seqs []uint64) ([]uint64, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryNextSeqRecv(ctx context.Context, height int64, channelid, portid string) (recvRes *chantypes.QueryNextSequenceReceiveResponse, err error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryPacketCommitment(ctx context.Context, height int64, channelid, portid string, seq uint64) (comRes *chantypes.QueryPacketCommitmentResponse, err error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryPacketAcknowledgement(ctx context.Context, height int64, channelid, portid string, seq uint64) (ackRes *chantypes.QueryPacketAcknowledgementResponse, err error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryPacketReceipt(ctx context.Context, height int64, channelid, portid string, seq uint64) (recRes *chantypes.QueryPacketReceiptResponse, err error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryDenomTrace(ctx context.Context, denom string) (*transfertypes.DenomTrace, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) QueryDenomTraces(ctx context.Context, offset, limit uint64, height int64) ([]transfertypes.DenomTrace, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) NewClientState(dstChainID string, dstIBCHeader provider.IBCHeader, dstTrustingPeriod, dstUbdPeriod time.Duration, allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour bool) (ibcexported.ClientState, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) MsgCreateClient(clientState ibcexported.ClientState, consensusState ibcexported.ConsensusState) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) MsgUpgradeClient(srcClientId string, consRes *clienttypes.QueryConsensusStateResponse, clientRes *clienttypes.QueryClientStateResponse) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) ValidatePacket(msgTransfer provider.PacketInfo, latestBlock provider.LatestBlock) error {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) PacketCommitment(ctx context.Context, msgTransfer provider.PacketInfo, height uint64) (provider.PacketProof, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) PacketAcknowledgement(ctx context.Context, msgRecvPacket provider.PacketInfo, height uint64) (provider.PacketProof, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) PacketReceipt(ctx context.Context, msgTransfer provider.PacketInfo, height uint64) (provider.PacketProof, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) NextSeqRecv(ctx context.Context, msgTransfer provider.PacketInfo, height uint64) (provider.PacketProof, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) MsgTransfer(dstAddr string, amount sdk.Coin, info provider.PacketInfo) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) MsgRecvPacket(msgTransfer provider.PacketInfo, proof provider.PacketProof) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) MsgAcknowledgement(msgRecvPacket provider.PacketInfo, proofAcked provider.PacketProof) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) MsgTimeout(msgTransfer provider.PacketInfo, proofUnreceived provider.PacketProof) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) MsgTimeoutOnClose(msgTransfer provider.PacketInfo, proofUnreceived provider.PacketProof) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) ConnectionHandshakeProof(ctx context.Context, msgOpenInit provider.ConnectionInfo, height uint64) (provider.ConnectionProof, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) ConnectionProof(ctx context.Context, msgOpenAck provider.ConnectionInfo, height uint64) (provider.ConnectionProof, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) MsgConnectionOpenInit(info provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) MsgConnectionOpenTry(msgOpenInit provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) MsgConnectionOpenAck(msgOpenTry provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) MsgConnectionOpenConfirm(msgOpenAck provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) ChannelProof(ctx context.Context, msg provider.ChannelInfo, height uint64) (provider.ChannelProof, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) MsgChannelOpenInit(info provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) MsgChannelOpenTry(msgOpenInit provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) MsgChannelOpenAck(msgOpenTry provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) MsgChannelOpenConfirm(msgOpenAck provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) MsgChannelCloseInit(info provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) MsgChannelCloseConfirm(msgCloseInit provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) MsgUpdateClientHeader(latestHeader provider.IBCHeader, trustedHeight clienttypes.Height, trustedHeader provider.IBCHeader) (ibcexported.Header, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) MsgUpdateClient(clientId string, counterpartyHeader ibcexported.Header) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) RelayPacketFromSequence(ctx context.Context, src provider.ChainProvider, srch, dsth, seq uint64, srcChanID, srcPortID string, order chantypes.Order) (provider.RelayerMessage, provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) AcknowledgementFromSequence(ctx context.Context, dst provider.ChainProvider, dsth, seq uint64, dstChanID, dstPortID, srcChanID, srcPortID string) (provider.RelayerMessage, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) SendMessage(ctx context.Context, msg provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) SendMessages(ctx context.Context, msgs []provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) ChainName() string {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) ChainId() string {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) Type() string {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) ProviderConfig() provider.ProviderConfig {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) Key() string {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) Address() (string, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) Timeout() string {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) TrustingPeriod(ctx context.Context) (time.Duration, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) WaitForNBlocks(ctx context.Context, n int64) error {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) Sprint(toPrint proto.Message) (string, error) {
	//TODO implement me
	panic("implement me")
}
