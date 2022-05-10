package substrate

import (
	"bytes"
	"context"
	"fmt"
	"go.uber.org/zap"
	"io"
	"reflect"
	"time"

	"github.com/ComposableFi/go-substrate-rpc-client/scale"
	rpcClient "github.com/ComposableFi/go-substrate-rpc-client/v4"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	ibcexported "github.com/cosmos/ibc-go/v3/modules/core/exported"
	beefyclient "github.com/cosmos/ibc-go/v3/modules/light-clients/11-beefy/types"
	beefyclientTypes "github.com/cosmos/ibc-go/v3/modules/light-clients/11-beefy/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/cosmos/relayer/v2/relayer/provider/substrate/keystore"
)

var (
	_ provider.ChainProvider  = (*SubstrateProvider)(nil)
	_ provider.ProviderConfig = (*SubstrateProviderConfig)(nil)
	_ provider.RelayPacket    = (*SubstrateRelayPacket)(nil)
	_ provider.RelayerMessage = (*SubstrateRelayerMessage)(nil)
)

type SubstrateProvider struct {
	Config    *SubstrateProviderConfig
	RPCClient *rpcClient.SubstrateAPI
	Keybase   keystore.Keyring
	Input     io.Reader
}

type SubstrateRelayerMessage struct {
	Msg Msg
}

// (ccc *ChainClientConfig, homepath string, input io.Reader, output io.Writer, kro ...keyring.Option) (*ChainClient, error) {
func NewSubstrateProvider(spc *SubstrateProviderConfig, homepath string) (*SubstrateProvider, error) {
	sp := &SubstrateProvider{
		//TODO: create keybase instance ?
		Keybase: keystore.NewInMemory(),
		Config:  spc,
	}
	err := sp.Init()
	if err != nil {
		return nil, err
	}

	return sp, nil
}

// Log takes a string and logs the data
func (sp *SubstrateProvider) Log(s string) {
	// TODO: implement logger
}

func (sp *SubstrateProvider) QueryConsensusStateABCI(clientID string, height ibcexported.Height) (*clienttypes.QueryConsensusStateResponse, error) {
	res, err := sp.RPCClient.RPC.IBC.QueryConsensusState(uint32(height.GetRevisionHeight()))
	if err != nil {
		return nil, err
	}

	// check if consensus state exists
	if len(res.Proof) == 0 {
		return nil, fmt.Errorf(ErrTextConsensusStateNotFound, clientID)
	}

	return &clienttypes.QueryConsensusStateResponse{
		ConsensusState: res.ConsensusState,
		Proof:          res.Proof,
		ProofHeight:    res.ProofHeight,
	}, nil
}

// queryTMClientState retrieves the latest consensus state for a client in state at a given height
// and unpacks/cast it to tendermint clientstate
func (sp *SubstrateProvider) queryTMClientState(ctx context.Context, srch int64, srcClientId string) (*beefyclientTypes.ClientState, error) {
	clientStateRes, err := sp.QueryClientStateResponse(ctx, srch, srcClientId)
	if err != nil {
		return &beefyclientTypes.ClientState{}, err
	}

	return castClientStateToBeefyType(clientStateRes.ClientState)
}

// TODO
func (sp *SubstrateProvider) BlockTime(ctx context.Context, height int64) (int64, error) {
	return 0, nil
}

type SubstrateProviderConfig struct {
	Timeout        string `json:"timeout" yaml:"timeout"`
	RPCAddr        string `json:"rpc-addr" yaml:"rpc-addr"`
	ChainID        string `json:"chain-id" yaml:"chain-id"`
	Key            string `json:"key" yaml:"key"`
	KeyringBackend string `json:"keyring-backend" yaml:"keyring-backend"`
	KeyDirectory   string `json:"key-directory" yaml:"key-directory"`
	Debug          bool   `json:"debug" yaml:"debug"`
}

func (spc *SubstrateProviderConfig) NewProvider(log *zap.Logger, homepath string, debug bool) (provider.ChainProvider, error) {
	return NewSubstrateProvider(spc, "")
}

func (spc *SubstrateProviderConfig) Validate() error {
	if _, err := time.ParseDuration(spc.Timeout); err != nil {
		return err
	}
	return nil
}

type SubstrateRelayPacket struct {
	packetData         []byte
	seq                uint64
	sourcePort         string
	destinationPort    string
	destinationChannel string
	timeout            clienttypes.Height
	timeoutStamp       uint64
	dstRecvRes         *chantypes.QueryPacketReceiptResponse
}

func (srp *SubstrateRelayPacket) Msg(
	src provider.ChainProvider,
	srcPortId,
	srcChanId,
	dstPortId,
	dstChanId string,
) (provider.RelayerMessage, error) {
	if srp.dstRecvRes == nil {
		return nil, fmt.Errorf("timeout packet [%s]seq{%d} has no associated proofs", src.ChainId(), srp.seq)
	}
	addr, err := src.Address()
	if err != nil {
		return nil, err
	}

	msg := &chantypes.MsgTimeout{
		Packet: chantypes.Packet{
			Sequence:           srp.seq,
			SourcePort:         srcPortId,
			SourceChannel:      srcChanId,
			DestinationPort:    dstPortId,
			DestinationChannel: dstChanId,
			Data:               srp.packetData,
			TimeoutHeight:      srp.timeout,
			TimeoutTimestamp:   srp.timeoutStamp,
		},
		ProofUnreceived:  srp.dstRecvRes.Proof,
		ProofHeight:      srp.dstRecvRes.ProofHeight,
		NextSequenceRecv: srp.seq,
		Signer:           addr,
	}

	return NewSubstrateRelayerMessage(msg), nil
}

// TODO: find out what FetchCommitResponse does
func (srp *SubstrateRelayPacket) FetchCommitResponse(ctx context.Context, dst provider.ChainProvider, queryHeight uint64, dstChanId, dstPortId string) error {
	dstRecvRes, err := dst.QueryPacketReceipt(ctx, int64(queryHeight)-1, dstChanId, dstPortId, srp.seq)
	switch {
	case err != nil:
		return err
	case dstRecvRes.Proof == nil:
		return fmt.Errorf("timeout packet receipt proof seq(%d) is nil", srp.seq)
	default:
		srp.dstRecvRes = dstRecvRes
		return nil
	}
}

func (srp *SubstrateRelayPacket) Data() []byte {
	return srp.packetData
}

func (srp *SubstrateRelayPacket) Seq() uint64 {
	return srp.seq
}

func (srp *SubstrateRelayPacket) Timeout() clienttypes.Height {
	return srp.timeout
}

func (srp *SubstrateRelayPacket) TimeoutStamp() uint64 {
	return srp.timeoutStamp
}

func (srm SubstrateRelayerMessage) Type() string {
	return "substrate"
}

func (srm SubstrateRelayerMessage) MsgBytes() ([]byte, error) {
	var buf bytes.Buffer
	enc := scale.NewEncoder(&buf)
	err := enc.Encode(srm.Msg)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// castClientStateToBeefyType casts client state to beefy type
func castClientStateToBeefyType(cs *codectypes.Any) (*beefyclient.ClientState, error) {
	clientStateExported, err := clienttypes.UnpackClientState(cs)
	if err != nil {
		return &beefyclientTypes.ClientState{}, err
	}

	// cast from interface to concrete type
	clientState, ok := clientStateExported.(*beefyclientTypes.ClientState)
	if !ok {
		return &beefyclientTypes.ClientState{},
			fmt.Errorf("error when casting exported clientstate to tendermint type")
	}

	return clientState, nil
}

// isMatchingClient determines if the two provided clients match in all fields
// except latest height. They are assumed to be IBC tendermint light clients.
// NOTE: we don't pass in a pointer so upstream references don't have a modified
// latest height set to zero.
func isMatchingClient(clientStateA, clientStateB *beefyclientTypes.ClientState) bool {
	// zero out latest client height since this is determined and incremented
	// by on-chain updates. Changing the latest height does not fundamentally
	// change the client. The associated consensus state at the latest height
	// determines this last check
	clientStateA.LatestBeefyHeight = 0
	clientStateB.LatestBeefyHeight = 0

	return reflect.DeepEqual(clientStateA, clientStateB)
}

// isMatchingConsensusState determines if the two provided consensus states are
// identical. They are assumed to be IBC tendermint light clients.
func isMatchingConsensusState(consensusStateA, consensusStateB *beefyclientTypes.ConsensusState) bool {
	return reflect.DeepEqual(*consensusStateA, *consensusStateB)
}

// MustGetHeight takes the height inteface and returns the actual height
func MustGetHeight(h ibcexported.Height) clienttypes.Height {
	height, ok := h.(clienttypes.Height)
	if !ok {
		panic("height is not an instance of height!")
	}
	return height
}

// relayPacketsFromPacket looks through the events in a *ctypes.ResultTx
// and returns relayPackets with the appropriate data
func relayPacketsFromPacket(ctx context.Context, src, dst provider.ChainProvider, dsth int64, allPackets []chantypes.Packet, dstChanId, dstPortId, srcChanId, srcPortId, srcClientId string) ([]provider.RelayPacket, []provider.RelayPacket, error) {
	var (
		rcvPackets     []provider.RelayPacket
		timeoutPackets []provider.RelayPacket
	)

	for _, e := range allPackets {
		// NOTE: Src and Dst are switched here
		rp := &relayMsgRecvPacket{pass: false}

		if e.SourceChannel != srcChanId {
			rp.pass = true
			continue
		}

		if e.DestinationChannel != dstChanId {
			rp.pass = true
			continue
		}

		if e.SourcePort != srcPortId {
			rp.pass = true
			continue
		}

		if e.DestinationPort != dstPortId {
			rp.pass = true
			continue
		}

		rp.packetData = e.Data
		timeout, err := clienttypes.ParseHeight(e.TimeoutHeight.String())
		if err != nil {
			return nil, nil, err
		}

		rp.timeout = timeout
		rp.timeoutStamp = e.TimeoutTimestamp
		rp.seq = e.Sequence

		// fetch the header which represents a block produced on destination
		block, err := dst.GetIBCUpdateHeader(ctx, dsth, src, srcClientId)
		if err != nil {
			return nil, nil, err
		}

		switch {
		// If the packet has a timeout height, and it has been reached, return a timeout packet
		case !rp.timeout.IsZero() && block.GetHeight().GTE(rp.timeout):
			timeoutPackets = append(timeoutPackets, rp.timeoutPacket())
		// If the packet matches the relay constraints relay it as a MsgReceivePacket
		case !rp.pass:
			rcvPackets = append(rcvPackets, rp)
		}

	}

	// If there is a relayPacket, return it
	if len(rcvPackets)+len(timeoutPackets) > 0 {
		return rcvPackets, timeoutPackets, nil
	}

	return nil, nil, fmt.Errorf("no packet data found")
}
