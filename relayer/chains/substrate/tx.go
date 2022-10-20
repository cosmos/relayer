package substrate

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ChainSafe/gossamer/lib/common"
	rpcclienttypes "github.com/ComposableFi/go-substrate-rpc-client/v4/types"
	beefyclienttypes "github.com/ComposableFi/ics11-beefy/types"
	"github.com/avast/retry-go/v4"
	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v5/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v5/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v5/modules/core/04-channel/types"
	commitmenttypes "github.com/cosmos/ibc-go/v5/modules/core/23-commitment/types"
	ibcexported "github.com/cosmos/ibc-go/v5/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/gogo/protobuf/proto"
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

const (
	callIbcDeliver     = "Ibc.deliver"
	callIbcDeliverPerm = "Ibc.deliver_permissioned"
	callSudo           = "Sudo.sudo"
)

// storage key prefix and methods
const (
	prefixSystem  = "System"
	methodAccount = "Account"
)

type Any struct {
	TypeUrl []byte `json:"type_url,omitempty"`
	Value   []byte `json:"value,omitempty"`
}

// SendMessage attempts to sign, encode & send a RelayerMessage
// This is used extensively in the relayer as an extension of the Provider interface
func (sp *SubstrateProvider) SendMessage(ctx context.Context, msg provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {
	return sp.SendMessages(ctx, []provider.RelayerMessage{msg}, memo)
}

// SendMessages attempts to sign, encode, & send a slice of RelayerMessages
// This is used extensively in the relayer as an extension of the Provider interface
//
// NOTE: An error is returned if there was an issue sending the transaction. A successfully sent, but failed
// transaction will not return an error. If a transaction is successfully sent, the result of the execution
// of that transaction will be logged. A boolean indicating if a transaction was successfully
// sent and executed successfully is returned.
func (sp *SubstrateProvider) SendMessages(ctx context.Context, msgs []provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {
	meta, err := sp.RPCClient.RPC.State.GetMetadataLatest()
	if err != nil {
		return nil, false, err
	}

	call, anyMsgs, err := sp.buildCallParams(msgs)
	if err != nil {
		return nil, false, err
	}

	c, err := rpcclienttypes.NewCall(meta, call, anyMsgs)
	if err != nil {
		return nil, false, err
	}

	sc, err := rpcclienttypes.NewCall(meta, callSudo, c)
	if err != nil {
		return nil, false, err
	}

	// Create the extrinsic
	ext := rpcclienttypes.NewExtrinsic(sc)

	genesisHash, err := sp.RPCClient.RPC.Chain.GetBlockHash(0)
	if err != nil {
		return nil, false, err
	}

	rv, err := sp.RPCClient.RPC.State.GetRuntimeVersionLatest()
	if err != nil {
		return nil, false, err
	}

	info, err := sp.Keybase.Key(sp.Key())
	if err != nil {
		return nil, false, err
	}

	key, err := rpcclienttypes.CreateStorageKey(meta, prefixSystem, methodAccount, info.GetPublicKey(), nil)
	if err != nil {
		return nil, false, err
	}

	var accountInfo rpcclienttypes.AccountInfo
	ok, err := sp.RPCClient.RPC.State.GetStorageLatest(key, &accountInfo)
	if err != nil || !ok {
		return nil, false, err
	}

	nonce := uint32(accountInfo.Nonce)

	o := rpcclienttypes.SignatureOptions{
		BlockHash:          genesisHash,
		Era:                rpcclienttypes.ExtrinsicEra{IsMortalEra: false},
		GenesisHash:        genesisHash,
		Nonce:              rpcclienttypes.NewUCompactFromUInt(uint64(nonce)),
		SpecVersion:        rv.SpecVersion,
		Tip:                rpcclienttypes.NewUCompactFromUInt(0),
		TransactionVersion: rv.TransactionVersion,
	}

	err = ext.Sign(info.GetKeyringPair(), o)
	if err != nil {
		return nil, false, err
	}

	// Send the extrinsic
	sub, err := sp.RPCClient.RPC.Author.SubmitAndWatchExtrinsic(ext)
	if err != nil {
		return nil, false, err
	}

	var status rpcclienttypes.ExtrinsicStatus
	defer sub.Unsubscribe()
	for {
		status = <-sub.Chan()
		// TODO: add zap log for waiting on transaction
		if status.IsInBlock {
			break
		}
	}

	encodedExt, err := rpcclienttypes.Encode(ext)
	if err != nil {
		return nil, false, err
	}

	var extHash [32]byte
	extHash, err = common.Blake2bHash(encodedExt)
	if err != nil {
		return nil, false, err
	}

	events, err := sp.fetchAndBuildEvents(ctx, status.AsInBlock, extHash)
	if err != nil {
		return nil, false, err
	}

	rlyRes := &provider.RelayerTxResponse{
		// TODO: pass in a proper block height
		TxHash: fmt.Sprintf("0x%x", extHash[:]),
		Code:   0, // if the process is reached this line, so there is no error
		Events: events,
	}

	return rlyRes, true, nil
}

// MsgCreateClient creates an sdk.Msg to update the client on src with consensus state from dst
func (sp *SubstrateProvider) MsgCreateClient(
	clientState ibcexported.ClientState,
	consensusState ibcexported.ConsensusState,
) (provider.RelayerMessage, error) {
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

// MsgUpdateClient constructs update client message into substrate message
func (sp *SubstrateProvider) MsgUpdateClient(srcClientId string, dstHeader ibcexported.Header) (provider.RelayerMessage, error) {
	acc, err := sp.Address()
	if err != nil {
		return nil, err
	}

	anyHeader, err := clienttypes.PackHeader(dstHeader)
	if err != nil {
		return nil, err
	}

	msg := &clienttypes.MsgUpdateClient{
		ClientId: srcClientId,
		Header:   anyHeader,
		Signer:   acc,
	}

	return NewSubstrateMessage(msg), nil
}

// MsgUpgradeClient constructs upgrade client message into substrate message
func (sp *SubstrateProvider) MsgUpgradeClient(srcClientId string, consRes *clienttypes.QueryConsensusStateResponse, clientRes *clienttypes.QueryClientStateResponse) (provider.RelayerMessage, error) {
	var (
		acc string
		err error
	)
	if acc, err = sp.Address(); err != nil {
		return nil, err
	}
	return NewSubstrateMessage(&clienttypes.MsgUpgradeClient{ClientId: srcClientId, ClientState: clientRes.ClientState,
		ConsensusState: consRes.ConsensusState, ProofUpgradeClient: consRes.GetProof(),
		ProofUpgradeConsensusState: consRes.ConsensusState.Value, Signer: acc}), nil
}

// MsgTransfer creates a new transfer message
func (sp *SubstrateProvider) MsgTransfer(
	dstAddr string,
	amount sdk.Coin,
	info provider.PacketInfo,
) (provider.RelayerMessage, error) {
	acc, err := sp.Address()
	if err != nil {
		return nil, err
	}
	msg := &transfertypes.MsgTransfer{
		SourcePort:       info.SourcePort,
		SourceChannel:    info.SourceChannel,
		Token:            amount,
		Sender:           acc,
		Receiver:         dstAddr,
		TimeoutTimestamp: info.TimeoutTimestamp,
	}

	// If the timeoutHeight is 0 then we don't need to explicitly set it on the MsgTransfer
	if info.TimeoutHeight.RevisionHeight != 0 {
		msg.TimeoutHeight = info.TimeoutHeight
	}

	return NewSubstrateMessage(msg), nil
}

// ValidatePacket validates transfer message
func (sp *SubstrateProvider) ValidatePacket(msgTransfer provider.PacketInfo, latest provider.LatestBlock) error {
	if msgTransfer.Sequence == 0 {
		return errors.New("refusing to relay packet with sequence: 0")
	}

	if len(msgTransfer.Data) == 0 {
		return errors.New("refusing to relay packet with empty data")
	}

	// This should not be possible, as it violates IBC spec
	if msgTransfer.TimeoutHeight.IsZero() && msgTransfer.TimeoutTimestamp == 0 {
		return errors.New("refusing to relay packet without a timeout (height or timestamp must be set)")
	}

	revision := clienttypes.ParseChainID(sp.Config.ChainID)
	latestClientTypesHeight := clienttypes.NewHeight(revision, latest.Height)
	if !msgTransfer.TimeoutHeight.IsZero() && latestClientTypesHeight.GTE(msgTransfer.TimeoutHeight) {
		return provider.NewTimeoutHeightError(latest.Height, msgTransfer.TimeoutHeight.RevisionHeight)
	}
	latestTimestamp := uint64(latest.Time.UnixNano())
	if msgTransfer.TimeoutTimestamp > 0 && latestTimestamp > msgTransfer.TimeoutTimestamp {
		return provider.NewTimeoutTimestampError(latestTimestamp, msgTransfer.TimeoutTimestamp)
	}

	return nil
}

// PacketCommitment constructs packet proof from packet at certain height
func (sp *SubstrateProvider) PacketCommitment(
	ctx context.Context,
	msgTransfer provider.PacketInfo,
	height uint64,
) (provider.PacketProof, error) {
	comRes, err := sp.RPCClient.RPC.IBC.QueryPacketCommitment(ctx, int64(height), msgTransfer.SourceChannel, msgTransfer.SourcePort)
	if err != nil {
		return provider.PacketProof{}, err
	}
	// check if packet commitment exists
	if len(comRes.Commitment) == 0 {
		return provider.PacketProof{}, chantypes.ErrPacketCommitmentNotFound
	}

	return provider.PacketProof{
		Proof:       comRes.Proof,
		ProofHeight: comRes.ProofHeight,
	}, nil
}

// MsgRecvPacket constructs receive packet message
func (sp *SubstrateProvider) MsgRecvPacket(
	msgTransfer provider.PacketInfo,
	proof provider.PacketProof,
) (provider.RelayerMessage, error) {
	signer, err := sp.Address()
	if err != nil {
		return nil, err
	}
	msg := &chantypes.MsgRecvPacket{
		Packet:          msgTransfer.Packet(),
		ProofCommitment: proof.Proof,
		ProofHeight:     proof.ProofHeight,
		Signer:          signer,
	}

	return NewSubstrateMessage(msg), nil
}

// PacketAcknowledgement constructs packet proof from receive packet
func (sp *SubstrateProvider) PacketAcknowledgement(
	ctx context.Context,
	msgRecvPacket provider.PacketInfo,
	height uint64,
) (provider.PacketProof, error) {
	ackRes, err := sp.RPCClient.RPC.IBC.QueryPacketAcknowledgement(ctx, uint32(height), msgRecvPacket.DestChannel, msgRecvPacket.DestPort, msgRecvPacket.Sequence)
	if err != nil {
		return provider.PacketProof{}, fmt.Errorf("error querying beefy proof for packet acknowledgement: %w", err)
	}
	if len(ackRes.Acknowledgement) == 0 {
		return provider.PacketProof{}, chantypes.ErrInvalidAcknowledgement
	}
	return provider.PacketProof{
		Proof:       ackRes.Proof,
		ProofHeight: ackRes.ProofHeight,
	}, nil
}

// MsgAcknowledgement constructs ack message
func (sp *SubstrateProvider) MsgAcknowledgement(
	msgRecvPacket provider.PacketInfo,
	proof provider.PacketProof,
) (provider.RelayerMessage, error) {
	signer, err := sp.Address()
	if err != nil {
		return nil, err
	}
	msg := &chantypes.MsgAcknowledgement{
		Packet:          msgRecvPacket.Packet(),
		Acknowledgement: msgRecvPacket.Ack,
		ProofAcked:      proof.Proof,
		ProofHeight:     proof.ProofHeight,
		Signer:          signer,
	}

	return NewSubstrateMessage(msg), nil
}

// PacketReceipt returns packet proof of the receipt
func (sp *SubstrateProvider) PacketReceipt(
	ctx context.Context,
	msgTransfer provider.PacketInfo,
	height uint64,
) (provider.PacketProof, error) {
	recRes, err := sp.RPCClient.RPC.IBC.QueryPacketReceipt(ctx, uint32(height), msgTransfer.DestChannel, msgTransfer.DestPort, msgTransfer.Sequence)
	if err != nil {
		return provider.PacketProof{}, err
	}
	return provider.PacketProof{
		Proof:       recRes.Proof,
		ProofHeight: recRes.ProofHeight,
	}, nil
}

// NextSeqRecv queries for the appropriate beefy proof required to prove the next expected packet sequence number
// for a given counterparty channel. This is used in ORDERED channels to ensure packets are being delivered in the
// exact same order as they were sent over the wire.
func (sp *SubstrateProvider) NextSeqRecv(
	ctx context.Context,
	msgTransfer provider.PacketInfo,
	height uint64,
) (provider.PacketProof, error) {
	recvRes, err := sp.RPCClient.RPC.IBC.QueryNextSeqRecv(ctx, uint32(height), msgTransfer.DestChannel, msgTransfer.DestPort)
	if err != nil {
		return provider.PacketProof{}, err
	}
	return provider.PacketProof{
		Proof:       recvRes.Proof,
		ProofHeight: recvRes.ProofHeight,
	}, nil
}

// MsgTimeout constructs timeout message from packet
func (sp *SubstrateProvider) MsgTimeout(msgTransfer provider.PacketInfo, proof provider.PacketProof) (provider.RelayerMessage, error) {
	signer, err := sp.Address()
	if err != nil {
		return nil, err
	}
	assembled := &chantypes.MsgTimeout{
		Packet:           msgTransfer.Packet(),
		ProofUnreceived:  proof.Proof,
		ProofHeight:      proof.ProofHeight,
		NextSequenceRecv: msgTransfer.Sequence,
		Signer:           signer,
	}

	return NewSubstrateMessage(assembled), nil
}

// MsgTimeoutOnClose constructs message from packet and the proof
func (sp *SubstrateProvider) MsgTimeoutOnClose(msgTransfer provider.PacketInfo, proof provider.PacketProof) (provider.RelayerMessage, error) {
	signer, err := sp.Address()
	if err != nil {
		return nil, err
	}
	assembled := &chantypes.MsgTimeoutOnClose{
		Packet:           msgTransfer.Packet(),
		ProofUnreceived:  proof.Proof,
		ProofHeight:      proof.ProofHeight,
		NextSequenceRecv: msgTransfer.Sequence,
		Signer:           signer,
	}

	return NewSubstrateMessage(assembled), nil
}

// MsgConnectionOpenInit constructs connection open init message drom proof and connection info
func (sp *SubstrateProvider) MsgConnectionOpenInit(info provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	signer, err := sp.Address()
	if err != nil {
		return nil, err
	}
	msg := &conntypes.MsgConnectionOpenInit{
		ClientId: info.ClientID,
		Counterparty: conntypes.Counterparty{
			ClientId:     info.CounterpartyClientID,
			ConnectionId: "",
			Prefix:       defaultChainPrefix,
		},
		Version:     nil,
		DelayPeriod: defaultDelayPeriod,
		Signer:      signer,
	}

	return NewSubstrateMessage(msg), nil
}

// ConnectionHandshakeProof returns connection proof from consensus state and proof
func (sp *SubstrateProvider) ConnectionHandshakeProof(
	ctx context.Context,
	msgOpenInit provider.ConnectionInfo,
	height uint64,
) (provider.ConnectionProof, error) {
	clientState, clientStateProof, consensusStateProof, connStateProof, proofHeight, err := sp.GenerateConnHandshakeProof(ctx, int64(height), msgOpenInit.ClientID, msgOpenInit.ConnID)
	if err != nil {
		return provider.ConnectionProof{}, err
	}

	if len(connStateProof) == 0 {
		// It is possible that we have asked for a proof too early.
		// If the connection state proof is empty, there is no point in returning the next message.
		// We are not using (*conntypes.MsgConnectionOpenTry).ValidateBasic here because
		// that chokes on cross-chain bech32 details in ibc-go.
		return provider.ConnectionProof{}, fmt.Errorf("received invalid zero-length connection state proof")
	}

	return provider.ConnectionProof{
		ClientState:          clientState,
		ClientStateProof:     clientStateProof,
		ConsensusStateProof:  consensusStateProof,
		ConnectionStateProof: connStateProof,
		ProofHeight:          proofHeight.(clienttypes.Height),
	}, nil
}

// MsgConnectionOpenTry constructs connection open try from connection info and proof
func (sp *SubstrateProvider) MsgConnectionOpenTry(msgOpenInit provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	signer, err := sp.Address()
	if err != nil {
		return nil, err
	}

	csAny, err := clienttypes.PackClientState(proof.ClientState)
	if err != nil {
		return nil, err
	}

	counterparty := conntypes.Counterparty{
		ClientId:     msgOpenInit.ClientID,
		ConnectionId: msgOpenInit.ConnID,
		Prefix:       defaultChainPrefix,
	}

	msg := &conntypes.MsgConnectionOpenTry{
		ClientId:             msgOpenInit.CounterpartyClientID,
		PreviousConnectionId: msgOpenInit.CounterpartyConnID,
		ClientState:          csAny,
		Counterparty:         counterparty,
		DelayPeriod:          defaultDelayPeriod,
		CounterpartyVersions: conntypes.ExportedVersionsToProto(conntypes.GetCompatibleVersions()),
		ProofHeight:          proof.ProofHeight,
		ProofInit:            proof.ConnectionStateProof,
		ProofClient:          proof.ClientStateProof,
		ProofConsensus:       proof.ConsensusStateProof,
		ConsensusHeight:      proof.ClientState.GetLatestHeight().(clienttypes.Height),
		Signer:               signer,
	}

	return NewSubstrateMessage(msg), nil
}

// MsgConnectionOpenAck constructs connection open ack message from connection info and proof
func (sp *SubstrateProvider) MsgConnectionOpenAck(msgOpenTry provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	signer, err := sp.Address()
	if err != nil {
		return nil, err
	}

	csAny, err := clienttypes.PackClientState(proof.ClientState)
	if err != nil {
		return nil, err
	}

	msg := &conntypes.MsgConnectionOpenAck{
		ConnectionId:             msgOpenTry.CounterpartyConnID,
		CounterpartyConnectionId: msgOpenTry.ConnID,
		Version:                  conntypes.DefaultIBCVersion,
		ClientState:              csAny,
		ProofHeight: clienttypes.Height{
			RevisionNumber: proof.ProofHeight.GetRevisionNumber(),
			RevisionHeight: proof.ProofHeight.GetRevisionHeight(),
		},
		ProofTry:        proof.ConnectionStateProof,
		ProofClient:     proof.ClientStateProof,
		ProofConsensus:  proof.ConsensusStateProof,
		ConsensusHeight: proof.ClientState.GetLatestHeight().(clienttypes.Height),
		Signer:          signer,
	}

	return NewSubstrateMessage(msg), nil
}

// ConnectionProof returns connection proof from connection state
func (sp *SubstrateProvider) ConnectionProof(
	ctx context.Context,
	msgOpenAck provider.ConnectionInfo,
	height uint64,
) (provider.ConnectionProof, error) {
	connState, err := sp.QueryConnection(ctx, int64(height), msgOpenAck.ConnID)
	if err != nil {
		return provider.ConnectionProof{}, err
	}

	return provider.ConnectionProof{
		ConnectionStateProof: connState.Proof,
		ProofHeight:          connState.ProofHeight,
	}, nil
}

// MsgConnectionOpenConfirm constructs connection open confirm from open ack message and connection proof
func (sp *SubstrateProvider) MsgConnectionOpenConfirm(msgOpenAck provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	signer, err := sp.Address()
	if err != nil {
		return nil, err
	}
	msg := &conntypes.MsgConnectionOpenConfirm{
		ConnectionId: msgOpenAck.CounterpartyConnID,
		ProofAck:     proof.ConnectionStateProof,
		ProofHeight:  proof.ProofHeight,
		Signer:       signer,
	}

	return NewSubstrateMessage(msg), nil
}

// MsgChannelOpenInit constructs channel open init message from provider info and proof
func (sp *SubstrateProvider) MsgChannelOpenInit(info provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	signer, err := sp.Address()
	if err != nil {
		return nil, err
	}
	msg := &chantypes.MsgChannelOpenInit{
		PortId: info.PortID,
		Channel: chantypes.Channel{
			State:    chantypes.INIT,
			Ordering: info.Order,
			Counterparty: chantypes.Counterparty{
				PortId:    info.CounterpartyPortID,
				ChannelId: "",
			},
			ConnectionHops: []string{info.ConnID},
			Version:        info.Version,
		},
		Signer: signer,
	}

	return NewSubstrateMessage(msg), nil
}

// ChannelProof returns proof of channel from channel info
func (sp *SubstrateProvider) ChannelProof(
	ctx context.Context,
	msg provider.ChannelInfo,
	height uint64,
) (provider.ChannelProof, error) {
	channelRes, err := sp.QueryChannel(ctx, int64(height), msg.ChannelID, msg.PortID)
	if err != nil {
		return provider.ChannelProof{}, err
	}
	return provider.ChannelProof{
		Proof:       channelRes.Proof,
		ProofHeight: channelRes.ProofHeight,
		Version:     channelRes.Channel.Version,
		Ordering:    channelRes.Channel.Ordering,
	}, nil
}

// MsgChannelOpenTry constructs channel open try from channel info and proof
func (sp *SubstrateProvider) MsgChannelOpenTry(msgOpenInit provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	signer, err := sp.Address()
	if err != nil {
		return nil, err
	}
	msg := &chantypes.MsgChannelOpenTry{
		PortId:            msgOpenInit.CounterpartyPortID,
		PreviousChannelId: msgOpenInit.CounterpartyChannelID,
		Channel: chantypes.Channel{
			State:    chantypes.TRYOPEN,
			Ordering: proof.Ordering,
			Counterparty: chantypes.Counterparty{
				PortId:    msgOpenInit.PortID,
				ChannelId: msgOpenInit.ChannelID,
			},
			ConnectionHops: []string{msgOpenInit.CounterpartyConnID},
			// In the future, may need to separate this from the CounterpartyVersion.
			// https://github.com/cosmos/ibc/tree/master/spec/core/ics-004-channel-and-packet-semantics#definitions
			// Using same version as counterparty for now.
			Version: proof.Version,
		},
		CounterpartyVersion: proof.Version,
		ProofInit:           proof.Proof,
		ProofHeight:         proof.ProofHeight,
		Signer:              signer,
	}

	return NewSubstrateMessage(msg), nil
}

// MsgChannelOpenAck constructs channel open acknowledgement from channel info and proof
func (sp *SubstrateProvider) MsgChannelOpenAck(msgOpenTry provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	signer, err := sp.Address()
	if err != nil {
		return nil, err
	}
	msg := &chantypes.MsgChannelOpenAck{
		PortId:                msgOpenTry.CounterpartyPortID,
		ChannelId:             msgOpenTry.CounterpartyChannelID,
		CounterpartyChannelId: msgOpenTry.ChannelID,
		CounterpartyVersion:   proof.Version,
		ProofTry:              proof.Proof,
		ProofHeight:           proof.ProofHeight,
		Signer:                signer,
	}

	return NewSubstrateMessage(msg), nil
}

// MsgChannelOpenConfirm constructs channel open confirm message from channel info and proof
func (sp *SubstrateProvider) MsgChannelOpenConfirm(msgOpenAck provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	signer, err := sp.Address()
	if err != nil {
		return nil, err
	}
	msg := &chantypes.MsgChannelOpenConfirm{
		PortId:      msgOpenAck.CounterpartyPortID,
		ChannelId:   msgOpenAck.CounterpartyChannelID,
		ProofAck:    proof.Proof,
		ProofHeight: proof.ProofHeight,
		Signer:      signer,
	}

	return NewSubstrateMessage(msg), nil
}

// MsgChannelCloseInit constructs message channel close initialization message drom info and proof
func (sp *SubstrateProvider) MsgChannelCloseInit(info provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	signer, err := sp.Address()
	if err != nil {
		return nil, err
	}
	msg := &chantypes.MsgChannelCloseInit{
		PortId:    info.PortID,
		ChannelId: info.ChannelID,
		Signer:    signer,
	}

	return NewSubstrateMessage(msg), nil
}

// MsgChannelCloseConfirm constructs channel close confirmation message form channel info and proof
func (sp *SubstrateProvider) MsgChannelCloseConfirm(msgCloseInit provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	signer, err := sp.Address()
	if err != nil {
		return nil, err
	}
	msg := &chantypes.MsgChannelCloseConfirm{
		PortId:      msgCloseInit.CounterpartyPortID,
		ChannelId:   msgCloseInit.CounterpartyChannelID,
		ProofInit:   proof.Proof,
		ProofHeight: proof.ProofHeight,
		Signer:      signer,
	}

	return NewSubstrateMessage(msg), nil
}

// MsgUpdateClientHeader constructs update client header message from ibc header and trusted header and height
func (sp *SubstrateProvider) MsgUpdateClientHeader(latestHeader provider.IBCHeader, trustedHeight clienttypes.Height, trustedHeader provider.IBCHeader) (ibcexported.Header, error) {
	_, ok := trustedHeader.(SubstrateIBCHeader)
	if !ok {
		return nil, fmt.Errorf("unsupported IBC trusted header type, expected: SubstrateIBCHeader, actual: %T", trustedHeader)
	}

	latestSubstrateHeader, ok := latestHeader.(SubstrateIBCHeader)
	if !ok {
		return nil, fmt.Errorf("unsupported IBC header type, expected: SubstrateIBCHeader, actual: %T", latestHeader)
	}

	return &beefyclienttypes.Header{
		HeadersWithProof: latestSubstrateHeader.SignedHeader.HeadersWithProof,
		MMRUpdateProof:   latestSubstrateHeader.SignedHeader.MMRUpdateProof,
	}, nil
}

// RelayPacketFromSequence relays a packet with a given seq on src and returns recvPacket msgs, timeoutPacketmsgs and error
func (sp *SubstrateProvider) RelayPacketFromSequence(
	ctx context.Context,
	src provider.ChainProvider,
	srch, dsth, seq uint64,
	srcChanID, srcPortID string,
	order chantypes.Order,
) (provider.RelayerMessage, provider.RelayerMessage, error) {
	msgTransfer, err := src.QuerySendPacket(ctx, srcChanID, srcPortID, seq)
	if err != nil {
		return nil, nil, err
	}

	dstTime, err := sp.BlockTime(ctx, int64(dsth))
	if err != nil {
		return nil, nil, err
	}

	if err := sp.ValidatePacket(msgTransfer, provider.LatestBlock{
		Height: dsth,
		Time:   dstTime,
	}); err != nil {
		switch err.(type) {
		case *provider.TimeoutHeightError, *provider.TimeoutTimestampError, *provider.TimeoutOnCloseError:
			var pp provider.PacketProof
			switch order {
			case chantypes.UNORDERED:
				pp, err = sp.PacketReceipt(ctx, msgTransfer, dsth)
				if err != nil {
					return nil, nil, err
				}
			case chantypes.ORDERED:
				pp, err = sp.NextSeqRecv(ctx, msgTransfer, dsth)
				if err != nil {
					return nil, nil, err
				}
			}
			if _, ok := err.(*provider.TimeoutOnCloseError); ok {
				timeout, err := src.MsgTimeoutOnClose(msgTransfer, pp)
				if err != nil {
					return nil, nil, err
				}
				return nil, timeout, nil
			} else {
				timeout, err := src.MsgTimeout(msgTransfer, pp)
				if err != nil {
					return nil, nil, err
				}
				return nil, timeout, nil
			}
		default:
			return nil, nil, err
		}
	}

	pp, err := src.PacketCommitment(ctx, msgTransfer, srch)
	if err != nil {
		return nil, nil, err
	}

	packet, err := sp.MsgRecvPacket(msgTransfer, pp)
	if err != nil {
		return nil, nil, err
	}

	return packet, nil, nil
}

// AcknowledgementFromSequence relays an acknowledgement with a given seq on src, source is the sending chain, destination is the receiving chain
func (sp *SubstrateProvider) AcknowledgementFromSequence(ctx context.Context, dst provider.ChainProvider, dsth, seq uint64, dstChanId, dstPortId, srcChanId, srcPortId string) (provider.RelayerMessage, error) {
	msgRecvPacket, err := dst.QueryRecvPacket(ctx, dstChanId, dstPortId, seq)
	if err != nil {
		return nil, err
	}

	pp, err := dst.PacketAcknowledgement(ctx, msgRecvPacket, dsth)
	if err != nil {
		return nil, err
	}
	msg, err := sp.MsgAcknowledgement(msgRecvPacket, pp)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// QueryIBCHeader returns the IBC compatible block header (SubstrateIBCHeader) at a specific height.
func (sp *SubstrateProvider) QueryIBCHeader(ctx context.Context, h int64) (provider.IBCHeader, error) {
	if h <= 0 {
		return nil, fmt.Errorf("must pass in valid height, %d not valid", h)
	}

	latestBeefyBlockHash, err := sp.RPCClient.RPC.Beefy.GetFinalizedHead()
	if err != nil {
		return nil, err
	}

	latestBeefyHeight, err := sp.RPCClient.RPC.Chain.GetBlock(latestBeefyBlockHash)
	if err != nil {
		return nil, err
	}

	if h > int64(latestBeefyHeight.Block.Header.Number) {
		return nil, fmt.Errorf("queried block is not finalized")
	}

	blockHash, err := sp.RPCClient.RPC.Chain.GetBlockHash(uint64(h))
	if err != nil {
		return nil, err
	}

	header, err := sp.constructBeefyHeader(latestBeefyBlockHash, &blockHash)
	if err != nil {
		return nil, err
	}

	return SubstrateIBCHeader{
		height:       uint64(h),
		SignedHeader: header,
	}, nil
}

// InjectTrustedFields injects the necessary trusted fields for a header to update a light
// client stored on the destination chain, using the information provided by the source
// chain.
// TrustedHeight is the latest height of the IBC client on dst
// TrustedValidators is the validator set of srcChain at the TrustedHeight
// InjectTrustedFields returns a copy of the header with TrustedFields modified
func (sp *SubstrateProvider) InjectTrustedFields(ctx context.Context, header ibcexported.Header, dst provider.ChainProvider, dstClientId string) (ibcexported.Header, error) {
	// make copy of header stored in mop
	h, ok := header.(*beefyclienttypes.Header)
	if !ok {
		return nil, fmt.Errorf("trying to inject fields into non-beefy headers")
	}

	// retrieve dst client from src chain
	// this is the client that will be updated
	cs, err := dst.QueryClientState(ctx, int64(h.GetHeight().GetRevisionHeight()), dstClientId)
	if err != nil {
		return nil, err
	}

	// inject TrustedHeight as latest height stored on dst client
	h.MMRUpdateProof.SignedCommitment.Commitment.BlockNumber = uint32(cs.GetLatestHeight().(clienttypes.Height).RevisionHeight)

	// NOTE: We need to get validators from the source chain at height: trustedHeight+1
	// since the last trusted validators for a header at height h is the NextValidators
	// at h+1 committed to in header h by NextValidatorsHash

	// place where we need to fix the upstream query proof issue?
	var trustedValidatorSetID uint64
	if err := retry.Do(func() error {
		ibcHeader, err := sp.QueryIBCHeader(ctx, int64(h.MMRUpdateProof.SignedCommitment.Commitment.BlockNumber+1))
		if err != nil {
			return err
		}

		trustedValidatorSetID = ibcHeader.(SubstrateIBCHeader).SignedHeader.MMRUpdateProof.SignedCommitment.Commitment.ValidatorSetID
		return err
	}, retry.Context(ctx), rtyAtt, rtyDel, rtyErr); err != nil {
		return nil, fmt.Errorf(
			"failed to get trusted header, please ensure header at the height %d has not been pruned by the connected node: %w",
			h.MMRUpdateProof.SignedCommitment.Commitment.BlockNumber, err,
		)
	}

	h.MMRUpdateProof.SignedCommitment.Commitment.ValidatorSetID = trustedValidatorSetID

	return h, nil
}

// NewClientState creates a new beefy client state tracking the dst chain.
func (sp *SubstrateProvider) NewClientState(
	dstChainID string,
	dstUpdateHeader provider.IBCHeader,
	dstTrustingPeriod,
	dstUbdPeriod time.Duration,
	allowUpdateAfterExpiry,
	allowUpdateAfterMisbehaviour bool,
) (ibcexported.ClientState, error) {
	substrateHeader, ok := dstUpdateHeader.(SubstrateIBCHeader)
	if !ok {
		return nil, fmt.Errorf("got data of type %T but wanted  substrate.SubstrateIBCHeader \n", dstUpdateHeader)
	}

	// TODO: this won't work because we need the height passed to GetBlockHash to be the previously finalized beefy height
	// from the relayer. However, the height from substrate.Height() is the height of the first parachain from the beefy header.
	blockHash, err := sp.RelayerRPCClient.RPC.Chain.GetBlockHash(substrateHeader.Height())
	if err != nil {
		return nil, err
	}

	commitment, err := sp.signedCommitment(blockHash)
	if err != nil {
		return nil, err
	}

	cs, err := sp.clientState(commitment)
	if err != nil {
		return nil, err
	}

	return cs, nil
}

// returns call method and messages of relayer
func (sp *SubstrateProvider) buildCallParams(msgs []provider.RelayerMessage) (call string, anyMsgs []Any, err error) {
	var msgTypeCall = func(msgType string) string {
		switch msgType {
		case "/" + string(proto.MessageName(&clienttypes.MsgCreateClient{})):
			return callIbcDeliverPerm
		default:
			return callIbcDeliver
		}
	}

	call = msgTypeCall(msgs[0].Type())
	for i := 0; i < len(msgs); i++ {
		msg := msgs[i]

		if call != msgTypeCall(msg.Type()) {
			return "", nil,
				fmt.Errorf("%s: %s", ErrDifferentTypesOfCallsMixed, msg)
		}

		msgBytes, err := msg.MsgBytes()
		if err != nil {
			return "", nil, err
		}

		anyMsgs = append(anyMsgs, Any{
			TypeUrl: []byte(msg.Type()),
			Value:   msgBytes,
		})
	}

	return
}

func (sp *SubstrateProvider) fetchAndBuildEvents(
	ctx context.Context,
	blockHash rpcclienttypes.Hash,
	extHash rpcclienttypes.Hash,
) (relayerEvents []provider.RelayerEvent, err error) {

	blockHashHex := blockHash.Hex()
	eventResult, err := sp.RPCClient.RPC.IBC.QueryIbcEvents(ctx, []rpcclienttypes.BlockNumberOrHash{{Hash: &blockHashHex}})
	if err != nil {
		return nil, err
	}

	events := parseRelayEventsFromEvents(eventResult)
	return events, nil
}

func parseRelayEventsFromEvents(eventResult rpcclienttypes.IBCEventsQueryResult) []provider.RelayerEvent {
	var events []provider.RelayerEvent

	if eventResult == nil {
		return events
	}

	for _, e := range eventResult {

		for key, attributes := range e {
			stringAttrs := make(map[string]string)

			attrs := attributes.(ibcEventQueryItem)
			for attrKey, a := range attrs {
				stringAttrs[attrKey] = fmt.Sprint(a)
			}

			events = append(events, provider.RelayerEvent{
				EventType:  key,
				Attributes: stringAttrs,
			})
		}

	}
	return events
}
