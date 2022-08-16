package substrate

import (
	"context"
	"time"

	"github.com/avast/retry-go/v4"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
	chantypes "github.com/cosmos/ibc-go/v5/modules/core/04-channel/types"
	commitmenttypes "github.com/cosmos/ibc-go/v5/modules/core/23-commitment/types"
	ibcexported "github.com/cosmos/ibc-go/v5/modules/core/exported"
	tmclient "github.com/cosmos/ibc-go/v5/modules/light-clients/07-tendermint/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
)

// Variables used for retries
var (
	rtyAttNum = uint(5)
	rtyAtt    = retry.Attempts(rtyAttNum)
	rtyDel    = retry.Delay(time.Millisecond * 400)
	rtyErr    = retry.LastErrorOnly(true)
)

// Default IBC settings
var (
	defaultChainPrefix = commitmenttypes.NewMerklePrefix([]byte("ibc"))
	defaultDelayPeriod = uint64(0)
)

// Strings for parsing events
var (
	spTag       = "send_packet"
	waTag       = "write_acknowledgement"
	srcChanTag  = "packet_src_channel"
	dstChanTag  = "packet_dst_channel"
	srcPortTag  = "packet_src_port"
	dstPortTag  = "packet_dst_port"
	dataTag     = "packet_data"
	ackTag      = "packet_ack"
	toHeightTag = "packet_timeout_height"
	toTSTag     = "packet_timeout_timestamp"
	seqTag      = "packet_sequence"
)

func (sp *SubstrateProvider) Init() error {
	return nil
}

// SendMessage attempts to sign, encode & send a RelayerMessage
// This is used extensively in the relayer as an extension of the Provider interface
func (cc *SubstrateProvider) SendMessage(ctx context.Context, msg provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {
	return cc.SendMessages(ctx, []provider.RelayerMessage{msg}, memo)
}

// SendMessages attempts to sign, encode, & send a slice of RelayerMessages
// This is used extensively in the relayer as an extension of the Provider interface
//
// NOTE: An error is returned if there was an issue sending the transaction. A successfully sent, but failed
// transaction will not return an error. If a transaction is successfully sent, the result of the execution
// of that transaction will be logged. A boolean indicating if a transaction was successfully
// sent and executed successfully is returned.
func (cc *SubstrateProvider) SendMessages(ctx context.Context, msgs []provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {
	return &provider.RelayerTxResponse{}, true, nil
}

func parseEventsFromTxResponse(resp *sdk.TxResponse) []provider.RelayerEvent {
	return []provider.RelayerEvent{}
}

func (cc *SubstrateProvider) buildMessages(ctx context.Context, msgs []provider.RelayerMessage, memo string) ([]byte, error) {
	return nil, nil
}

// MsgCreateClient creates an sdk.Msg to update the client on src with consensus state from dst
func (cc *SubstrateProvider) MsgCreateClient(
	clientState ibcexported.ClientState,
	consensusState ibcexported.ConsensusState,
) (provider.RelayerMessage, error) {
	return nil, nil
}

func (cc *SubstrateProvider) MsgUpdateClient(srcClientId string, dstHeader ibcexported.Header) (provider.RelayerMessage, error) {
	return nil, nil
}

func (cc *SubstrateProvider) MsgUpgradeClient(srcClientId string, consRes *clienttypes.QueryConsensusStateResponse, clientRes *clienttypes.QueryClientStateResponse) (provider.RelayerMessage, error) {
	return nil, nil
}

// mustGetHeight takes the height inteface and returns the actual height
func mustGetHeight(h ibcexported.Height) clienttypes.Height {
	return clienttypes.Height{}
}

// MsgTransfer creates a new transfer message
func (cc *SubstrateProvider) MsgTransfer(
	dstAddr string,
	amount sdk.Coin,
	info provider.PacketInfo,
) (provider.RelayerMessage, error) {
	return nil, nil
}

func (cc *SubstrateProvider) ValidatePacket(msgTransfer provider.PacketInfo, latest provider.LatestBlock) error {
	return nil
}

func (cc *SubstrateProvider) PacketCommitment(
	ctx context.Context,
	msgTransfer provider.PacketInfo,
	height uint64,
) (provider.PacketProof, error) {
	return provider.PacketProof{}, nil
}

func (cc *SubstrateProvider) MsgRecvPacket(
	msgTransfer provider.PacketInfo,
	proof provider.PacketProof,
) (provider.RelayerMessage, error) {
	return nil, nil
}

func (cc *SubstrateProvider) PacketAcknowledgement(
	ctx context.Context,
	msgRecvPacket provider.PacketInfo,
	height uint64,
) (provider.PacketProof, error) {
	return provider.PacketProof{}, nil
}

func (cc *SubstrateProvider) MsgAcknowledgement(
	msgRecvPacket provider.PacketInfo,
	proof provider.PacketProof,
) (provider.RelayerMessage, error) {
	return nil, nil
}

func (cc *SubstrateProvider) PacketReceipt(
	ctx context.Context,
	msgTransfer provider.PacketInfo,
	height uint64,
) (provider.PacketProof, error) {
	return provider.PacketProof{}, nil
}

// NextSeqRecv queries for the appropriate Tendermint proof required to prove the next expected packet sequence number
// for a given counterparty channel. This is used in ORDERED channels to ensure packets are being delivered in the
// exact same order as they were sent over the wire.
func (cc *SubstrateProvider) NextSeqRecv(
	ctx context.Context,
	msgTransfer provider.PacketInfo,
	height uint64,
) (provider.PacketProof, error) {
	return provider.PacketProof{}, nil
}

func (cc *SubstrateProvider) MsgTimeout(msgTransfer provider.PacketInfo, proof provider.PacketProof) (provider.RelayerMessage, error) {
	return nil, nil
}

func (cc *SubstrateProvider) MsgTimeoutOnClose(msgTransfer provider.PacketInfo, proof provider.PacketProof) (provider.RelayerMessage, error) {
	return nil, nil
}

func (cc *SubstrateProvider) MsgConnectionOpenInit(info provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	return nil, nil
}

func (cc *SubstrateProvider) ConnectionHandshakeProof(
	ctx context.Context,
	msgOpenInit provider.ConnectionInfo,
	height uint64,
) (provider.ConnectionProof, error) {
	return provider.ConnectionProof{}, nil
}

func (cc *SubstrateProvider) MsgConnectionOpenTry(msgOpenInit provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	return nil, nil
}

func (cc *SubstrateProvider) MsgConnectionOpenAck(msgOpenTry provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	return nil, nil
}

func (cc *SubstrateProvider) ConnectionProof(
	ctx context.Context,
	msgOpenAck provider.ConnectionInfo,
	height uint64,
) (provider.ConnectionProof, error) {
	return provider.ConnectionProof{}, nil
}

func (cc *SubstrateProvider) MsgConnectionOpenConfirm(msgOpenAck provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	return nil, nil
}

func (cc *SubstrateProvider) MsgChannelOpenInit(info provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	return nil, nil
}

func (cc *SubstrateProvider) ChannelProof(
	ctx context.Context,
	msg provider.ChannelInfo,
	height uint64,
) (provider.ChannelProof, error) {
	return provider.ChannelProof{}, nil
}

func (cc *SubstrateProvider) MsgChannelOpenTry(msgOpenInit provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	return nil, nil
}

func (cc *SubstrateProvider) MsgChannelOpenAck(msgOpenTry provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	return nil, nil
}

func (cc *SubstrateProvider) MsgChannelOpenConfirm(msgOpenAck provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	return nil, nil
}

func (cc *SubstrateProvider) MsgChannelCloseInit(info provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	return nil, nil
}

func (cc *SubstrateProvider) MsgChannelCloseConfirm(msgCloseInit provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	return nil, nil
}

func (cc *SubstrateProvider) MsgUpdateClientHeader(latestHeader provider.IBCHeader, trustedHeight clienttypes.Height, trustedHeader provider.IBCHeader) (ibcexported.Header, error) {
	return &tmclient.Header{}, nil
}

// RelayPacketFromSequence relays a packet with a given seq on src and returns recvPacket msgs, timeoutPacketmsgs and error
func (cc *SubstrateProvider) RelayPacketFromSequence(
	ctx context.Context,
	src provider.ChainProvider,
	srch, dsth, seq uint64,
	srcChanID, srcPortID string,
	order chantypes.Order,
) (provider.RelayerMessage, provider.RelayerMessage, error) {
	return nil, nil, nil
}

// AcknowledgementFromSequence relays an acknowledgement with a given seq on src, source is the sending chain, destination is the receiving chain
func (cc *SubstrateProvider) AcknowledgementFromSequence(ctx context.Context, dst provider.ChainProvider, dsth, seq uint64, dstChanId, dstPortId, srcChanId, srcPortId string) (provider.RelayerMessage, error) {
	return nil, nil
}

// QueryIBCHeader returns the IBC compatible block header (CosmosIBCHeader) at a specific height.
func (cc *SubstrateProvider) QueryIBCHeader(ctx context.Context, h int64) (provider.IBCHeader, error) {
	return nil, nil
}

// InjectTrustedFields injects the necessary trusted fields for a header to update a light
// client stored on the destination chain, using the information provided by the source
// chain.
// TrustedHeight is the latest height of the IBC client on dst
// TrustedValidators is the validator set of srcChain at the TrustedHeight
// InjectTrustedFields returns a copy of the header with TrustedFields modified
func (cc *SubstrateProvider) InjectTrustedFields(ctx context.Context, header ibcexported.Header, dst provider.ChainProvider, dstClientId string) (ibcexported.Header, error) {
	return nil, nil
}

// queryTMClientState retrieves the latest consensus state for a client in state at a given height
// and unpacks/cast it to tendermint clientstate
func (cc *SubstrateProvider) queryTMClientState(ctx context.Context, srch int64, srcClientId string) (*tmclient.ClientState, error) {
	return &tmclient.ClientState{}, nil
}

// castClientStateToTMType casts client state to tendermint type
func castClientStateToTMType(cs *codectypes.Any) (*tmclient.ClientState, error) {
	return &tmclient.ClientState{}, nil
}

// DefaultUpgradePath is the default IBC upgrade path set for an on-chain light client
var defaultUpgradePath = []string{"upgrade", "upgradedIBCState"}

// NewClientState creates a new tendermint client state tracking the dst chain.
func (cc *SubstrateProvider) NewClientState(
	dstChainID string,
	dstUpdateHeader provider.IBCHeader,
	dstTrustingPeriod,
	dstUbdPeriod time.Duration,
	allowUpdateAfterExpiry,
	allowUpdateAfterMisbehaviour bool,
) (ibcexported.ClientState, error) {
	// Create the ClientState we want on 'c' tracking 'dst'
	return &tmclient.ClientState{}, nil
}
