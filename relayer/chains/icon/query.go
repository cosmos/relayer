package icon

import (
	"context"
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/ibc-go/v7/modules/core/exported"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/provider"

	transfertypes "github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	committypes "github.com/cosmos/ibc-go/v7/modules/core/23-commitment/types"
	//change this to icon types after original repo merge
)

var _ provider.QueryProvider = &IconProvider{}

const (
	epoch = 24 * 3600 * 1000
)

func (icp *IconProvider) prepareCallParamForQuery(methodName string, param map[string]interface{}) *types.CallParam {

	callData := &types.CallData{
		Method: methodName,
		Params: param,
	}
	return &types.CallParam{
		FromAddress: types.NewAddress(make([]byte, 0)),
		ToAddress:   types.Address(icp.PCfg.IbcHandlerAddress),
		DataType:    "call",
		Data:        callData,
	}
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

	blk, err := icp.client.GetLastBlock()
	if err != nil {
		return 0, err
	}
	return blk.Height, nil
}

func (icp *IconProvider) QueryIBCHeader(ctx context.Context, h int64) (provider.IBCHeader, error) {
	return IconIBCHeader{}, nil
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

	callParams := icp.prepareCallParamForQuery(MethodGetClientState, map[string]interface{}{
		"height":   height,
		"clientId": srcClientId,
	})

	// get proof

	//similar should be implemented
	var clientStateByte []byte
	err := icp.client.Call(callParams, clientStateByte)
	if err != nil {
		return nil, err
	}

	var clientState exported.ClientState
	if err := icp.Codec().UnmarshalInterface(clientStateByte, &clientState); err != nil {
		return nil, err
	}
	any, err := clienttypes.PackClientState(clientState)
	if err != nil {
		return nil, err
	}

	return clienttypes.NewQueryClientStateResponse(any, nil, clienttypes.NewHeight(0, uint64(height))), nil
}

func (icp *IconProvider) QueryClientConsensusState(ctx context.Context, chainHeight int64, clientid string, clientHeight ibcexported.Height) (*clienttypes.QueryConsensusStateResponse, error) {
	callParams := icp.prepareCallParamForQuery(MethodGetConsensusState, map[string]interface{}{
		"clientId": clientid,
		"height":   clientHeight,
	})
	var cnsStateByte []byte
	err := icp.client.Call(callParams, cnsStateByte)
	if err != nil {
		return nil, err
	}
	var cnsState exported.ConsensusState
	if err := icp.codec.UnmarshalInterface(cnsStateByte, &cnsState); err != nil {
		return nil, err
	}

	any, err := clienttypes.PackConsensusState(cnsState)
	if err != nil {
		return nil, err
	}

	// get proof and proofheight from BTP Proof
	return &clienttypes.QueryConsensusStateResponse{
		ConsensusState: any,
		Proof:          nil,
		ProofHeight:    clienttypes.NewHeight(0, 0),
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
	// TODO: implement method to get all clients
	return nil, nil
}

// query connection to the ibc host based on the connection-id
func (icp *IconProvider) QueryConnection(ctx context.Context, height int64, connectionid string) (*conntypes.QueryConnectionResponse, error) {

	callParam := icp.prepareCallParamForQuery(MethodGetQueryConnection, map[string]interface{}{
		"connection_id": connectionid,
	})

	var conn conntypes.ConnectionEnd
	err := icp.client.Call(callParam, &conn)
	if err != nil {
		return emptyConnRes, err
	}
	return conntypes.NewQueryConnectionResponse(conn, nil, clienttypes.NewHeight(0, uint64(height))), nil

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
	// TODO: Get all connections in IBC
	return nil, nil
}
func (icp *IconProvider) QueryConnectionsUsingClient(ctx context.Context, height int64, clientid string) (*conntypes.QueryConnectionsResponse, error) {
	// TODO
	return nil, nil
}
func (icp *IconProvider) GenerateConnHandshakeProof(ctx context.Context, height int64, clientId, connId string) (clientState ibcexported.ClientState,
	clientStateProof []byte, consensusProof []byte, connectionProof []byte,
	connectionProofHeight ibcexported.Height, err error) {
	var (
		clientStateRes     *clienttypes.QueryClientStateResponse
		consensusStateRes  *clienttypes.QueryConsensusStateResponse
		connectionStateRes *conntypes.QueryConnectionResponse
		eg                 = new(errgroup.Group)
	)

	// query for the client state for the proof and get the height to query the consensus state at.
	clientStateRes, err = icp.QueryClientStateResponse(ctx, height, clientId)
	if err != nil {
		return nil, nil, nil, nil, clienttypes.Height{}, err
	}

	clientState, err = clienttypes.UnpackClientState(clientStateRes.ClientState)
	if err != nil {
		return nil, nil, nil, nil, clienttypes.Height{}, err
	}

	eg.Go(func() error {
		var err error
		consensusStateRes, err = icp.QueryClientConsensusState(ctx, height, clientId, clientState.GetLatestHeight())
		return err
	})
	eg.Go(func() error {
		var err error
		connectionStateRes, err = icp.QueryConnection(ctx, height, connId)
		return err
	})

	if err := eg.Wait(); err != nil {
		return nil, nil, nil, nil, clienttypes.Height{}, err
	}

	return clientState, clientStateRes.Proof, consensusStateRes.Proof, connectionStateRes.Proof, connectionStateRes.ProofHeight, nil
}

// ics 04 - channel
func (icp *IconProvider) QueryChannel(ctx context.Context, height int64, channelid, portid string) (chanRes *chantypes.QueryChannelResponse, err error) {

	callParam := icp.prepareCallParamForQuery(MethodGetChannel, map[string]interface{}{
		"channelId": channelid,
	})

	var channelRes chantypes.Channel
	err = icp.client.Call(callParam, &channelRes)
	if err != nil {
		return emptyChannelRes, err
	}

	//TODO: get Proof and proofHeight for channel to incude it response

	return chantypes.NewQueryChannelResponse(channelRes, []byte{}, clienttypes.NewHeight(0, uint64(height))), nil
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

	//get all the identified channels listed in the handler

	return nil, nil

}
func (icp *IconProvider) QueryPacketCommitments(ctx context.Context, height uint64, channelid, portid string) (commitments *chantypes.QueryPacketCommitmentsResponse, err error) {

	//get-all-packets
	return nil, nil

}
func (icp *IconProvider) QueryPacketAcknowledgements(ctx context.Context, height uint64, channelid, portid string) (acknowledgements []*chantypes.PacketState, err error) {
	return nil, nil
}

func (icp *IconProvider) QueryUnreceivedPackets(ctx context.Context, height uint64, channelid, portid string, seqs []uint64) ([]uint64, error) {
	// TODO: Implement
	return nil, nil
}

func (icp *IconProvider) QueryUnreceivedAcknowledgements(ctx context.Context, height uint64, channelid, portid string, seqs []uint64) ([]uint64, error) {
	// TODO: Implement
	return nil, nil
}

func (icp *IconProvider) QueryNextSeqRecv(ctx context.Context, height int64, channelid, portid string) (recvRes *chantypes.QueryNextSequenceReceiveResponse, err error) {
	callParam := icp.prepareCallParamForQuery(MethodGetNextSequenceReceive, map[string]interface{}{
		"portId":    portid,
		"channelId": channelid,
	})
	var nextSeqRecv uint64
	if err := icp.client.Call(callParam, &nextSeqRecv); err != nil {
		return nil, err
	}
	// TODO: Get proof and proofheight
	return &chantypes.QueryNextSequenceReceiveResponse{
		NextSequenceReceive: nextSeqRecv,
		Proof:               nil,
		ProofHeight:         clienttypes.NewHeight(0, 0),
	}, nil
}

func (icp *IconProvider) QueryPacketCommitment(ctx context.Context, height int64, channelid, portid string, seq uint64) (comRes *chantypes.QueryPacketCommitmentResponse, err error) {
	callParam := icp.prepareCallParamForQuery(MethodGetPacketCommitment, map[string]interface{}{
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
	// TODO: Get proof and proofheight
	return &chantypes.QueryPacketCommitmentResponse{
		Commitment:  packetCommitmentBytes,
		Proof:       nil,
		ProofHeight: clienttypes.NewHeight(0, 0),
	}, nil
}

func (icp *IconProvider) QueryPacketAcknowledgement(ctx context.Context, height int64, channelid, portid string, seq uint64) (ackRes *chantypes.QueryPacketAcknowledgementResponse, err error) {
	callParam := icp.prepareCallParamForQuery(MethodGetPacketAcknowledgementCommitment, map[string]interface{}{
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
	return &chantypes.QueryPacketAcknowledgementResponse{
		Acknowledgement: packetAckBytes,
		Proof:           nil,
		ProofHeight:     clienttypes.NewHeight(0, 0),
	}, nil
	return nil, nil
}

func (icp *IconProvider) QueryPacketReceipt(ctx context.Context, height int64, channelid, portid string, seq uint64) (recRes *chantypes.QueryPacketReceiptResponse, err error) {
	callParam := icp.prepareCallParamForQuery(MethodHasPacketReceipt, map[string]interface{}{
		"portId":    portid,
		"channelId": channelid,
		"sequence":  seq,
	})
	var hasPacketReceipt bool
	if err := icp.client.Call(callParam, &hasPacketReceipt); err != nil {
		return nil, err
	}
	// TODO: Get proof and proofheight
	return &chantypes.QueryPacketReceiptResponse{
		Received:    hasPacketReceipt,
		Proof:       nil,
		ProofHeight: clienttypes.NewHeight(0, 0),
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
