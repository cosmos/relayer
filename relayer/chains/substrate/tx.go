package substrate

import (
	"context"
	"errors"
	"fmt"
	"time"

	rpcClientTypes "github.com/ComposableFi/go-substrate-rpc-client/v4/types"
	beefyclienttypes "github.com/ComposableFi/ics11-beefy/types"
	"github.com/avast/retry-go/v4"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v5/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v5/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v5/modules/core/04-channel/types"
	commitmenttypes "github.com/cosmos/ibc-go/v5/modules/core/23-commitment/types"
	ibcexported "github.com/cosmos/ibc-go/v5/modules/core/exported"
	tmclient "github.com/cosmos/ibc-go/v5/modules/light-clients/07-tendermint/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/tendermint/tendermint/light"
	"go.uber.org/zap"
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
	var rlyResp *provider.RelayerTxResponse

	if err := retry.Do(func() error {
		meta, err := sp.RPCClient.RPC.State.GetMetadataLatest()
		if err != nil {
			return err
		}

		c, err := rpcClientTypes.NewCall(meta, "IBC.deliver", msgs)
		if err != nil {
			return err
		}

		// Create the extrinsic
		ext := rpcClientTypes.NewExtrinsic(c)

		genesisHash, err := sp.RPCClient.RPC.Chain.GetBlockHash(0)
		if err != nil {
			return err
		}

		rv, err := sp.RPCClient.RPC.State.GetRuntimeVersionLatest()
		if err != nil {
			return err
		}

		info, err := sp.Keybase.Key(sp.Key())
		if err != nil {
			return err
		}

		key, err := rpcClientTypes.CreateStorageKey(meta, "System", "Account", info.GetPublicKey(), nil)
		if err != nil {
			return err
		}

		var accountInfo rpcClientTypes.AccountInfo
		ok, err := sp.RPCClient.RPC.State.GetStorageLatest(key, &accountInfo)
		if err != nil || !ok {
			return err
		}

		nonce := uint32(accountInfo.Nonce)

		o := rpcClientTypes.SignatureOptions{
			BlockHash:   genesisHash,
			Era:         rpcClientTypes.ExtrinsicEra{IsMortalEra: false},
			GenesisHash: genesisHash,
			Nonce:       rpcClientTypes.NewUCompactFromUInt(uint64(nonce)),
			SpecVersion: rv.SpecVersion,
			Tip:         rpcClientTypes.NewUCompactFromUInt(0),
		}

		err = ext.Sign(info.GetKeyringPair(), o)
		if err != nil {
			return err
		}

		// Send the extrinsic
		hash, err := sp.RPCClient.RPC.Author.SubmitExtrinsic(ext)
		if err != nil {
			return err
		}

		// TODO: check if there's a go substrate rpc method to wait for finalization
		rlyResp = &provider.RelayerTxResponse{
			// TODO: What height is the height field in this struct? Is the transaction added to the blockchain right away?
			TxHash: hash.Hex(),
		}

		return nil
	}, retry.Context(ctx), rtyAtt, rtyDel, rtyErr, retry.OnRetry(func(n uint, err error) {
		sp.log.Info(
			"Error building or broadcasting transaction",
			zap.String("chain_id", sp.Config.ChainID),
			zap.Uint("attempt", n+1),
			zap.Uint("max_attempts", rtyAttNum),
			zap.Error(err),
		)
	})); err != nil || rlyResp == nil {
		return nil, false, err
	}

	// transaction was executed, log the success or failure using the tx response code
	// NOTE: error is nil, logic should use the returned error to determine if the
	// transaction was successfully executed.
	if rlyResp.Code != 0 {
		sp.LogFailedTx(rlyResp, nil, msgs)
		return rlyResp, false, fmt.Errorf("transaction failed with code: %d", rlyResp.Code)
	}

	sp.LogSuccessTx(rlyResp, msgs)
	return rlyResp, true, nil
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

func (sp *SubstrateProvider) PacketAcknowledgement(
	ctx context.Context,
	msgRecvPacket provider.PacketInfo,
	height uint64,
) (provider.PacketProof, error) {
	ackRes, err := sp.RPCClient.RPC.IBC.QueryPacketAcknowledgement(ctx, uint32(height), msgRecvPacket.DestChannel, msgRecvPacket.DestPort, msgRecvPacket.Sequence)
	if err != nil {
		return provider.PacketProof{}, fmt.Errorf("error querying tendermint proof for packet acknowledgement: %w", err)
	}
	if len(ackRes.Acknowledgement) == 0 {
		return provider.PacketProof{}, chantypes.ErrInvalidAcknowledgement
	}
	return provider.PacketProof{
		Proof:       ackRes.Proof,
		ProofHeight: ackRes.ProofHeight,
	}, nil
}

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

// NextSeqRecv queries for the appropriate Tendermint proof required to prove the next expected packet sequence number
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
		return nil, fmt.Errorf("trying to inject fields into non-tendermint headers")
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

	// TODO: this is likely a source of off by 1 errors but may be impossible to change? Maybe this is the
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

// queryTMClientState retrieves the latest consensus state for a client in state at a given height
// and unpacks/cast it to tendermint clientstate
func (sp *SubstrateProvider) queryTMClientState(ctx context.Context, srch int64, srcClientId string) (*tmclient.ClientState, error) {
	clientStateRes, err := sp.QueryClientStateResponse(ctx, srch, srcClientId)
	if err != nil {
		return &tmclient.ClientState{}, err
	}

	return castClientStateToTMType(clientStateRes.ClientState)
}

// castClientStateToTMType casts client state to tendermint type
func castClientStateToTMType(cs *codectypes.Any) (*tmclient.ClientState, error) {
	clientStateExported, err := clienttypes.UnpackClientState(cs)
	if err != nil {
		return &tmclient.ClientState{}, err
	}

	// cast from interface to concrete type
	clientState, ok := clientStateExported.(*tmclient.ClientState)
	if !ok {
		return &tmclient.ClientState{},
			fmt.Errorf("error when casting exported clientstate to tendermint type")
	}

	return clientState, nil
}

// DefaultUpgradePath is the default IBC upgrade path set for an on-chain light client
var defaultUpgradePath = []string{"upgrade", "upgradedIBCState"}

// NewClientState creates a new tendermint client state tracking the dst chain.
func (sp *SubstrateProvider) NewClientState(
	dstChainID string,
	dstUpdateHeader provider.IBCHeader,
	dstTrustingPeriod,
	dstUbdPeriod time.Duration,
	allowUpdateAfterExpiry,
	allowUpdateAfterMisbehaviour bool,
) (ibcexported.ClientState, error) {
	revisionNumber := clienttypes.ParseChainID(dstChainID)

	// Create the ClientState we want on 'c' tracking 'dst'
	return &tmclient.ClientState{
		ChainId:         dstChainID,
		TrustLevel:      tmclient.NewFractionFromTm(light.DefaultTrustLevel),
		TrustingPeriod:  dstTrustingPeriod,
		UnbondingPeriod: dstUbdPeriod,
		MaxClockDrift:   time.Minute * 10,
		FrozenHeight:    clienttypes.ZeroHeight(),
		LatestHeight: clienttypes.Height{
			RevisionNumber: revisionNumber,
			RevisionHeight: dstUpdateHeader.Height(),
		},
		ProofSpecs:                   commitmenttypes.GetSDKSpecs(),
		UpgradePath:                  defaultUpgradePath,
		AllowUpdateAfterExpiry:       allowUpdateAfterExpiry,
		AllowUpdateAfterMisbehaviour: allowUpdateAfterMisbehaviour,
	}, nil
}
