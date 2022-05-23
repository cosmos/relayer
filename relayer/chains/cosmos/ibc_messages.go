package cosmos

import (
	"errors"
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	commitmenttypes "github.com/cosmos/ibc-go/v3/modules/core/23-commitment/types"
	host "github.com/cosmos/ibc-go/v3/modules/core/24-host"
	ibcexported "github.com/cosmos/ibc-go/v3/modules/core/exported"
	tmclient "github.com/cosmos/ibc-go/v3/modules/light-clients/07-tendermint/types"
	"github.com/cosmos/relayer/v2/relayer/ibc"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	abci "github.com/tendermint/tendermint/abci/types"
)

func (ccp *CosmosChainProcessor) GetMsgRecvPacket(signer string, msgRecvPacket provider.RelayerMessage) (provider.RelayerMessage, error) {
	if msgRecvPacket == nil {
		return nil, errors.New("msg is nil")
	}
	cosmosMsg := cosmos.CosmosMsg(msgRecvPacket)
	if cosmosMsg == nil {
		return nil, errors.New("cosmosMsg is nil")
	}
	msg, ok := cosmosMsg.(*chantypes.MsgRecvPacket)
	if !ok {
		return nil, errors.New("error casting msg to chantypes.MsgRecvPacket")
	}

	key := host.PacketCommitmentKey(msg.Packet.SourcePort, msg.Packet.SourceChannel, msg.Packet.Sequence)
	_, proof, proofHeight, err := ccp.QueryTendermintProof(ccp.latestHeight(), key)
	if err != nil {
		return nil, fmt.Errorf("error querying tendermint proof for packet: %w", err)
	}

	msg.ProofCommitment = proof
	msg.ProofHeight = proofHeight
	msg.Signer = signer

	return cosmos.NewCosmosMessage(msg), nil
}

func (ccp *CosmosChainProcessor) GetMsgAcknowledgement(signer string, msgAcknowledgement provider.RelayerMessage) (provider.RelayerMessage, error) {
	if msgAcknowledgement == nil {
		return nil, errors.New("msg is nil")
	}
	cosmosMsg := cosmos.CosmosMsg(msgAcknowledgement)
	if cosmosMsg == nil {
		return nil, errors.New("cosmosMsg is nil")
	}
	msg, ok := cosmosMsg.(*chantypes.MsgAcknowledgement)
	if !ok {
		return nil, errors.New("error casting msg to chantypes.MsgAcknowledgement")
	}

	key := host.PacketAcknowledgementKey(msg.Packet.SourcePort, msg.Packet.SourceChannel, msg.Packet.Sequence)
	_, proof, proofHeight, err := ccp.QueryTendermintProof(ccp.latestHeight(), key)
	if err != nil {
		return nil, fmt.Errorf("error querying tendermint proof for packet: %w", err)
	}

	msg.ProofAcked = proof
	msg.ProofHeight = proofHeight
	msg.Signer = signer

	return cosmos.NewCosmosMessage(msg), nil
}

func (ccp *CosmosChainProcessor) GetMsgTimeout(signer string, msgRecvPacket provider.RelayerMessage) (provider.RelayerMessage, error) {
	if msgRecvPacket == nil {
		return nil, errors.New("msg is nil")
	}
	cosmosMsg := cosmos.CosmosMsg(msgRecvPacket)
	if cosmosMsg == nil {
		return nil, errors.New("cosmosMsg is nil")
	}
	msg, ok := cosmosMsg.(*chantypes.MsgRecvPacket)
	if !ok {
		return nil, errors.New("error casting msg to chantypes.MsgRecvPacket")
	}

	key := host.PacketReceiptKey(msg.Packet.SourcePort, msg.Packet.SourceChannel, msg.Packet.Sequence)
	_, proof, proofHeight, err := ccp.QueryTendermintProof(ccp.latestHeight(), key)
	if err != nil {
		return nil, fmt.Errorf("error querying tendermint proof for packet: %w", err)
	}

	msgTimeout := &chantypes.MsgTimeout{
		Packet:           msg.Packet,
		ProofUnreceived:  proof,
		ProofHeight:      proofHeight,
		NextSequenceRecv: msg.Packet.Sequence,
		Signer:           signer,
	}

	return cosmos.NewCosmosMessage(msgTimeout), nil
}

func (ccp *CosmosChainProcessor) GetMsgTimeoutOnClose(signer string, msgRecvPacket provider.RelayerMessage) (provider.RelayerMessage, error) {
	if msgRecvPacket == nil {
		return nil, errors.New("msg is nil")
	}
	cosmosMsg := cosmos.CosmosMsg(msgRecvPacket)
	if cosmosMsg == nil {
		return nil, errors.New("cosmosMsg is nil")
	}
	msg, ok := cosmosMsg.(*chantypes.MsgRecvPacket)
	if !ok {
		return nil, errors.New("error casting msg to chantypes.MsgRecvPacket")
	}

	key := host.PacketReceiptKey(msg.Packet.SourcePort, msg.Packet.SourceChannel, msg.Packet.Sequence)
	_, proof, proofHeight, err := ccp.QueryTendermintProof(ccp.latestHeight(), key)
	if err != nil {
		return nil, fmt.Errorf("error querying tendermint proof for packet: %w", err)
	}

	msgTimeout := &chantypes.MsgTimeoutOnClose{
		Packet:           msg.Packet,
		ProofUnreceived:  proof,
		ProofHeight:      proofHeight,
		NextSequenceRecv: msg.Packet.Sequence,
		Signer:           signer,
	}

	return cosmos.NewCosmosMessage(msgTimeout), nil
}

func (ccp *CosmosChainProcessor) GetMsgUpdateClient(clientID string, counterpartyChainLatestHeader ibcexported.Header) (provider.RelayerMessage, error) {
	acc, err := ccp.ChainProvider.Address()
	if err != nil {
		return nil, err
	}

	counterpartyHeader, ok := counterpartyChainLatestHeader.(*tmclient.Header)
	if !ok {
		return nil, fmt.Errorf("header is not a tendermint header: %v\n", err)
	}

	clientHeight, err := ccp.ClientHeight(clientID)
	if err != nil {
		return nil, err
	}

	tmHeader := &tmclient.Header{
		SignedHeader:      counterpartyHeader.SignedHeader,
		ValidatorSet:      counterpartyHeader.ValidatorSet,
		TrustedValidators: counterpartyHeader.TrustedValidators,
		TrustedHeight:     clientHeight,
	}

	anyHeader, err := clienttypes.PackHeader(tmHeader)
	if err != nil {
		return nil, err
	}

	msg := &clienttypes.MsgUpdateClient{
		ClientId: clientID,
		Header:   anyHeader,
		Signer:   acc,
	}
	return cosmos.NewCosmosMessage(msg), nil
}

func (ccp *CosmosChainProcessor) latestHeight() int64 {
	ccp.latestBlockLock.Lock()
	defer ccp.latestBlockLock.Unlock()
	return int64(ccp.latestBlock.Height)
}

// makes sure packet is valid to be relayed
// should return ibc.TimeoutError or ibc.TimeoutOnCloseError if packet is timed out so that Timeout can be written to other chain (already handled by PathProcessor)
func (ccp *CosmosChainProcessor) ValidatePacket(msgTransfer provider.RelayerMessage) error {
	if msgTransfer == nil {
		return errors.New("msg is nil")
	}
	cosmosMsg := cosmos.CosmosMsg(msgTransfer)
	if cosmosMsg == nil {
		return errors.New("cosmosMsg is nil")
	}
	msg, ok := cosmosMsg.(*chantypes.MsgRecvPacket)
	if !ok {
		return errors.New("error casting msg to chantypes.MsgRecvPacket")
	}

	if msg.Packet.Sequence == 0 {
		return errors.New("refusing to relay packet with sequence: 0")
	}

	if len(msg.Packet.Data) == 0 {
		return errors.New("refusing to relay packet with empty data")
	}

	// TODO: Are we sure we want to do this? this packet would then never be relayed
	if msg.Packet.TimeoutHeight.IsZero() && msg.Packet.TimeoutTimestamp == 0 {
		return errors.New("refusing to relay packet without a timeout (height or timestamp must be set)")
	}

	latest := ccp.Latest()
	latestClientTypesHeight := clienttypes.Height{RevisionNumber: ccp.revisionNumber, RevisionHeight: latest.Height}
	if !msg.Packet.TimeoutHeight.IsZero() && latestClientTypesHeight.GTE(msg.Packet.TimeoutHeight) {
		return ibc.NewTimeoutError(fmt.Sprintf("Latest height %d is greater than expiration height: %d\n", latest.Height, msg.Packet.TimeoutHeight.RevisionHeight))
	}
	latestTimestamp := uint64(latest.Time.UnixNano())
	if msg.Packet.TimeoutTimestamp > 0 && latestTimestamp > msg.Packet.TimeoutTimestamp {
		return ibc.NewTimeoutError(fmt.Sprintf("Latest block timestamp %d is greater than expiration timestamp: %d\n", latestTimestamp, msg.Packet.TimeoutTimestamp))
	}

	return nil
}

func (ccp *CosmosChainProcessor) QueryTendermintProof(height int64, key []byte) ([]byte, []byte, clienttypes.Height, error) {
	// ABCI queries at heights 1, 2 or less than or equal to 0 are not supported.
	// Base app does not support queries for height less than or equal to 1.
	// Therefore, a query at height 2 would be equivalent to a query at height 3.
	// A height of 0 will query with the lastest state.
	if height != 0 && height <= 2 {
		return nil, nil, clienttypes.Height{}, fmt.Errorf("proof queries at height <= 2 are not supported")
	}

	// Use the IAVL height if a valid tendermint height is passed in.
	// A height of 0 will query with the latest state.
	if height != 0 {
		height--
	}

	req := abci.RequestQuery{
		Path:   fmt.Sprintf("store/%s/key", host.StoreKey),
		Height: height,
		Data:   key,
		Prove:  true,
	}

	res, err := ccp.cc.QueryABCI(req)
	if err != nil {
		return nil, nil, clienttypes.Height{}, err
	}

	merkleProof, err := commitmenttypes.ConvertProofs(res.ProofOps)
	if err != nil {
		return nil, nil, clienttypes.Height{}, err
	}

	cdc := codec.NewProtoCodec(ccp.cc.InterfaceRegistry)

	proofBz, err := cdc.Marshal(&merkleProof)
	if err != nil {
		return nil, nil, clienttypes.Height{}, err
	}

	revision := clienttypes.ParseChainID(ccp.ChainProvider.ChainId())
	return res.Value, proofBz, clienttypes.NewHeight(revision, uint64(res.Height)+1), nil
}
