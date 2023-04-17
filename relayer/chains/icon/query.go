package icon

import (
	"context"
	"encoding/base64"
	"fmt"
	"math/big"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
	"github.com/cosmos/ibc-go/v7/modules/core/exported"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/exported"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	//this import should be letter converted to icon types

	"github.com/cosmos/relayer/v2/relayer/chains/icon/cryptoutils"
	"github.com/cosmos/relayer/v2/relayer/chains/icon/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/icon-project/IBC-Integration/libraries/go/common/icon"
	itm "github.com/icon-project/IBC-Integration/libraries/go/common/tendermint"
	"github.com/icon-project/goloop/common/codec"

	transfertypes "github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	committypes "github.com/cosmos/ibc-go/v7/modules/core/23-commitment/types"
	//change this to icon types after original repo merge
)

// ***************** methods marked with legacy should be updated only when relayer is runned through legacy method *****************

var _ provider.QueryProvider = &IconProvider{}

const (
	epoch = 24 * 3600 * 1000
)

type CallParamOption func(*types.CallParam)

func callParamsWithHeight(height types.HexInt) CallParamOption {
	return func(cp *types.CallParam) {
		cp.Height = height
	}
}

func (icp *IconProvider) prepareCallParams(methodName string, param map[string]interface{}, options ...CallParamOption) *types.CallParam {

	callData := &types.CallData{
		Method: methodName,
		Params: param,
	}

	callParam := &types.CallParam{
		FromAddress: types.NewAddress(icp.wallet.Address().Bytes()),
		ToAddress:   types.Address(icp.PCfg.IbcHandlerAddress),
		DataType:    "call",
		Data:        callData,
	}

	for _, option := range options {
		option(callParam)
	}

	return callParam

}

func (icp *IconProvider) BlockTime(ctx context.Context, height int64) (time.Time, error) {
	header, err := icp.client.GetBlockHeaderByHeight(height)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(header.Timestamp, 0), nil
}

// required for cosmos only
func (icp *IconProvider) QueryTx(ctx context.Context, hashHex string) (*provider.RelayerTxResponse, error) {
	return nil, fmt.Errorf("Not implemented for ICON")
}

// required for cosmos only
func (icp *IconProvider) QueryTxs(ctx context.Context, page, limit int, events []string) ([]*provider.RelayerTxResponse, error) {
	return nil, fmt.Errorf("Not implemented for ICON")
}

func (icp *IconProvider) QueryLatestHeight(ctx context.Context) (int64, error) {
	// icp.lastBTPBlockHeightMu.Lock()
	// defer icp.lastBTPBlockHeightMu.Unlock()

	lastBtpHeight := icp.lastBTPBlockHeight
	return int64(lastBtpHeight), nil
}

// legacy
func (icp *IconProvider) QueryIBCHeader(ctx context.Context, h int64) (provider.IBCHeader, error) {
	param := &types.BTPBlockParam{
		Height:    types.NewHexInt(h),
		NetworkId: types.NewHexInt(icp.PCfg.BTPNetworkID),
	}
	btpHeader, err := icp.client.GetBTPHeader(param)
	if err != nil {
		return nil, err
	}

	// convert to rlp
	rlpBTPHeader, err := base64.StdEncoding.DecodeString(btpHeader)
	if err != nil {
		return nil, err
	}

	// rlp to hex
	var header types.BTPBlockHeader
	_, err = codec.RLP.UnmarshalFromBytes(rlpBTPHeader, &header)
	if err != nil {
		zap.Error(err)
		return nil, err
	}

	return &IconIBCHeader{
		Header: &header,
	}, nil
}

func (icp *IconProvider) QuerySendPacket(ctx context.Context, srcChanID, srcPortID string, sequence uint64) (provider.PacketInfo, error) {
	return provider.PacketInfo{}, nil
}

func (icp *IconProvider) QueryRecvPacket(ctx context.Context, dstChanID, dstPortID string, sequence uint64) (provider.PacketInfo, error) {
	return provider.PacketInfo{}, nil
}

func (icp *IconProvider) QueryBalance(ctx context.Context, keyName string) (sdk.Coins, error) {
	addr, err := icp.ShowAddress(keyName)
	if err != nil {
		return nil, err
	}

	return icp.QueryBalanceWithAddress(ctx, addr)
}

// implementing is not required
func (icp *IconProvider) QueryBalanceWithAddress(ctx context.Context, addr string) (sdk.Coins, error) {
	return sdk.Coins{}, fmt.Errorf("Not implemented for ICON")
}

func (icp *IconProvider) QueryUnbondingPeriod(context.Context) (time.Duration, error) {
	return epoch, nil
}

// ****************ClientStates*******************  //
// ics 02 - client

func (icp *IconProvider) QueryClientState(ctx context.Context, height int64, clientid string) (ibcexported.ClientState, error) {

	clientStateRes, err := icp.QueryClientStateResponse(ctx, height, clientid)
	if err != nil {
		return nil, err
	}

	clientStateExported, err := clienttypes.UnpackClientState(clientStateRes.ClientState)
	if err != nil {
		return nil, err
	}

	return clientStateExported, nil

}

func (icp *IconProvider) QueryClientStateResponse(ctx context.Context, height int64, srcClientId string) (*clienttypes.QueryClientStateResponse, error) {

	callParams := icp.prepareCallParams(MethodGetClientState, map[string]interface{}{
		"clientId": srcClientId,
	})

	//similar should be implemented
	var clientStateB types.HexBytes
	err := icp.client.Call(callParams, &clientStateB)
	if err != nil {
		return nil, err
	}

	clientStateByte, err := clientStateB.Value()
	if err != nil {
		return nil, err
	}

	var clientState itm.ClientState
	if err = proto.Unmarshal(clientStateByte, &clientState); err != nil {
		return nil, err
	}

	var ibcExportedClientState ibcexported.ClientState = &clientState

	any, err := clienttypes.PackClientState(ibcExportedClientState)
	if err != nil {
		return nil, err
	}
	// key := cryptoutils.GetClientStateCommitmentKey(srcClientId)
	// keyHash := cryptoutils.Sha3keccak256(key, clientStateByte)
	// proofs, err := icp.QueryIconProof(ctx, height, keyHash)
	// if err != nil {
	// 	return nil, err
	// }

	// // TODO: marshal proof to bytes
	// log.Println("client Proofs: ", proofs)

	return clienttypes.NewQueryClientStateResponse(any, nil, clienttypes.NewHeight(0, uint64(height))), nil
}

func (icp *IconProvider) QueryClientConsensusState(ctx context.Context, chainHeight int64, clientid string, clientHeight ibcexported.Height) (*clienttypes.QueryConsensusStateResponse, error) {
	callParams := icp.prepareCallParams(MethodGetConsensusState, map[string]interface{}{
		"clientId": clientid,
		"height":   clientHeight,
	})
	var cnsStateByte []byte
	err := icp.client.Call(callParams, cnsStateByte)
	if err != nil {
		return nil, err
	}
	var cnsState exported.ConsensusState

	if err := icp.codec.Marshaler.UnmarshalInterface(cnsStateByte, &cnsState); err != nil {
		return nil, err
	}

	any, err := clienttypes.PackConsensusState(cnsState)
	if err != nil {
		return nil, err
	}

	key := cryptoutils.GetConsensusStateCommitmentKey(clientid, big.NewInt(0), big.NewInt(chainHeight))
	keyHash := cryptoutils.Sha3keccak256(key, cnsStateByte)
	proof, err := icp.QueryIconProof(ctx, chainHeight, keyHash)
	if err != nil {
		return nil, err
	}

	// TODO: marshal proof using protobuf
	fmt.Println("Proof of QueryClientConsensusState", proof)

	return &clienttypes.QueryConsensusStateResponse{
		ConsensusState: any,
		Proof:          nil,
		ProofHeight:    clienttypes.NewHeight(0, uint64(chainHeight)),
	}, nil
}

func (icp *IconProvider) QueryUpgradedClient(ctx context.Context, height int64) (*clienttypes.QueryClientStateResponse, error) {
	return nil, nil
}

func (icp *IconProvider) QueryUpgradedConsState(ctx context.Context, height int64) (*clienttypes.QueryConsensusStateResponse, error) {
	return nil, nil
}
func (icp *IconProvider) QueryConsensusState(ctx context.Context, height int64) (ibcexported.ConsensusState, int64, error) {
	return nil, height, fmt.Errorf("Not implemented for ICON. Check QueryClientConsensusState instead")
}

// query all the clients of the chain
func (icp *IconProvider) QueryClients(ctx context.Context) (clienttypes.IdentifiedClientStates, error) {
	// callParam := icp.prepareCallParams(MethodGetNextClientSequence, map[string]interface{}{})
	// var clientSequence types.HexInt
	// if err := icp.client.Call(callParam, &clientSequence); err != nil {
	// 	return nil, err
	// }
	// seq, err := clientSequence.Int()
	// if err != nil {
	// 	return nil, err
	// }

	// identifiedClientStates := make(clienttypes.IdentifiedClientStates, 0)
	// for i := 0; i < seq-2; i++ {
	// 	clientIdentifier := fmt.Sprintf("client-%d", i)
	// 	callParams := icp.prepareCallParams(MethodGetClientState, map[string]interface{}{
	// 		"clientId": clientIdentifier,
	// 	})

	// 	//similar should be implemented
	// 	var clientStateB types.HexBytes
	// 	err := icp.client.Call(callParams, &clientStateB)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	// identifiedClientStates = append(identifiedClientStates, err)

	// }
	// TODO: implement method to get all clients
	return nil, nil
}

// query connection to the ibc host based on the connection-id
func (icp *IconProvider) QueryConnection(ctx context.Context, height int64, connectionid string) (*conntypes.QueryConnectionResponse, error) {

	callParam := icp.prepareCallParams(MethodGetConnection, map[string]interface{}{
		"connectionId": connectionid,
	})

	var conn_string_ string
	err := icp.client.Call(callParam, &conn_string_)
	if err != nil {
		return emptyConnRes, err
	}

	var conn conntypes.ConnectionEnd
	_, err = HexStringToProtoUnmarshal(conn_string_, &conn)
	if err != nil {
		return emptyConnRes, err
	}

	// key := cryptoutils.GetConnectionCommitmentKey(connectionid)
	// connectionBytes, err := conn.Marshal()
	// if err != nil {
	// 	return emptyConnRes, err
	// }

	// keyHash := cryptoutils.Sha3keccak256(key, connectionBytes)

	// proof, err := icp.QueryIconProof(ctx, height, keyHash)
	// if err != nil {
	// 	return emptyConnRes, err
	// }

	// fmt.Println("check the proof ", proof)

	return conntypes.NewQueryConnectionResponse(conn, []byte(""), clienttypes.NewHeight(0, uint64(height))), nil

}

var emptyConnRes = conntypes.NewQueryConnectionResponse(
	conntypes.NewConnectionEnd(
		conntypes.UNINITIALIZED,
		"client",
		conntypes.NewCounterparty(
			"client",
			"connection",
			committypes.MerklePrefix(committypes.NewMerklePrefix(make([]byte, 0))),
		),
		[]*conntypes.Version{},
		0,
	),
	[]byte{},
	clienttypes.NewHeight(0, 0),
)

// ics 03 - connection
func (icp *IconProvider) QueryConnections(ctx context.Context) (conns []*conntypes.IdentifiedConnection, err error) {

	nextSeq, err := icp.getNextSequence(ctx, MethodGetNextConnectionSequence)
	if err != nil {
		return nil, err
	}

	for i := 0; i <= int(nextSeq)-1; i++ {
		connectionId := fmt.Sprintf("connection-%d", i)
		var conn_string_ string
		err := icp.client.Call(icp.prepareCallParams(MethodGetConnection, map[string]interface{}{
			"connectionId": connectionId,
		}), &conn_string_)
		if err != nil {
			icp.log.Error("unable to fetch connection for  ", zap.String("connection id", connectionId))
			continue
		}

		var conn conntypes.ConnectionEnd
		_, err = HexStringToProtoUnmarshal(conn_string_, &conn)
		if err != nil {
			icp.log.Info("unable to unmarshal connection for ", zap.String("connection id ", connectionId))
			continue
		}

		if conn.State == 3 {
			identifiedConn := conntypes.IdentifiedConnection{
				Id:           connectionId,
				ClientId:     conn.ClientId,
				Versions:     conn.Versions,
				State:        conn.State,
				Counterparty: conn.Counterparty,
				DelayPeriod:  conn.DelayPeriod,
			}
			conns = append(conns, &identifiedConn)
		}
	}

	return conns, nil
}

func (icp *IconProvider) getNextSequence(ctx context.Context, methodName string) (uint64, error) {

	var seq types.HexInt
	switch methodName {
	case MethodGetNextClientSequence:
		callParam := icp.prepareCallParams(MethodGetNextClientSequence, map[string]interface{}{})
		if err := icp.client.Call(callParam, &seq); err != nil {
			return 0, err
		}
	case MethodGetNextChannelSequence:
		callParam := icp.prepareCallParams(MethodGetNextChannelSequence, map[string]interface{}{})
		if err := icp.client.Call(callParam, &seq); err != nil {
			return 0, err
		}
	case MethodGetNextConnectionSequence:
		callParam := icp.prepareCallParams(MethodGetNextConnectionSequence, map[string]interface{}{})
		if err := icp.client.Call(callParam, &seq); err != nil {
			return 0, err
		}
	default:
		return 0, errors.New("Invalid method name")
	}
	val, _ := seq.Value()
	return uint64(val), nil
}

func (icp *IconProvider) QueryConnectionsUsingClient(ctx context.Context, height int64, clientid string) (*conntypes.QueryConnectionsResponse, error) {
	// TODO
	return nil, nil
}
func (icp *IconProvider) GenerateConnHandshakeProof(ctx context.Context, height int64, clientId, connId string) (clientState ibcexported.ClientState,
	clientStateProof []byte, consensusProof []byte, connectionProof []byte,
	connectionProofHeight ibcexported.Height, err error) {
	// var (
	// 	clientStateRes     *clienttypes.QueryClientStateResponse
	// 	consensusStateRes  *clienttypes.QueryConsensusStateResponse
	// 	connectionStateRes *conntypes.QueryConnectionResponse
	// 	// eg                 = new(errgroup.Group)
	// )

	// // query for the client state for the proof and get the height to query the consensus state at.
	// clientStateRes, err = icp.QueryClientStateResponse(ctx, height, clientId)

	// if err != nil {
	// 	return nil, nil, nil, nil, clienttypes.Height{}, err
	// }

	// clientState, err = clienttypes.UnpackClientState(clientStateRes.ClientState)
	// if err != nil {
	// 	return nil, nil, nil, nil, clienttypes.Height{}, err
	// }

	// eg.Go(func() error {
	// 	var err error
	// 	consensusStateRes, err = icp.QueryClientConsensusState(ctx, height, clientId, clientState.GetLatestHeight())
	// 	return err
	// })
	// eg.Go(func() error {
	// 	var err error
	// 	connectionStateRes, err = icp.QueryConnection(ctx, height, connId)
	// 	return err
	// })

	// if err := eg.Wait(); err != nil {
	// 	return nil, nil, nil, nil, clienttypes.Height{}, err
	// }

	// return clientState, clientStateRes.Proof, consensusStateRes.Proof, connectionStateRes.Proof, connectionStateRes.ProofHeight, nil
	return clientState, []byte(""), []byte(""), []byte(""), clienttypes.NewHeight(0, uint64(height)), nil
}

// ics 04 - channel
func (icp *IconProvider) QueryChannel(ctx context.Context, height int64, channelid, portid string) (chanRes *chantypes.QueryChannelResponse, err error) {

	callParam := icp.prepareCallParams(MethodGetChannel, map[string]interface{}{
		"channelId": channelid,
		"portId":    "mock", // TODO: change this
	}, callParamsWithHeight(types.NewHexInt(height)))

	var channel_string_ string
	err = icp.client.Call(callParam, &channel_string_)
	if err != nil {
		return emptyChannelRes, err
	}

	var channel chantypes.Channel
	_, err = HexStringToProtoUnmarshal(channel_string_, &channel)
	if err != nil {
		return emptyChannelRes, err
	}

	// keyHash := cryptoutils.GetChannelCommitmentKey(portid, channelid)
	// value, err := channelRes.Marshal()
	// if err != nil {
	// 	return emptyChannelRes, err
	// }

	// keyHash = cryptoutils.Sha3keccak256(keyHash, value)
	// proofs, err := icp.QueryIconProof(ctx, height, keyHash)
	// if err != nil {
	// 	return emptyChannelRes, err
	// }

	// // TODO: proto for the ICON commitment proofs
	// log.Println(proofs)

	return chantypes.NewQueryChannelResponse(channel, []byte(""), clienttypes.NewHeight(0, uint64(height))), nil
}

var emptyChannelRes = chantypes.NewQueryChannelResponse(
	chantypes.NewChannel(
		chantypes.UNINITIALIZED,
		chantypes.UNORDERED,
		chantypes.NewCounterparty(
			"port",
			"channel",
		),
		[]string{},
		"version",
	),
	[]byte{},
	clienttypes.NewHeight(0, 0),
)

func (icp *IconProvider) QueryChannelClient(ctx context.Context, height int64, channelid, portid string) (*clienttypes.IdentifiedClientState, error) {
	// TODO:
	//if method given can be easily fetched...
	return nil, nil
}

// is not needed currently for the operation
// get all the channel and start the init-process
func (icp *IconProvider) QueryConnectionChannels(ctx context.Context, height int64, connectionid string) ([]*chantypes.IdentifiedChannel, error) {
	// TODO:
	//get all the channel of a connection
	return nil, nil

}

func (icp *IconProvider) QueryChannels(ctx context.Context) ([]*chantypes.IdentifiedChannel, error) {
	nextSeq, err := icp.getNextSequence(ctx, MethodGetNextChannelSequence)
	if err != nil {
		return nil, err
	}
	var channels []*chantypes.IdentifiedChannel

	testPort := "mock" //TODO:

	for i := 0; i <= int(nextSeq)-1; i++ {
		channelId := fmt.Sprintf("channel-%d", i)
		var channel_string_ string
		err := icp.client.Call(icp.prepareCallParams(MethodGetChannel, map[string]interface{}{
			"channelId": channelId,
			"portId":    testPort,
		}), &channel_string_)
		if err != nil {
			icp.log.Error("unable to fetch channel for  ", zap.String("channel id ", channelId), zap.Error(err))
			continue
		}

		var channel chantypes.Channel
		_, err = HexStringToProtoUnmarshal(channel_string_, &channel)
		if err != nil {
			icp.log.Info("Unable to unmarshal channel for ",
				zap.String("channel id ", channelId), zap.Error(err))
			continue
		}

		if channel.State == 3 {
			identifiedChannel := chantypes.IdentifiedChannel{
				State:          channel.State,
				Ordering:       channel.Ordering,
				Counterparty:   channel.Counterparty,
				ConnectionHops: channel.ConnectionHops,
				Version:        channel.Version,
				PortId:         testPort,
				ChannelId:      channelId,
			}
			channels = append(channels, &identifiedChannel)
		}
	}

	return channels, nil
}

// required to flush packets
func (icp *IconProvider) QueryPacketCommitments(ctx context.Context, height uint64, channelid, portid string) (commitments *chantypes.QueryPacketCommitmentsResponse, err error) {

	//get-all-packets
	return nil, nil

}
func (icp *IconProvider) QueryPacketAcknowledgements(ctx context.Context, height uint64, channelid, portid string) (acknowledgements []*chantypes.PacketState, err error) {
	return nil, nil
}

// legacy
func (icp *IconProvider) QueryUnreceivedPackets(ctx context.Context, height uint64, channelid, portid string, seqs []uint64) ([]uint64, error) {
	// TODO: onl
	return nil, nil
}

// legacy
func (icp *IconProvider) QueryUnreceivedAcknowledgements(ctx context.Context, height uint64, channelid, portid string, seqs []uint64) ([]uint64, error) {
	// TODO: Implement
	return nil, nil
}

// legacy
func (icp *IconProvider) QueryNextSeqRecv(ctx context.Context, height int64, channelid, portid string) (recvRes *chantypes.QueryNextSequenceReceiveResponse, err error) {
	callParam := icp.prepareCallParams(MethodGetNextSequenceReceive, map[string]interface{}{
		"portId":    portid,
		"channelId": channelid,
	})
	var nextSeqRecv uint64
	if err := icp.client.Call(callParam, &nextSeqRecv); err != nil {
		return nil, err
	}
	// TODO: Get proof and proofheight
	key := cryptoutils.GetNextSequenceRecvCommitmentKey(portid, channelid)
	keyHash := cryptoutils.Sha3keccak256(key, []byte(types.NewHexInt(int64(nextSeqRecv))))

	proof, err := icp.QueryIconProof(ctx, height, keyHash)
	if err != nil {
		return nil, err
	}

	// TODO: marshal proof using protobuf
	fmt.Println("QueryNextSeqRecv:", proof)

	return &chantypes.QueryNextSequenceReceiveResponse{
		NextSequenceReceive: nextSeqRecv,
		Proof:               nil,
		ProofHeight:         clienttypes.NewHeight(0, 0),
	}, nil
}

func (icp *IconProvider) QueryPacketCommitment(ctx context.Context, height int64, channelid, portid string, seq uint64) (comRes *chantypes.QueryPacketCommitmentResponse, err error) {
	callParam := icp.prepareCallParams(MethodGetPacketCommitment, map[string]interface{}{
		"portId":    portid,
		"channelId": channelid,
		"sequence":  seq,
	})
	var packetCommitmentBytes []byte
	if err := icp.client.Call(callParam, &packetCommitmentBytes); err != nil {
		return nil, err
	}
	if len(packetCommitmentBytes) == 0 {
		return nil, fmt.Errorf("Invalid commitment bytes")
	}

	key := cryptoutils.GetPacketCommitmentKey(portid, channelid, big.NewInt(int64(seq)))
	keyHash := cryptoutils.Sha3keccak256(key, packetCommitmentBytes)

	proof, err := icp.QueryIconProof(ctx, height, keyHash)
	if err != nil {
		return nil, err
	}

	// TODO marshal proof from Commitment
	fmt.Println("query packet commitment proofs:", proof)

	return &chantypes.QueryPacketCommitmentResponse{
		Commitment:  packetCommitmentBytes,
		Proof:       nil,
		ProofHeight: clienttypes.NewHeight(0, uint64(height)),
	}, nil
}

func (icp *IconProvider) QueryPacketAcknowledgement(ctx context.Context, height int64, channelid, portid string, seq uint64) (ackRes *chantypes.QueryPacketAcknowledgementResponse, err error) {
	callParam := icp.prepareCallParams(MethodGetPacketAcknowledgementCommitment, map[string]interface{}{
		"portId":    portid,
		"channelId": channelid,
		"sequence":  seq,
	})
	var packetAckBytes []byte
	if err := icp.client.Call(callParam, &packetAckBytes); err != nil {
		return nil, err
	}
	if len(packetAckBytes) == 0 {
		return nil, fmt.Errorf("Invalid packet bytes")
	}

	// TODO: Get proof and proofheight
	key := cryptoutils.GetPacketAcknowledgementCommitmentKey(portid, channelid, big.NewInt(height))
	keyhash := cryptoutils.Sha3keccak256(key, packetAckBytes)

	proof, err := icp.QueryIconProof(ctx, height, keyhash)
	if err != nil {
		return nil, err
	}

	// TODO : proof marshal from protobuf
	fmt.Println("QueryPacketAcknowledgement: ", proof)

	return &chantypes.QueryPacketAcknowledgementResponse{
		Acknowledgement: packetAckBytes,
		Proof:           nil,
		ProofHeight:     clienttypes.NewHeight(0, 0),
	}, nil
}

func (icp *IconProvider) QueryPacketReceipt(ctx context.Context, height int64, channelid, portid string, seq uint64) (recRes *chantypes.QueryPacketReceiptResponse, err error) {
	callParam := icp.prepareCallParams(MethodGetPacketReceipt, map[string]interface{}{
		"portId":    portid,
		"channelId": channelid,
		"sequence":  seq,
	})
	var packetReceipt []byte
	if err := icp.client.Call(callParam, &packetReceipt); err != nil {
		return nil, err
	}
	key := cryptoutils.GetPacketReceiptCommitmentKey(portid, channelid, big.NewInt(int64(seq)))
	keyHash := cryptoutils.Sha3keccak256(key, packetReceipt)

	proof, err := icp.QueryIconProof(ctx, height, keyHash)
	if err != nil {
		return nil, err
	}

	// TODO: proof -> marshal protobuf
	fmt.Println("query packet receipt:", proof)

	return &chantypes.QueryPacketReceiptResponse{
		Received:    packetReceipt != nil,
		Proof:       nil,
		ProofHeight: clienttypes.NewHeight(0, uint64(height)),
	}, nil
}

// ics 20 - transfer
// not required for icon
func (icp *IconProvider) QueryDenomTrace(ctx context.Context, denom string) (*transfertypes.DenomTrace, error) {
	return nil, fmt.Errorf("Not implemented for ICON")
}

// not required for icon
func (icp *IconProvider) QueryDenomTraces(ctx context.Context, offset, limit uint64, height int64) ([]transfertypes.DenomTrace, error) {
	return nil, fmt.Errorf("Not implemented for ICON")
}

func (icp *IconProvider) QueryIconProof(ctx context.Context, height int64, keyHash []byte) ([]icon.MerkleNode, error) {
	messages, err := icp.GetBtpMessage(height)
	if err != nil {
		return nil, err
	}
	merkleHashTree := cryptoutils.NewMerkleHashTree(messages)
	if err != nil {
		return nil, err
	}
	hashIndex := merkleHashTree.Hashes.FindIndex(keyHash)
	if hashIndex == -1 {
		return nil, errors.New("Btp message for this hash not found")
	}
	proof := merkleHashTree.MerkleProof(hashIndex)
	return proof, nil
}
