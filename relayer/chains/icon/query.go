package icon

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/exported"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	//this import should be letter converted to icon types

	"github.com/cosmos/relayer/v2/relayer/chains/icon/cryptoutils"
	"github.com/cosmos/relayer/v2/relayer/chains/icon/types"
	"github.com/cosmos/relayer/v2/relayer/common"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/icon-project/IBC-Integration/libraries/go/common/icon"
	itm "github.com/icon-project/IBC-Integration/libraries/go/common/tendermint"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
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
	epoch            = 24 * 3600 * 1000
	clientNameString = "07-tendermint-"
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
	var block *types.Block
	var err error
	retry.Do(func() error {
		block, err = icp.client.GetLastBlock()
		return err
	}, retry.Context(ctx),
		retry.Attempts(queryRetries),
		retry.OnRetry(func(n uint, err error) {
			icp.log.Warn("failed to query latestHeight", zap.String("Chain Id", icp.ChainId()))
		}))
	return block.Height, err
}

// legacy
func (icp *IconProvider) QueryIBCHeader(ctx context.Context, h int64) (provider.IBCHeader, error) {

	validators, err := icp.GetProofContextByHeight(h)
	if err != nil {
		return nil, err
	}
	header, err := icp.GetBtpHeader(h)
	if err != nil {
		if btpBlockNotPresent(err) {
			return NewIconIBCHeader(nil, validators, int64(h)), nil
		}
	}
	return NewIconIBCHeader(header, validators, int64(header.MainHeight)), err
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

func (icp *IconProvider) QueryClientStateWithoutProof(ctx context.Context, height int64, clientid string) (ibcexported.ClientState, error) {
	callParams := icp.prepareCallParams(MethodGetClientState, map[string]interface{}{
		"clientId": clientid,
	}, callParamsWithHeight(types.NewHexInt(height)))

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

	// TODO: Use ICON Client State after cosmos chain integrated--
	any, err := icp.ClientToAny(clientid, clientStateByte)
	if err != nil {
		return nil, err
	}

	clientStateRes := clienttypes.NewQueryClientStateResponse(any, nil, clienttypes.NewHeight(0, uint64(height)))
	clientStateExported, err := clienttypes.UnpackClientState(clientStateRes.ClientState)
	if err != nil {
		return nil, err
	}

	return clientStateExported, nil

}

// Implement when a new chain is added to ICON IBC Contract
func (icp *IconProvider) ClientToAny(clientId string, clientStateB []byte) (*codectypes.Any, error) {
	if strings.Contains(clientId, "icon") {
		var clientState icon.ClientState
		err := icp.codec.Marshaler.Unmarshal(clientStateB, &clientState)
		if err != nil {
			return nil, err
		}
		return clienttypes.PackClientState(&clientState)
	}
	if strings.Contains(clientId, "tendermint") {
		var clientState itm.ClientState
		err := icp.codec.Marshaler.Unmarshal(clientStateB, &clientState)
		if err != nil {
			return nil, err
		}

		return clienttypes.PackClientState(&clientState)
	}
	return nil, fmt.Errorf("unknown client type")
}

func (icp *IconProvider) ConsensusToAny(clientId string, cb []byte) (*codectypes.Any, error) {
	if strings.Contains(clientId, "icon") {
		var consensusState icon.ConsensusState
		err := icp.codec.Marshaler.Unmarshal(cb, &consensusState)
		if err != nil {
			return nil, err
		}
		return clienttypes.PackConsensusState(&consensusState)
	}
	if strings.Contains(clientId, "tendermint") {
		var consensusState itm.ConsensusState
		err := icp.codec.Marshaler.Unmarshal(cb, &consensusState)
		if err != nil {
			return nil, err
		}

		return clienttypes.PackConsensusState(&consensusState)
	}
	return nil, fmt.Errorf("unknown consensus type")
}

func (icp *IconProvider) QueryClientStateResponse(ctx context.Context, height int64, srcClientId string) (*clienttypes.QueryClientStateResponse, error) {

	callParams := icp.prepareCallParams(MethodGetClientState, map[string]interface{}{
		"clientId": srcClientId,
	}, callParamsWithHeight(types.NewHexInt(height)))

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

	// TODO: Use ICON Client State after cosmos chain integrated--
	any, err := icp.ClientToAny(srcClientId, clientStateByte)
	if err != nil {
		return nil, err
	}

	commitmentHash := getCommitmentHash(common.GetClientStateCommitmentKey(srcClientId), clientStateByte)

	proof, err := icp.QueryIconProof(ctx, height, commitmentHash)
	if err != nil {
		return nil, err
	}

	return clienttypes.NewQueryClientStateResponse(any, proof, clienttypes.NewHeight(0, uint64(height))), nil
}

func (icp *IconProvider) QueryClientConsensusState(ctx context.Context, chainHeight int64, clientid string, clientHeight ibcexported.Height) (*clienttypes.QueryConsensusStateResponse, error) {
	callParams := icp.prepareCallParams(MethodGetConsensusState, map[string]interface{}{
		"clientId": clientid,
		"height":   clientHeight,
	})
	var cnsStateHexByte types.HexBytes
	err := icp.client.Call(callParams, cnsStateHexByte)
	if err != nil {
		return nil, err
	}
	cnsStateByte, err := cnsStateHexByte.Value()
	if err != nil {
		return nil, err
	}

	any, err := icp.ConsensusToAny(clientid, cnsStateByte)
	if err != nil {
		return nil, err
	}

	key := common.GetConsensusStateCommitmentKey(clientid, big.NewInt(0), big.NewInt(chainHeight))
	commitmentHash := getCommitmentHash(key, cnsStateByte)

	proof, err := icp.QueryIconProof(ctx, chainHeight, commitmentHash)
	if err != nil {
		return nil, err
	}

	return &clienttypes.QueryConsensusStateResponse{
		ConsensusState: any,
		Proof:          proof,
		ProofHeight:    clienttypes.NewHeight(0, uint64(chainHeight)),
	}, nil
}

func (icp *IconProvider) QueryUpgradedClient(ctx context.Context, height int64) (*clienttypes.QueryClientStateResponse, error) {
	return nil, fmt.Errorf("Not implemented for ICON")
}

func (icp *IconProvider) QueryUpgradedConsState(ctx context.Context, height int64) (*clienttypes.QueryConsensusStateResponse, error) {
	return nil, fmt.Errorf("Not implemented for ICON")
}

func (icp *IconProvider) QueryConsensusState(ctx context.Context, height int64) (ibcexported.ConsensusState, int64, error) {
	return nil, height, fmt.Errorf("Not implemented for ICON. Check QueryClientConsensusState instead")
}

// query all the clients of the chain
func (icp *IconProvider) QueryClients(ctx context.Context) (clienttypes.IdentifiedClientStates, error) {
	seq, err := icp.getNextSequence(ctx, MethodGetNextClientSequence)

	if err != nil {
		return nil, err
	}

	if seq == 0 {
		return nil, nil
	}

	identifiedClientStates := make(clienttypes.IdentifiedClientStates, 0)
	for i := 0; i <= int(seq)-1; i++ {
		clientIdentifier := fmt.Sprintf("%s%d", clientNameString, i)
		callParams := icp.prepareCallParams(MethodGetClientState, map[string]interface{}{
			"clientId": clientIdentifier,
		})

		//similar should be implemented
		var clientStateB types.HexBytes
		err := icp.client.Call(callParams, &clientStateB)
		if err != nil {
			return nil, err
		}
		clientStateBytes, _ := clientStateB.Value()

		// TODO: Use ICON Client State after cosmos chain integrated--
		var clientState itm.ClientState
		if err = icp.codec.Marshaler.Unmarshal(clientStateBytes, &clientState); err != nil {
			return nil, err
		}

		identifiedClientStates = append(identifiedClientStates, clienttypes.NewIdentifiedClientState(clientIdentifier, &clientState))

	}
	return identifiedClientStates, nil
}

// query connection to the ibc host based on the connection-id
func (icp *IconProvider) QueryConnection(ctx context.Context, height int64, connectionid string) (*conntypes.QueryConnectionResponse, error) {

	callParam := icp.prepareCallParams(MethodGetConnection, map[string]interface{}{
		"connectionId": connectionid,
	}, callParamsWithHeight(types.NewHexInt(height)))

	var conn_string_ types.HexBytes
	err := icp.client.Call(callParam, &conn_string_)
	if err != nil {
		return emptyConnRes, err
	}

	connectionBytes, err := conn_string_.Value()
	if err != nil {
		return emptyConnRes, err
	}

	var conn conntypes.ConnectionEnd
	_, err = icp.HexBytesToProtoUnmarshal(connectionBytes, &conn)
	if err != nil {
		return emptyConnRes, err
	}

	key := common.GetConnectionCommitmentKey(connectionid)
	commitmentHash := getCommitmentHash(key, connectionBytes)

	proof, err := icp.QueryIconProof(ctx, height, commitmentHash)
	if err != nil {
		return emptyConnRes, err
	}

	return conntypes.NewQueryConnectionResponse(conn, proof, clienttypes.NewHeight(0, uint64(height))), nil

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
	if nextSeq == 0 {
		return nil, nil
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
		// Only return open conenctions
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
	return nil, fmt.Errorf("not implemented")
}
func (icp *IconProvider) GenerateConnHandshakeProof(ctx context.Context, height int64, clientId, connId string) (clientState ibcexported.ClientState,
	clientStateProof []byte, consensusProof []byte, connectionProof []byte,
	connectionProofHeight ibcexported.Height, err error) {

	// clientProof
	clientResponse, err := icp.QueryClientStateResponse(ctx, height, clientId)
	if err != nil {
		return nil, nil, nil, nil, clienttypes.Height{}, err
	}

	// consensusProof
	anyClientState := clientResponse.ClientState
	clientState_, err := clienttypes.UnpackClientState(anyClientState)
	if err != nil {
		return nil, nil, nil, nil, clienttypes.Height{}, err
	}

	// TODO: Once you stop using mock client, uncomment this block of code
	// x := clientState_.GetLatestHeight()
	// consensusResponse, err := icp.QueryClientConsensusState(ctx, height, clientId, x)
	// if err != nil {
	// 	return nil, nil, nil, nil, clienttypes.Height{}, err
	// }
	///// AND REMOVE THIS LINE  /////
	type consensusResponseX struct {
		Proof []byte
	}

	consensusResponse := consensusResponseX{
		Proof: []byte("0x"),
	}

	// connectionProof
	connResponse, err := icp.QueryConnection(ctx, height, connId)
	if err != nil {
		return nil, nil, nil, nil, clienttypes.Height{}, err
	}

	return clientState_, clientResponse.Proof, consensusResponse.Proof, connResponse.Proof, clienttypes.NewHeight(0, uint64(height)), nil
}

// ics 04 - channel
func (icp *IconProvider) QueryChannel(ctx context.Context, height int64, channelid, portid string) (chanRes *chantypes.QueryChannelResponse, err error) {

	callParam := icp.prepareCallParams(MethodGetChannel, map[string]interface{}{
		"channelId": channelid,
		"portId":    portid,
	}, callParamsWithHeight(types.NewHexInt(height)))

	var channel_string_ types.HexBytes
	err = icp.client.Call(callParam, &channel_string_)
	if err != nil {
		return emptyChannelRes, err
	}

	channelBytes, err := channel_string_.Value()
	if err != nil {
		return emptyChannelRes, err
	}

	var channel icon.Channel
	_, err = icp.HexBytesToProtoUnmarshal(channelBytes, &channel)
	if err != nil {
		return emptyChannelRes, err
	}

	channelCommitment := common.GetChannelCommitmentKey(portid, channelid)
	commitmentHash := getCommitmentHash(channelCommitment, channelBytes)
	proof, err := icp.QueryIconProof(ctx, height, commitmentHash)
	if err != nil {
		return emptyChannelRes, err
	}

	cosmosChan := chantypes.NewChannel(
		chantypes.State(channel.State),
		chantypes.Order(channel.Ordering),
		chantypes.NewCounterparty(
			channel.Counterparty.PortId,
			channel.Counterparty.ChannelId),
		channel.ConnectionHops,
		channel.Version,
	)

	return chantypes.NewQueryChannelResponse(cosmosChan, proof, clienttypes.NewHeight(0, uint64(height))), nil
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

		// check if the channel is open
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
	}, callParamsWithHeight(types.NewHexInt(height)))
	var nextSeqRecv uint64
	if err := icp.client.Call(callParam, &nextSeqRecv); err != nil {
		return nil, err
	}
	// TODO: Get proof and proofheight
	key := common.GetNextSequenceRecvCommitmentKey(portid, channelid)
	keyHash := common.Sha3keccak256(key, []byte(types.NewHexInt(int64(nextSeqRecv))))

	proof, err := icp.QueryIconProof(ctx, height, keyHash)
	if err != nil {
		return nil, err
	}

	return &chantypes.QueryNextSequenceReceiveResponse{
		NextSequenceReceive: nextSeqRecv,
		Proof:               proof,
		ProofHeight:         clienttypes.NewHeight(0, uint64(height)),
	}, nil
}

func (icp *IconProvider) QueryPacketCommitment(ctx context.Context, height int64, channelid, portid string, seq uint64) (comRes *chantypes.QueryPacketCommitmentResponse, err error) {
	callParam := icp.prepareCallParams(MethodGetPacketCommitment, map[string]interface{}{
		"portId":    portid,
		"channelId": channelid,
		"sequence":  seq,
	}, callParamsWithHeight(types.NewHexInt(height)))
	var packetCommitmentHexBytes types.HexBytes
	if err := icp.client.Call(callParam, &packetCommitmentHexBytes); err != nil {
		return nil, err
	}
	packetCommitmentBytes, err := packetCommitmentHexBytes.Value()
	if err != nil {
		return nil, err
	}
	if len(packetCommitmentBytes) == 0 {
		return nil, fmt.Errorf("Invalid commitment bytes")
	}

	key := common.GetPacketCommitmentKey(portid, channelid, big.NewInt(int64(seq)))
	keyHash := common.Sha3keccak256(key, packetCommitmentBytes)
	proof, err := icp.QueryIconProof(ctx, height, keyHash)
	if err != nil {
		return nil, err
	}

	return &chantypes.QueryPacketCommitmentResponse{
		Commitment:  packetCommitmentBytes,
		Proof:       proof,
		ProofHeight: clienttypes.NewHeight(0, uint64(height)),
	}, nil
}

func (icp *IconProvider) QueryPacketAcknowledgement(ctx context.Context, height int64, channelid, portid string, seq uint64) (ackRes *chantypes.QueryPacketAcknowledgementResponse, err error) {
	callParam := icp.prepareCallParams(MethodGetPacketAcknowledgementCommitment, map[string]interface{}{
		"portId":    portid,
		"channelId": channelid,
		"sequence":  seq,
	}, callParamsWithHeight(types.NewHexInt(height)))

	var packetAckHexBytes types.HexBytes
	if err := icp.client.Call(callParam, &packetAckHexBytes); err != nil {
		return nil, err
	}
	packetAckBytes, err := packetAckHexBytes.Value()
	if err != nil {
		return nil, err
	}
	if len(packetAckBytes) == 0 {
		return nil, fmt.Errorf("Invalid packet bytes")
	}

	key := common.GetPacketAcknowledgementCommitmentKey(portid, channelid, big.NewInt(height))
	keyhash := common.Sha3keccak256(key, packetAckBytes)

	proof, err := icp.QueryIconProof(ctx, height, keyhash)
	if err != nil {
		return nil, err
	}

	return &chantypes.QueryPacketAcknowledgementResponse{
		Acknowledgement: packetAckBytes,
		Proof:           proof,
		ProofHeight:     clienttypes.NewHeight(0, uint64(height)),
	}, nil
}

func (icp *IconProvider) QueryPacketReceipt(ctx context.Context, height int64, channelid, portid string, seq uint64) (recRes *chantypes.QueryPacketReceiptResponse, err error) {
	callParam := icp.prepareCallParams(MethodHasPacketReceipt, map[string]interface{}{
		"portId":    portid,
		"channelId": channelid,
		"sequence":  seq,
	})
	var packetReceiptHexByte types.HexInt
	if err := icp.client.Call(callParam, &packetReceiptHexByte); err != nil {
		return nil, err
	}
	packetReceipt, err := packetReceiptHexByte.Value()
	if err != nil {
		return nil, err
	}
	// TODO:: Is there packetReceipt proof on ICON??
	// key := common.GetPacketReceiptCommitmentKey(portid, channelid, big.NewInt(int64(seq)))
	// keyHash := common.Sha3keccak256(key)

	// proof, err := icp.QueryIconProof(ctx, height, keyHash)
	// if err != nil {
	// 	return nil, err
	// }

	return &chantypes.QueryPacketReceiptResponse{
		Received:    packetReceipt == 1,
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

func (icp *IconProvider) QueryIconProof(ctx context.Context, height int64, keyHash []byte) ([]byte, error) {
	merkleProofs := icon.MerkleProofs{}

	messages, err := icp.GetBtpMessage(height)
	if err != nil {
		return nil, err
	}
	if len(messages) == 0 {
		icp.log.Info("BTP Message not present",
			zap.Int64("Height", height),
			zap.Int64("BtpNetwork", icp.PCfg.BTPNetworkID))
		return nil, err
	}

	if len(messages) > 1 {
		merkleHashTree := cryptoutils.NewMerkleHashTree(messages)
		if err != nil {
			return nil, err
		}
		hashIndex := merkleHashTree.Hashes.FindIndex(keyHash)
		if hashIndex == -1 {
			return nil, errors.New("Btp message for this hash not found")
		}
		proof := merkleHashTree.MerkleProof(hashIndex)

		merkleProofs = icon.MerkleProofs{
			Proofs: proof,
		}
	}

	proofBytes, err := icp.codec.Marshaler.Marshal(&merkleProofs)
	return proofBytes, nil
}

func (icp *IconProvider) HexStringToProtoUnmarshal(encoded string, v proto.Message) ([]byte, error) {
	if encoded == "" {
		return nil, fmt.Errorf("Encoded string is empty ")
	}

	input_ := strings.TrimPrefix(encoded, "0x")
	inputBytes, err := hex.DecodeString(input_)
	if err != nil {
		return nil, err
	}

	err = icp.codec.Marshaler.UnmarshalInterface(inputBytes, v)
	if err != nil {
		return nil, err
	}
	return inputBytes, nil

}

func (icp *IconProvider) HexBytesToProtoUnmarshal(inputBytes []byte, v proto.Message) ([]byte, error) {

	if bytes.Equal(inputBytes, make([]byte, 0)) {
		return nil, fmt.Errorf("Encoded hexbyte is empty ")
	}

	// TODO: To use this, register all to codec
	// err := icp.codec.Marshaler.UnmarshalInterface(inputBytes, v)
	err := proto.Unmarshal(inputBytes, v)
	if err != nil {
		return nil, err
	}
	return inputBytes, nil

}

func getCommitmentHash(key, msg []byte) []byte {
	msgHash := common.Sha3keccak256(msg)
	return common.Sha3keccak256(key, msgHash)
}
