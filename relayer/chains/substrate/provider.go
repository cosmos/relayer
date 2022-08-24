package substrate

import (
	"context"
	"fmt"
	"io"
	"math"
	"path"
	"time"

	rpcclient "github.com/ComposableFi/go-substrate-rpc-client/v4"
	beefyclienttypes "github.com/ComposableFi/ics11-beefy/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
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

type SubstrateProvider struct {
	log     *zap.Logger
	Keybase keystore.Keyring
	PCfg    SubstrateProviderConfig
	Input   io.Reader

	RPCClient        *rpcclient.SubstrateAPI
	RelayerRPCClient *rpcclient.SubstrateAPI
}

func (sp *SubstrateProvider) Init() error {
	keybase, err := keystore.New(sp.PCfg.ChainID, sp.PCfg.KeyringBackend, sp.PCfg.KeyDirectory, sp.Input)
	if err != nil {
		return err
	}

	sp.Keybase = keybase
	return nil
}

type SubstrateIBCHeader struct {
	height       uint64
	SignedHeader *beefyclienttypes.Header
}

func (h SubstrateIBCHeader) Height() uint64 {
	return h.height
}

func (h SubstrateIBCHeader) ConsensusState() ibcexported.ConsensusState {
	return h.SignedHeader.ConsensusState()
}

func (sp *SubstrateProvider) BlockTime(ctx context.Context, height int64) (time.Time, error) {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) MsgCreateClient(clientState ibcexported.ClientState, consensusState ibcexported.ConsensusState) (provider.RelayerMessage, error) {
	signer, err := sp.Address()
	if err != nil {
		return nil, err
	}

	anyClientState, err := clienttypes.PackClientState(clientState)
	if err != nil {
		return nil, err
	}

	anyConsensusState, err := clienttypes.PackConsensusState(consensusState)
	if err != nil {
		return nil, err
	}

	msg := &clienttypes.MsgCreateClient{
		ClientState:    anyClientState,
		ConsensusState: anyConsensusState,
		Signer:         signer,
	}

	return NewSubstrateMessage(msg), nil
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

func (sp *SubstrateProvider) ChainName() string {
	return sp.PCfg.ChainName
}

func (sp *SubstrateProvider) ChainId() string {
	return sp.PCfg.ChainID
}

func (sp *SubstrateProvider) Type() string {
	return "substrate"
}

func (sp *SubstrateProvider) ProviderConfig() provider.ProviderConfig {
	return sp.PCfg
}

func (sp *SubstrateProvider) Key() string {
	return sp.PCfg.Key
}

func (sp *SubstrateProvider) Address() (string, error) {
	info, err := sp.Keybase.Key(sp.Key())
	if err != nil {
		return "", nil
	}

	return info.GetAddress(), nil
}

func (sp *SubstrateProvider) Timeout() string {
	return sp.PCfg.Timeout
}

func (sp *SubstrateProvider) TrustingPeriod(ctx context.Context) (time.Duration, error) {
	// TODO: implement a proper trusting period
	return time.Duration(math.MaxInt), nil
}

func (sp *SubstrateProvider) WaitForNBlocks(ctx context.Context, n int64) error {
	//TODO implement me
	panic("implement me")
}

func (sp *SubstrateProvider) Sprint(toPrint proto.Message) (string, error) {
	//TODO implement me
	panic("implement me")
}
