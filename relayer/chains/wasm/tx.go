package wasm

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/cosmos/cosmos-sdk/codec/types"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"

	coretypes "github.com/cometbft/cometbft/rpc/core/types"

	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/cosmos/cosmos-sdk/store/rootmulti"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/avast/retry-go/v4"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/light"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	commitmenttypes "github.com/cosmos/ibc-go/v7/modules/core/23-commitment/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	itm "github.com/icon-project/IBC-Integration/libraries/go/common/tendermint"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/input"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/exported"
)

var (
	rtyAttNum                   = uint(5)
	rtyAtt                      = retry.Attempts(rtyAttNum)
	rtyDel                      = retry.Delay(time.Millisecond * 400)
	rtyErr                      = retry.LastErrorOnly(true)
	numRegex                    = regexp.MustCompile("[0-9]+")
	defaultBroadcastWaitTimeout = 10 * time.Minute
	errUnknown                  = "unknown"
)

// Default IBC settings
var (
	defaultChainPrefix = commitmenttypes.NewMerklePrefix([]byte("commitments"))
	defaultDelayPeriod = uint64(0)
)

func (ap *WasmProvider) TxFactory() tx.Factory {
	return tx.Factory{}.
		WithAccountRetriever(ap).
		WithChainID(ap.PCfg.ChainID).
		WithTxConfig(ap.Cdc.TxConfig).
		WithGasAdjustment(ap.PCfg.GasAdjustment).
		WithGasPrices(ap.PCfg.GasPrices).
		WithKeybase(ap.Keybase).
		WithSignMode(ap.PCfg.SignMode()).
		WithSimulateAndExecute(true)
}

// PrepareFactory mutates the tx factory with the appropriate account number, sequence number, and min gas settings.
func (ap *WasmProvider) PrepareFactory(txf tx.Factory) (tx.Factory, error) {
	var (
		err      error
		from     sdk.AccAddress
		num, seq uint64
	)

	// Get key address and retry if fail
	if err = retry.Do(func() error {
		from, err = ap.GetKeyAddress()
		if err != nil {
			return err
		}
		return err
	}, rtyAtt, rtyDel, rtyErr); err != nil {
		return tx.Factory{}, err
	}

	cliCtx := client.Context{}.WithClient(ap.RPCClient).
		WithInterfaceRegistry(ap.Cdc.InterfaceRegistry).
		WithChainID(ap.PCfg.ChainID).
		WithCodec(ap.Cdc.Marshaler)

	// Set the account number and sequence on the transaction factory and retry if fail
	if err = retry.Do(func() error {
		if err = txf.AccountRetriever().EnsureExists(cliCtx, from); err != nil {
			return err
		}
		return err
	}, rtyAtt, rtyDel, rtyErr); err != nil {
		return txf, err
	}

	// TODO: why this code? this may potentially require another query when we don't want one
	initNum, initSeq := txf.AccountNumber(), txf.Sequence()
	if initNum == 0 || initSeq == 0 {
		if err = retry.Do(func() error {
			num, seq, err = txf.AccountRetriever().GetAccountNumberSequence(cliCtx, from)
			if err != nil {
				return err
			}
			return err
		}, rtyAtt, rtyDel, rtyErr); err != nil {
			return txf, err
		}

		if initNum == 0 {
			txf = txf.WithAccountNumber(num)
		}

		if initSeq == 0 {
			txf = txf.WithSequence(seq)
		}
	}

	if ap.PCfg.MinGasAmount != 0 {
		txf = txf.WithGas(ap.PCfg.MinGasAmount)
	}

	return txf, nil
}

func (pc *WasmProviderConfig) SignMode() signing.SignMode {
	signMode := signing.SignMode_SIGN_MODE_UNSPECIFIED
	switch pc.SignModeStr {
	case "direct":
		signMode = signing.SignMode_SIGN_MODE_DIRECT
	case "amino-json":
		signMode = signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON
	}
	return signMode
}

func (ap *WasmProvider) NewClientState(dstChainID string, dstIBCHeader provider.IBCHeader, dstTrustingPeriod, dstUbdPeriod time.Duration, allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour bool, clientType string) (ibcexported.ClientState, error) {

	return &itm.ClientState{
		ChainId:                      dstChainID,
		TrustLevel:                   &itm.Fraction{Numerator: light.DefaultTrustLevel.Numerator, Denominator: light.DefaultTrustLevel.Denominator},
		TrustingPeriod:               &itm.Duration{Seconds: int64(dstTrustingPeriod.Seconds())},
		UnbondingPeriod:              &itm.Duration{Seconds: int64(dstUbdPeriod.Seconds())},
		FrozenHeight:                 0,
		LatestHeight:                 int64(dstIBCHeader.Height()),
		AllowUpdateAfterExpiry:       allowUpdateAfterExpiry,
		AllowUpdateAfterMisbehaviour: allowUpdateAfterMisbehaviour,
	}, nil
}

func (ap *WasmProvider) MsgCreateClient(clientState ibcexported.ClientState, consensusState ibcexported.ConsensusState) (provider.RelayerMessage, error) {
	signer, err := ap.Address()
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

	params := &clienttypes.MsgCreateClient{
		ClientState:    anyClientState,
		ConsensusState: anyConsensusState,
		Signer:         signer,
	}

	return ap.NewWasmContractMessage(MethodCreateClient, params)
}

func (ap *WasmProvider) MsgUpgradeClient(srcClientId string, consRes *clienttypes.QueryConsensusStateResponse, clientRes *clienttypes.QueryClientStateResponse) (provider.RelayerMessage, error) {
	panic(fmt.Sprintf("%s%s", ap.ChainName(), NOT_IMPLEMENTED))
}

func (ap *WasmProvider) MsgSubmitMisbehaviour(clientID string, misbehaviour ibcexported.ClientMessage) (provider.RelayerMessage, error) {
	panic(fmt.Sprintf("%s%s", ap.ChainName(), NOT_IMPLEMENTED))
}

func (ap *WasmProvider) ValidatePacket(msgTransfer provider.PacketInfo, latest provider.LatestBlock) error {

	if msgTransfer.Sequence == 0 {
		return errors.New("refusing to relay packet with sequence: 0")
	}

	if len(msgTransfer.Data) == 0 {
		return errors.New("refusing to relay packet with empty data")
	}

	// This should not be possible, as it violates IBC spec
	if msgTransfer.TimeoutHeight.IsZero() {
		return errors.New("refusing to relay packet without a timeout (height or timestamp must be set)")
	}

	revisionNumber := 0
	latestClientTypesHeight := clienttypes.NewHeight(uint64(revisionNumber), latest.Height)
	if !msgTransfer.TimeoutHeight.IsZero() && latestClientTypesHeight.GTE(msgTransfer.TimeoutHeight) {

		return provider.NewTimeoutHeightError(latest.Height, msgTransfer.TimeoutHeight.RevisionHeight)
	}
	// latestTimestamp := uint64(latest.Time.UnixNano())
	// if msgTransfer.TimeoutTimestamp > 0 && latestTimestamp > msgTransfer.TimeoutTimestamp {
	// 	return provider.NewTimeoutTimestampError(latestTimestamp, msgTransfer.TimeoutTimestamp)
	// }

	return nil
}

func (ap *WasmProvider) PacketCommitment(ctx context.Context, msgTransfer provider.PacketInfo, height uint64) (provider.PacketProof, error) {
	packetCommitmentResponse, err := ap.QueryPacketCommitment(
		ctx, int64(height), msgTransfer.SourceChannel, msgTransfer.SourcePort, msgTransfer.Sequence,
	)

	if err != nil {
		return provider.PacketProof{}, err
	}
	return provider.PacketProof{
		Proof:       packetCommitmentResponse.Proof,
		ProofHeight: packetCommitmentResponse.ProofHeight,
	}, nil
}

func (ap *WasmProvider) PacketAcknowledgement(ctx context.Context, msgRecvPacket provider.PacketInfo, height uint64) (provider.PacketProof, error) {
	packetAckResponse, err := ap.QueryPacketAcknowledgement(ctx, int64(height), msgRecvPacket.DestChannel, msgRecvPacket.DestPort, msgRecvPacket.Sequence)
	if err != nil {
		return provider.PacketProof{}, err
	}
	return provider.PacketProof{
		Proof:       packetAckResponse.Proof,
		ProofHeight: packetAckResponse.GetProofHeight(),
	}, nil
}

func (ap *WasmProvider) PacketReceipt(ctx context.Context, msgTransfer provider.PacketInfo, height uint64) (provider.PacketProof, error) {

	packetReceiptResponse, err := ap.QueryPacketReceipt(ctx, int64(height), msgTransfer.DestChannel, msgTransfer.DestPort, msgTransfer.Sequence)

	if err != nil {
		return provider.PacketProof{}, err
	}
	return provider.PacketProof{
		Proof:       packetReceiptResponse.Proof,
		ProofHeight: packetReceiptResponse.ProofHeight,
	}, nil
}

func (ap *WasmProvider) NextSeqRecv(ctx context.Context, msgTransfer provider.PacketInfo, height uint64) (provider.PacketProof, error) {
	nextSeqRecvResponse, err := ap.QueryNextSeqRecv(ctx, int64(height), msgTransfer.DestChannel, msgTransfer.DestPort)
	if err != nil {
		return provider.PacketProof{}, err
	}
	return provider.PacketProof{
		Proof:       nextSeqRecvResponse.Proof,
		ProofHeight: nextSeqRecvResponse.ProofHeight,
	}, nil
}

func (ap *WasmProvider) MsgTransfer(dstAddr string, amount sdk.Coin, info provider.PacketInfo) (provider.RelayerMessage, error) {
	panic(fmt.Sprintf("%s%s", ap.ChainName(), NOT_IMPLEMENTED))
}

func (ap *WasmProvider) MsgRecvPacket(msgTransfer provider.PacketInfo, proof provider.PacketProof) (provider.RelayerMessage, error) {
	signer, err := ap.Address()
	if err != nil {
		return nil, err
	}

	params := &chantypes.MsgRecvPacket{
		Packet:          msgTransfer.Packet(),
		ProofCommitment: proof.Proof,
		ProofHeight:     proof.ProofHeight,
		Signer:          signer,
	}
	return ap.NewWasmContractMessage(MethodRecvPacket, params)
}

func (ap *WasmProvider) MsgAcknowledgement(msgRecvPacket provider.PacketInfo, proof provider.PacketProof) (provider.RelayerMessage, error) {
	signer, err := ap.Address()
	if err != nil {
		return nil, err
	}

	params := &chantypes.MsgAcknowledgement{
		Packet:          msgRecvPacket.Packet(),
		Acknowledgement: msgRecvPacket.Ack,
		ProofAcked:      proof.Proof,
		ProofHeight:     proof.ProofHeight,
		Signer:          signer,
	}
	return ap.NewWasmContractMessage(MethodAcknowledgePacket, params)

}

func (ap *WasmProvider) MsgTimeout(msgTransfer provider.PacketInfo, proof provider.PacketProof) (provider.RelayerMessage, error) {
	signer, err := ap.Address()
	if err != nil {
		return nil, err
	}

	params := &chantypes.MsgTimeout{
		Packet:           msgTransfer.Packet(),
		ProofUnreceived:  proof.Proof,
		ProofHeight:      proof.ProofHeight,
		NextSequenceRecv: msgTransfer.Sequence,
		Signer:           signer,
	}

	return ap.NewWasmContractMessage(MethodTimeoutPacket, params)
}

func (ap *WasmProvider) MsgTimeoutRequest(msgTransfer provider.PacketInfo, proof provider.PacketProof) (provider.RelayerMessage, error) {
	panic(fmt.Sprintf("%s%s", ap.ChainName(), NOT_IMPLEMENTED))
}

// panic
func (ap *WasmProvider) MsgTimeoutOnClose(msgTransfer provider.PacketInfo, proofUnreceived provider.PacketProof) (provider.RelayerMessage, error) {
	panic(fmt.Sprintf("%s%s", ap.ChainName(), NOT_IMPLEMENTED))
}

func (ap *WasmProvider) ConnectionHandshakeProof(ctx context.Context, msgOpenInit provider.ConnectionInfo, height uint64) (provider.ConnectionProof, error) {
	clientState, clientStateProof, consensusStateProof, connStateProof, proofHeight, err := ap.GenerateConnHandshakeProof(ctx, int64(height), msgOpenInit.ClientID, msgOpenInit.ConnID)
	if err != nil {
		return provider.ConnectionProof{}, err
	}

	return provider.ConnectionProof{
		ClientState:          clientState,
		ClientStateProof:     clientStateProof,
		ConsensusStateProof:  consensusStateProof,
		ConnectionStateProof: connStateProof,
		ProofHeight:          proofHeight.(clienttypes.Height),
	}, nil
}

func (ap *WasmProvider) ConnectionProof(ctx context.Context, msgOpenAck provider.ConnectionInfo, height uint64) (provider.ConnectionProof, error) {
	connState, err := ap.QueryConnection(ctx, int64(height), msgOpenAck.ConnID)
	if err != nil {
		return provider.ConnectionProof{}, err
	}
	return provider.ConnectionProof{
		ConnectionStateProof: connState.Proof,
		ProofHeight:          connState.ProofHeight,
	}, nil
}

func (ap *WasmProvider) MsgConnectionOpenInit(info provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	signer, err := ap.Address()
	if err != nil {
		return nil, err
	}
	params := &conntypes.MsgConnectionOpenInit{
		ClientId: info.ClientID,
		Counterparty: conntypes.Counterparty{
			ClientId:     info.CounterpartyClientID,
			ConnectionId: "",
			Prefix:       info.CounterpartyCommitmentPrefix,
		},
		Version:     nil,
		DelayPeriod: defaultDelayPeriod,
		Signer:      signer,
	}

	return ap.NewWasmContractMessage(MethodConnectionOpenInit, params)

}

func (ap *WasmProvider) MsgConnectionOpenTry(msgOpenInit provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	signer, err := ap.Address()
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
		Prefix:       msgOpenInit.CommitmentPrefix,
	}

	params := &conntypes.MsgConnectionOpenTry{
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

	return ap.NewWasmContractMessage(MethodConnectionOpenTry, params)

}

func (ap *WasmProvider) MsgConnectionOpenAck(msgOpenTry provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	signer, err := ap.Address()
	if err != nil {
		return nil, err
	}

	csAny, err := clienttypes.PackClientState(proof.ClientState)
	if err != nil {
		return nil, err
	}

	params := &conntypes.MsgConnectionOpenAck{
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

	return ap.NewWasmContractMessage(MethodConnectionOpenAck, params)
}

func (ap *WasmProvider) MsgConnectionOpenConfirm(msgOpenAck provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	signer, err := ap.Address()
	if err != nil {
		return nil, err
	}
	params := &conntypes.MsgConnectionOpenConfirm{
		ConnectionId: msgOpenAck.CounterpartyConnID,
		ProofAck:     proof.ConnectionStateProof,
		ProofHeight:  proof.ProofHeight,
		Signer:       signer,
	}
	return ap.NewWasmContractMessage(MethodConnectionOpenConfirm, params)
}

func (ap *WasmProvider) ChannelProof(ctx context.Context, msg provider.ChannelInfo, height uint64) (provider.ChannelProof, error) {
	channelResult, err := ap.QueryChannel(ctx, int64(height), msg.ChannelID, msg.PortID)
	if err != nil {
		return provider.ChannelProof{}, nil
	}
	return provider.ChannelProof{
		Proof: channelResult.Proof,
		ProofHeight: clienttypes.Height{
			RevisionNumber: 0,
			RevisionHeight: height,
		},
		Ordering: chantypes.Order(channelResult.Channel.GetOrdering()),
		Version:  channelResult.Channel.Version,
	}, nil
}

func (ap *WasmProvider) MsgChannelOpenInit(info provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	signer, err := ap.Address()
	if err != nil {
		return nil, err
	}
	params := &chantypes.MsgChannelOpenInit{
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
	return ap.NewWasmContractMessage(MethodChannelOpenInit, params)
}

func (ap *WasmProvider) MsgChannelOpenTry(msgOpenInit provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	signer, err := ap.Address()
	if err != nil {
		return nil, err
	}

	params := &chantypes.MsgChannelOpenTry{
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
	return ap.NewWasmContractMessage(MethodChannelOpenTry, params)
}

func (ap *WasmProvider) MsgChannelOpenAck(msgOpenTry provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	signer, err := ap.Address()
	if err != nil {
		return nil, err
	}

	params := &chantypes.MsgChannelOpenAck{
		PortId:                msgOpenTry.CounterpartyPortID,
		ChannelId:             msgOpenTry.CounterpartyChannelID,
		CounterpartyChannelId: msgOpenTry.ChannelID,
		CounterpartyVersion:   proof.Version,
		ProofTry:              proof.Proof,
		ProofHeight:           proof.ProofHeight,
		Signer:                signer,
	}
	return ap.NewWasmContractMessage(MethodChannelOpenAck, params)
}

func (ap *WasmProvider) MsgChannelOpenConfirm(msgOpenAck provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	signer, err := ap.Address()
	if err != nil {
		return nil, err
	}

	params := &chantypes.MsgChannelOpenConfirm{
		PortId:      msgOpenAck.CounterpartyPortID,
		ChannelId:   msgOpenAck.CounterpartyChannelID,
		ProofAck:    proof.Proof,
		ProofHeight: proof.ProofHeight,
		Signer:      signer,
	}
	return ap.NewWasmContractMessage(MethodChannelOpenConfirm, params)
}

func (ap *WasmProvider) MsgChannelCloseInit(info provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	signer, err := ap.Address()
	if err != nil {
		return nil, err
	}

	params := &chantypes.MsgChannelCloseInit{
		PortId:    info.PortID,
		ChannelId: info.ChannelID,
		Signer:    signer,
	}

	return ap.NewWasmContractMessage(MethodChannelCloseInit, params)
}

func (ap *WasmProvider) MsgChannelCloseConfirm(msgCloseInit provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	signer, err := ap.Address()
	if err != nil {
		return nil, err
	}

	params := &chantypes.MsgChannelCloseConfirm{
		PortId:      msgCloseInit.CounterpartyPortID,
		ChannelId:   msgCloseInit.CounterpartyChannelID,
		ProofInit:   proof.Proof,
		ProofHeight: proof.ProofHeight,
		Signer:      signer,
	}

	return ap.NewWasmContractMessage(MethodChannelCloseConfirm, params)
}

func (ap *WasmProvider) MsgUpdateClientHeader(latestHeader provider.IBCHeader, trustedHeight clienttypes.Height, trustedHeader provider.IBCHeader, clientType string) (ibcexported.ClientMessage, error) {
	trustedWasmHeader, ok := trustedHeader.(WasmIBCHeader)
	if !ok {
		return nil, fmt.Errorf("unsupported IBC trusted header type, expected: TendermintIBCHeader, actual: %T", trustedHeader)
	}

	latestWasmHeader, ok := latestHeader.(WasmIBCHeader)
	if !ok {
		return nil, fmt.Errorf("unsupported IBC header type, expected: TendermintIBCHeader, actual: %T", latestHeader)
	}

	return &itm.TmHeader{
		SignedHeader:      latestWasmHeader.SignedHeader,
		ValidatorSet:      latestWasmHeader.ValidatorSet,
		TrustedValidators: trustedWasmHeader.ValidatorSet,
		TrustedHeight:     int64(trustedHeight.RevisionHeight),
	}, nil
}

func (ap *WasmProvider) MsgUpdateClient(clientID string, dstHeader ibcexported.ClientMessage) (provider.RelayerMessage, error) {
	signer, err := ap.Address()
	if err != nil {
		return nil, err
	}
	clientMsg, err := clienttypes.PackClientMessage(dstHeader)
	if err != nil {
		return nil, err
	}

	params := &clienttypes.MsgUpdateClient{
		ClientId:      clientID,
		ClientMessage: clientMsg,
		Signer:        signer,
	}

	return ap.NewWasmContractMessage(MethodUpdateClient, params)

}

func (ap *WasmProvider) QueryICQWithProof(ctx context.Context, msgType string, request []byte, height uint64) (provider.ICQProof, error) {
	panic(fmt.Sprintf("%s%s", ap.ChainName(), NOT_IMPLEMENTED))
}

func (ap *WasmProvider) MsgSubmitQueryResponse(chainID string, queryID provider.ClientICQQueryID, proof provider.ICQProof) (provider.RelayerMessage, error) {
	panic(fmt.Sprintf("%s%s", ap.ChainName(), NOT_IMPLEMENTED))
}

func (ap *WasmProvider) RelayPacketFromSequence(ctx context.Context, src provider.ChainProvider, srch, dsth, seq uint64, srcChanID, srcPortID string, order chantypes.Order) (provider.RelayerMessage, provider.RelayerMessage, error) {
	panic(fmt.Sprintf("%s%s", ap.ChainName(), NOT_IMPLEMENTED))
}

func (ap *WasmProvider) AcknowledgementFromSequence(ctx context.Context, dst provider.ChainProvider, dsth, seq uint64, dstChanID, dstPortID, srcChanID, srcPortID string) (provider.RelayerMessage, error) {
	panic(fmt.Sprintf("%s%s", ap.ChainName(), NOT_IMPLEMENTED))
}

func (ap *WasmProvider) SendMessage(ctx context.Context, msg provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {
	return ap.SendMessages(ctx, []provider.RelayerMessage{msg}, memo)
}

func (ap *WasmProvider) SendMessages(ctx context.Context, msgs []provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {
	var (
		rlyResp     *provider.RelayerTxResponse
		callbackErr error
		wg          sync.WaitGroup
	)

	callback := func(rtr *provider.RelayerTxResponse, err error) {
		callbackErr = err

		if err != nil {
			wg.Done()
			return
		}

		for i, e := range rtr.Events {
			if startsWithWasm(e.EventType) {
				rtr.Events[i].EventType = findEventType(e.EventType)
			}
		}
		rlyResp = rtr
		wg.Done()
	}

	wg.Add(1)

	if err := retry.Do(func() error {
		return ap.SendMessagesToMempool(ctx, msgs, memo, ctx, callback)
	}, retry.Context(ctx), rtyAtt, rtyDel, rtyErr, retry.OnRetry(func(n uint, err error) {
		ap.log.Info(
			"Error building or broadcasting transaction",
			zap.String("chain_id", ap.PCfg.ChainID),
			zap.Uint("attempt", n+1),
			zap.Uint("max_attempts", rtyAttNum),
			zap.Error(err),
		)
	})); err != nil {
		return nil, false, err
	}

	wg.Wait()

	if callbackErr != nil {
		return rlyResp, false, callbackErr
	}

	if rlyResp.Code != 0 {
		return rlyResp, false, fmt.Errorf("transaction failed with code: %d", rlyResp.Code)
	}

	return rlyResp, true, callbackErr
}

func (ap *WasmProvider) SendMessagesToMempool(
	ctx context.Context,
	msgs []provider.RelayerMessage,
	memo string,

	asyncCtx context.Context,
	asyncCallback func(*provider.RelayerTxResponse, error),
) error {
	ap.txMu.Lock()
	defer ap.txMu.Unlock()

	cliCtx := ap.ClientContext()
	factory, err := ap.PrepareFactory(ap.TxFactory())
	if err != nil {
		return err
	}

	// uncomment for saving msg
	// SaveMsgToFile(WasmDebugMessagePath, msgs)

	for _, msg := range msgs {
		if msg == nil {
			ap.log.Debug("One of the message of is nil ")
			continue
		}

		wasmMsg, ok := msg.(*WasmContractMessage)
		if !ok {
			return fmt.Errorf("Wasm Message is not valid %s", wasmMsg.Type())
		}

		txBytes, sequence, err := ap.buildMessages(cliCtx, factory, wasmMsg.Msg)
		if err != nil {
			return err
		}

		// if msg.Type() == MethodUpdateClient {
		// 	if err := retry.Do(func() error {
		// 		if err := ap.BroadcastTx(cliCtx, txBytes, []provider.RelayerMessage{msg}, asyncCtx, defaultBroadcastWaitTimeout, asyncCallback, true); err != nil {
		// 			if strings.Contains(err.Error(), sdkerrors.ErrWrongSequence.Error()) {
		// 				ap.handleAccountSequenceMismatchError(err)
		// 			}
		// 		}
		// 		return err
		// 	}, retry.Context(ctx), rtyAtt, retry.Delay(time.Millisecond*time.Duration(ap.PCfg.BlockInterval)), rtyErr); err != nil {
		// 		ap.log.Error("Failed to update client", zap.Any("Message", msg))
		// 		return err
		// 	}
		// 	continue
		// }
		if err := ap.BroadcastTx(cliCtx, txBytes, []provider.RelayerMessage{msg}, asyncCtx, defaultBroadcastWaitTimeout, asyncCallback, false); err != nil {
			if strings.Contains(err.Error(), sdkerrors.ErrWrongSequence.Error()) {
				ap.handleAccountSequenceMismatchError(err)
			}
		}
		ap.updateNextAccountSequence(sequence + 1)
	}

	return nil

}

func (ap *WasmProvider) LogFailedTx(res *provider.RelayerTxResponse, err error, msgs []provider.RelayerMessage) {

	fields := []zapcore.Field{zap.String("chain_id", ap.ChainId())}
	// if res != nil {
	// 		channels := getChannelsIfPresent(res.Events)
	// 		fields = append(fields, channels...)
	// 	}
	fields = append(fields, msgTypesField(msgs))

	if err != nil {
		// Make a copy since we may continue to the warning
		errorFields := append(fields, zap.Error(err))
		ap.log.Error(
			"Failed sending wasm transaction",
			errorFields...,
		)

		if res == nil {
			return
		}
	}
	if res.Code != 0 {
		if sdkErr := ap.sdkError(res.Codespace, res.Code); err != nil {
			fields = append(fields, zap.NamedError("sdk_error", sdkErr))
		}
		fields = append(fields, zap.Object("response", res))
		ap.log.Warn(
			"Sent transaction but received failure response",
			fields...,
		)
	}
}

// LogSuccessTx take the transaction and the messages to create it and logs the appropriate data
func (ap *WasmProvider) LogSuccessTx(res *sdk.TxResponse, msgs []provider.RelayerMessage) {
	// Include the chain_id
	fields := []zapcore.Field{zap.String("chain_id", ap.ChainId())}
	done := ap.SetSDKContext()
	defer done()

	// Include the gas used
	fields = append(fields, zap.Int64("gas_used", res.GasUsed))

	// Extract fees and fee_payer if present
	ir := types.NewInterfaceRegistry()
	var m sdk.Msg
	if err := ir.UnpackAny(res.Tx, &m); err == nil {
		if tx, ok := m.(*txtypes.Tx); ok {
			fields = append(fields, zap.Stringer("fees", tx.GetFee()))
			if feePayer := getFeePayer(tx); feePayer != "" {
				fields = append(fields, zap.String("fee_payer", feePayer))
			}
		} else {
			ap.log.Debug(
				"Failed to convert message to Tx type",
				zap.Stringer("type", reflect.TypeOf(m)),
			)
		}
	} else {
		ap.log.Debug("Failed to unpack response Tx into sdk.Msg", zap.Error(err))
	}

	// Include the height, msgType, and tx_hash
	fields = append(fields,
		zap.Int64("height", res.Height),
		msgTypesField(msgs),
		zap.String("tx_hash", res.TxHash),
	)

	// Log the succesful transaction with fields
	ap.log.Info(
		"Successful transaction",
		fields...,
	)

}

// getFeePayer returns the bech32 address of the fee payer of a transaction.
// This uses the fee payer field if set,
// otherwise falls back to the address of whoever signed the first message.
func getFeePayer(tx *txtypes.Tx) string {
	payer := tx.AuthInfo.Fee.Payer
	if payer != "" {
		return payer
	}
	switch firstMsg := tx.GetMsgs()[0].(type) {

	case *clienttypes.MsgCreateClient:
		// Without this particular special case, there is a panic in ibc-go
		// due to the sdk config singleton expecting one bech32 prefix but seeing another.
		return firstMsg.Signer
	case *clienttypes.MsgUpdateClient:
		// Same failure mode as MsgCreateClient.
		return firstMsg.Signer
	default:
		return firstMsg.GetSigners()[0].String()
	}

}

func (ap *WasmProvider) sdkError(codespace string, code uint32) error {
	// ABCIError will return an error other than "unknown" if syncRes.Code is a registered error in syncRes.Codespace
	// This catches all of the sdk errors https://github.com/cosmos/cosmos-sdk/blob/f10f5e5974d2ecbf9efc05bc0bfe1c99fdeed4b6/types/errors/errors.go
	err := errors.Unwrap(sdkerrors.ABCIError(codespace, code, "error broadcasting transaction"))
	if err.Error() != errUnknown {
		return err
	}
	return nil
}

func (ap *WasmProvider) buildMessages(clientCtx client.Context, txf tx.Factory, msgs ...sdk.Msg) ([]byte, uint64, error) {
	done := ap.SetSDKContext()
	defer done()

	for _, msg := range msgs {
		if err := msg.ValidateBasic(); err != nil {
			return nil, 0, err
		}
	}

	// If the --aux flag is set, we simply generate and print the AuxSignerData.
	// tf is aux? do we need it? prolly not
	if clientCtx.IsAux {
		auxSignerData, err := makeAuxSignerData(clientCtx, txf, msgs...)
		if err != nil {
			return nil, 0, err
		}

		return nil, 0, clientCtx.PrintProto(&auxSignerData)
	}

	if clientCtx.GenerateOnly {
		return nil, 0, txf.PrintUnsignedTx(clientCtx, msgs...)
	}

	txf, err := txf.Prepare(clientCtx)
	if err != nil {
		return nil, 0, err
	}

	sequence := txf.Sequence()
	ap.updateNextAccountSequence(sequence)
	if sequence < ap.nextAccountSeq {
		sequence = ap.nextAccountSeq
		txf = txf.WithSequence(sequence)
	}

	if txf.SimulateAndExecute() || clientCtx.Simulate {
		if clientCtx.Offline {
			return nil, 0, errors.New("cannot estimate gas in offline mode")
		}

		_, adjusted, err := tx.CalculateGas(clientCtx, txf, msgs...)
		if err != nil {
			return nil, 0, err
		}

		txf = txf.WithGas(adjusted)
		// _, _ = fmt.Fprintf(os.Stderr, "%s\n", tx.GasEstimateResponse{GasEstimate: txf.Gas()})
	}

	if clientCtx.Simulate {
		return nil, 0, nil
	}

	txn, err := txf.BuildUnsignedTx(msgs...)
	if err != nil {
		return nil, 0, err
	}

	if !clientCtx.SkipConfirm {
		txBytes, err := clientCtx.TxConfig.TxJSONEncoder()(txn.GetTx())
		if err != nil {
			return nil, 0, err
		}

		if err := clientCtx.PrintRaw(json.RawMessage(txBytes)); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "%s\n", txBytes)
		}

		buf := bufio.NewReader(os.Stdin)
		ok, err := input.GetConfirmation("confirm transaction before signing and broadcasting", buf, os.Stderr)

		if err != nil || !ok {
			_, _ = fmt.Fprintf(os.Stderr, "%s\n", "cancelled transaction")
			return nil, 0, err
		}
	}

	err = tx.Sign(txf, clientCtx.GetFromName(), txn, true)
	if err != nil {
		return nil, 0, err
	}

	res, err := clientCtx.TxConfig.TxEncoder()(txn.GetTx())
	return res, sequence, nil
}

func (ap *WasmProvider) BroadcastTx(
	clientCtx client.Context,
	txBytes []byte,
	msgs []provider.RelayerMessage,
	asyncCtx context.Context, // context for async wait for block inclusion after successful tx broadcast
	asyncTimeout time.Duration, // timeout for waiting for block inclusion
	asyncCallback func(*provider.RelayerTxResponse, error), // callback for success/fail of the wait for block inclusion
	shouldWait bool,
) error {
	res, err := clientCtx.BroadcastTx(txBytes)
	// log submitted txn

	isErr := err != nil
	isFailed := res != nil && res.Code != 0
	if isErr || isFailed {
		if isErr && res == nil {
			// There are some cases where BroadcastTxSync will return an error but the associated
			// ResultBroadcastTx will be nil.
			return err
		}
		rlyResp := &provider.RelayerTxResponse{
			TxHash:    res.TxHash,
			Codespace: res.Codespace,
			Code:      res.Code,
			Data:      res.Data,
		}
		if isFailed {
			err = ap.sdkError(res.Codespace, res.Code)
			if err == nil {
				err = fmt.Errorf("transaction failed to execute")
			}
		}
		ap.LogFailedTx(rlyResp, err, msgs)
		return err
	}

	hexTx, err := hex.DecodeString(res.TxHash)
	if err != nil {
		return err
	}

	ap.log.Info("Submitted transaction",
		zap.String("chain_id", ap.PCfg.ChainID),
		zap.String("tx_hash", res.TxHash),
		msgTypesField(msgs),
	)

	if shouldWait {
		return ap.waitForTx(asyncCtx, hexTx, msgs, asyncTimeout, asyncCallback)
	}
	go ap.waitForTx(asyncCtx, hexTx, msgs, asyncTimeout, asyncCallback)
	return nil
}

// BroadcastTx attempts to generate, sign and broadcast a transaction with the
// given set of messages. It will also simulate gas requirements if necessary.
// It will return an error upon failure.
// UNUSED: PANIC
func (ap *WasmProvider) broadcastTx(
	ctx context.Context, // context for tx broadcast
	tx []byte, // raw tx to be broadcasted
	msgs []provider.RelayerMessage, // used for logging only
	fees sdk.Coins, // used for metrics

	asyncCtx context.Context, // context for async wait for block inclusion after successful tx broadcast
	asyncTimeout time.Duration, // timeout for waiting for block inclusion
	asyncCallback func(*provider.RelayerTxResponse, error), // callback for success/fail of the wait for block inclusion
) error {
	panic(fmt.Sprintf("%s%s", ap.ChainName(), NOT_IMPLEMENTED))
}

func (ap *WasmProvider) waitForTx(
	ctx context.Context,
	txHash []byte,
	msgs []provider.RelayerMessage, // used for logging only
	waitTimeout time.Duration,
	callback func(*provider.RelayerTxResponse, error),
) error {
	res, err := ap.waitForTxResult(ctx, txHash, waitTimeout)
	if err != nil {
		ap.log.Error("Failed to wait for block inclusion", zap.Error(err))
		if callback != nil {
			callback(nil, err)
		}
		return err
	}

	rlyResp := &provider.RelayerTxResponse{
		Height:    res.Height,
		TxHash:    res.TxHash,
		Codespace: res.Codespace,
		Code:      res.Code,
		Data:      res.Data,
		Events:    parseEventsFromTxResponse(res),
	}

	// transaction was executed, log the success or failure using the tx response code
	// NOTE: error is nil, logic should use the returned error to determine if the
	// transaction was successfully executed.

	if res.Code != 0 {
		// Check for any registered SDK errors
		err := ap.sdkError(res.Codespace, res.Code)
		if err == nil {
			err = fmt.Errorf("transaction failed to execute")
		}
		if callback != nil {
			callback(nil, err)
		}
		ap.LogFailedTx(rlyResp, nil, msgs)
		return err
	}

	if callback != nil {
		callback(rlyResp, nil)
	}
	ap.LogSuccessTx(res, msgs)
	return nil
}

func (ap *WasmProvider) waitForTxResult(
	ctx context.Context,
	txHash []byte,
	waitTimeout time.Duration,
) (*sdk.TxResponse, error) {
	exitAfter := time.After(waitTimeout)
	for {
		select {
		case <-exitAfter:
			return nil, fmt.Errorf("timed out after: %d; %s", waitTimeout, ErrTimeoutAfterWaitingForTxBroadcast)
		case <-time.After(time.Millisecond * 100):
			res, err := ap.RPCClient.Tx(ctx, txHash, false)
			if err == nil && res == nil {
				continue
			}
			if err == nil {
				return ap.mkTxResult(res)
			}
			if strings.Contains(err.Error(), "transaction indexing is disabled") {
				return nil, fmt.Errorf("cannot determine success/failure of tx because transaction indexing is disabled on rpc url")
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func msgTypesField(msgs []provider.RelayerMessage) zap.Field {
	msgTypes := make([]string, len(msgs))
	for i, m := range msgs {
		if m == nil {
			continue
		}
		msgTypes[i] = m.Type()
	}
	return zap.Strings("msg_types", msgTypes)
}

const (
	ErrTimeoutAfterWaitingForTxBroadcast string = "timed out after waiting for tx to get included in the block"
)

type intoAny interface {
	AsAny() *codectypes.Any
}

func (ap *WasmProvider) mkTxResult(resTx *coretypes.ResultTx) (*sdk.TxResponse, error) {
	txbz, err := ap.Cdc.TxConfig.TxDecoder()(resTx.Tx)
	if err != nil {
		return nil, err
	}
	p, ok := txbz.(intoAny)
	if !ok {
		return nil, fmt.Errorf("expecting a type implementing intoAny, got: %T", txbz)
	}
	any := p.AsAny()
	return sdk.NewResponseResultTx(resTx, any, ""), nil
}

func makeAuxSignerData(clientCtx client.Context, f tx.Factory, msgs ...sdk.Msg) (txtypes.AuxSignerData, error) {
	b := tx.NewAuxTxBuilder()
	fromAddress, name, _, err := client.GetFromFields(clientCtx, clientCtx.Keyring, clientCtx.From)
	if err != nil {
		return txtypes.AuxSignerData{}, err
	}

	b.SetAddress(fromAddress.String())
	if clientCtx.Offline {
		b.SetAccountNumber(f.AccountNumber())
		b.SetSequence(f.Sequence())
	} else {
		accNum, seq, err := clientCtx.AccountRetriever.GetAccountNumberSequence(clientCtx, fromAddress)
		if err != nil {
			return txtypes.AuxSignerData{}, err
		}
		b.SetAccountNumber(accNum)
		b.SetSequence(seq)
	}

	err = b.SetMsgs(msgs...)
	if err != nil {
		return txtypes.AuxSignerData{}, err
	}

	// if f.tip != nil {
	// 	if _, err := sdk.AccAddressFromBech32(f.tip.Tipper); err != nil {
	// 		return txtypes.AuxSignerData{}, sdkerrors.ErrInvalidAddress.Wrap("tipper must be a bech32 address")
	// 	}
	// 	b.SetTip(f.tip)
	// }

	err = b.SetSignMode(f.SignMode())
	if err != nil {
		return txtypes.AuxSignerData{}, err
	}

	key, err := clientCtx.Keyring.Key(name)
	if err != nil {
		return txtypes.AuxSignerData{}, err
	}

	pub, err := key.GetPubKey()
	if err != nil {
		return txtypes.AuxSignerData{}, err
	}

	err = b.SetPubKey(pub)
	if err != nil {
		return txtypes.AuxSignerData{}, err
	}

	b.SetChainID(clientCtx.ChainID)
	signBz, err := b.GetSignBytes()
	if err != nil {
		return txtypes.AuxSignerData{}, err
	}

	sig, _, err := clientCtx.Keyring.Sign(name, signBz)
	if err != nil {
		return txtypes.AuxSignerData{}, err
	}
	b.SetSignature(sig)

	return b.GetAuxSignerData()
}

func parseEventsFromTxResponse(resp *sdk.TxResponse) []provider.RelayerEvent {
	var events []provider.RelayerEvent

	if resp == nil {
		return events
	}

	for _, logs := range resp.Logs {
		for _, event := range logs.Events {
			attributes := make(map[string]string)
			for _, attribute := range event.Attributes {
				attributes[attribute.Key] = attribute.Value
			}
			events = append(events, provider.RelayerEvent{
				EventType:  event.Type,
				Attributes: attributes,
			})
		}
	}
	return events
}

// QueryABCI performs an ABCI query and returns the appropriate response and error sdk error code.
func (cc *WasmProvider) QueryABCI(ctx context.Context, req abci.RequestQuery) (abci.ResponseQuery, error) {
	opts := rpcclient.ABCIQueryOptions{
		Height: req.Height,
		Prove:  req.Prove,
	}
	result, err := cc.RPCClient.ABCIQueryWithOptions(ctx, req.Path, req.Data, opts)
	if err != nil {
		return abci.ResponseQuery{}, err
	}

	if !result.Response.IsOK() {
		return abci.ResponseQuery{}, sdkErrorToGRPCError(result.Response)
	}

	// data from trusted node or subspace query doesn't need verification
	if !opts.Prove || !isQueryStoreWithProof(req.Path) {
		return result.Response, nil
	}

	return result.Response, nil
}

func (cc *WasmProvider) handleAccountSequenceMismatchError(err error) {

	clientCtx := cc.ClientContext()
	_, seq, err := cc.ClientCtx.AccountRetriever.GetAccountNumberSequence(clientCtx, clientCtx.GetFromAddress())

	// sequences := numRegex.FindAllString(err.Error(), -1)
	// if len(sequences) != 2 {
	// 	return
	// }
	// nextSeq, err := strconv.ParseUint(sequences[0], 10, 64)
	if err != nil {
		return
	}

	cc.nextAccountSeq = seq
}

func sdkErrorToGRPCError(resp abci.ResponseQuery) error {
	switch resp.Code {
	case sdkerrors.ErrInvalidRequest.ABCICode():
		return status.Error(codes.InvalidArgument, resp.Log)
	case sdkerrors.ErrUnauthorized.ABCICode():
		return status.Error(codes.Unauthenticated, resp.Log)
	case sdkerrors.ErrKeyNotFound.ABCICode():
		return status.Error(codes.NotFound, resp.Log)
	default:
		return status.Error(codes.Unknown, resp.Log)
	}
}

// isQueryStoreWithProof expects a format like /<queryType>/<storeName>/<subpath>
// queryType must be "store" and subpath must be "key" to require a proof.
func isQueryStoreWithProof(path string) bool {
	if !strings.HasPrefix(path, "/") {
		return false
	}

	paths := strings.SplitN(path[1:], "/", 3)

	switch {
	case len(paths) != 3:
		return false
	case paths[0] != "store":
		return false
	case rootmulti.RequireProof("/" + paths[2]):
		return true
	}

	return false
}
