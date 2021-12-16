package cosmos

import (
	"context"
	"errors"
	"fmt"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	tmclient "github.com/cosmos/ibc-go/v2/modules/light-clients/07-tendermint/types"
	"github.com/cosmos/relayer/relayer/provider"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/store/rootmulti"
	querytypes "github.com/cosmos/cosmos-sdk/types/query"
	clienttypes "github.com/cosmos/ibc-go/v2/modules/core/02-client/types"
	committypes "github.com/cosmos/ibc-go/v2/modules/core/23-commitment/types"
	abci "github.com/tendermint/tendermint/abci/types"
	rpcclient "github.com/tendermint/tendermint/rpc/client"

	sdkCtx "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authTypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	libclient "github.com/tendermint/tendermint/rpc/jsonrpc/client"
)

var (
	// Strings for parsing events
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

func newRPCClient(addr string, timeout time.Duration) (*rpchttp.HTTP, error) {
	httpClient, err := libclient.DefaultHTTPClient(addr)
	if err != nil {
		return nil, err
	}

	httpClient.Timeout = timeout
	rpcClient, err := rpchttp.NewWithClient(addr, "/websocket", httpClient)
	if err != nil {
		return nil, err
	}

	return rpcClient, nil
}

func getMsgAction(msgs []sdk.Msg) string {
	var out string
	for i, msg := range msgs {
		out += fmt.Sprintf("%d:%s,", i, sdk.MsgTypeURL(msg))
	}
	return strings.TrimSuffix(out, ",")
}

// GetAddress returns the sdk.AccAddress associated with the configured key
func (cp *CosmosProvider) GetAddress() (sdk.AccAddress, error) {
	done := cp.UseSDKContext()
	defer done()
	if cp.address != nil {
		return cp.address, nil
	}

	// Signing key for c chain
	srcAddr, err := cp.Keybase.Key(cp.Config.Key)
	if err != nil {
		return nil, err
	}

	return srcAddr.GetAddress(), nil
}

// MustGetAddress used for brevity
func (cp *CosmosProvider) MustGetAddress() string {
	srcAddr, err := cp.GetAddress()
	if err != nil {
		panic(err)
	}
	done := cp.UseSDKContext()
	defer done()
	return srcAddr.String()
}

// CLIContext returns an instance of client.Context derived from Chain
func (cp *CosmosProvider) CLIContext(height int64) sdkCtx.Context {
	addr, err := cp.GetAddress()
	if err != nil {
		panic(err)
	}
	return sdkCtx.Context{}.
		WithChainID(cp.Config.ChainID).
		WithCodec(cp.Encoding.Marshaler).
		WithInterfaceRegistry(cp.Encoding.InterfaceRegistry).
		WithTxConfig(cp.Encoding.TxConfig).
		WithLegacyAmino(cp.Encoding.Amino).
		WithInput(os.Stdin).
		WithNodeURI(cp.Config.RPCAddr).
		WithClient(cp.Client).
		WithAccountRetriever(authTypes.AccountRetriever{}).
		WithBroadcastMode(flags.BroadcastBlock).
		WithKeyring(cp.Keybase).
		WithOutputFormat("json").
		WithFrom(cp.Config.Key).
		WithFromName(cp.Config.Key).
		WithFromAddress(addr).
		WithSkipConfirmation(true).
		WithNodeURI(cp.Config.RPCAddr).
		WithHeight(height)
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

// QueryABCI is an affordance for querying the ABCI server associated with a chain
// Similar to cliCtx.QueryABCI
func (cp *CosmosProvider) QueryABCI(req abci.RequestQuery) (res abci.ResponseQuery, err error) {
	opts := rpcclient.ABCIQueryOptions{
		Height: req.GetHeight(),
		Prove:  req.Prove,
	}

	result, err := cp.Client.ABCIQueryWithOptions(context.Background(), req.Path, req.Data, opts)
	if err != nil {
		// retry queries on EOF
		if strings.Contains(err.Error(), "EOF") {
			if cp.debug {
				cp.Error(err)
			}
			return cp.QueryABCI(req)
		}
		return res, err
	}

	if !result.Response.IsOK() {
		return res, errors.New(result.Response.Log)
	}

	// data from trusted node or subspace query doesn't need verification
	if !isQueryStoreWithProof(req.Path) {
		return result.Response, nil
	}

	// TODO: figure out how to verify queries?

	return result.Response, nil
}

// QueryUpgradeProof performs an abci query with the given key and returns the proto encoded merkle proof
// for the query and the height at which the proof will succeed on a tendermint verifier.
func (cp *CosmosProvider) QueryUpgradeProof(key []byte, height uint64) ([]byte, clienttypes.Height, error) {
	res, err := cp.QueryABCI(abci.RequestQuery{
		Path:   "store/upgrade/key",
		Height: int64(height - 1),
		Data:   key,
		Prove:  true,
	})
	if err != nil {
		return nil, clienttypes.Height{}, err
	}

	merkleProof, err := committypes.ConvertProofs(res.ProofOps)
	if err != nil {
		return nil, clienttypes.Height{}, err
	}

	proof, err := cp.Encoding.Marshaler.Marshal(&merkleProof)
	if err != nil {
		return nil, clienttypes.Height{}, err
	}

	revision := clienttypes.ParseChainID(cp.Config.ChainID)

	// proof height + 1 is returned as the proof created corresponds to the height the proof
	// was created in the IAVL tree. Tendermint and subsequently the clients that rely on it
	// have heights 1 above the IAVL tree. Thus we return proof height + 1
	return proof, clienttypes.NewHeight(revision, uint64(res.Height+1)), nil
}

func DefaultPageRequest() *querytypes.PageRequest {
	return &querytypes.PageRequest{
		Key:        []byte(""),
		Offset:     0,
		Limit:      1000,
		CountTotal: true,
	}
}

func rcvPacketQuery(channelID string, seq int) []string {
	return []string{fmt.Sprintf("%s.packet_src_channel='%s'", spTag, channelID),
		fmt.Sprintf("%s.packet_sequence='%d'", spTag, seq)}
}

func ackPacketQuery(channelID string, seq int) []string {
	return []string{fmt.Sprintf("%s.packet_dst_channel='%s'", waTag, channelID),
		fmt.Sprintf("%s.packet_sequence='%d'", waTag, seq)}
}

// relayPacketsFromResultTx looks through the events in a *ctypes.ResultTx
// and returns relayPackets with the appropriate data
func relayPacketsFromResultTx(src, dst provider.ChainProvider, dsth int64, res *ctypes.ResultTx, dstChanId, dstPortId, srcChanId, srcPortId, srcClientId string) ([]provider.RelayPacket, []provider.RelayPacket, error) {
	var (
		rcvPackets     []provider.RelayPacket
		timeoutPackets []provider.RelayPacket
	)

	for _, e := range res.TxResult.Events {
		if e.Type == spTag {
			// NOTE: Src and Dst are switched here
			rp := &relayMsgRecvPacket{pass: false}
			for _, p := range e.Attributes {
				if string(p.Key) == srcChanTag {
					if string(p.Value) != srcChanId {
						rp.pass = true
						continue
					}
				}
				if string(p.Key) == dstChanTag {
					if string(p.Value) != dstChanId {
						rp.pass = true
						continue
					}
				}
				if string(p.Key) == srcPortTag {
					if string(p.Value) != srcPortId {
						rp.pass = true
						continue
					}
				}
				if string(p.Key) == dstPortTag {
					if string(p.Value) != dstPortId {
						rp.pass = true
						continue
					}
				}
				if string(p.Key) == dataTag {
					rp.packetData = p.Value
				}
				if string(p.Key) == toHeightTag {
					timeout, err := clienttypes.ParseHeight(string(p.Value))
					if err != nil {
						return nil, nil, err
					}

					rp.timeout = timeout
				}
				if string(p.Key) == toTSTag {
					timeout, _ := strconv.ParseUint(string(p.Value), 10, 64)
					rp.timeoutStamp = timeout
				}
				if string(p.Key) == seqTag {
					seq, _ := strconv.ParseUint(string(p.Value), 10, 64)
					rp.seq = seq
				}
			}

			// fetch the header which represents a block produced on destination
			block, err := dst.GetIBCUpdateHeader(dsth, src, srcClientId)
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
	}

	// If there is a relayPacket, return it
	if len(rcvPackets)+len(timeoutPackets) > 0 {
		return rcvPackets, timeoutPackets, nil
	}

	return nil, nil, fmt.Errorf("no packet data found")
}

// acknowledgementsFromResultTx looks through the events in a *ctypes.ResultTx and returns
// relayPackets with the appropriate data
func acknowledgementsFromResultTx(dstChanId, dstPortId, srcChanId, srcPortId string, res *ctypes.ResultTx) ([]provider.RelayPacket, error) {
	var ackPackets []provider.RelayPacket
	for _, e := range res.TxResult.Events {
		if e.Type == waTag {
			// NOTE: Src and Dst are switched here
			rp := &relayMsgPacketAck{pass: false}
			for _, p := range e.Attributes {
				if string(p.Key) == srcChanTag {
					if string(p.Value) != srcChanId {
						rp.pass = true
						continue
					}
				}
				if string(p.Key) == dstChanTag {
					if string(p.Value) != dstChanId {
						rp.pass = true
						continue
					}
				}
				if string(p.Key) == srcPortTag {
					if string(p.Value) != srcPortId {
						rp.pass = true
						continue
					}
				}
				if string(p.Key) == dstPortTag {
					if string(p.Value) != dstPortId {
						rp.pass = true
						continue
					}
				}
				if string(p.Key) == ackTag {
					rp.ack = p.Value
				}
				if string(p.Key) == dataTag {
					rp.packetData = p.Value
				}
				if string(p.Key) == toHeightTag {
					timeout, err := clienttypes.ParseHeight(string(p.Value))
					if err != nil {
						return nil, err
					}
					rp.timeout = timeout
				}
				if string(p.Key) == toTSTag {
					timeout, _ := strconv.ParseUint(string(p.Value), 10, 64)
					rp.timeoutStamp = timeout
				}
				if string(p.Key) == seqTag {
					seq, _ := strconv.ParseUint(string(p.Value), 10, 64)
					rp.seq = seq
				}
			}
			if !rp.pass {
				ackPackets = append(ackPackets, rp)
			}
		}
	}

	// If there is a relayPacket, return it
	if len(ackPackets) > 0 {
		return ackPackets, nil
	}

	return nil, fmt.Errorf("no packet data found")
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

// isMatchingClient determines if the two provided clients match in all fields
// except latest height. They are assumed to be IBC tendermint light clients.
// NOTE: we don't pass in a pointer so upstream references don't have a modified
// latest height set to zero.
func isMatchingClient(clientStateA, clientStateB *tmclient.ClientState) bool {
	// zero out latest client height since this is determined and incremented
	// by on-chain updates. Changing the latest height does not fundamentally
	// change the client. The associated consensus state at the latest height
	// determines this last check
	clientStateA.LatestHeight = clienttypes.ZeroHeight()
	clientStateB.LatestHeight = clienttypes.ZeroHeight()

	return reflect.DeepEqual(clientStateA, clientStateB)
}

// isMatchingConsensusState determines if the two provided consensus states are
// identical. They are assumed to be IBC tendermint light clients.
func isMatchingConsensusState(consensusStateA, consensusStateB *tmclient.ConsensusState) bool {
	return reflect.DeepEqual(*consensusStateA, *consensusStateB)
}
