package archway

import (
	"context"
	"time"

	commitmenttypes "github.com/cosmos/ibc-go/v7/modules/core/23-commitment/types"
	"github.com/cosmos/relayer/v2/relayer/provider"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/exported"
)

// Default IBC settings
var (
	defaultChainPrefix = commitmenttypes.NewMerklePrefix([]byte("ibc"))
	defaultDelayPeriod = uint64(0)
)

func (ap *ArchwayProvider) NewClientState(dstChainID string, dstIBCHeader provider.IBCHeader, dstTrustingPeriod, dstUbdPeriod time.Duration, allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour bool) (ibcexported.ClientState, error) {
	return nil, nil
}
func (ap *ArchwayProvider) NewClientStateMock(dstChainID string, dstIBCHeader provider.IBCHeader, dstTrustingPeriod, dstUbdPeriod time.Duration, allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour bool) (ibcexported.ClientState, error) {
	return nil, nil
}
func (ap *ArchwayProvider) MsgCreateClient(clientState ibcexported.ClientState, consensusState ibcexported.ConsensusState) (provider.RelayerMessage, error) {
	return nil, nil
}
func (ap *ArchwayProvider) MsgUpgradeClient(srcClientId string, consRes *clienttypes.QueryConsensusStateResponse, clientRes *clienttypes.QueryClientStateResponse) (provider.RelayerMessage, error) {
	return nil, nil
}
func (ap *ArchwayProvider) MsgSubmitMisbehaviour(clientID string, misbehaviour ibcexported.ClientMessage) (provider.RelayerMessage, error) {
	return nil, nil
}
func (ap *ArchwayProvider) ValidatePacket(msgTransfer provider.PacketInfo, latestBlock provider.LatestBlock) error {
	return nil
}
func (ap *ArchwayProvider) PacketCommitment(ctx context.Context, msgTransfer provider.PacketInfo, height uint64) (provider.PacketProof, error) {
	return provider.PacketProof{}, nil
}
func (ap *ArchwayProvider) PacketAcknowledgement(ctx context.Context, msgRecvPacket provider.PacketInfo, height uint64) (provider.PacketProof, error) {
	return provider.PacketProof{}, nil
}
func (ap *ArchwayProvider) PacketReceipt(ctx context.Context, msgTransfer provider.PacketInfo, height uint64) (provider.PacketProof, error) {
	return provider.PacketProof{}, nil
}
func (ap *ArchwayProvider) NextSeqRecv(ctx context.Context, msgTransfer provider.PacketInfo, height uint64) (provider.PacketProof, error) {
	return provider.PacketProof{}, nil
}
func (ap *ArchwayProvider) MsgTransfer(dstAddr string, amount sdk.Coin, info provider.PacketInfo) (provider.RelayerMessage, error) {
	return nil, nil
}
func (ap *ArchwayProvider) MsgRecvPacket(msgTransfer provider.PacketInfo, proof provider.PacketProof) (provider.RelayerMessage, error) {
	return nil, nil
}
func (ap *ArchwayProvider) MsgAcknowledgement(msgRecvPacket provider.PacketInfo, proofAcked provider.PacketProof) (provider.RelayerMessage, error) {
	return nil, nil
}
func (ap *ArchwayProvider) MsgTimeout(msgTransfer provider.PacketInfo, proofUnreceived provider.PacketProof) (provider.RelayerMessage, error) {
	return nil, nil
}
func (ap *ArchwayProvider) MsgTimeoutOnClose(msgTransfer provider.PacketInfo, proofUnreceived provider.PacketProof) (provider.RelayerMessage, error) {
	return nil, nil
}
func (ap *ArchwayProvider) ConnectionHandshakeProof(ctx context.Context, msgOpenInit provider.ConnectionInfo, height uint64) (provider.ConnectionProof, error) {
	return provider.ConnectionProof{}, nil
}
func (ap *ArchwayProvider) ConnectionProof(ctx context.Context, msgOpenAck provider.ConnectionInfo, height uint64) (provider.ConnectionProof, error) {
	return provider.ConnectionProof{}, nil
}
func (ap *ArchwayProvider) MsgConnectionOpenInit(info provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	return nil, nil
}
func (ap *ArchwayProvider) MsgConnectionOpenTry(msgOpenInit provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	return nil, nil
}
func (ap *ArchwayProvider) MsgConnectionOpenAck(msgOpenTry provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	return nil, nil
}
func (ap *ArchwayProvider) MsgConnectionOpenConfirm(msgOpenAck provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	return nil, nil
}
func (ap *ArchwayProvider) ChannelProof(ctx context.Context, msg provider.ChannelInfo, height uint64) (provider.ChannelProof, error) {
	return provider.ChannelProof{}, nil
}
func (ap *ArchwayProvider) MsgChannelOpenInit(info provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	return nil, nil
}
func (ap *ArchwayProvider) MsgChannelOpenTry(msgOpenInit provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	return nil, nil
}
func (ap *ArchwayProvider) MsgChannelOpenAck(msgOpenTry provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	return nil, nil
}
func (ap *ArchwayProvider) MsgChannelOpenConfirm(msgOpenAck provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	return nil, nil
}
func (ap *ArchwayProvider) MsgChannelCloseInit(info provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	return nil, nil
}
func (ap *ArchwayProvider) MsgChannelCloseConfirm(msgCloseInit provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	return nil, nil
}
func (ap *ArchwayProvider) MsgUpdateClientHeader(latestHeader provider.IBCHeader, trustedHeight clienttypes.Height, trustedHeader provider.IBCHeader) (ibcexported.ClientMessage, error) {
	return nil, nil
}
func (ap *ArchwayProvider) MsgUpdateClient(clientID string, counterpartyHeader ibcexported.ClientMessage) (provider.RelayerMessage, error) {
	return nil, nil
}
func (ap *ArchwayProvider) QueryICQWithProof(ctx context.Context, msgType string, request []byte, height uint64) (provider.ICQProof, error) {
	return provider.ICQProof{}, nil
}
func (ap *ArchwayProvider) MsgSubmitQueryResponse(chainID string, queryID provider.ClientICQQueryID, proof provider.ICQProof) (provider.RelayerMessage, error) {
	return nil, nil
}
func (ap *ArchwayProvider) RelayPacketFromSequence(ctx context.Context, src provider.ChainProvider, srch, dsth, seq uint64, srcChanID, srcPortID string, order chantypes.Order) (provider.RelayerMessage, provider.RelayerMessage, error) {
	return nil, nil, nil
}
func (ap *ArchwayProvider) AcknowledgementFromSequence(ctx context.Context, dst provider.ChainProvider, dsth, seq uint64, dstChanID, dstPortID, srcChanID, srcPortID string) (provider.RelayerMessage, error) {
	return nil, nil
}
func (ap *ArchwayProvider) SendMessage(ctx context.Context, msg provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {
	return nil, false, nil
}
func (ap *ArchwayProvider) SendMessages(ctx context.Context, msgs []provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {
	return nil, false, nil
}

func (ap *ArchwayProvider) SendMessagesToMempool(
	ctx context.Context,
	msgs []provider.RelayerMessage,
	memo string,

	asyncCtx context.Context,
	asyncCallback func(*provider.RelayerTxResponse, error),
) error {
	return nil
}
