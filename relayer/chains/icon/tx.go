package icon

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/cosmos/gogoproto/proto"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/chains/icon/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/icon-project/IBC-Integration/libraries/go/common/icon"
	"go.uber.org/zap"
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

	return NewIconMessage(clS, MethodCreateClient), nil
}

func (icp *IconProvider) MsgUpgradeClient(srcClientId string, consRes *clienttypes.QueryConsensusStateResponse, clientRes *clienttypes.QueryClientStateResponse) (provider.RelayerMessage, error) {

	clU := &types.MsgUpdateClient{
		ClientId:      srcClientId,
		ClientMessage: types.HexBytes(""),
	}

	return NewIconMessage(clU, MethodUpdateClient), nil
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

	return NewIconMessage(recvPacketMsg, MethodRecvPacket), nil
}

func (icp *IconProvider) MsgAcknowledgement(msgRecvPacket provider.PacketInfo, proofAcked provider.PacketProof) (provider.RelayerMessage, error) {
	pkt := &icon.Packet{
		Sequence:           msgRecvPacket.Sequence,
		SourcePort:         msgRecvPacket.SourcePort,
		SourceChannel:      msgRecvPacket.SourceChannel,
		DestinationPort:    msgRecvPacket.DestPort,
		DestinationChannel: msgRecvPacket.DestChannel,
		TimeoutHeight: &icon.Height{
			RevisionNumber: msgRecvPacket.TimeoutHeight.RevisionNumber,
			RevisionHeight: msgRecvPacket.TimeoutHeight.RevisionHeight,
		},
		TimeoutTimestamp: msgRecvPacket.TimeoutTimestamp,
	}

	pktEncode, err := proto.Marshal(pkt)
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
	return NewIconMessage(packetAckMsg, MethodWriteAck), nil
}

func (icp *IconProvider) MsgTimeout(msgTransfer provider.PacketInfo, proofUnreceived provider.PacketProof) (provider.RelayerMessage, error) {
	return nil, fmt.Errorf("Not implemented on icon")
}

func (icp *IconProvider) MsgTimeoutOnClose(msgTransfer provider.PacketInfo, proofUnreceived provider.PacketProof) (provider.RelayerMessage, error) {
	return nil, fmt.Errorf("Not implemented on icon")
}

func (icp *IconProvider) MsgConnectionOpenInit(info provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	cc := &icon.Counterparty{
		ClientId:     info.CounterpartyClientID,
		ConnectionId: info.CounterpartyConnID,
	}
	ccEncode, err := proto.Marshal(cc)
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
	return NewIconMessage(connectionOpenMsg, MethodConnectionOpenInit), nil
}

func (icp *IconProvider) MsgConnectionOpenTry(msgOpenInit provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	cc := &icon.Counterparty{
		ClientId:     msgOpenInit.ClientID,
		ConnectionId: msgOpenInit.ConnID,
		Prefix:       &icon.MerklePrefix{KeyPrefix: []byte("ibc")},
	}

	ccEncode, err := proto.Marshal(cc)
	if err != nil {
		return nil, err
	}
	// clientStateEncode, err := proto.Marshal(proof.ClientState)
	// if err != nil {
	// 	return nil, err
	// }

	// ht := &icon.Height{
	// 	RevisionNumber: proof.ProofHeight.RevisionNumber,
	// 	RevisionHeight: proof.ProofHeight.RevisionHeight,
	// }
	ht := &icon.Height{
		RevisionNumber: 0,
		RevisionHeight: 1999,
	}
	htEncode, err := proto.Marshal(ht)
	if err != nil {
		return nil, err
	}

	consHt := &icon.Height{
		RevisionNumber: 0,
		RevisionHeight: 2000, //proof.ClientState.GetLatestHeight().GetRevisionHeight(),
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
		ClientId:             msgOpenInit.CounterpartyClientID, //msgOpenInit.CounterpartyClientID,
		PreviousConnectionId: "connection-0",                   //msgOpenInit.CounterpartyConnID,
		ClientStateBytes:     types.NewHexBytes([]byte("0x")),  //types.NewHexBytes(clientStateEncode),
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
	return NewIconMessage(connectionOpenTryMsg, MethodConnectionOpenTry), nil
}

func (icp *IconProvider) MsgConnectionOpenAck(msgOpenTry provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {

	// clientStateEncode, err := proto.Marshal(proof.ClientState)
	// if err != nil {
	// 	return nil, err
	// }

	// RevisionNumber: proof.ProofHeight.RevisionNumber,
	// RevisionHeight: proof.ProofHeight.RevisionHeight,
	ht := &icon.Height{
		RevisionNumber: 0,
		RevisionHeight: 100,
	}
	htEncode, err := proto.Marshal(ht)
	if err != nil {
		return nil, err
	}

	// RevisionNumber: proof.ClientState.GetLatestHeight().GetRevisionNumber(),
	// RevisionHeight: proof.ClientState.GetLatestHeight().GetRevisionHeight(),
	consHt := &icon.Height{
		RevisionNumber: 0,
		RevisionHeight: 100,
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
		ClientStateBytes:         types.NewHexBytes([]byte("helloooo")), // TODO
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
	return NewIconMessage(connectionOpenAckMsg, MethodConnectionOpenAck), nil
}

func (icp *IconProvider) MsgConnectionOpenConfirm(msgOpenAck provider.ConnectionInfo, proof provider.ConnectionProof) (provider.RelayerMessage, error) {
	// ht := &icon.Height{
	// 	RevisionNumber: proof.ProofHeight.RevisionNumber,
	// 	RevisionHeight: proof.ProofHeight.RevisionHeight,
	// }
	// htEncode, err := proto.Marshal(ht)
	// if err != nil {
	// 	return nil, err
	// }
	msg := types.MsgConnectionOpenConfirm{
		ConnectionId: msgOpenAck.CounterpartyConnID,
		// ProofAck:     types.NewHexBytes(proof.ConnectionStateProof),
		ProofAck:    "0x",
		ProofHeight: "0x",
	}
	connectionOpenConfirmMsg := &types.GenericConnectionParam[types.MsgConnectionOpenConfirm]{
		Msg: msg,
	}
	return NewIconMessage(connectionOpenConfirmMsg, MethodConnectionOpenConfirm), nil
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
	channel := &icon.Channel{
		State:    icon.Channel_STATE_INIT,
		Ordering: icon.Channel_ORDER_ORDERED,
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
	return NewIconMessage(channelOpenMsg, MethodChannelOpenInit), nil
}

func (icp *IconProvider) MsgChannelOpenTry(msgOpenInit provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	channel := &icon.Channel{
		State:    icon.Channel_STATE_TRYOPEN,
		Ordering: icon.Channel_ORDER_ORDERED,
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
	return NewIconMessage(channelOpenTryMsg, MethodChannelOpenTry), nil
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
	return NewIconMessage(channelOpenAckMsg, MethodChannelOpenAck), nil
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
	return NewIconMessage(channelOpenConfirmMsg, MethodChannelOpenConfirm), nil
}

func (icp *IconProvider) MsgChannelCloseInit(info provider.ChannelInfo, proof provider.ChannelProof) (provider.RelayerMessage, error) {
	msg := types.MsgChannelCloseInit{
		PortId:    info.PortID,
		ChannelId: info.ChannelID,
	}

	channelCloseInitMsg := &types.GenericChannelParam[types.MsgChannelCloseInit]{
		Msg: msg,
	}
	return NewIconMessage(channelCloseInitMsg, MethodChannelCloseInit), nil
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
	return NewIconMessage(channelCloseConfirmMsg, MethodChannelCloseConfirm), nil
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
	// 	trustedHeight: icon.Height{
	// 		RevisionNumber: *big.NewInt(int64(trustedHeight.RevisionNumber)),
	// 		RevisionHeight: *big.NewInt(int64(trustedHeight.RevisionHeight)),
	// 	},
	// 	trustedValidators: trustedIconHeader.trustedValidators,
	// }, nil

}

func (icp *IconProvider) MsgUpdateClient(clientID string, counterpartyHeader ibcexported.ClientMessage) (provider.RelayerMessage, error) {
	clientMsg, err := proto.Marshal(counterpartyHeader)
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
	return NewIconMessage(updateClientMsg, MethodUpdateClient), nil
}

func (icp *IconProvider) SendMessageIcon(ctx context.Context, msg provider.RelayerMessage) (*types.TransactionResult, bool, error) {
	m := msg.(*IconMessage)
	txParam := &types.TransactionParam{
		Version:     types.NewHexInt(types.JsonrpcApiVersion),
		FromAddress: types.Address(icp.wallet.Address().String()),
		ToAddress:   types.Address(icp.PCfg.IbcHandlerAddress),
		NetworkID:   types.NewHexInt(icp.PCfg.ICONNetworkID),
		StepLimit:   types.NewHexInt(int64(defaultStepLimit)),
		DataType:    "call",
		Data: types.CallData{
			Method: m.Method,
			Params: m.Params,
		},
	}

	if err := icp.client.SignTransaction(icp.wallet, txParam); err != nil {
		return nil, false, err
	}
	_, err := icp.client.SendTransaction(txParam)
	if err != nil {
		return nil, false, err
	}

	txhash, _ := txParam.TxHash.Value()

	fmt.Printf("the transaction hash is %x \n", txhash)

	txResParams := &types.TransactionHashParam{
		Hash: txParam.TxHash,
	}

	time.Sleep(2 * time.Second)

	txResult, err := icp.client.GetTransactionResult(txResParams)

	if err != nil {
		return nil, false, err
	}

	if txResult.Status != types.NewHexInt(1) {
		return nil, false, fmt.Errorf("Transaction Failed")
	}

	return txResult, true, err
}

func (icp *IconProvider) SendMessage(ctx context.Context, msg provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {

	txRes, success, err := icp.SendMessageIcon(ctx, msg)
	if err != nil {
		return nil, false, err
	}

	height, err := txRes.BlockHeight.Value()
	if err != nil {
		return nil, false, nil
	}

	var eventLogs []provider.RelayerEvent
	events := txRes.EventLogs
	for _, event := range events {
		if IconCosmosEventMap[event.Indexed[0]] != "" {
			if event.Addr == types.Address(icp.PCfg.IbcHandlerAddress) {
				eventName := event.Indexed[0]
				var evt provider.RelayerEvent
				switch eventName {
				case EventTypeCreateClient:
					evt = provider.RelayerEvent{
						EventType: IconCosmosEventMap[eventName],
						Attributes: map[string]string{
							clienttypes.AttributeKeyClientID: event.Indexed[1],
						},
					}

				case EventTypeConnectionOpenConfirm:
					protoConn, err := hex.DecodeString(strings.TrimPrefix(event.Data[0], "0x"))
					if err != nil {
						panic("huhh")
					}
					var connEnd icon.ConnectionEnd
					err = proto.Unmarshal(protoConn, &connEnd)
					if err != nil {
						panic("")
					}
					evt = provider.RelayerEvent{
						EventType: IconCosmosEventMap[eventName],
						Attributes: map[string]string{
							conntypes.AttributeKeyConnectionID:             event.Indexed[1],
							conntypes.AttributeKeyClientID:                 connEnd.ClientId,
							conntypes.AttributeKeyCounterpartyClientID:     connEnd.Counterparty.ClientId,
							conntypes.AttributeKeyCounterpartyConnectionID: connEnd.Counterparty.ConnectionId,
						},
					}

				case EventTypeChannelOpenConfirm, EventTypeChannelCloseConfirm:

					eventa := ToEventLogBytes(event)
					ibcMsg := parseIBCMessageFromEvent(&zap.Logger{}, eventa, uint64(height))
					channelAttrs := ibcMsg.info.(*channelInfo)
					// protoConn, err := hex.DecodeString(strings.TrimPrefix(event.Data[0], "0x"))
					// if err != nil {
					// 	icp.log.Error("Error decoding string ")
					// 	continue
					// }
					// var channel icon.Channel
					// err = proto.Unmarshal(protoConn, &channel)
					// if err != nil {
					// 	icp.log.Error("")
					// 	continue
					// }
					evt = provider.RelayerEvent{
						EventType: IconCosmosEventMap[eventName],
						Attributes: map[string]string{
							chantypes.AttributeKeyPortID:             channelAttrs.PortID,
							chantypes.AttributeKeyChannelID:          channelAttrs.ChannelID,
							chantypes.AttributeCounterpartyPortID:    channelAttrs.CounterpartyPortID,
							chantypes.AttributeCounterpartyChannelID: channelAttrs.CounterpartyChannelID,
							chantypes.AttributeKeyConnectionID:       channelAttrs.ConnID,
						},
					}
				}
				eventLogs = append(eventLogs, evt)
			}
		}
	}

	status, err := txRes.Status.Int()

	rlyResp := &provider.RelayerTxResponse{
		Height: height,
		TxHash: string(txRes.TxHash),
		Code:   uint32(status),
		Data:   memo,
		Events: eventLogs,
	}

	return rlyResp, success, err
}

func (icp *IconProvider) SendMessages(ctx context.Context, msgs []provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {
	// Handles 1st msg only
	for _, msg := range msgs {
		return icp.SendMessage(ctx, msg, memo)
	}
	return nil, false, fmt.Errorf("Use SendMessage and one txn at a time")
}
