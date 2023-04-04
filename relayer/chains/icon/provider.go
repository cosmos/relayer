package icon

import (
	"context"
	"encoding/base64"
	"fmt"
	"math/big"
	"os"
	"sync"
	"time"

	"github.com/cosmos/gogoproto/proto"
	"github.com/cosmos/relayer/v2/relayer/chains/icon/types"
	"github.com/cosmos/relayer/v2/relayer/processor"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/icon-project/goloop/common/wallet"
	"github.com/icon-project/goloop/module"
	"go.uber.org/zap"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	commitmenttypes "github.com/cosmos/ibc-go/v7/modules/core/23-commitment/types"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/exported"
	tendermint "github.com/cosmos/ibc-go/v7/modules/light-clients/07-tendermint"
)

var (
	_ provider.ChainProvider  = &IconProvider{}
	_ provider.KeyProvider    = &IconProvider{}
	_ provider.ProviderConfig = &IconProviderConfig{}
)

// Default IBC settings
var (
	defaultChainPrefix = types.NewMerklePrefix([]byte("ibc"))
	defaultDelayPeriod = big.NewInt(0)

	DefaultIBCVersionIdentifier = "1"

	DefaultIBCVersion = types.Version{
		Identifier: DefaultIBCVersionIdentifier,
		Features:   []string{"ORDER_ORDERED", "ORDER_UNORDERED"},
	}
)

type IconProviderConfig struct {
	Key               string `json:"key" yaml:"key"`
	ChainName         string `json:"-" yaml:"-"`
	ChainID           string `json:"chain-id" yaml:"chain-id"`
	RPCAddr           string `json:"rpc-addr" yaml:"rpc-addr"`
	Timeout           string `json:"timeout" yaml:"timeout"`
	Keystore          string `json:"keystore" yaml:"keystore"`
	Password          string `json:"password" yaml:"password"`
	ICONNetworkID     int64  `json:"icon-network-id" yaml:"icon-network-id" default:"3"`
	BTPNetworkID      int64  `json:"btp-network-id" yaml:"btp-network-id"`
	IbcHandlerAddress string `json:"ibc-handler-address"`
}

func (pp IconProviderConfig) Validate() error {
	if _, err := time.ParseDuration(pp.Timeout); err != nil {
		return fmt.Errorf("invalid Timeout: %w", err)
	}
	return nil
}

func (pp IconProviderConfig) BroadcastMode() provider.BroadcastMode {
	return ""
}

// NewProvider should provide a new Icon provider
// NewProvider should provide a new Icon provider
func (pp IconProviderConfig) NewProvider(log *zap.Logger, homepath string, debug bool, chainName string) (provider.ChainProvider, error) {

	pp.ChainName = chainName
	if _, err := os.Stat(pp.Keystore); err != nil {
		return nil, err
	}

	if err := pp.Validate(); err != nil {
		return nil, err
	}

	ksByte, err := os.ReadFile(pp.Keystore)
	if err != nil {
		return nil, err
	}

	wallet, err := wallet.NewFromKeyStore(ksByte, []byte(pp.Password))
	if err != nil {
		return nil, err
	}

	return &IconProvider{
		log:    log.With(zap.String("sys", "chain_client")),
		client: NewClient(pp.getRPCAddr(), log),
		PCfg:   pp,
		wallet: wallet,
	}, nil
}

func (icp *IconProvider) AddWallet(w module.Wallet) {
	icp.wallet = w
}

func (pp IconProviderConfig) getRPCAddr() string {
	return pp.RPCAddr
}

type IconProvider struct {
	log     *zap.Logger
	PCfg    IconProviderConfig
	txMu    sync.Mutex
	client  *Client
	wallet  module.Wallet
	metrics *processor.PrometheusMetrics
	codec   codec.ProtoCodecMarshaler
}

type SignedHeader struct {
	header     types.BTPBlockHeader
	signatures []types.HexBytes
}

type ValidatorSet struct {
	validators []types.HexBytes
}

type IconIBCHeader struct {
	Header *types.BTPBlockHeader
	// Proof  types.HexBytes
}

func NewIconIBCHeader(header *types.BTPBlockHeader) *IconIBCHeader {
	return &IconIBCHeader{
		Header: header,
		// Proof:  proof,
	}
}

func (h IconIBCHeader) Height() uint64 {
	return uint64(h.Header.MainHeight)
}

func (h IconIBCHeader) ConsensusState() ibcexported.ConsensusState {
	return nil
}

// TODO:
func (h IconIBCHeader) NextValidatorsHash() []byte {
	return nil
}

func (h *IconIBCHeader) Reset()         { *h = IconIBCHeader{} }
func (h *IconIBCHeader) String() string { return proto.CompactTextString(h) }
func (*IconIBCHeader) ProtoMessage()    {}

//ChainProvider Methods

func (icp *IconProvider) Init(ctx context.Context) error {

	return nil
}

func (icp *IconProvider) Codec() codec.ProtoCodecMarshaler {
	return icp.codec
}

func (icp *IconProvider) NewClientState(
	dstChainID string,
	dstUpdateHeader provider.IBCHeader,
	dstTrustingPeriod,
	dstUbdPeriod time.Duration,
	allowUpdateAfterExpiry,
	allowUpdateAfterMisbehaviour bool,
) (ibcexported.ClientState, error) {
	return &tendermint.ClientState{}, fmt.Errorf("Not implemented for ICON. Use NewClientStateIcon instead.")

}

func (icp *IconProvider) MsgCreateClient(clientState ibcexported.ClientState, consensusState ibcexported.ConsensusState) (provider.RelayerMessage, error) {

	anyClientState, err := clienttypes.PackClientState(clientState)
	if err != nil {
		return nil, err
	}

	anyConsensusState, err := clienttypes.PackConsensusState(consensusState)
	if err != nil {
		return nil, err
	}

	clS := &types.MsgCreateClient{
		ClientState:    anyClientState.Value,
		ConsensusState: anyConsensusState.Value,
		ClientType:     clientState.ClientType(),
	}

	return NewIconMessage(clS, MethodCreateClient), nil
}

func (icp *IconProvider) MsgUpgradeClient(srcClientId string, consRes *clienttypes.QueryConsensusStateResponse, clientRes *clienttypes.QueryClientStateResponse) (provider.RelayerMessage, error) {

	clU := &types.MsgUpdateClient{
		ClientId:      srcClientId,
		ClientMessage: nil,
	}

	return NewIconMessage(clU, MethodUpdateClient), nil
}

func (icp *IconProvider) MsgSubmitMisbehaviour(clientID string, misbehaviour ibcexported.ClientMessage) (provider.RelayerMessage, error) {
	return nil, nil
}

func (icp *IconProvider) ValidatePacket(msgTransfer provider.PacketInfo, latestBlock provider.LatestBlock) error {
	if msgTransfer.Sequence <= 0 {
		return fmt.Errorf("Refuse to relay packet with sequence 0")
	}
	if len(msgTransfer.Data) == 0 {
		return fmt.Errorf("Refuse to relay packet with empty data")
	}
	// This should not be possible, as it violates IBC spec
	if msgTransfer.TimeoutHeight.IsZero() && msgTransfer.TimeoutTimestamp == 0 {
		return fmt.Errorf("refusing to relay packet without a timeout (height or timestamp must be set)")
	}

	revision := clienttypes.ParseChainID(icp.PCfg.ChainID)
	latestClientTypesHeight := clienttypes.NewHeight(revision, latestBlock.Height)
	if !msgTransfer.TimeoutHeight.IsZero() && latestClientTypesHeight.GTE(msgTransfer.TimeoutHeight) {
		return provider.NewTimeoutHeightError(latestBlock.Height, msgTransfer.TimeoutHeight.RevisionHeight)
	}
	latestTimestamp := uint64(latestBlock.Time.UnixNano())
	if msgTransfer.TimeoutTimestamp > 0 && latestTimestamp > msgTransfer.TimeoutTimestamp {
		return provider.NewTimeoutTimestampError(latestTimestamp, msgTransfer.TimeoutTimestamp)
	}

	return nil
}

func (icp *IconProvider) PacketCommitment(ctx context.Context, msgTransfer provider.PacketInfo, height uint64) (provider.PacketProof, error) {
	packetCommitmentResponse, err := icp.QueryPacketCommitment(
		ctx, int64(height), msgTransfer.SourceChannel, msgTransfer.SourcePort, msgTransfer.Sequence,
	)

	if err != nil {
		return provider.PacketProof{}, nil
	}
	return provider.PacketProof{
		Proof:       packetCommitmentResponse.Proof,
		ProofHeight: packetCommitmentResponse.ProofHeight,
	}, nil
}

func (icp *IconProvider) PacketAcknowledgement(ctx context.Context, msgRecvPacket provider.PacketInfo, height uint64) (provider.PacketProof, error) {
	packetAckResponse, err := icp.QueryPacketAcknowledgement(ctx, int64(height), msgRecvPacket.SourceChannel, msgRecvPacket.SourcePort, msgRecvPacket.Sequence)
	if err != nil {
		return provider.PacketProof{}, nil
	}
	return provider.PacketProof{
		Proof:       packetAckResponse.Proof,
		ProofHeight: packetAckResponse.GetProofHeight(),
	}, nil

}

func (icp *IconProvider) PacketReceipt(ctx context.Context, msgTransfer provider.PacketInfo, height uint64) (provider.PacketProof, error) {
	packetReceiptResponse, err := icp.QueryPacketReceipt(ctx, int64(height), msgTransfer.SourceChannel, msgTransfer.SourcePort, msgTransfer.Sequence)

	if err != nil {
		return provider.PacketProof{}, nil
	}
	return provider.PacketProof{
		Proof:       packetReceiptResponse.Proof,
		ProofHeight: packetReceiptResponse.ProofHeight,
	}, nil

}

func (icp *IconProvider) NextSeqRecv(ctx context.Context, msgTransfer provider.PacketInfo, height uint64) (provider.PacketProof, error) {
	nextSeqRecvResponse, err := icp.QueryNextSeqRecv(ctx, int64(height), msgTransfer.DestChannel, msgTransfer.DestPort)
	if err != nil {
		return provider.PacketProof{}, nil
	}
	return provider.PacketProof{
		Proof:       nextSeqRecvResponse.Proof,
		ProofHeight: nextSeqRecvResponse.ProofHeight,
	}, nil

}

func (icp *IconProvider) MsgTransfer(dstAddr string, amount sdk.Coin, info provider.PacketInfo) (provider.RelayerMessage, error) {
	return nil, fmt.Errorf("Method not supported on ICON")
}

func (icp *IconProvider) MsgRecvPacket(msgTransfer provider.PacketInfo, proof provider.PacketProof) (provider.RelayerMessage, error) {
	recvPacket := &types.MsgPacketRecv{
		Packet: types.Packet{
			Sequence:           *big.NewInt(int64(msgTransfer.Sequence)),
			SourcePort:         msgTransfer.SourcePort,
			SourceChannel:      msgTransfer.SourceChannel,
			DestinationPort:    msgTransfer.DestPort,
			DestinationChannel: msgTransfer.DestChannel,
			TimeoutHeight: types.Height{
				RevisionNumber: *big.NewInt(int64(msgTransfer.TimeoutHeight.RevisionNumber)),
				RevisionHeight: *big.NewInt(int64(msgTransfer.TimeoutHeight.RevisionHeight)),
			},
			Timestamp: *big.NewInt(int64(msgTransfer.TimeoutTimestamp)),
		},
		Proof: proof.Proof,
		ProofHeight: types.Height{
			RevisionNumber: *big.NewInt(int64(proof.ProofHeight.RevisionNumber)),
			RevisionHeight: *big.NewInt(int64(proof.ProofHeight.RevisionHeight)),
		},
	}
	return NewIconMessage(recvPacket, MethodRecvPacket), nil
}

func (icp *IconProvider) MsgAcknowledgement(msgRecvPacket provider.PacketInfo, proofAcked provider.PacketProof) (provider.RelayerMessage, error) {
	msg := &types.MsgPacketAcknowledgement{
		Packet: types.Packet{
			Sequence:           *big.NewInt(int64(msgRecvPacket.Sequence)),
			SourcePort:         msgRecvPacket.SourcePort,
			SourceChannel:      msgRecvPacket.SourceChannel,
			DestinationPort:    msgRecvPacket.DestPort,
			DestinationChannel: msgRecvPacket.DestChannel,
			TimeoutHeight: types.Height{
				RevisionNumber: *big.NewInt(int64(msgRecvPacket.TimeoutHeight.RevisionNumber)),
				RevisionHeight: *big.NewInt(int64(msgRecvPacket.TimeoutHeight.RevisionHeight)),
			},
			Timestamp: *big.NewInt(int64(msgRecvPacket.TimeoutTimestamp)),
		},
		Acknowledgement: msgRecvPacket.Ack,
		Proof:           proofAcked.Proof,
		ProofHeight: types.Height{
			RevisionNumber: *big.NewInt(int64(proofAcked.ProofHeight.RevisionNumber)),
			RevisionHeight: *big.NewInt(int64(proofAcked.ProofHeight.RevisionHeight)),
		},
	}
	return NewIconMessage(msg, MethodWriteAck), nil
}

func (icp *IconProvider) MsgTimeout(msgTransfer provider.PacketInfo, proofUnreceived provider.PacketProof) (provider.RelayerMessage, error) {
	return nil, fmt.Errorf("Not implemented on icon")
}

func (icp *IconProvider) MsgTimeoutOnClose(msgTransfer provider.PacketInfo, proofUnreceived provider.PacketProof) (provider.RelayerMessage, error) {
	return nil, fmt.Errorf("Not implemented on icon")
}

func (icp *IconProvider) ConnectionHandshakeProof(ctx context.Context, msgOpenInit provider.ConnectionInfo, height uint64) (provider.ConnectionProof, error) {

	clientState, clientStateProof, consensusStateProof, connStateProof, proofHeight, err := icp.GenerateConnHandshakeProof(ctx, int64(height), msgOpenInit.ClientID, msgOpenInit.ConnID)
	if err != nil {
		return provider.ConnectionProof{}, err
	}
	if len(connStateProof) == 0 {
		return provider.ConnectionProof{}, fmt.Errorf("Received invalid zero length connection state proof")
	}
	return provider.ConnectionProof{
		ClientState:          clientState,
		ClientStateProof:     clientStateProof,
		ConsensusStateProof:  consensusStateProof,
		ConnectionStateProof: connStateProof,
		ProofHeight:          proofHeight.(clienttypes.Height),
	}, nil

}

func (icp *IconProvider) ConnectionProof(ctx context.Context, msgOpenAck provider.ConnectionInfo, height uint64) (provider.ConnectionProof, error) {
	connState, err := icp.QueryConnection(ctx, int64(height), msgOpenAck.ConnID)
	if err != nil {
		return provider.ConnectionProof{}, err
	}
	return provider.ConnectionProof{
		ConnectionStateProof: connState.Proof,
		ProofHeight:          connState.ProofHeight,
	}, nil
}

func (icp *IconProvider) MsgConnectionOpenInit(info provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	msg := &types.MsgConnectionOpenInit{
		ClientId: info.ClientID,
		Counterparty: types.ConnectionCounterparty{
			ClientId:     info.CounterpartyClientID,
			ConnectionId: info.CounterpartyConnID,
		},
		DelayPeriod: *defaultDelayPeriod,
	}
	return NewIconMessage(msg, MethodConnectionOpenInit), nil
}

func (icp *IconProvider) MsgConnectionOpenTry(msgOpenInit provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	counterparty := &types.ConnectionCounterparty{
		ClientId:     msgOpenInit.ClientID,
		ConnectionId: msgOpenInit.ConnID,
		Prefix:       defaultChainPrefix,
	}

	csAny, err := clienttypes.PackClientState(proof.ClientState)
	if err != nil {
		return nil, err
	}

	msg := &types.MsgConnectionOpenTry{
		ClientId:             msgOpenInit.CounterpartyClientID,
		PreviousConnectionId: msgOpenInit.CounterpartyConnID,
		ClientStateBytes:     csAny.Value,
		Counterparty:         *counterparty,
		DelayPeriod:          *defaultDelayPeriod,
		CounterpartyVersions: []types.Version{DefaultIBCVersion},
		ProofInit:            proof.ConnectionStateProof,
		ProofHeight: types.Height{
			RevisionNumber: *big.NewInt(int64(proof.ProofHeight.RevisionNumber)),
			RevisionHeight: *big.NewInt(int64(proof.ProofHeight.RevisionHeight)),
		},
		ProofClient:    proof.ClientStateProof,
		ProofConsensus: proof.ConsensusStateProof,
		ConsensusHeight: types.Height{
			RevisionNumber: *big.NewInt(int64(proof.ClientState.GetLatestHeight().GetRevisionNumber())),
			RevisionHeight: *big.NewInt(int64(proof.ClientState.GetLatestHeight().GetRevisionHeight())),
		},
	}
	return NewIconMessage(msg, MethodConnectionOpenTry), nil
}

func (icp *IconProvider) MsgConnectionOpenAck(msgOpenTry provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {

	csAny, err := clienttypes.PackClientState(proof.ClientState)
	if err != nil {
		return nil, err
	}

	msg := &types.MsgConnectionOpenAck{
		ConnectionId:             msgOpenTry.CounterpartyConnID,
		ClientStateBytes:         csAny.GetValue(), // TODO
		Version:                  DefaultIBCVersion,
		CounterpartyConnectionID: msgOpenTry.ConnID,
		ProofTry:                 proof.ConnectionStateProof,
		ProofClient:              proof.ClientStateProof,
		ProofConsensus:           proof.ConsensusStateProof,
		ProofHeight: types.Height{
			RevisionNumber: *big.NewInt(int64(proof.ProofHeight.RevisionNumber)),
			RevisionHeight: *big.NewInt(int64(proof.ProofHeight.RevisionHeight)),
		},
		ConsensusHeight: types.Height{
			RevisionNumber: *big.NewInt(int64(proof.ClientState.GetLatestHeight().GetRevisionNumber())),
			RevisionHeight: *big.NewInt(int64(proof.ClientState.GetLatestHeight().GetRevisionHeight())),
		},
	}
	return NewIconMessage(msg, MethodConnectionOpenAck), nil
}

func (icp *IconProvider) MsgConnectionOpenConfirm(msgOpenAck provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	msg := &types.MsgConnectionOpenConfirm{
		ConnectionId: msgOpenAck.CounterpartyConnID,
		ProofAck:     proof.ConnectionStateProof,
		ProofHeight: types.Height{
			RevisionNumber: *big.NewInt(int64(proof.ProofHeight.RevisionNumber)),
			RevisionHeight: *big.NewInt(int64(proof.ProofHeight.RevisionHeight)),
		},
	}
	return NewIconMessage(msg, MethodConnectionOpenConfirm), nil
}

func (icp *IconProvider) ChannelProof(ctx context.Context, msg provider.ChannelInfo, height uint64) (provider.ChannelProof, error) {
	channelResult, err := icp.QueryChannel(ctx, int64(height), msg.ChannelID, msg.PortID)
	if err != nil {
		return provider.ChannelProof{}, nil
	}
	// TODO
	return provider.ChannelProof{
		Proof: make([]byte, 0),
		ProofHeight: clienttypes.Height{
			RevisionNumber: 0,
			RevisionHeight: 0,
		},
		Ordering: chantypes.Order(channelResult.Channel.GetOrdering()),
		Version:  channelResult.Channel.Version,
	}, nil
}

func (icp *IconProvider) MsgChannelOpenInit(info provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	msg := &types.MsgChannelOpenInit{
		PortId: info.PortID,
		Channel: types.Channel{
			State:    chantypes.UNINITIALIZED,
			Ordering: info.Order,
			Counterparty: types.ChannelCounterparty{
				PortId:    info.CounterpartyPortID,
				ChannelId: "",
			},
			ConnectionHops: []string{info.ConnID},
			Version:        info.Version,
		},
	}
	return NewIconMessage(msg, MethodChannelOpenInit), nil
}

func (icp *IconProvider) MsgChannelOpenTry(msgOpenInit provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	msg := &types.MsgChannelOpenTry{
		PortId:            msgOpenInit.CounterpartyPortID,
		PreviousChannelId: msgOpenInit.CounterpartyChannelID,
		Channel: types.Channel{
			State:    chantypes.TRYOPEN,
			Ordering: proof.Ordering,
			Counterparty: types.ChannelCounterparty{
				PortId:    msgOpenInit.PortID,
				ChannelId: msgOpenInit.ChannelID,
			},
			ConnectionHops: []string{msgOpenInit.CounterpartyConnID},
			Version:        proof.Version,
		},
		CounterpartyVersion: proof.Version,
		ProofInit:           proof.Proof,
		ProofHeight: types.Height{
			RevisionNumber: *big.NewInt(int64(proof.ProofHeight.RevisionNumber)),
			RevisionHeight: *big.NewInt(int64(proof.ProofHeight.RevisionHeight)),
		},
	}
	return NewIconMessage(msg, MethodChannelOpenTry), nil
}

func (icp *IconProvider) MsgChannelOpenAck(msgOpenTry provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	msg := &types.MsgChannelOpenAck{
		PortId:                msgOpenTry.CounterpartyPortID,
		ChannelId:             msgOpenTry.CounterpartyChannelID,
		CounterpartyVersion:   proof.Version,
		CounterpartyChannelId: msgOpenTry.ChannelID,
		ProofTry:              proof.Proof,
		ProofHeight: types.Height{
			RevisionNumber: *big.NewInt(int64(proof.ProofHeight.RevisionNumber)),
			RevisionHeight: *big.NewInt(int64(proof.ProofHeight.RevisionHeight)),
		},
	}
	return NewIconMessage(msg, MethodChannelOpenAck), nil
}

func (icp *IconProvider) MsgChannelOpenConfirm(msgOpenAck provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	msg := &types.MsgChannelOpenConfirm{
		PortId:    msgOpenAck.CounterpartyPortID,
		ChannelId: msgOpenAck.CounterpartyChannelID,
		ProofAck:  proof.Proof,
		ProofHeight: types.Height{
			RevisionNumber: *big.NewInt(int64(proof.ProofHeight.RevisionNumber)),
			RevisionHeight: *big.NewInt(int64(proof.ProofHeight.RevisionHeight)),
		},
	}
	return NewIconMessage(msg, MethodChannelOpenConfirm), nil
}

func (icp *IconProvider) MsgChannelCloseInit(info provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	msg := &types.MsgChannelCloseInit{
		PortId:    info.PortID,
		ChannelId: info.ChannelID,
	}
	return NewIconMessage(msg, MethodChannelCloseInit), nil
}

func (icp *IconProvider) MsgChannelCloseConfirm(msgCloseInit provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	msg := &types.MsgChannelCloseConfirm{
		PortId:    msgCloseInit.CounterpartyPortID,
		ChannelId: msgCloseInit.CounterpartyChannelID,
		ProofInit: proof.Proof,
		ProofHeight: types.Height{
			RevisionNumber: *big.NewInt(int64(proof.ProofHeight.RevisionNumber)),
			RevisionHeight: *big.NewInt(int64(proof.ProofHeight.RevisionHeight)),
		},
	}
	return NewIconMessage(msg, MethodChannelCloseConfirm), nil
}

func (icp *IconProvider) MsgUpdateClientHeader(latestHeader provider.IBCHeader, trustedHeight clienttypes.Height, trustedHeader provider.IBCHeader) (ibcexported.ClientMessage, error) {
	// trustedIconHeader, ok := trustedHeader.(IconIBCHeader)
	// if !ok {
	// 	return nil, fmt.Errorf("Unsupported IBC trusted header type. Expected: IconIBCHeader,actual: %T", trustedHeader)
	// }
	// latestIconHeader, ok := latestHeader.(IconIBCHeader)
	// if !ok {
	// 	return nil, fmt.Errorf("Unsupported IBC trusted header type. Expected: IconIBCHeader,actual: %T", trustedHeader)
	// }

	// TODO: implementation remaining
	return nil, nil
	// return &IconIBCHeader{
	// 	header: latestIconHeader.header,
	// 	trustedHeight: types.Height{
	// 		RevisionNumber: *big.NewInt(int64(trustedHeight.RevisionNumber)),
	// 		RevisionHeight: *big.NewInt(int64(trustedHeight.RevisionHeight)),
	// 	},
	// 	trustedValidators: trustedIconHeader.trustedValidators,
	// }, nil

}

func (icp *IconProvider) MsgUpdateClient(clientID string, counterpartyHeader ibcexported.ClientMessage) (provider.RelayerMessage, error) {
	clientMsg, err := clienttypes.PackClientMessage(counterpartyHeader)
	if err != nil {
		return nil, err
	}
	msg := &types.MsgUpdateClient{
		ClientId:      clientID,
		ClientMessage: clientMsg.GetValue(),
	}
	return NewIconMessage(msg, MethodUpdateClient), nil
}

func (icp *IconProvider) QueryICQWithProof(ctx context.Context, msgType string, request []byte, height uint64) (provider.ICQProof, error) {
	return provider.ICQProof{}, nil
}

func (icp *IconProvider) MsgSubmitQueryResponse(chainID string, queryID provider.ClientICQQueryID, proof provider.ICQProof) (provider.RelayerMessage, error) {
	return nil, nil
}

func (icp *IconProvider) RelayPacketFromSequence(ctx context.Context, src provider.ChainProvider, srch, dsth, seq uint64, srcChanID, srcPortID string, order chantypes.Order) (provider.RelayerMessage, provider.RelayerMessage, error) {
	msg, err := src.QuerySendPacket(ctx, srcChanID, srcPortID, seq)
	if err != nil {
		return nil, nil, err
	}
	dstTime, err := icp.BlockTime(ctx, int64(dsth))
	if err != nil {
		return nil, nil, err
	}

	if err := icp.ValidatePacket(msg, provider.LatestBlock{
		Height: dsth,
		Time:   dstTime,
	}); err != nil {
		// TODO: handle
	}

	return nil, nil, nil
}

func (icp *IconProvider) AcknowledgementFromSequence(ctx context.Context, dst provider.ChainProvider, dsth, seq uint64, dstChanID, dstPortID, srcChanID, srcPortID string) (provider.RelayerMessage, error) {
	msgRecvPacket, err := dst.QueryRecvPacket(ctx, dst.ChainId(), dstPortID, seq)
	if err != nil {
		return nil, err
	}
	pp, err := dst.PacketAcknowledgement(ctx, msgRecvPacket, dsth)
	if err != nil {
		return nil, err
	}
	msg, err := icp.MsgAcknowledgement(msgRecvPacket, pp)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (icp *IconProvider) SendMessageIcon(ctx context.Context, msg provider.RelayerMessage) (*types.TransactionResult, bool, error) {
	m := msg.(*IconMessage)
	txParam := &types.TransactionParam{
		Version:     types.NewHexInt(types.JsonrpcApiVersion),
		FromAddress: types.Address(icp.wallet.Address().String()),
		ToAddress:   types.Address("IBC Handler"),
		NetworkID:   types.HexInt(icp.ChainId()),
		StepLimit:   types.NewHexInt(int64(defaultStepLimit)),
		DataType:    "call",
		Data: &IconMessage{
			Method: m.Method,
			Params: m.Params,
		},
	}
	if err := icp.client.SignTransaction(icp.wallet, txParam); err != nil {
		return nil, false, err
	}
	txHash, err := icp.client.SendTransaction(txParam)
	if txHash != nil {
		txH := &types.TransactionHashParam{
			Hash: *txHash,
		}
		txResult, e := icp.client.GetTransactionResult(txH)
		if e != nil {
			return nil, true, err
		}
		return txResult, true, nil
	}
	return nil, false, err
}

func (icp *IconProvider) SendMessage(ctx context.Context, msg provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {
	// return icp.SendMessages(ctx, []provider.RelayerMessage{msg}, memo)

	return nil, false, fmt.Errorf("Not implemented for ICON")
}

func (icp *IconProvider) SendMessages(ctx context.Context, msgs []provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {
	return nil, false, fmt.Errorf("Not implemented for ICON")

	//a transaction should be implemented
}

func (icp *IconProvider) ChainName() string {
	return icp.PCfg.ChainName
}

func (icp *IconProvider) ChainId() string {
	return icp.PCfg.ChainID
}

func (icp *IconProvider) Type() string {
	return "icon"
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
	return icp.wallet.Address().String(), nil
}

func (icp *IconProvider) Timeout() string {
	return icp.PCfg.Timeout
}

func (icp *IconProvider) TrustingPeriod(ctx context.Context) (time.Duration, error) {
	return 0, nil
}

// not required initially
func (icp *IconProvider) WaitForNBlocks(ctx context.Context, n int64) error {
	return nil
}

func (icp *IconProvider) Sprint(toPrint proto.Message) (string, error) {
	return "", nil
}

func (icp *IconProvider) GetBtpMessage(height int64) ([][]byte, error) {
	pr := types.BTPBlockParam{
		Height:    types.NewHexInt(height),
		NetworkId: types.NewHexInt(icp.PCfg.BTPNetworkID),
	}
	msgs, err := icp.client.GetBTPMessage(&pr)
	if err != nil {
		return nil, err
	}

	results := make([][]byte, 0)
	for _, mg := range msgs {
		m, err := base64.StdEncoding.DecodeString(mg)
		if err != nil {
			fmt.Println(err)
		}
		results = append(results, m)
	}
	return results, nil
}

// TODO:
func (icp *IconProvider) SendMessagesToMempool(
	ctx context.Context,
	msgs []provider.RelayerMessage,
	memo string,

	asyncCtx context.Context,
	asyncCallback func(*provider.RelayerTxResponse, error),
) error {
	return nil
}
