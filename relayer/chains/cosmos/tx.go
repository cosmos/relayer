package cosmos

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	transfertypes "github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	commitmenttypes "github.com/cosmos/ibc-go/v7/modules/core/23-commitment/types"
	host "github.com/cosmos/ibc-go/v7/modules/core/24-host"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/exported"
	tmclient "github.com/cosmos/ibc-go/v7/modules/light-clients/07-tendermint"
	strideicqtypes "github.com/cosmos/relayer/v2/relayer/chains/cosmos/stride"
	"github.com/cosmos/relayer/v2/relayer/provider"
	lensclient "github.com/strangelove-ventures/lens/client"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/light"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
	"go.uber.org/zap"
)

// Variables used for retries
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
func (cc *CosmosProvider) SendMessage(ctx context.Context, msg provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {
	return cc.SendMessages(ctx, []provider.RelayerMessage{msg}, memo)
}

// SendMessages attempts to sign, encode, & send a slice of RelayerMessages
// This is used extensively in the relayer as an extension of the Provider interface
//
// NOTE: An error is returned if there was an issue sending the transaction. A successfully sent, but failed
// transaction will not return an error. If a transaction is successfully sent, the result of the execution
// of that transaction will be logged. A boolean indicating if a transaction was successfully
// sent and executed successfully is returned.
func (cc *CosmosProvider) SendMessages(ctx context.Context, msgs []provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {
	var resp *sdk.TxResponse
	var fees sdk.Coins

	// Guard against account sequence number mismatch errors by locking for the specific wallet for
	// the account sequence query all the way through the transaction broadcast success/fail.
	cc.txMu.Lock()
	defer cc.txMu.Unlock()

	if err := retry.Do(func() error {
		txBytes, sequence, f, err := cc.buildMessages(ctx, msgs, memo)
		fees = f
		if err != nil {
			errMsg := err.Error()

			// Account sequence mismatch errors can happen on the simulated transaction also.
			if strings.Contains(errMsg, sdkerrors.ErrWrongSequence.Error()) {
				cc.handleAccountSequenceMismatchError(err)
				return err
			}

			// Occasionally the client will be out of date,
			// and we will receive an RPC error like:
			//     rpc error: code = InvalidArgument desc = failed to execute message; message index: 1: channel handshake open try failed: failed channel state verification for client (07-tendermint-0): client state height < proof height ({0 58} < {0 59}), please ensure the client has been updated: invalid height: invalid request
			// or
			//     rpc error: code = InvalidArgument desc = failed to execute message; message index: 1: receive packet verification failed: couldn't verify counterparty packet commitment: failed packet commitment verification for client (07-tendermint-0): client state height < proof height ({0 142} < {0 143}), please ensure the client has been updated: invalid height: invalid request
			//
			// No amount of retrying will fix this. The client needs to be updated.
			// Unfortunately, the entirety of that error message originates on the server,
			// so there is not an obvious way to access a more structured error value.
			//
			// If this logic should ever fail due to the string values of the error messages on the server
			// changing from the client's version of the library,
			// at worst this will run more unnecessary retries.
			if strings.Contains(errMsg, sdkerrors.ErrInvalidHeight.Error()) {
				cc.log.Info(
					"Skipping retry due to invalid height error",
					zap.Error(err),
				)
				return retry.Unrecoverable(err)
			}

			// On a fast retry, it is possible to have an invalid connection state.
			// Retrying that message also won't fix the underlying state mismatch,
			// so log it and mark it as unrecoverable.
			if strings.Contains(errMsg, conntypes.ErrInvalidConnectionState.Error()) {
				cc.log.Info(
					"Skipping retry due to invalid connection state",
					zap.Error(err),
				)
				return retry.Unrecoverable(err)
			}

			// Also possible to have an invalid channel state on a fast retry.
			if strings.Contains(errMsg, chantypes.ErrInvalidChannelState.Error()) {
				cc.log.Info(
					"Skipping retry due to invalid channel state",
					zap.Error(err),
				)
				return retry.Unrecoverable(err)
			}

			// If the message reported an invalid proof, back off.
			// NOTE: this error string ("invalid proof") will match other errors too,
			// but presumably it is safe to stop retrying in those cases as well.
			if strings.Contains(errMsg, commitmenttypes.ErrInvalidProof.Error()) {
				cc.log.Info(
					"Skipping retry due to invalid proof",
					zap.Error(err),
				)
				return retry.Unrecoverable(err)
			}

			// Invalid packets should not be retried either.
			if strings.Contains(errMsg, chantypes.ErrInvalidPacket.Error()) {
				cc.log.Info(
					"Skipping retry due to invalid packet",
					zap.Error(err),
				)
				return retry.Unrecoverable(err)
			}

			return err
		}

		resp, err = cc.BroadcastTx(ctx, txBytes)
		if err != nil {
			if strings.Contains(err.Error(), sdkerrors.ErrWrongSequence.Error()) {
				cc.handleAccountSequenceMismatchError(err)
				return err
			}

			// Don't retry if BroadcastTx resulted in any other error.
			return retry.Unrecoverable(err)
		}

		// we had a successful tx with this sequence, so update it to the next
		cc.updateNextAccountSequence(sequence + 1)

		return nil
	}, retry.Context(ctx), rtyAtt, rtyDel, rtyErr, retry.OnRetry(func(n uint, err error) {
		cc.log.Info(
			"Error building or broadcasting transaction",
			zap.String("chain_id", cc.PCfg.ChainID),
			zap.Uint("attempt", n+1),
			zap.Uint("max_attempts", rtyAttNum),
			zap.Error(err),
		)
	})); err != nil || resp == nil {
		return nil, false, err
	}

	rlyResp := &provider.RelayerTxResponse{
		Height:    resp.Height,
		TxHash:    resp.TxHash,
		Codespace: resp.Codespace,
		Code:      resp.Code,
		Data:      resp.Data,
		Events:    parseEventsFromTxResponse(resp),
	}

	// transaction was executed, log the success or failure using the tx response code
	// NOTE: error is nil, logic should use the returned error to determine if the
	// transaction was successfully executed.
	if rlyResp.Code != 0 {
		cc.LogFailedTx(rlyResp, nil, msgs)
		cc.UpdateFeesSpent(cc.ChainId(), cc.Key(), fees)
		return rlyResp, false, fmt.Errorf("transaction failed with code: %d", resp.Code)
	}

	cc.LogSuccessTx(resp, msgs)
	cc.UpdateFeesSpent(cc.ChainId(), cc.Key(), fees)

	return rlyResp, true, nil
}

// SendMessagesToMempool simulates and broadcasts a transaction with the given msgs and memo.
// This method will return once the transaction has entered the mempool.
// In an async goroutine, will wait for the tx to be included in the block unless asyncCtx exits.
// If there is no error broadcasting, the asyncCallback will be called with success/failure of the wait for block inclusion.
func (cc *CosmosProvider) SendMessagesToMempool(
	ctx context.Context,
	msgs []provider.RelayerMessage,
	memo string,

	asyncCtx context.Context,
	asyncCallback func(*provider.RelayerTxResponse, error),
) error {
	// Guard against account sequence number mismatch errors by locking for the specific wallet for
	// the account sequence query all the way through the transaction broadcast success/fail.
	cc.txMu.Lock()
	defer cc.txMu.Unlock()

	txBytes, sequence, fees, err := cc.buildMessages(ctx, msgs, memo)
	if err != nil {
		// Account sequence mismatch errors can happen on the simulated transaction also.
		if strings.Contains(err.Error(), sdkerrors.ErrWrongSequence.Error()) {
			cc.handleAccountSequenceMismatchError(err)
		}

		return err
	}

	if err := cc.broadcastTx(ctx, txBytes, msgs, fees, asyncCtx, defaultBroadcastWaitTimeout, asyncCallback); err != nil {
		if strings.Contains(err.Error(), sdkerrors.ErrWrongSequence.Error()) {
			cc.handleAccountSequenceMismatchError(err)
		}

		return err
	}

	// we had a successful tx broadcast with this sequence, so update it to the next
	cc.updateNextAccountSequence(sequence + 1)

	return nil
}

// sdkError will return the Cosmos SDK registered error for a given codespace/code combo if registered, otherwise nil.
func (cc *CosmosProvider) sdkError(codespace string, code uint32) error {
	// ABCIError will return an error other than "unknown" if syncRes.Code is a registered error in syncRes.Codespace
	// This catches all of the sdk errors https://github.com/cosmos/cosmos-sdk/blob/f10f5e5974d2ecbf9efc05bc0bfe1c99fdeed4b6/types/errors/errors.go
	err := errors.Unwrap(sdkerrors.ABCIError(codespace, code, "error broadcasting transaction"))
	if err.Error() != errUnknown {
		return err
	}
	return nil
}

// broadcastTx broadcasts a transaction with the given raw bytes and then, in an async goroutine, waits for the tx to be included in the block.
// The wait will end after either the asyncTimeout has run out or the asyncCtx exits.
// If there is no error broadcasting, the asyncCallback will be called with success/failure of the wait for block inclusion.
func (cc *CosmosProvider) broadcastTx(
	ctx context.Context, // context for tx broadcast
	tx []byte, // raw tx to be broadcasted
	msgs []provider.RelayerMessage, // used for logging only
	fees sdk.Coins, // used for metrics

	asyncCtx context.Context, // context for async wait for block inclusion after successful tx broadcast
	asyncTimeout time.Duration, // timeout for waiting for block inclusion
	asyncCallback func(*provider.RelayerTxResponse, error), // callback for success/fail of the wait for block inclusion
) error {
	res, err := cc.ChainClient.RPCClient.BroadcastTxSync(ctx, tx)
	if err != nil {
		if res == nil {
			// There are some cases where BroadcastTxSync will return an error but the associated
			// ResultBroadcastTx will be nil.
			return err
		}
		rlyResp := &provider.RelayerTxResponse{
			TxHash:    res.Hash.String(),
			Codespace: res.Codespace,
			Code:      res.Code,
			Data:      res.Data.String(),
		}
		cc.LogFailedTx(rlyResp, err, msgs)
		return err
	}

	cc.UpdateFeesSpent(cc.ChainId(), cc.Key(), fees)

	// TODO: maybe we need to check if the node has tx indexing enabled?
	// if not, we need to find a new way to block until inclusion in a block

	go cc.waitForTx(asyncCtx, res.Hash, msgs, asyncTimeout, asyncCallback)

	return nil
}

// waitForTx waits for a transaction to be included in a block, logs success/fail, then invokes callback.
// This is intended to be called as an async goroutine.
func (cc *CosmosProvider) waitForTx(
	ctx context.Context,
	txHash []byte,
	msgs []provider.RelayerMessage, // used for logging only
	waitTimeout time.Duration,
	callback func(*provider.RelayerTxResponse, error),
) {
	res, err := cc.waitForBlockInclusion(ctx, txHash, waitTimeout)
	if err != nil {
		cc.log.Error("Failed to wait for block inclusion", zap.Error(err))
		if callback != nil {
			callback(nil, err)
		}
		return
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
		err := cc.sdkError(res.Codespace, res.Code)
		if err == nil {
			err = fmt.Errorf("transaction failed to execute")
		}
		if callback != nil {
			callback(nil, err)
		}
		cc.LogFailedTx(rlyResp, nil, msgs)
		return
	}

	if callback != nil {
		callback(rlyResp, nil)
	}
	cc.LogSuccessTx(res, msgs)
}

// waitForBlockInclusion will wait for a transaction to be included in a block, up to waitTimeout or context cancellation.
func (cc *CosmosProvider) waitForBlockInclusion(
	ctx context.Context,
	txHash []byte,
	waitTimeout time.Duration,
) (*sdk.TxResponse, error) {
	exitAfter := time.After(waitTimeout)
	for {
		select {
		case <-exitAfter:
			return nil, fmt.Errorf("timed out after: %d; %w", waitTimeout, lensclient.ErrTimeoutAfterWaitingForTxBroadcast)
		// This fixed poll is fine because it's only for logging and updating prometheus metrics currently.
		case <-time.After(time.Millisecond * 100):
			res, err := cc.ChainClient.RPCClient.Tx(ctx, txHash, false)
			if err == nil {
				return cc.mkTxResult(res)
			}
			if strings.Contains(err.Error(), "transaction indexing is disabled") {
				return nil, fmt.Errorf("cannot determine success/failure of tx because transaction indexing is disabled on rpc url")
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// mkTxResult decodes a tendermint transaction into an SDK TxResponse.
func (cc *CosmosProvider) mkTxResult(resTx *coretypes.ResultTx) (*sdk.TxResponse, error) {
	txbz, err := cc.ChainClient.Codec.TxConfig.TxDecoder()(resTx.Tx)
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

type intoAny interface {
	AsAny() *codectypes.Any
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

func (cc *CosmosProvider) buildMessages(ctx context.Context, msgs []provider.RelayerMessage, memo string) ([]byte, uint64, sdk.Coins, error) {
	// Query account details
	txf, err := cc.PrepareFactory(cc.TxFactory())
	if err != nil {
		return nil, 0, sdk.Coins{}, err
	}

	if memo != "" {
		txf = txf.WithMemo(memo)
	}

	sequence := txf.Sequence()
	cc.updateNextAccountSequence(sequence)
	if sequence < cc.nextAccountSeq {
		sequence = cc.nextAccountSeq
		txf = txf.WithSequence(sequence)
	}

	// TODO: Make this work with new CalculateGas method
	// TODO: This is related to GRPC client stuff?
	// https://github.com/cosmos/cosmos-sdk/blob/5725659684fc93790a63981c653feee33ecf3225/client/tx/tx.go#L297
	// If users pass gas adjustment, then calculate gas
	_, adjusted, err := cc.CalculateGas(ctx, txf, CosmosMsgs(msgs...)...)
	if err != nil {
		return nil, 0, sdk.Coins{}, err
	}

	// Set the gas amount on the transaction factory
	txf = txf.WithGas(adjusted)

	var txb client.TxBuilder
	// Build the transaction builder & retry on failures
	if err := retry.Do(func() error {
		txb, err = txf.BuildUnsignedTx(CosmosMsgs(msgs...)...)
		if err != nil {
			return err
		}
		return nil
	}, retry.Context(ctx), rtyAtt, rtyDel, rtyErr); err != nil {
		return nil, 0, sdk.Coins{}, err
	}

	done := cc.SetSDKContext()

	if err := retry.Do(func() error {
		if err := tx.Sign(txf, cc.PCfg.Key, txb, false); err != nil {
			return err
		}
		return nil
	}, retry.Context(ctx), rtyAtt, rtyDel, rtyErr); err != nil {
		return nil, 0, sdk.Coins{}, err
	}

	done()

	tx := txb.GetTx()
	fees := tx.GetFee()

	var txBytes []byte
	// Generate the transaction bytes
	if err := retry.Do(func() error {
		var err error
		txBytes, err = cc.Codec.TxConfig.TxEncoder()(tx)
		if err != nil {
			return err
		}
		return nil
	}, retry.Context(ctx), rtyAtt, rtyDel, rtyErr); err != nil {
		return nil, 0, sdk.Coins{}, err
	}

	return txBytes, sequence, fees, nil
}

// handleAccountSequenceMismatchError will parse the error string, e.g.:
// "account sequence mismatch, expected 10, got 9: incorrect account sequence"
// and update the next account sequence with the expected value.
func (cc *CosmosProvider) handleAccountSequenceMismatchError(err error) {
	sequences := numRegex.FindAllString(err.Error(), -1)
	if len(sequences) != 2 {
		return
	}
	nextSeq, err := strconv.ParseUint(sequences[0], 10, 64)
	if err != nil {
		return
	}
	cc.nextAccountSeq = nextSeq
}

// MsgCreateClient creates an sdk.Msg to update the client on src with consensus state from dst
func (cc *CosmosProvider) MsgCreateClient(
	clientState ibcexported.ClientState,
	consensusState ibcexported.ConsensusState,
) (provider.RelayerMessage, error) {
	signer, err := cc.Address()
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

	return NewCosmosMessage(msg), nil
}

func (cc *CosmosProvider) MsgUpdateClient(srcClientID string, dstHeader ibcexported.ClientMessage) (provider.RelayerMessage, error) {
	acc, err := cc.Address()
	if err != nil {
		return nil, err
	}

	clientMsg, err := clienttypes.PackClientMessage(dstHeader)
	if err != nil {
		return nil, err
	}
	msg := &clienttypes.MsgUpdateClient{
		ClientId:      srcClientID,
		ClientMessage: clientMsg,
		Signer:        acc,
	}
	return NewCosmosMessage(msg), nil
}

func (cc *CosmosProvider) MsgUpgradeClient(srcClientId string, consRes *clienttypes.QueryConsensusStateResponse, clientRes *clienttypes.QueryClientStateResponse) (provider.RelayerMessage, error) {
	var (
		acc string
		err error
	)
	if acc, err = cc.Address(); err != nil {
		return nil, err
	}
	return NewCosmosMessage(&clienttypes.MsgUpgradeClient{ClientId: srcClientId, ClientState: clientRes.ClientState,
		ConsensusState: consRes.ConsensusState, ProofUpgradeClient: consRes.GetProof(),
		ProofUpgradeConsensusState: consRes.ConsensusState.Value, Signer: acc}), nil
}

// mustGetHeight takes the height inteface and returns the actual height
func mustGetHeight(h ibcexported.Height) clienttypes.Height {
	height, ok := h.(clienttypes.Height)
	if !ok {
		panic("height is not an instance of height!")
	}
	return height
}

// MsgTransfer creates a new transfer message
func (cc *CosmosProvider) MsgTransfer(
	dstAddr string,
	amount sdk.Coin,
	info provider.PacketInfo,
) (provider.RelayerMessage, error) {
	acc, err := cc.Address()
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

	return NewCosmosMessage(msg), nil
}

func (cc *CosmosProvider) ValidatePacket(msgTransfer provider.PacketInfo, latest provider.LatestBlock) error {
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

	revision := clienttypes.ParseChainID(cc.PCfg.ChainID)
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

func (cc *CosmosProvider) PacketCommitment(
	ctx context.Context,
	msgTransfer provider.PacketInfo,
	height uint64,
) (provider.PacketProof, error) {
	key := host.PacketCommitmentKey(msgTransfer.SourcePort, msgTransfer.SourceChannel, msgTransfer.Sequence)
	commitment, proof, proofHeight, err := cc.QueryTendermintProof(ctx, int64(height), key)
	if err != nil {
		return provider.PacketProof{}, fmt.Errorf("error querying tendermint proof for packet commitment: %w", err)
	}
	// check if packet commitment exists
	if len(commitment) == 0 {
		return provider.PacketProof{}, chantypes.ErrPacketCommitmentNotFound
	}

	return provider.PacketProof{
		Proof:       proof,
		ProofHeight: proofHeight,
	}, nil
}

func (cc *CosmosProvider) MsgRecvPacket(
	msgTransfer provider.PacketInfo,
	proof provider.PacketProof,
) (provider.RelayerMessage, error) {
	signer, err := cc.Address()
	if err != nil {
		return nil, err
	}
	msg := &chantypes.MsgRecvPacket{
		Packet:          msgTransfer.Packet(),
		ProofCommitment: proof.Proof,
		ProofHeight:     proof.ProofHeight,
		Signer:          signer,
	}

	return NewCosmosMessage(msg), nil
}

func (cc *CosmosProvider) PacketAcknowledgement(
	ctx context.Context,
	msgRecvPacket provider.PacketInfo,
	height uint64,
) (provider.PacketProof, error) {
	key := host.PacketAcknowledgementKey(msgRecvPacket.DestPort, msgRecvPacket.DestChannel, msgRecvPacket.Sequence)
	ack, proof, proofHeight, err := cc.QueryTendermintProof(ctx, int64(height), key)
	if err != nil {
		return provider.PacketProof{}, fmt.Errorf("error querying tendermint proof for packet acknowledgement: %w", err)
	}
	if len(ack) == 0 {
		return provider.PacketProof{}, chantypes.ErrInvalidAcknowledgement
	}
	return provider.PacketProof{
		Proof:       proof,
		ProofHeight: proofHeight,
	}, nil
}

func (cc *CosmosProvider) MsgAcknowledgement(
	msgRecvPacket provider.PacketInfo,
	proof provider.PacketProof,
) (provider.RelayerMessage, error) {
	signer, err := cc.Address()
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

	return NewCosmosMessage(msg), nil
}

func (cc *CosmosProvider) PacketReceipt(
	ctx context.Context,
	msgTransfer provider.PacketInfo,
	height uint64,
) (provider.PacketProof, error) {
	key := host.PacketReceiptKey(msgTransfer.DestPort, msgTransfer.DestChannel, msgTransfer.Sequence)
	_, proof, proofHeight, err := cc.QueryTendermintProof(ctx, int64(height), key)
	if err != nil {
		return provider.PacketProof{}, fmt.Errorf("error querying tendermint proof for packet receipt: %w", err)
	}

	return provider.PacketProof{
		Proof:       proof,
		ProofHeight: proofHeight,
	}, nil
}

// NextSeqRecv queries for the appropriate Tendermint proof required to prove the next expected packet sequence number
// for a given counterparty channel. This is used in ORDERED channels to ensure packets are being delivered in the
// exact same order as they were sent over the wire.
func (cc *CosmosProvider) NextSeqRecv(
	ctx context.Context,
	msgTransfer provider.PacketInfo,
	height uint64,
) (provider.PacketProof, error) {
	key := host.NextSequenceRecvKey(msgTransfer.DestPort, msgTransfer.DestChannel)
	_, proof, proofHeight, err := cc.QueryTendermintProof(ctx, int64(height), key)
	if err != nil {
		return provider.PacketProof{}, fmt.Errorf("error querying tendermint proof for next sequence receive: %w", err)
	}

	return provider.PacketProof{
		Proof:       proof,
		ProofHeight: proofHeight,
	}, nil
}

func (cc *CosmosProvider) MsgTimeout(msgTransfer provider.PacketInfo, proof provider.PacketProof) (provider.RelayerMessage, error) {
	signer, err := cc.Address()
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

	return NewCosmosMessage(assembled), nil
}

func (cc *CosmosProvider) MsgTimeoutOnClose(msgTransfer provider.PacketInfo, proof provider.PacketProof) (provider.RelayerMessage, error) {
	signer, err := cc.Address()
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

	return NewCosmosMessage(assembled), nil
}

func (cc *CosmosProvider) MsgConnectionOpenInit(info provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	signer, err := cc.Address()
	if err != nil {
		return nil, err
	}
	msg := &conntypes.MsgConnectionOpenInit{
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

	return NewCosmosMessage(msg), nil
}

func (cc *CosmosProvider) ConnectionHandshakeProof(
	ctx context.Context,
	msgOpenInit provider.ConnectionInfo,
	height uint64,
) (provider.ConnectionProof, error) {
	clientState, clientStateProof, consensusStateProof, connStateProof, proofHeight, err := cc.GenerateConnHandshakeProof(ctx, int64(height), msgOpenInit.ClientID, msgOpenInit.ConnID)
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

func (cc *CosmosProvider) MsgConnectionOpenTry(msgOpenInit provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	signer, err := cc.Address()
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

	return NewCosmosMessage(msg), nil
}

func (cc *CosmosProvider) MsgConnectionOpenAck(msgOpenTry provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	signer, err := cc.Address()
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

	return NewCosmosMessage(msg), nil
}

func (cc *CosmosProvider) ConnectionProof(
	ctx context.Context,
	msgOpenAck provider.ConnectionInfo,
	height uint64,
) (provider.ConnectionProof, error) {
	connState, err := cc.QueryConnection(ctx, int64(height), msgOpenAck.ConnID)
	if err != nil {
		return provider.ConnectionProof{}, err
	}

	return provider.ConnectionProof{
		ConnectionStateProof: connState.Proof,
		ProofHeight:          connState.ProofHeight,
	}, nil
}

func (cc *CosmosProvider) MsgConnectionOpenConfirm(msgOpenAck provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	signer, err := cc.Address()
	if err != nil {
		return nil, err
	}
	msg := &conntypes.MsgConnectionOpenConfirm{
		ConnectionId: msgOpenAck.CounterpartyConnID,
		ProofAck:     proof.ConnectionStateProof,
		ProofHeight:  proof.ProofHeight,
		Signer:       signer,
	}

	return NewCosmosMessage(msg), nil
}

func (cc *CosmosProvider) MsgChannelOpenInit(info provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	signer, err := cc.Address()
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

	return NewCosmosMessage(msg), nil
}

func (cc *CosmosProvider) ChannelProof(
	ctx context.Context,
	msg provider.ChannelInfo,
	height uint64,
) (provider.ChannelProof, error) {
	channelRes, err := cc.QueryChannel(ctx, int64(height), msg.ChannelID, msg.PortID)
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

func (cc *CosmosProvider) MsgChannelOpenTry(msgOpenInit provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	signer, err := cc.Address()
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

	return NewCosmosMessage(msg), nil
}

func (cc *CosmosProvider) MsgChannelOpenAck(msgOpenTry provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	signer, err := cc.Address()
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

	return NewCosmosMessage(msg), nil
}

func (cc *CosmosProvider) MsgChannelOpenConfirm(msgOpenAck provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	signer, err := cc.Address()
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

	return NewCosmosMessage(msg), nil
}

func (cc *CosmosProvider) MsgChannelCloseInit(info provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	signer, err := cc.Address()
	if err != nil {
		return nil, err
	}
	msg := &chantypes.MsgChannelCloseInit{
		PortId:    info.PortID,
		ChannelId: info.ChannelID,
		Signer:    signer,
	}

	return NewCosmosMessage(msg), nil
}

func (cc *CosmosProvider) MsgChannelCloseConfirm(msgCloseInit provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	signer, err := cc.Address()
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

	return NewCosmosMessage(msg), nil
}

func (cc *CosmosProvider) MsgUpdateClientHeader(latestHeader provider.IBCHeader, trustedHeight clienttypes.Height, trustedHeader provider.IBCHeader) (ibcexported.ClientMessage, error) {
	trustedCosmosHeader, ok := trustedHeader.(CosmosIBCHeader)
	if !ok {
		return nil, fmt.Errorf("unsupported IBC trusted header type, expected: CosmosIBCHeader, actual: %T", trustedHeader)
	}

	latestCosmosHeader, ok := latestHeader.(CosmosIBCHeader)
	if !ok {
		return nil, fmt.Errorf("unsupported IBC header type, expected: CosmosIBCHeader, actual: %T", latestHeader)
	}

	trustedValidatorsProto, err := trustedCosmosHeader.ValidatorSet.ToProto()
	if err != nil {
		return nil, fmt.Errorf("error converting trusted validators to proto object: %w", err)
	}

	signedHeaderProto := latestCosmosHeader.SignedHeader.ToProto()

	validatorSetProto, err := latestCosmosHeader.ValidatorSet.ToProto()
	if err != nil {
		return nil, fmt.Errorf("error converting validator set to proto object: %w", err)
	}

	return &tmclient.Header{
		SignedHeader:      signedHeaderProto,
		ValidatorSet:      validatorSetProto,
		TrustedValidators: trustedValidatorsProto,
		TrustedHeight:     trustedHeight,
	}, nil
}

func (cc *CosmosProvider) QueryICQWithProof(ctx context.Context, path string, request []byte, height uint64) (provider.ICQProof, error) {
	slashSplit := strings.Split(path, "/")
	req := abci.RequestQuery{
		Path:   path,
		Height: int64(height),
		Data:   request,
		Prove:  slashSplit[len(slashSplit)-1] == "key",
	}

	res, err := cc.QueryABCI(ctx, req)
	if err != nil {
		return provider.ICQProof{}, fmt.Errorf("failed to execute interchain query: %w", err)
	}
	return provider.ICQProof{
		Result:   res.Value,
		ProofOps: res.ProofOps,
		Height:   res.Height,
	}, nil
}

func (cc *CosmosProvider) MsgSubmitQueryResponse(chainID string, queryID provider.ClientICQQueryID, proof provider.ICQProof) (provider.RelayerMessage, error) {
	signer, err := cc.Address()
	if err != nil {
		return nil, err
	}
	msg := &strideicqtypes.MsgSubmitQueryResponse{
		ChainId:     chainID,
		QueryId:     string(queryID),
		Result:      proof.Result,
		ProofOps:    proof.ProofOps,
		Height:      proof.Height,
		FromAddress: signer,
	}

	return NewCosmosMessage(msg), nil
}

// RelayPacketFromSequence relays a packet with a given seq on src and returns recvPacket msgs, timeoutPacketmsgs and error
func (cc *CosmosProvider) RelayPacketFromSequence(
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

	dstTime, err := cc.BlockTime(ctx, int64(dsth))
	if err != nil {
		return nil, nil, err
	}

	if err := cc.ValidatePacket(msgTransfer, provider.LatestBlock{
		Height: dsth,
		Time:   dstTime,
	}); err != nil {
		switch err.(type) {
		case *provider.TimeoutHeightError, *provider.TimeoutTimestampError, *provider.TimeoutOnCloseError:
			var pp provider.PacketProof
			switch order {
			case chantypes.UNORDERED:
				pp, err = cc.PacketReceipt(ctx, msgTransfer, dsth)
				if err != nil {
					return nil, nil, err
				}
			case chantypes.ORDERED:
				pp, err = cc.NextSeqRecv(ctx, msgTransfer, dsth)
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

	packet, err := cc.MsgRecvPacket(msgTransfer, pp)
	if err != nil {
		return nil, nil, err
	}

	return packet, nil, nil
}

// AcknowledgementFromSequence relays an acknowledgement with a given seq on src, source is the sending chain, destination is the receiving chain
func (cc *CosmosProvider) AcknowledgementFromSequence(ctx context.Context, dst provider.ChainProvider, dsth, seq uint64, dstChanId, dstPortId, srcChanId, srcPortId string) (provider.RelayerMessage, error) {
	msgRecvPacket, err := dst.QueryRecvPacket(ctx, dstChanId, dstPortId, seq)
	if err != nil {
		return nil, err
	}

	pp, err := dst.PacketAcknowledgement(ctx, msgRecvPacket, dsth)
	if err != nil {
		return nil, err
	}
	msg, err := cc.MsgAcknowledgement(msgRecvPacket, pp)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// QueryIBCHeader returns the IBC compatible block header (CosmosIBCHeader) at a specific height.
func (cc *CosmosProvider) QueryIBCHeader(ctx context.Context, h int64) (provider.IBCHeader, error) {
	if h == 0 {
		return nil, fmt.Errorf("height cannot be 0")
	}

	lightBlock, err := cc.LightProvider.LightBlock(ctx, h)
	if err != nil {
		return nil, err
	}

	return CosmosIBCHeader{
		SignedHeader: lightBlock.SignedHeader,
		ValidatorSet: lightBlock.ValidatorSet,
	}, nil
}

// InjectTrustedFields injects the necessary trusted fields for a header to update a light
// client stored on the destination chain, using the information provided by the source
// chain.
// TrustedHeight is the latest height of the IBC client on dst
// TrustedValidators is the validator set of srcChain at the TrustedHeight
// InjectTrustedFields returns a copy of the header with TrustedFields modified
func (cc *CosmosProvider) InjectTrustedFields(ctx context.Context, header ibcexported.ClientMessage, dst provider.ChainProvider, dstClientId string) (ibcexported.ClientMessage, error) {
	// make copy of header stored in mop
	h, ok := header.(*tmclient.Header)
	if !ok {
		return nil, fmt.Errorf("trying to inject fields into non-tendermint headers")
	}

	// retrieve dst client from src chain
	// this is the client that will be updated
	cs, err := dst.QueryClientState(ctx, int64(h.TrustedHeight.RevisionHeight), dstClientId)
	if err != nil {
		return nil, err
	}

	// inject TrustedHeight as latest height stored on dst client
	h.TrustedHeight = cs.GetLatestHeight().(clienttypes.Height)

	// NOTE: We need to get validators from the source chain at height: trustedHeight+1
	// since the last trusted validators for a header at height h is the NextValidators
	// at h+1 committed to in header h by NextValidatorsHash

	// TODO: this is likely a source of off by 1 errors but may be impossible to change? Maybe this is the
	// place where we need to fix the upstream query proof issue?
	var trustedValidators *tmtypes.ValidatorSet
	if err := retry.Do(func() error {
		ibcHeader, err := cc.QueryIBCHeader(ctx, int64(h.TrustedHeight.RevisionHeight+1))
		if err != nil {
			return err
		}

		trustedValidators = ibcHeader.(CosmosIBCHeader).ValidatorSet
		return err
	}, retry.Context(ctx), rtyAtt, rtyDel, rtyErr); err != nil {
		return nil, fmt.Errorf(
			"failed to get trusted header, please ensure header at the height %d has not been pruned by the connected node: %w",
			h.TrustedHeight.RevisionHeight, err,
		)
	}

	tvProto, err := trustedValidators.ToProto()
	if err != nil {
		return nil, fmt.Errorf("failed to convert trusted validators to proto: %w", err)
	}

	// inject TrustedValidators into header
	h.TrustedValidators = tvProto
	return h, nil
}

// queryTMClientState retrieves the latest consensus state for a client in state at a given height
// and unpacks/cast it to tendermint clientstate
func (cc *CosmosProvider) queryTMClientState(ctx context.Context, srch int64, srcClientId string) (*tmclient.ClientState, error) {
	clientStateRes, err := cc.QueryClientStateResponse(ctx, srch, srcClientId)
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
func (cc *CosmosProvider) NewClientState(
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

func (cc *CosmosProvider) UpdateFeesSpent(chain, key string, fees sdk.Coins) {
	// Don't set the metrics in testing
	if cc.metrics == nil {
		return
	}

	cc.totalFeesMu.Lock()
	cc.TotalFees = cc.TotalFees.Add(fees...)
	cc.totalFeesMu.Unlock()

	for _, fee := range cc.TotalFees {
		// Convert to a big float to get a float64 for metrics
		f, _ := big.NewFloat(0.0).SetInt(fee.Amount.BigInt()).Float64()
		cc.metrics.SetFeesSpent(chain, key, fee.GetDenom(), f)
	}
}
