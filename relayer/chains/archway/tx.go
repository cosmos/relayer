package archway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/avast/retry-go/v4"
	abci "github.com/cometbft/cometbft/abci/types"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	commitmenttypes "github.com/cosmos/ibc-go/v7/modules/core/23-commitment/types"
	"github.com/cosmos/relayer/v2/relayer/chains/archway/types"
	"github.com/cosmos/relayer/v2/relayer/provider"

	"github.com/cosmos/cosmos-sdk/client"
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
	defaultChainPrefix = commitmenttypes.NewMerklePrefix([]byte("ibc"))
	defaultDelayPeriod = uint64(0)
)

func (ap *ArchwayProvider) TxFactory() tx.Factory {
	return tx.Factory{}.
		WithAccountRetriever(ap).
		WithChainID(ap.PCfg.ChainID).
		WithTxConfig(ap.Cdc.TxConfig).
		WithGasAdjustment(ap.PCfg.GasAdjustment).
		WithGasPrices(ap.PCfg.GasPrices).
		WithKeybase(ap.Keybase).
		WithSignMode(ap.PCfg.SignMode())
}

// PrepareFactory mutates the tx factory with the appropriate account number, sequence number, and min gas settings.
func (ap *ArchwayProvider) PrepareFactory(txf tx.Factory) (tx.Factory, error) {
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

func (pc *ArchwayProviderConfig) SignMode() signing.SignMode {
	signMode := signing.SignMode_SIGN_MODE_UNSPECIFIED
	switch pc.SignModeStr {
	case "direct":
		signMode = signing.SignMode_SIGN_MODE_DIRECT
	case "amino-json":
		signMode = signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON
	}
	return signMode
}

func (ap *ArchwayProvider) NewClientState(dstChainID string, dstIBCHeader provider.IBCHeader, dstTrustingPeriod, dstUbdPeriod time.Duration, allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour bool) (ibcexported.ClientState, error) {

	btpHeader := dstIBCHeader.(*iconchain.IconIBCHeader)

	return &icon.ClientState{
		TrustingPeriod:     uint64(dstTrustingPeriod),
		FrozenHeight:       0,
		MaxClockDrift:      20 * 60,
		LatestHeight:       dstIBCHeader.Height(),
		NetworkSectionHash: btpHeader.Header.PrevNetworkSectionHash,
		Validators:         btpHeader.ValidatorSet,
	}, nil
}

func (ap *ArchwayProvider) NewClientStateMock(dstChainID string, dstIBCHeader provider.IBCHeader, dstTrustingPeriod, dstUbdPeriod time.Duration, allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour bool) (ibcexported.ClientState, error) {

	btpHeader := dstIBCHeader.(*iconchain.IconIBCHeader)

	return &icon.ClientState{
		TrustingPeriod:     uint64(dstTrustingPeriod),
		FrozenHeight:       0,
		MaxClockDrift:      20 * 60,
		LatestHeight:       dstIBCHeader.Height(),
		NetworkSectionHash: btpHeader.Header.PrevNetworkSectionHash,
		Validators:         btpHeader.ValidatorSet,
	}, nil
}

func (ap *ArchwayProvider) MsgCreateClient(clientState ibcexported.ClientState, consensusState ibcexported.ConsensusState) (provider.RelayerMessage, error) {
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

	msg := types.MsgCreateClient(anyClientState, anyConsensusState, signer)

	msgParam, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	return NewWasmContractMessage(signer, ap.PCfg.IbcHandlerAddress, msgParam), nil
}

func (ap *ArchwayProvider) MsgUpgradeClient(srcClientId string, consRes *clienttypes.QueryConsensusStateResponse, clientRes *clienttypes.QueryClientStateResponse) (provider.RelayerMessage, error) {
	return nil, fmt.Errorf("Not implemented for Archway")
}

func (ap *ArchwayProvider) MsgSubmitMisbehaviour(clientID string, misbehaviour ibcexported.ClientMessage) (provider.RelayerMessage, error) {
	return nil, fmt.Errorf("Not implemented for Archway")
}

func (ap *ArchwayProvider) ValidatePacket(msgTransfer provider.PacketInfo, latest provider.LatestBlock) error {
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

	revision := clienttypes.ParseChainID(ap.PCfg.ChainID)
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

func (ap *ArchwayProvider) PacketCommitment(ctx context.Context, msgTransfer provider.PacketInfo, height uint64) (provider.PacketProof, error) {
	// TODO : Proofs
	return provider.PacketProof{}, nil
}

func (ap *ArchwayProvider) PacketAcknowledgement(ctx context.Context, msgRecvPacket provider.PacketInfo, height uint64) (provider.PacketProof, error) {
	// TODO: Proods
	return provider.PacketProof{}, nil
}

func (ap *ArchwayProvider) PacketReceipt(ctx context.Context, msgTransfer provider.PacketInfo, height uint64) (provider.PacketProof, error) {
	// TODO: Proofs
	return provider.PacketProof{}, nil
}

func (ap *ArchwayProvider) NextSeqRecv(ctx context.Context, msgTransfer provider.PacketInfo, height uint64) (provider.PacketProof, error) {
	// TODO: Proofs
	return provider.PacketProof{}, nil
}

func (ap *ArchwayProvider) MsgTransfer(dstAddr string, amount sdk.Coin, info provider.PacketInfo) (provider.RelayerMessage, error) {
	return nil, fmt.Errorf("Not implemented for Archway")
}

func (ap *ArchwayProvider) MsgRecvPacket(msgTransfer provider.PacketInfo, proof provider.PacketProof) (provider.RelayerMessage, error) {
	signer, err := ap.Address()
	if err != nil {
		return nil, err
	}

	msg := &types.ReceivePacket{
		Msg: chantypes.MsgRecvPacket{
			Packet:          msgTransfer.Packet(),
			ProofCommitment: proof.Proof,
			ProofHeight:     proof.ProofHeight,
			Signer:          signer,
		}}

	msgParam, err := json.Marshal(msg)

	if err != nil {
		return nil, err
	}
	return NewWasmContractMessage(signer, ap.PCfg.IbcHandlerAddress, msgParam), nil
}

func (ap *ArchwayProvider) MsgAcknowledgement(msgRecvPacket provider.PacketInfo, proof provider.PacketProof) (provider.RelayerMessage, error) {
	signer, err := ap.Address()
	if err != nil {
		return nil, err
	}

	msg := &types.AcknowledgementPacket{
		Msg: chantypes.MsgAcknowledgement{
			Packet:          msgRecvPacket.Packet(),
			Acknowledgement: msgRecvPacket.Ack,
			ProofAcked:      proof.Proof,
			ProofHeight:     proof.ProofHeight,
			Signer:          signer,
		},
	}

	msgParam, err := json.Marshal(msg)

	if err != nil {
		return nil, err
	}
	return NewWasmContractMessage(signer, ap.PCfg.IbcHandlerAddress, msgParam), nil
}

func (ap *ArchwayProvider) MsgTimeout(msgTransfer provider.PacketInfo, proof provider.PacketProof) (provider.RelayerMessage, error) {
	signer, err := ap.Address()
	if err != nil {
		return nil, err
	}

	msg := &types.TimeoutPacket{
		Msg: chantypes.MsgTimeout{
			Packet:           msgTransfer.Packet(),
			ProofUnreceived:  proof.Proof,
			ProofHeight:      proof.ProofHeight,
			NextSequenceRecv: msgTransfer.Sequence,
			Signer:           signer,
		},
	}

	msgParam, err := json.Marshal(msg)

	if err != nil {
		return nil, err
	}
	return NewWasmContractMessage(signer, ap.PCfg.IbcHandlerAddress, msgParam), nil
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
	signer, err := ap.Address()
	if err != nil {
		return nil, err
	}
	msg := &types.ConnectionOpenInit{
		Msg: conntypes.MsgConnectionOpenInit{
			ClientId: info.ClientID,
			Counterparty: conntypes.Counterparty{
				ClientId:     info.CounterpartyClientID,
				ConnectionId: "",
				Prefix:       info.CounterpartyCommitmentPrefix,
			},
			Version:     nil,
			DelayPeriod: defaultDelayPeriod,
			Signer:      signer,
		},
	}
	msgParam, err := json.Marshal(msg)

	if err != nil {
		return nil, err
	}
	return NewWasmContractMessage(signer, ap.PCfg.IbcHandlerAddress, msgParam), nil
}

func (ap *ArchwayProvider) MsgConnectionOpenTry(msgOpenInit provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
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
		Prefix:       defaultChainPrefix,
	}

	msg := &types.ConnectionOpenTry{
		Msg: conntypes.MsgConnectionOpenTry{
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
		}}

	msgParam, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	return NewWasmContractMessage(signer, ap.PCfg.IbcHandlerAddress, msgParam), nil
}

func (ap *ArchwayProvider) MsgConnectionOpenAck(msgOpenTry provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	signer, err := ap.Address()
	if err != nil {
		return nil, err
	}

	csAny, err := clienttypes.PackClientState(proof.ClientState)
	if err != nil {
		return nil, err
	}

	msg := &types.ConnectionOpenAck{
		Msg: conntypes.MsgConnectionOpenAck{
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
		}}
	msgParam, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	return NewWasmContractMessage(signer, ap.PCfg.IbcHandlerAddress, msgParam), nil
}

func (ap *ArchwayProvider) MsgConnectionOpenConfirm(msgOpenAck provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	signer, err := ap.Address()
	if err != nil {
		return nil, err
	}
	msg := &types.ConnectionOpenConfirm{
		Msg: conntypes.MsgConnectionOpenConfirm{
			ConnectionId: msgOpenAck.CounterpartyConnID,
			ProofAck:     proof.ConnectionStateProof,
			ProofHeight:  proof.ProofHeight,
			Signer:       signer,
		}}
	msgParam, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	return NewWasmContractMessage(signer, ap.PCfg.IbcHandlerAddress, msgParam), nil
}

func (ap *ArchwayProvider) ChannelProof(ctx context.Context, msg provider.ChannelInfo, height uint64) (provider.ChannelProof, error) {
	return provider.ChannelProof{}, nil
}

func (ap *ArchwayProvider) MsgChannelOpenInit(info provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	signer, err := ap.Address()
	if err != nil {
		return nil, err
	}
	msg := &types.ChannelOpenInit{
		Msg: chantypes.MsgChannelOpenInit{
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
		}}
	msgParam, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	return NewWasmContractMessage(signer, ap.PCfg.IbcHandlerAddress, msgParam), nil
}

func (ap *ArchwayProvider) MsgChannelOpenTry(msgOpenInit provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	signer, err := ap.Address()
	if err != nil {
		return nil, err
	}

	msg := &types.ChannelOpenTry{
		Msg: chantypes.MsgChannelOpenTry{
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
		}}
	msgParam, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	return NewWasmContractMessage(signer, ap.PCfg.IbcHandlerAddress, msgParam), nil
}

func (ap *ArchwayProvider) MsgChannelOpenAck(msgOpenTry provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	signer, err := ap.Address()
	if err != nil {
		return nil, err
	}

	msg := &types.ChannelOpenAck{
		Msg: chantypes.MsgChannelOpenAck{
			PortId:                msgOpenTry.CounterpartyPortID,
			ChannelId:             msgOpenTry.CounterpartyChannelID,
			CounterpartyChannelId: msgOpenTry.ChannelID,
			CounterpartyVersion:   proof.Version,
			ProofTry:              proof.Proof,
			ProofHeight:           proof.ProofHeight,
			Signer:                signer,
		},
	}
	msgParam, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	return NewWasmContractMessage(signer, ap.PCfg.IbcHandlerAddress, msgParam), nil
}

func (ap *ArchwayProvider) MsgChannelOpenConfirm(msgOpenAck provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	signer, err := ap.Address()
	if err != nil {
		return nil, err
	}

	msg := &types.ChannelOpenConfirm{
		Msg: chantypes.MsgChannelOpenConfirm{
			PortId:      msgOpenAck.CounterpartyPortID,
			ChannelId:   msgOpenAck.CounterpartyChannelID,
			ProofAck:    proof.Proof,
			ProofHeight: proof.ProofHeight,
			Signer:      signer,
		},
	}
	msgParam, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	return NewWasmContractMessage(signer, ap.PCfg.IbcHandlerAddress, msgParam), nil
}

func (ap *ArchwayProvider) MsgChannelCloseInit(info provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	signer, err := ap.Address()
	if err != nil {
		return nil, err
	}

	msg := &types.ChannelCloseInit{
		Msg: chantypes.MsgChannelCloseInit{
			PortId:    info.PortID,
			ChannelId: info.ChannelID,
			Signer:    signer,
		},
	}
	msgParam, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	return NewWasmContractMessage(signer, ap.PCfg.IbcHandlerAddress, msgParam), nil
}

func (ap *ArchwayProvider) MsgChannelCloseConfirm(msgCloseInit provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	signer, err := ap.Address()
	if err != nil {
		return nil, err
	}

	msg := &types.ChannelCloseConfirm{
		Msg: chantypes.MsgChannelCloseConfirm{
			PortId:      msgCloseInit.CounterpartyPortID,
			ChannelId:   msgCloseInit.CounterpartyChannelID,
			ProofInit:   proof.Proof,
			ProofHeight: proof.ProofHeight,
			Signer:      signer,
		},
	}
	msgParam, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	return NewWasmContractMessage(signer, ap.PCfg.IbcHandlerAddress, msgParam), nil
}

func (ap *ArchwayProvider) MsgUpdateClientHeader(latestHeader provider.IBCHeader, trustedHeight clienttypes.Height, trustedHeader provider.IBCHeader) (ibcexported.ClientMessage, error) {
	return nil, nil
}

func (ap *ArchwayProvider) MsgUpdateClient(clientID string, dstHeader ibcexported.ClientMessage) (provider.RelayerMessage, error) {
	signer, err := ap.Address()
	if err != nil {
		return nil, err
	}
	clientMsg, err := clienttypes.PackClientMessage(dstHeader)
	if err != nil {
		return nil, err
	}
	msg := types.MsgUpdateClient(clientID, clientMsg, signer)
	msgParam, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	return NewWasmContractMessage(signer, ap.PCfg.IbcHandlerAddress, msgParam), nil
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

// broadcastTx broadcasts a transaction with the given raw bytes and then, in an async goroutine, waits for the tx to be included in the block.
// The wait will end after either the asyncTimeout has run out or the asyncCtx exits.
// If there is no error broadcasting, the asyncCallback will be called with success/failure of the wait for block inclusion.
func (ap *ArchwayProvider) broadcastTx(
	ctx context.Context, // context for tx broadcast
	tx []byte, // raw tx to be broadcasted
	msgs []provider.RelayerMessage, // used for logging only
	fees sdk.Coins, // used for metrics

	asyncCtx context.Context, // context for async wait for block inclusion after successful tx broadcast
	asyncTimeout time.Duration, // timeout for waiting for block inclusion
	asyncCallback func(*provider.RelayerTxResponse, error), // callback for success/fail of the wait for block inclusion
) error {
	return nil
}

// QueryABCI performs an ABCI query and returns the appropriate response and error sdk error code.
func (cc *ArchwayProvider) QueryABCI(ctx context.Context, req abci.RequestQuery) (abci.ResponseQuery, error) {
	opts := rpcclient.ABCIQueryOptions{
		Height: req.Height,
		Prove:  req.Prove,
	}
	result, err := cc.RPCClient.ABCIQueryWithOptions(ctx, req.Path, req.Data, opts)
	if err != nil {
		return abci.ResponseQuery{}, err
	}

	// if !result.Response.IsOK() {
	// 	return abci.ResponseQuery{}, sdkErrorToGRPCError(result.Response)
	// }

	// // data from trusted node or subspace query doesn't need verification
	// if !opts.Prove || !isQueryStoreWithProof(req.Path) {
	// 	return result.Response, nil
	// }

	return result.Response, nil
}
