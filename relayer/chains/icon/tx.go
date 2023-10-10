package icon

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/cosmos/gogoproto/proto"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/exported"

	// tmclient "github.com/cosmos/ibc-go/v7/modules/light-clients/07-tendermint"
	itm "github.com/icon-project/IBC-Integration/libraries/go/common/tendermint"

	"github.com/cosmos/relayer/v2/relayer/chains/icon/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/icon-project/IBC-Integration/libraries/go/common/icon"
	"go.uber.org/zap"
)

var (
	rtyAttNum = uint(5)
	rtyAtt    = retry.Attempts(rtyAttNum)
	rtyDel    = retry.Delay(time.Millisecond * 400)
	rtyErr    = retry.LastErrorOnly(true)

	defaultBroadcastWaitTimeout = 10 * time.Minute
)

func (icp *IconProvider) MsgCreateClient(clientState ibcexported.ClientState, consensusState ibcexported.ConsensusState) (provider.RelayerMessage, error) {

	clientStateBytes, err := proto.Marshal(clientState)
	if err != nil {
		return nil, err
	}

	consensusStateBytes, err := proto.Marshal(consensusState)
	if err != nil {
		return nil, err
	}

	clS := &types.GenericClientParams[types.MsgCreateClient]{
		Msg: types.MsgCreateClient{
			ClientState:    types.NewHexBytes(clientStateBytes),
			ConsensusState: types.NewHexBytes(consensusStateBytes),
			ClientType:     clientState.ClientType(),
			BtpNetworkId:   types.NewHexInt(icp.PCfg.BTPNetworkID),
		},
	}

	return icp.NewIconMessage(clS, MethodCreateClient), nil
}

// Upgrade Client Not Implemented implemented
func (icp *IconProvider) MsgUpgradeClient(srcClientId string, consRes *clienttypes.QueryConsensusStateResponse, clientRes *clienttypes.QueryClientStateResponse) (provider.RelayerMessage, error) {

	clU := &types.MsgUpdateClient{
		ClientId:      srcClientId,
		ClientMessage: types.HexBytes(""),
	}

	return icp.NewIconMessage(clU, MethodUpdateClient), nil
}

func (icp *IconProvider) MsgRecvPacket(msgTransfer provider.PacketInfo, proof provider.PacketProof) (provider.RelayerMessage, error) {
	pkt := &icon.Packet{
		Sequence:           msgTransfer.Sequence,
		SourcePort:         msgTransfer.SourcePort,
		SourceChannel:      msgTransfer.SourceChannel,
		DestinationPort:    msgTransfer.DestPort,
		DestinationChannel: msgTransfer.DestChannel,
		TimeoutHeight: &icon.Height{
			RevisionNumber: msgTransfer.TimeoutHeight.RevisionNumber,
			RevisionHeight: msgTransfer.TimeoutHeight.RevisionHeight,
		},
		TimeoutTimestamp: msgTransfer.TimeoutTimestamp,
		Data:             msgTransfer.Data,
	}
	pktEncode, err := proto.Marshal(pkt)
	if err != nil {
		return nil, err
	}

	ht := &icon.Height{
		RevisionNumber: proof.ProofHeight.RevisionNumber,
		RevisionHeight: proof.ProofHeight.RevisionHeight,
	}
	htEncode, err := proto.Marshal(ht)
	if err != nil {
		return nil, err
	}
	recvPacket := types.MsgPacketRecv{
		Packet:      types.NewHexBytes(pktEncode),
		Proof:       types.NewHexBytes(proof.Proof),
		ProofHeight: types.NewHexBytes(htEncode),
	}

	recvPacketMsg := &types.GenericPacketParams[types.MsgPacketRecv]{
		Msg: recvPacket,
	}

	return icp.NewIconMessage(recvPacketMsg, MethodRecvPacket), nil
}

func (icp *IconProvider) MsgAcknowledgement(msgRecvPacket provider.PacketInfo, proofAcked provider.PacketProof) (provider.RelayerMessage, error) {

	pktEncode, err := getIconPacketEncodedBytes(msgRecvPacket)
	if err != nil {
		return nil, err
	}
	ht := &icon.Height{
		RevisionNumber: proofAcked.ProofHeight.RevisionNumber,
		RevisionHeight: proofAcked.ProofHeight.RevisionHeight,
	}
	htEncode, err := proto.Marshal(ht)
	if err != nil {
		return nil, err
	}
	msg := types.MsgPacketAcknowledgement{
		Packet:          types.NewHexBytes(pktEncode),
		Acknowledgement: types.NewHexBytes(msgRecvPacket.Ack),
		Proof:           types.NewHexBytes(proofAcked.Proof),
		ProofHeight:     types.NewHexBytes(htEncode),
	}

	packetAckMsg := &types.GenericPacketParams[types.MsgPacketAcknowledgement]{
		Msg: msg,
	}
	return icp.NewIconMessage(packetAckMsg, MethodAckPacket), nil
}

func (icp *IconProvider) MsgTimeout(msgTransfer provider.PacketInfo, proofUnreceived provider.PacketProof) (provider.RelayerMessage, error) {
	// return nil, fmt.Errorf("Not implemented on icon")
	pktEncode, err := getIconPacketEncodedBytes(msgTransfer)
	if err != nil {
		return nil, err
	}

	htEncode, err := proto.Marshal(&icon.Height{
		RevisionNumber: proofUnreceived.ProofHeight.RevisionNumber,
		RevisionHeight: proofUnreceived.ProofHeight.RevisionHeight,
	})
	if err != nil {
		return nil, err
	}

	msg := types.MsgTimeoutPacket{
		Packet:           types.NewHexBytes(pktEncode),
		Proof:            types.NewHexBytes(proofUnreceived.Proof),
		ProofHeight:      types.NewHexBytes(htEncode),
		NextSequenceRecv: types.NewHexInt(int64(msgTransfer.Sequence)),
	}

	packetTimeoutMsg := types.GenericPacketParams[types.MsgTimeoutPacket]{
		Msg: msg,
	}
	return icp.NewIconMessage(packetTimeoutMsg, MethodTimeoutPacket), nil
}

func (ip *IconProvider) MsgTimeoutRequest(msgTransfer provider.PacketInfo, proof provider.PacketProof) (provider.RelayerMessage, error) {

	pktEncode, err := getIconPacketEncodedBytes(msgTransfer)
	if err != nil {
		return nil, err
	}

	proofHeight, err := proto.Marshal(&proof.ProofHeight)
	if err != nil {
		return nil, err
	}

	timeoutMsg := types.MsgRequestTimeout{
		Packet:      types.NewHexBytes(pktEncode),
		Proof:       types.NewHexBytes(proof.Proof),
		ProofHeight: types.NewHexBytes(proofHeight),
	}

	msg := types.GenericPacketParams[types.MsgRequestTimeout]{
		Msg: timeoutMsg,
	}
	return ip.NewIconMessage(msg, MethodRequestTimeout), nil

}

func (icp *IconProvider) MsgTimeoutOnClose(msgTransfer provider.PacketInfo, proofUnreceived provider.PacketProof) (provider.RelayerMessage, error) {
	return nil, fmt.Errorf("Not implemented on icon")
}

func (icp *IconProvider) MsgConnectionOpenInit(info provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	cc := &icon.Counterparty{
		ClientId:     info.CounterpartyClientID,
		ConnectionId: info.CounterpartyConnID,
		Prefix:       (*icon.MerklePrefix)(&info.CounterpartyCommitmentPrefix),
	}
	ccEncode, err := icp.codec.Marshaler.Marshal(cc)
	if err != nil {
		return nil, err
	}

	msg := types.MsgConnectionOpenInit{
		ClientId:     info.ClientID,
		Counterparty: types.NewHexBytes(ccEncode),
		DelayPeriod:  defaultDelayPeriod,
	}

	connectionOpenMsg := &types.GenericConnectionParam[types.MsgConnectionOpenInit]{
		Msg: msg,
	}
	return icp.NewIconMessage(connectionOpenMsg, MethodConnectionOpenInit), nil
}

func (icp *IconProvider) MsgConnectionOpenTry(msgOpenInit provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	cc := &icon.Counterparty{
		ClientId:     msgOpenInit.ClientID,
		ConnectionId: msgOpenInit.ConnID,
		Prefix:       (*icon.MerklePrefix)(&msgOpenInit.CommitmentPrefix),
	}

	ccEncode, err := proto.Marshal(cc)
	if err != nil {
		return nil, err
	}
	clientStateEncode, err := proto.Marshal(proof.ClientState)
	if err != nil {
		return nil, err
	}

	ht := &icon.Height{
		RevisionNumber: proof.ProofHeight.RevisionNumber,
		RevisionHeight: proof.ProofHeight.RevisionHeight,
	}
	htEncode, err := proto.Marshal(ht)
	if err != nil {
		return nil, err
	}

	consHt := &icon.Height{
		RevisionNumber: 0,
		RevisionHeight: proof.ClientState.GetLatestHeight().GetRevisionHeight(),
	}
	consHtEncode, err := proto.Marshal(consHt)
	if err != nil {
		return nil, err
	}

	versionEnc, err := proto.Marshal(DefaultIBCVersion)
	if err != nil {
		return nil, err
	}

	msg := types.MsgConnectionOpenTry{
		ClientId:             msgOpenInit.CounterpartyClientID,
		PreviousConnectionId: msgOpenInit.CounterpartyConnID,
		ClientStateBytes:     types.NewHexBytes(clientStateEncode),
		Counterparty:         types.NewHexBytes(ccEncode),
		DelayPeriod:          defaultDelayPeriod,
		CounterpartyVersions: []types.HexBytes{types.NewHexBytes(versionEnc)},
		ProofInit:            types.NewHexBytes(proof.ConnectionStateProof),
		ProofHeight:          types.NewHexBytes(htEncode),
		ProofClient:          types.NewHexBytes(proof.ClientStateProof),
		ProofConsensus:       types.NewHexBytes(proof.ConsensusStateProof),
		ConsensusHeight:      types.NewHexBytes(consHtEncode),
	}

	connectionOpenTryMsg := &types.GenericConnectionParam[types.MsgConnectionOpenTry]{
		Msg: msg,
	}
	return icp.NewIconMessage(connectionOpenTryMsg, MethodConnectionOpenTry), nil
}

func (icp *IconProvider) MsgConnectionOpenAck(msgOpenTry provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {

	// proof from chainB should return clientState of chainB tracking chainA
	iconClientState, err := icp.MustReturnIconClientState(proof.ClientState)
	if err != nil {
		return nil, err
	}

	clientStateEncode, err := icp.codec.Marshaler.Marshal(iconClientState)
	if err != nil {
		return nil, err
	}

	ht := &icon.Height{
		RevisionNumber: proof.ProofHeight.RevisionNumber,
		RevisionHeight: proof.ProofHeight.RevisionHeight,
	}
	htEncode, err := proto.Marshal(ht)
	if err != nil {
		return nil, err
	}

	consHt := &icon.Height{
		RevisionNumber: proof.ClientState.GetLatestHeight().GetRevisionNumber(),
		RevisionHeight: proof.ClientState.GetLatestHeight().GetRevisionHeight(),
	}
	consHtEncode, err := proto.Marshal(consHt)
	if err != nil {
		return nil, err
	}

	versionEnc, err := proto.Marshal(DefaultIBCVersion)
	if err != nil {
		return nil, err
	}

	msg := types.MsgConnectionOpenAck{
		ConnectionId:             msgOpenTry.CounterpartyConnID,
		ClientStateBytes:         types.NewHexBytes(clientStateEncode),
		Version:                  types.NewHexBytes(versionEnc),
		CounterpartyConnectionID: msgOpenTry.ConnID,
		ProofTry:                 types.NewHexBytes(proof.ConnectionStateProof),
		ProofClient:              types.NewHexBytes(proof.ClientStateProof),
		ProofConsensus:           types.NewHexBytes(proof.ConsensusStateProof),
		ProofHeight:              types.NewHexBytes(htEncode),
		ConsensusHeight:          types.NewHexBytes(consHtEncode),
	}

	connectionOpenAckMsg := &types.GenericConnectionParam[types.MsgConnectionOpenAck]{
		Msg: msg,
	}
	return icp.NewIconMessage(connectionOpenAckMsg, MethodConnectionOpenAck), nil
}

func (icp *IconProvider) MsgConnectionOpenConfirm(msgOpenAck provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	ht := &icon.Height{
		RevisionNumber: proof.ProofHeight.RevisionNumber,
		RevisionHeight: proof.ProofHeight.RevisionHeight,
	}
	htEncode, err := proto.Marshal(ht)
	if err != nil {
		return nil, err
	}
	msg := types.MsgConnectionOpenConfirm{
		ConnectionId: msgOpenAck.CounterpartyConnID,
		ProofAck:     types.NewHexBytes(proof.ConnectionStateProof),
		ProofHeight:  types.NewHexBytes(htEncode),
	}
	connectionOpenConfirmMsg := &types.GenericConnectionParam[types.MsgConnectionOpenConfirm]{
		Msg: msg,
	}
	return icp.NewIconMessage(connectionOpenConfirmMsg, MethodConnectionOpenConfirm), nil
}

func (icp *IconProvider) MsgChannelOpenInit(info provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	channel := &icon.Channel{
		State:    icon.Channel_STATE_INIT,
		Ordering: icon.Channel_Order(info.Order),
		Counterparty: &icon.Channel_Counterparty{
			PortId:    info.CounterpartyPortID,
			ChannelId: "",
		},
		ConnectionHops: []string{info.ConnID},
		Version:        info.Version,
	}
	channelEncode, err := proto.Marshal(channel)
	if err != nil {
		return nil, err
	}
	msg := types.MsgChannelOpenInit{
		PortId:  info.PortID,
		Channel: types.NewHexBytes(channelEncode),
	}

	channelOpenMsg := &types.GenericChannelParam[types.MsgChannelOpenInit]{
		Msg: msg,
	}
	return icp.NewIconMessage(channelOpenMsg, MethodChannelOpenInit), nil
}

func (icp *IconProvider) MsgChannelOpenTry(msgOpenInit provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	channel := &icon.Channel{
		State:    icon.Channel_STATE_TRYOPEN,
		Ordering: icon.Channel_Order(proof.Ordering),
		Counterparty: &icon.Channel_Counterparty{
			PortId:    msgOpenInit.PortID,
			ChannelId: msgOpenInit.ChannelID,
		},
		ConnectionHops: []string{msgOpenInit.CounterpartyConnID},
		Version:        proof.Version,
	}

	channeEncode, err := proto.Marshal(channel)
	if err != nil {
		return nil, err
	}
	htEncode, err := proto.Marshal(&proof.ProofHeight)
	if err != nil {
		return nil, err
	}
	msg := types.MsgChannelOpenTry{
		PortId:              msgOpenInit.CounterpartyPortID,
		PreviousChannelId:   msgOpenInit.CounterpartyChannelID,
		Channel:             types.NewHexBytes(channeEncode),
		CounterpartyVersion: proof.Version,
		ProofInit:           types.NewHexBytes(proof.Proof),
		ProofHeight:         types.NewHexBytes(htEncode),
	}

	channelOpenTryMsg := &types.GenericChannelParam[types.MsgChannelOpenTry]{
		Msg: msg,
	}
	return icp.NewIconMessage(channelOpenTryMsg, MethodChannelOpenTry), nil
}

func (icp *IconProvider) MsgChannelOpenAck(msgOpenTry provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	ht := &icon.Height{
		RevisionNumber: proof.ProofHeight.RevisionNumber,
		RevisionHeight: proof.ProofHeight.RevisionHeight,
	}
	htEncode, err := proto.Marshal(ht)
	if err != nil {
		return nil, err
	}
	msg := types.MsgChannelOpenAck{
		PortId:                msgOpenTry.CounterpartyPortID,
		ChannelId:             msgOpenTry.CounterpartyChannelID,
		CounterpartyVersion:   proof.Version,
		CounterpartyChannelId: msgOpenTry.ChannelID,
		ProofTry:              types.NewHexBytes(proof.Proof),
		ProofHeight:           types.NewHexBytes(htEncode),
	}
	channelOpenAckMsg := &types.GenericChannelParam[types.MsgChannelOpenAck]{
		Msg: msg,
	}
	return icp.NewIconMessage(channelOpenAckMsg, MethodChannelOpenAck), nil
}

func (icp *IconProvider) MsgChannelOpenConfirm(msgOpenAck provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	ht := &icon.Height{
		RevisionNumber: proof.ProofHeight.RevisionNumber,
		RevisionHeight: proof.ProofHeight.RevisionHeight,
	}
	htEncode, err := proto.Marshal(ht)
	if err != nil {
		return nil, err
	}
	msg := types.MsgChannelOpenConfirm{
		PortId:      msgOpenAck.CounterpartyPortID,
		ChannelId:   msgOpenAck.CounterpartyChannelID,
		ProofAck:    types.NewHexBytes(proof.Proof),
		ProofHeight: types.NewHexBytes(htEncode),
	}
	channelOpenConfirmMsg := &types.GenericChannelParam[types.MsgChannelOpenConfirm]{
		Msg: msg,
	}
	return icp.NewIconMessage(channelOpenConfirmMsg, MethodChannelOpenConfirm), nil
}

func (icp *IconProvider) MsgChannelCloseInit(info provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	msg := types.MsgChannelCloseInit{
		PortId:    info.PortID,
		ChannelId: info.ChannelID,
	}

	channelCloseInitMsg := &types.GenericChannelParam[types.MsgChannelCloseInit]{
		Msg: msg,
	}
	return icp.NewIconMessage(channelCloseInitMsg, MethodChannelCloseInit), nil
}

func (icp *IconProvider) MsgChannelCloseConfirm(msgCloseInit provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	ht := &icon.Height{
		RevisionNumber: proof.ProofHeight.RevisionNumber,
		RevisionHeight: proof.ProofHeight.RevisionHeight,
	}
	htEncode, err := proto.Marshal(ht)
	if err != nil {
		return nil, err
	}

	msg := types.MsgChannelCloseConfirm{
		PortId:      msgCloseInit.CounterpartyPortID,
		ChannelId:   msgCloseInit.CounterpartyChannelID,
		ProofInit:   types.NewHexBytes(proof.Proof),
		ProofHeight: types.NewHexBytes(htEncode),
	}

	channelCloseConfirmMsg := &types.GenericChannelParam[types.MsgChannelCloseConfirm]{
		Msg: msg,
	}
	return icp.NewIconMessage(channelCloseConfirmMsg, MethodChannelCloseConfirm), nil
}

func (icp *IconProvider) MsgUpdateClientHeader(latestHeader provider.IBCHeader, trustedHeight clienttypes.Height, trustedHeader provider.IBCHeader, clientType string) (ibcexported.ClientMessage, error) {

	latestIconHeader, ok := latestHeader.(IconIBCHeader)
	if !ok {
		return nil, fmt.Errorf("Unsupported IBC Header type. Expected: IconIBCHeader,actual: %T", latestHeader)
	}

	btp_proof, err := icp.GetBTPProof(int64(latestIconHeader.Header.MainHeight))
	if err != nil {
		return nil, err
	}

	var currentValidatorList types.ValidatorList
	// subtract 1 because it is a current validator not last validator
	info, err := icp.client.GetNetworkTypeInfo(int64(latestIconHeader.Header.MainHeight-1), icp.PCfg.BTPNetworkTypeID)
	if err != nil {
		return nil, err
	}

	_, err = Base64ToData(string(info.NextProofContext), &currentValidatorList)
	if err != nil {
		return nil, err
	}

	var nextValidators types.ValidatorList
	// subtract 1 because it is a current validator not last validator
	next_info, err := icp.client.GetNetworkTypeInfo(int64(latestIconHeader.Header.MainHeight), icp.PCfg.BTPNetworkTypeID)
	if err != nil {
		return nil, err
	}

	_, err = Base64ToData(string(next_info.NextProofContext), &nextValidators)
	if err != nil {
		return nil, err
	}

	signedHeader := &icon.SignedHeader{
		Header: &icon.BTPHeader{
			MainHeight:             uint64(latestIconHeader.Header.MainHeight),
			Round:                  uint32(latestIconHeader.Header.Round),
			NextProofContextHash:   latestIconHeader.Header.NextProofContextHash,
			NetworkSectionToRoot:   latestIconHeader.Header.NetworkSectionToRoot,
			NetworkId:              latestIconHeader.Header.NetworkID,
			UpdateNumber:           latestIconHeader.Header.UpdateNumber,
			PrevNetworkSectionHash: latestIconHeader.Header.PrevNetworkSectionHash,
			MessageCount:           latestIconHeader.Header.MessageCount,
			MessageRoot:            latestIconHeader.Header.MessageRoot,
			NextValidators:         nextValidators.Validators,
		},
		Signatures:        btp_proof,
		TrustedHeight:     trustedHeight.RevisionHeight,
		CurrentValidators: currentValidatorList.Validators,
	}

	return signedHeader, nil

}

func (icp *IconProvider) MsgUpdateClient(clientID string, counterpartyHeader ibcexported.ClientMessage) (provider.RelayerMessage, error) {

	cs := counterpartyHeader.(*itm.TmHeader)
	clientMsg, err := proto.Marshal(cs)
	if err != nil {
		return nil, err
	}

	msg := types.MsgUpdateClient{
		ClientId:      clientID,
		ClientMessage: types.NewHexBytes(clientMsg),
	}
	updateClientMsg := &types.GenericClientParams[types.MsgUpdateClient]{
		Msg: msg,
	}
	return icp.NewIconMessage(updateClientMsg, MethodUpdateClient), nil
}

func (icp *IconProvider) SendMessage(ctx context.Context, msg provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {

	var (
		rlyResp     *provider.RelayerTxResponse
		callbackErr error
		wg          sync.WaitGroup
	)

	callback := func(rtr *provider.RelayerTxResponse, err error) {
		rlyResp = rtr
		callbackErr = err
		wg.Done()
	}

	wg.Add(1)
	if err := retry.Do(func() error {
		return icp.SendMessagesToMempool(ctx, []provider.RelayerMessage{msg}, memo, ctx, callback)
	}, retry.Context(ctx), rtyAtt, rtyDel, rtyErr, retry.OnRetry(func(n uint, err error) {
		icp.log.Info(
			"Error building or broadcasting transaction",
			zap.String("chain_id", icp.PCfg.ChainID),
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

	if rlyResp.Code != 1 {
		return rlyResp, false, fmt.Errorf("transaction failed with code: %d", rlyResp.Code)
	}

	return rlyResp, true, callbackErr
}

func (icp *IconProvider) parseConfirmedEventLogStr(event types.EventLogStr) provider.RelayerEvent {

	eventName := event.Indexed[0]
	switch eventName {

	case EventTypeCreateClient:
		return provider.RelayerEvent{
			EventType: IconCosmosEventMap[eventName],
			Attributes: map[string]string{
				clienttypes.AttributeKeyClientID: event.Indexed[1],
			},
		}

	case EventTypeConnectionOpenConfirm:
		protoConn, err := hex.DecodeString(strings.TrimPrefix(event.Data[0], "0x"))
		if err != nil {
			icp.log.Error("Error decoding data for ConnectionOpenConfirm", zap.String("connectionData", event.Data[0]))
			break
		}
		var connEnd icon.ConnectionEnd
		err = proto.Unmarshal(protoConn, &connEnd)
		if err != nil {
			icp.log.Error("Error marshaling connectionEnd", zap.String("connectionData", string(protoConn)))
			break
		}
		return provider.RelayerEvent{
			EventType: IconCosmosEventMap[eventName],
			Attributes: map[string]string{
				conntypes.AttributeKeyConnectionID:             event.Indexed[1],
				conntypes.AttributeKeyClientID:                 connEnd.ClientId,
				conntypes.AttributeKeyCounterpartyClientID:     connEnd.Counterparty.ClientId,
				conntypes.AttributeKeyCounterpartyConnectionID: connEnd.Counterparty.ConnectionId,
			},
		}

	case EventTypeChannelOpenConfirm, EventTypeChannelCloseConfirm:
		protoChannel, err := hex.DecodeString(strings.TrimPrefix(event.Data[0], "0x"))
		if err != nil {
			icp.log.Error("Error decoding data for ChanOpenConfirm", zap.String("channelData", event.Data[0]))
			break
		}
		var channel icon.Channel

		if err := proto.Unmarshal(protoChannel, &channel); err != nil {
			icp.log.Error("Error when unmarshalling chanOpenConfirm", zap.String("channelData", string(protoChannel)))
		}
		return provider.RelayerEvent{
			EventType: IconCosmosEventMap[eventName],
			Attributes: map[string]string{
				chantypes.AttributeKeyPortID:             string(event.Indexed[1]),
				chantypes.AttributeKeyChannelID:          string(event.Indexed[2]),
				chantypes.AttributeCounterpartyPortID:    channel.Counterparty.PortId,
				chantypes.AttributeCounterpartyChannelID: channel.Counterparty.ChannelId,
				chantypes.AttributeKeyConnectionID:       channel.ConnectionHops[0],
			},
		}

	}

	return provider.RelayerEvent{}
}

func (icp *IconProvider) SendMessages(ctx context.Context, msgs []provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {
	// Handles 1st msg only
	for _, msg := range msgs {
		return icp.SendMessage(ctx, msg, memo)
	}
	return nil, false, fmt.Errorf("Use SendMessage and one txn at a time")
}

func (icp *IconProvider) SendMessagesToMempool(
	ctx context.Context,
	msgs []provider.RelayerMessage,
	memo string,
	asyncCtx context.Context,
	asyncCallback func(*provider.RelayerTxResponse, error),
) error {
	icp.txMu.Lock()
	defer icp.txMu.Unlock()
	if len(msgs) == 0 {
		icp.log.Info("Length of Messages is empty ")
		return nil
	}

	for _, msg := range msgs {
		if msg != nil {
			err := icp.SendIconTransaction(ctx, msg, asyncCtx, asyncCallback)
			if err != nil {
				icp.log.Warn("Send Icon Transaction Error", zap.String("method", msg.Type()), zap.Error(err))
				continue
			}
		}
	}

	return nil
}

func (icp *IconProvider) SendIconTransaction(
	ctx context.Context,
	msg provider.RelayerMessage,
	asyncCtx context.Context,
	asyncCallback func(*provider.RelayerTxResponse, error)) error {
	m := msg.(*IconMessage)
	wallet, err := icp.Wallet()
	if err != nil {
		return err
	}

	txParamEst := &types.TransactionParamForEstimate{
		Version:     types.NewHexInt(types.JsonrpcApiVersion),
		FromAddress: types.Address(wallet.Address().String()),
		ToAddress:   types.Address(icp.PCfg.IbcHandlerAddress),
		NetworkID:   types.NewHexInt(icp.PCfg.ICONNetworkID),
		DataType:    "call",
		Data: types.CallData{
			Method: m.Method,
			Params: m.Params,
		},
	}

	step, err := icp.client.EstimateStep(txParamEst)
	if err != nil {
		return fmt.Errorf("failed estimating step: %w", err)
	}
	stepVal, err := step.Int()
	if err != nil {
		return err
	}
	stepLimit := types.NewHexInt(int64(stepVal + 200_000))

	txParam := &types.TransactionParam{
		Version:     types.NewHexInt(types.JsonrpcApiVersion),
		FromAddress: types.Address(wallet.Address().String()),
		ToAddress:   types.Address(icp.PCfg.IbcHandlerAddress),
		NetworkID:   types.NewHexInt(icp.PCfg.ICONNetworkID),
		StepLimit:   stepLimit,
		DataType:    "call",
		Data: types.CallData{
			Method: m.Method,
			Params: m.Params,
		},
	}

	if err := icp.client.SignTransaction(wallet, txParam); err != nil {
		return err
	}
	_, err = icp.client.SendTransaction(txParam)
	if err != nil {
		return err
	}

	txhash, err := txParam.TxHash.Value()
	if err != nil {
		return err
	}
	icp.log.Info("Submitted Transaction", zap.String("chain_id", icp.ChainId()), zap.String("method", m.Method), zap.String("tx_hash", string(txParam.TxHash)))

	// wait for the update client but dont cancel sending message.
	// If update fails, the subsequent txn will fail, result of update not being fetched concurrently
	switch m.Method {
	case MethodUpdateClient:
		icp.WaitForTxResult(asyncCtx, txhash, m.Method, defaultBroadcastWaitTimeout, asyncCallback)
	default:
		go icp.WaitForTxResult(asyncCtx, txhash, m.Method, defaultBroadcastWaitTimeout, asyncCallback)
	}

	return nil
}

// TODO: review try to remove wait for Tx from packet-transfer and only use this for client and connection creation
func (icp *IconProvider) WaitForTxResult(
	asyncCtx context.Context,
	txHash []byte,
	method string,
	timeout time.Duration,
	callback func(*provider.RelayerTxResponse, error),
) error {
	txhash := types.NewHexBytes(txHash)
	_, txRes, err := icp.client.WaitForResults(asyncCtx, &types.TransactionHashParam{Hash: txhash})
	if err != nil {
		icp.log.Error("Failed to get txn result", zap.String("txHash", string(txhash)), zap.String("method", method), zap.Error(err))
		if callback != nil {
			callback(nil, err)
		}
		return err
	}

	height, err := txRes.BlockHeight.Value()
	if err != nil {
		return err
	}

	var eventLogs []provider.RelayerEvent
	events := txRes.EventLogs
	for _, event := range events {
		if IconCosmosEventMap[event.Indexed[0]] != "" {
			if event.Addr == types.Address(icp.PCfg.IbcHandlerAddress) {
				evt := icp.parseConfirmedEventLogStr(event)
				eventLogs = append(eventLogs, evt)
			}
		}
	}

	status, err := txRes.Status.Int()
	if status != 1 {
		err = fmt.Errorf("Transaction Failed to Execute")
		if callback != nil {
			callback(nil, err)
		}
		icp.LogFailedTx(method, txRes, err)
		return err

	}

	rlyResp := &provider.RelayerTxResponse{
		Height: height,
		TxHash: string(txRes.TxHash),
		Code:   uint32(status),
		Data:   string(txRes.SCOREAddress),
		Events: eventLogs,
	}
	if callback != nil {
		callback(rlyResp, nil)
	}
	// log successful txn
	icp.LogSuccessTx(method, txRes)
	return nil
}

func (icp *IconProvider) LogSuccessTx(method string, result *types.TransactionResult) {
	stepUsed, _ := result.StepUsed.Value()
	height, _ := result.BlockHeight.Value()

	icp.log.Info("Successful Transaction",
		zap.String("chain_id", icp.ChainId()),
		zap.String("method", method),
		zap.String("tx_hash", string(result.TxHash)),
		zap.Int64("height", height),
		zap.Int64("step_used", stepUsed),
	)
}

func (icp *IconProvider) LogFailedTx(method string, result *types.TransactionResult, err error) {
	stepUsed, _ := result.StepUsed.Value()
	height, _ := result.BlockHeight.Value()

	icp.log.Info("Failed Transaction",
		zap.String("chain_id", icp.ChainId()),
		zap.String("method", method),
		zap.String("tx_hash", string(result.TxHash)),
		zap.Int64("height", height),
		zap.Int64("step_used", stepUsed),
		zap.Error(err),
	)
}
