package cosmos

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"golang.org/x/sync/errgroup"
	"os"
	"strings"
	"time"

	sdkTx "github.com/cosmos/cosmos-sdk/client/tx"
	keys "github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/simapp/params"
	sdk "github.com/cosmos/cosmos-sdk/types"
	querytypes "github.com/cosmos/cosmos-sdk/types/query"
	bankTypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	transfertypes "github.com/cosmos/ibc-go/v2/modules/apps/transfer/types"
	clientutils "github.com/cosmos/ibc-go/v2/modules/core/02-client/client/utils"
	clienttypes "github.com/cosmos/ibc-go/v2/modules/core/02-client/types"
	connutils "github.com/cosmos/ibc-go/v2/modules/core/03-connection/client/utils"
	conntypes "github.com/cosmos/ibc-go/v2/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v2/modules/core/04-channel/types"
	commitmenttypes "github.com/cosmos/ibc-go/v2/modules/core/23-commitment/types"
	committypes "github.com/cosmos/ibc-go/v2/modules/core/23-commitment/types"
	ibcexported "github.com/cosmos/ibc-go/v2/modules/core/exported"
	"github.com/cosmos/relayer/relayer/provider"
	"github.com/tendermint/tendermint/libs/log"
	provtypes "github.com/tendermint/tendermint/light/provider"
	prov "github.com/tendermint/tendermint/light/provider/http"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

var (
	_                  provider.QueryProvider  = &CosmosProvider{}
	_                  provider.TxProvider     = &CosmosProvider{}
	_                  provider.RelayerMessage = CosmosMessage{}
	defaultChainPrefix                         = commitmenttypes.NewMerklePrefix([]byte("ibc"))
	defaultDelayPeriod                         = uint64(0)
)

type CosmosProviderConfig struct {
	Key            string  `yaml:"key" json:"key"`
	ChainID        string  `yaml:"chain-id" json:"chain-id"`
	RPCAddr        string  `yaml:"rpc-addr" json:"rpc-addr"`
	AccountPrefix  string  `yaml:"account-prefix" json:"account-prefix"`
	GasAdjustment  float64 `yaml:"gas-adjustment" json:"gas-adjustment"`
	GasPrices      string  `yaml:"gas-prices" json:"gas-prices"`
	TrustingPeriod string  `yaml:"trusting-period" json:"trusting-period"`
	Timeout        string  `yaml:"timeout" json:"timeout"`
}

type CosmosMessage struct {
	Msg sdk.Msg
}

func NewCosmosMessage(msg sdk.Msg) provider.RelayerMessage {
	return CosmosMessage{
		Msg: msg,
	}
}

func CosmosMsg(rm provider.RelayerMessage) sdk.Msg {
	if val, ok := rm.(CosmosMessage); !ok {
		// add warning output later to tell invalid msg type
		return nil
	} else {
		return val.Msg
	}
}

func CosmosMsgs(rm ...provider.RelayerMessage) []sdk.Msg {
	sdkMsgs := make([]sdk.Msg, 0)
	for _, rMsg := range rm {
		if val, ok := rMsg.(CosmosMessage); !ok {
			// add warning output later to tell invalid msg type
		} else {
			sdkMsgs = append(sdkMsgs, val.Msg)
		}
	}
	return sdkMsgs
}

func (cp CosmosProvider) Validate() error {
	// TODO: validate all config fields, optionally add unexported config fields to hold parsed results
	return nil
}

func NewCosmosProvider(config *CosmosProviderConfig, homePath string, debug bool) (*CosmosProvider, error) {
	cp := &CosmosProvider{Config: config, HomePath: homePath, debug: debug}
	if err := cp.Init(); err != nil {
		return nil, err
	}
	return cp, nil
}

type CosmosProvider struct {
	Config   *CosmosProviderConfig
	HomePath string

	Keybase  keys.Keyring
	Client   rpcclient.Client
	Encoding params.EncodingConfig
	Provider provtypes.Provider

	address sdk.AccAddress
	logger  log.Logger
	debug   bool
}

func (cp *CosmosProvider) Init() error {
	keybase, err := keys.New(cp.Config.ChainID, "test", KeysDir(cp.HomePath, cp.Config.ChainID), nil)
	if err != nil {
		return err
	}

	timeout, err := time.ParseDuration(cp.Config.Timeout)
	if err != nil {
		return fmt.Errorf("failed to parse timeout (%s) for chain %s", cp.Config.Timeout, cp.Config.ChainID)
	}

	client, err := newRPCClient(cp.Config.RPCAddr, timeout)
	if err != nil {
		return err
	}

	liteprovider, err := prov.New(cp.Config.ChainID, cp.Config.RPCAddr)
	if err != nil {
		return err
	}

	_, err = time.ParseDuration(cp.Config.TrustingPeriod)
	if err != nil {
		return fmt.Errorf("failed to parse trusting period (%s) for chain %s", cp.Config.TrustingPeriod, cp.Config.ChainID)
	}

	_, err = sdk.ParseDecCoins(cp.Config.GasPrices)
	if err != nil {
		return fmt.Errorf("failed to parse gas prices (%s) for chain %s", cp.Config.GasPrices, cp.Config.ChainID)
	}

	encodingConfig := cp.MakeEncodingConfig()

	cp.Keybase = keybase
	cp.Client = client
	cp.Encoding = encodingConfig
	cp.logger = log.NewTMLogger(log.NewSyncWriter(os.Stdout)) // switch to json logging? add option for json logging?
	cp.Provider = liteprovider
	return nil
}

func (cp *CosmosProvider) CreateClient(clientState ibcexported.ClientState, dstHeader ibcexported.Header) (provider.RelayerMessage, error) {
	if err := dstHeader.ValidateBasic(); err != nil {
		return nil, err
	}

	cs, _, err := cp.QueryConsensusState(int64(dstHeader.GetHeight().GetRevisionHeight()))
	if err != nil {
		return nil, err
	}

	msg, err := clienttypes.NewMsgCreateClient(
		clientState,
		cs,
		cp.MustGetAddress(), // 'MustGetAddress' must be called directly before calling 'NewMsg...'
	)
	if err != nil {
		return nil, err
	}

	if err = msg.ValidateBasic(); err != nil {
		return nil, err
	}
	return NewCosmosMessage(msg), nil
}

func (cp *CosmosProvider) SubmitMisbehavior( /*TBD*/ ) (provider.RelayerMessage, error) {
	return nil, nil
}

func (cp *CosmosProvider) UpdateClient(srcClientId string, dstHeader ibcexported.Header) (provider.RelayerMessage, error) {
	if err := dstHeader.ValidateBasic(); err != nil {
		return nil, err
	}
	msg, err := clienttypes.NewMsgUpdateClient(
		srcClientId,
		dstHeader,
		cp.MustGetAddress(), // 'MustGetAddress' must be called directly before calling 'NewMsg...'
	)
	if err != nil {
		return nil, err
	}
	return NewCosmosMessage(msg), nil
}

func (cp *CosmosProvider) ConnectionOpenInit(srcClientId, dstClientId string, dstHeader ibcexported.Header) ([]provider.RelayerMessage, error) {
	updateMsg, err := cp.UpdateClient(srcClientId, dstHeader)
	if err != nil {
		return nil, err
	}

	var version *conntypes.Version
	msg := conntypes.NewMsgConnectionOpenInit(
		srcClientId,
		dstClientId,
		defaultChainPrefix,
		version,
		defaultDelayPeriod,
		cp.MustGetAddress(), // 'MustGetAddress' must be called directly before calling 'NewMsg...'
	)

	return []provider.RelayerMessage{updateMsg, NewCosmosMessage(msg)}, nil
}

func (cp *CosmosProvider) ConnectionOpenTry(dstQueryProvider provider.QueryProvider, dstHeader ibcexported.Header, srcClientId, dstClientId, srcConnId, dstConnId string) ([]provider.RelayerMessage, error) {
	updateMsg, err := cp.UpdateClient(srcClientId, dstHeader)
	if err != nil {
		return nil, err
	}

	cph, err := dstQueryProvider.QueryLatestHeight()
	if err != nil {
		return nil, err
	}

	clientState, clientStateProof, consensusStateProof, connStateProof,
		proofHeight, err := dstQueryProvider.GenerateConnHandshakeProof(cph, dstClientId, dstConnId)
	if err != nil {
		return nil, err
	}

	// TODO: Get DelayPeriod from counterparty connection rather than using default value
	msg := conntypes.NewMsgConnectionOpenTry(
		srcConnId,
		srcClientId,
		dstConnId,
		dstClientId,
		clientState,
		defaultChainPrefix,
		conntypes.ExportedVersionsToProto(conntypes.GetCompatibleVersions()),
		defaultDelayPeriod,
		connStateProof,
		clientStateProof,
		consensusStateProof,
		clienttypes.NewHeight(proofHeight.GetRevisionNumber(), proofHeight.GetRevisionHeight()),
		clientState.GetLatestHeight().(clienttypes.Height),
		cp.MustGetAddress(), // 'MustGetAddress' must be called directly before calling 'NewMsg...'
	)
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	return []provider.RelayerMessage{updateMsg, NewCosmosMessage(msg)}, nil
}

func (cp *CosmosProvider) ConnectionOpenAck(dstQueryProvider provider.QueryProvider, dstHeader ibcexported.Header, srcClientId, srcConnId, dstClientId, dstConnId string) ([]provider.RelayerMessage, error) {
	updateMsg, err := cp.UpdateClient(srcClientId, dstHeader)
	if err != nil {
		return nil, err
	}
	cph, err := dstQueryProvider.QueryLatestHeight()
	if err != nil {
		return nil, err
	}

	clientState, clientStateProof, consensusStateProof, connStateProof,
		proofHeight, err := dstQueryProvider.GenerateConnHandshakeProof(cph, dstClientId, dstConnId)
	if err != nil {
		return nil, err
	}

	msg := conntypes.NewMsgConnectionOpenAck(
		srcConnId,
		dstConnId,
		clientState,
		connStateProof,
		clientStateProof,
		consensusStateProof,
		clienttypes.NewHeight(proofHeight.GetRevisionNumber(), proofHeight.GetRevisionHeight()),
		clientState.GetLatestHeight().(clienttypes.Height),
		conntypes.DefaultIBCVersion,
		cp.MustGetAddress(), // 'MustGetAddress' must be called directly before calling 'NewMsg...'
	)

	return []provider.RelayerMessage{updateMsg, NewCosmosMessage(msg)}, nil
}

func (cp *CosmosProvider) ConnectionOpenConfirm(dstQueryProvider provider.QueryProvider, dstHeader ibcexported.Header, dstConnId, srcClientId, srcConnId string) ([]provider.RelayerMessage, error) {
	updateMsg, err := cp.UpdateClient(srcClientId, dstHeader)
	if err != nil {
		return nil, err
	}

	cph, err := dstQueryProvider.QueryLatestHeight()
	if err != nil {
		return nil, err
	}
	counterpartyConnState, err := dstQueryProvider.QueryConnection(cph, dstConnId)
	if err != nil {
		return nil, err
	}

	msg := conntypes.NewMsgConnectionOpenConfirm(
		srcConnId,
		counterpartyConnState.Proof,
		counterpartyConnState.ProofHeight,
		cp.MustGetAddress(), // 'MustGetAddress' must be called directly before calling 'NewMsg...'
	)

	return []provider.RelayerMessage{updateMsg, NewCosmosMessage(msg)}, nil
}

func (cp *CosmosProvider) ChannelOpenInit(srcClientId, srcConnId, srcPortId, srcVersion, dstPortId string, order chantypes.Order, dstHeader ibcexported.Header) ([]provider.RelayerMessage, error) {
	updateMsg, err := cp.UpdateClient(srcClientId, dstHeader)
	if err != nil {
		return nil, err
	}

	msg := chantypes.NewMsgChannelOpenInit(
		srcPortId,
		srcVersion,
		order,
		[]string{srcConnId},
		dstPortId,
		cp.MustGetAddress(), // 'MustGetAddress' must be called directly before calling 'NewMsg...'
	)

	return []provider.RelayerMessage{updateMsg, NewCosmosMessage(msg)}, nil
}

func (cp *CosmosProvider) ChannelOpenTry(dstQueryProvider provider.QueryProvider, dstHeader ibcexported.Header, srcPortId, dstPortId, srcChanId, dstChanId, srcVersion, srcConnectionId, srcClientId string) ([]provider.RelayerMessage, error) {
	updateMsg, err := cp.UpdateClient(srcClientId, dstHeader)
	if err != nil {
		return nil, err
	}
	cph, err := dstQueryProvider.QueryLatestHeight()
	if err != nil {
		return nil, err
	}

	counterpartyChannelRes, err := dstQueryProvider.QueryChannel(cph, dstChanId, dstPortId)
	if err != nil {
		return nil, err
	}

	msg := chantypes.NewMsgChannelOpenTry(
		srcPortId,
		srcChanId,
		srcVersion,
		counterpartyChannelRes.Channel.Ordering,
		[]string{srcConnectionId},
		dstPortId,
		dstChanId,
		counterpartyChannelRes.Channel.Version,
		counterpartyChannelRes.Proof,
		counterpartyChannelRes.ProofHeight,
		cp.MustGetAddress(), // 'MustGetAddress' must be called directly before calling 'NewMsg...'
	)

	return []provider.RelayerMessage{updateMsg, NewCosmosMessage(msg)}, nil
}

func (cp *CosmosProvider) ChannelOpenAck(dstQueryProvider provider.QueryProvider, dstHeader ibcexported.Header, srcClientId, srcPortId, srcChanId, dstChanId, dstPortId string) ([]provider.RelayerMessage, error) {
	updateMsg, err := cp.UpdateClient(srcClientId, dstHeader)
	if err != nil {
		return nil, err
	}

	cph, err := dstQueryProvider.QueryLatestHeight()
	if err != nil {
		return nil, err
	}

	counterpartyChannelRes, err := dstQueryProvider.QueryChannel(cph, dstChanId, dstPortId)
	if err != nil {
		return nil, err
	}

	msg := chantypes.NewMsgChannelOpenAck(
		srcPortId,
		srcChanId,
		dstChanId,
		counterpartyChannelRes.Channel.Version,
		counterpartyChannelRes.Proof,
		counterpartyChannelRes.ProofHeight,
		cp.MustGetAddress(), // 'MustGetAddress' must be called directly before calling 'NewMsg...'
	)

	return []provider.RelayerMessage{updateMsg, NewCosmosMessage(msg)}, nil
}

func (cp *CosmosProvider) ChannelOpenConfirm(dstQueryProvider provider.QueryProvider, dstHeader ibcexported.Header, srcClientId, srcPortId, srcChanId, dstPortId, dstChanId string) ([]provider.RelayerMessage, error) {
	updateMsg, err := cp.UpdateClient(srcClientId, dstHeader)
	if err != nil {
		return nil, err
	}
	cph, err := dstQueryProvider.QueryLatestHeight()
	if err != nil {
		return nil, err
	}

	counterpartyChanState, err := dstQueryProvider.QueryChannel(cph, dstChanId, dstPortId)
	if err != nil {
		return nil, err
	}

	msg := chantypes.NewMsgChannelOpenConfirm(
		srcPortId,
		srcChanId,
		counterpartyChanState.Proof,
		counterpartyChanState.ProofHeight,
		cp.MustGetAddress(), // 'MustGetAddress' must be called directly before calling 'NewMsg...'
	)

	return []provider.RelayerMessage{updateMsg, NewCosmosMessage(msg)}, nil
}

func (cp *CosmosProvider) ChannelCloseInit(srcPortId, srcChanId string) provider.RelayerMessage {
	return NewCosmosMessage(chantypes.NewMsgChannelCloseInit(
		srcPortId,
		srcChanId,
		cp.MustGetAddress(), // 'MustGetAddress' must be called directly before calling 'NewMsg...'
	))
}

func (cp *CosmosProvider) ChannelCloseConfirm(dstQueryProvider provider.QueryProvider, dsth int64, dstChanId, dstPortId, srcPortId, srcChanId string) (provider.RelayerMessage, error) {
	dstChanResp, err := dstQueryProvider.QueryChannel(dsth, dstChanId, dstPortId)
	if err != nil {
		return nil, err
	}

	return NewCosmosMessage(chantypes.NewMsgChannelCloseConfirm(
		srcPortId,
		srcChanId,
		dstChanResp.Proof,
		dstChanResp.ProofHeight,
		cp.MustGetAddress(), // 'MustGetAddress' must be called directly before calling 'NewMsg...'
	)), nil
}

func (cp *CosmosProvider) SendMessage(msg provider.RelayerMessage) (*provider.RelayerTxResponse, bool, error) {
	return cp.SendMessages([]provider.RelayerMessage{msg})
}

func (cp *CosmosProvider) SendMessages(msgs []provider.RelayerMessage) (*provider.RelayerTxResponse, bool, error) {
	// Instantiate the client context
	ctx := cp.CLIContext(0)

	// Query account details
	txFactory, txConfig := cp.TxFactory(0)
	txf, err := prepareFactory(ctx, txFactory)
	if err != nil {
		return nil, false, err
	}

	// TODO: Make this work with new CalculateGas method
	// https://github.com/cosmos/cosmos-sdk/blob/5725659684fc93790a63981c653feee33ecf3225/client/tx/tx.go#L297
	// If users pass gas adjustment, then calculate gas
	_, adjusted, err := CalculateGas(ctx.QueryWithData, txf, txConfig, msgs...)
	if err != nil {
		return nil, false, err
	}

	// Set the gas amount on the transaction factory
	txf = txf.WithGas(adjusted)

	// Build the transaction builder
	txb, err := BuildUnsignedTx(txf, txConfig, msgs...)
	if err != nil {
		return nil, false, err
	}

	// Attach the signature to the transaction
	// c.LogFailedTx(nil, err, msgs)
	// Force encoding in the chain specific address
	for _, msg := range msgs {
		cp.Encoding.Marshaler.MustMarshalJSON(CosmosMsg(msg))
	}
	err = sdkTx.Sign(txf, cp.Config.Key, txb, false)
	if err != nil {
		return nil, false, err
	}

	// Generate the transaction bytes
	txBytes, err := ctx.TxConfig.TxEncoder()(txb.GetTx())
	if err != nil {
		return nil, false, err
	}

	// Broadcast those bytes
	res, err := ctx.BroadcastTx(txBytes)
	if err != nil {
		return nil, false, err
	}

	// TODO helper/wrapper function for TxResponse->RelayerTxResponse
	rlyRes := &provider.RelayerTxResponse{
		Code:  int(res.Code),
		Error: "", // do we need errors in RlyTxRes?
	}

	// transaction was executed, log the success or failure using the tx response code
	// NOTE: error is nil, logic should use the returned error to determine if the
	// transaction was successfully executed.
	if rlyRes.Code != 0 {
		cp.LogFailedTx(res, err, CosmosMsgs(msgs))
		return rlyRes, false, nil
	}

	cp.LogSuccessTx(res, CosmosMsgs(msgs))
	return rlyRes, true, nil
}

// QueryTx takes a transaction hash and returns the transaction
func (cp *CosmosProvider) QueryTx(hashHex string) (*ctypes.ResultTx, error) {
	hash, err := hex.DecodeString(hashHex)
	if err != nil {
		return &ctypes.ResultTx{}, err
	}

	return cp.Client.Tx(context.Background(), hash, true)
}

// QueryTxs returns an array of transactions given a tag
func (cp *CosmosProvider) QueryTxs(page, limit int, events []string) ([]*ctypes.ResultTx, error) {
	if len(events) == 0 {
		return nil, errors.New("must declare at least one event to search")
	}

	if page <= 0 {
		return nil, errors.New("page must greater than 0")
	}

	if limit <= 0 {
		return nil, errors.New("limit must greater than 0")
	}

	res, err := cp.Client.TxSearch(context.Background(), strings.Join(events, " AND "), true, &page, &limit, "")
	if err != nil {
		return nil, err
	}
	return res.Txs, nil
}

// QueryLatestHeight queries the chain for the latest height and returns it
func (cp *CosmosProvider) QueryLatestHeight() (int64, error) {
	res, err := cp.Client.Status(context.Background())
	if err != nil {
		return -1, err
	} else if res.SyncInfo.CatchingUp {
		return -1, fmt.Errorf("node at %s running chain %s not caught up", cp.Config.RPCAddr, cp.Config.ChainID)
	}

	return res.SyncInfo.LatestBlockHeight, nil
}

// QueryBalance returns the amount of coins in the relayer account
func (cp *CosmosProvider) QueryBalance(keyName string) (sdk.Coins, error) {
	var addr string
	if keyName == "" {
		addr = cp.MustGetAddress()
	} else {
		info, err := cp.Keybase.Key(keyName)
		if err != nil {
			return nil, err
		}
		done := cp.UseSDKContext()
		addr = info.GetAddress().String()
		done()
	}
	return cp.QueryBalanceWithAddress(addr)
}

// QueryBalanceWithAddress returns the amount of coins in the relayer account with address as input
func (cp *CosmosProvider) QueryBalanceWithAddress(address string) (sdk.Coins, error) {
	done := cp.UseSDKContext()
	addr, err := sdk.AccAddressFromBech32(address)
	done()
	if err != nil {
		return nil, err
	}

	p := bankTypes.NewQueryAllBalancesRequest(addr, &querytypes.PageRequest{
		Key:        []byte(""),
		Offset:     0,
		Limit:      1000,
		CountTotal: true,
	})

	queryClient := bankTypes.NewQueryClient(cp.CLIContext(0))

	res, err := queryClient.AllBalances(context.Background(), p)
	if err != nil {
		return nil, err
	}

	return res.Balances, nil
}

// QueryUnbondingPeriod returns the unbonding period of the chain
func (cp *CosmosProvider) QueryUnbondingPeriod() (time.Duration, error) {
	req := stakingtypes.QueryParamsRequest{}

	queryClient := stakingtypes.NewQueryClient(cp.CLIContext(0))

	res, err := queryClient.Params(context.Background(), &req)
	if err != nil {
		return 0, err
	}

	return res.Params.UnbondingTime, nil
}

// QueryClientStateResponse retrevies the latest consensus state for a client in state at a given height
func (cp *CosmosProvider) QueryClientStateResponse(height int64, srcClientId string) (*clienttypes.QueryClientStateResponse, error) {
	return clientutils.QueryClientStateABCI(cp.CLIContext(height), srcClientId)
}

// QueryClientState retrevies the latest consensus state for a client in state at a given height
// and unpacks it to exported client state interface
func (cp *CosmosProvider) QueryClientState(height int64, clientid string) (ibcexported.ClientState, error) {
	clientStateRes, err := cp.QueryClientStateResponse(height, clientid)
	if err != nil {
		return nil, err
	}

	clientStateExported, err := clienttypes.UnpackClientState(clientStateRes.ClientState)
	if err != nil {
		return nil, err
	}

	return clientStateExported, nil
}

// QueryClientConsensusState retrieves the latest consensus state for a client in state at a given height
func (cp *CosmosProvider) QueryClientConsensusState(chainHeight int64, clientid string, clientHeight ibcexported.Height) (*clienttypes.QueryConsensusStateResponse, error) {
	return clientutils.QueryConsensusStateABCI(
		cp.CLIContext(chainHeight),
		clientid,
		clientHeight,
	)
}

// QueryUpgradedClient returns upgraded client info
func (cp *CosmosProvider) QueryUpgradedClient(height int64) (*clienttypes.QueryClientStateResponse, error) {
	return nil, nil
}

// QueryUpgradedConsState returns upgraded consensus state and height of client
func (cp *CosmosProvider) QueryUpgradedConsState(height int64) (*clienttypes.QueryConsensusStateResponse, error) {
	return nil, nil
}

// QueryConsensusState returns a consensus state for a given chain to be used as a
// client in another chain, fetches latest height when passed 0 as arg
func (cp *CosmosProvider) QueryConsensusState(height int64) (ibcexported.ConsensusState, int64, error) {
	return clientutils.QuerySelfConsensusState(cp.CLIContext(height))
}

// QueryClients queries all the clients!
func (cp *CosmosProvider) QueryClients() ([]*clienttypes.IdentifiedClientState, error) {
	return nil, nil
}

// QueryConnection returns the remote end of a given connection
func (cp *CosmosProvider) QueryConnection(height int64, connectionid string) (*conntypes.QueryConnectionResponse, error) {
	res, err := connutils.QueryConnection(cp.CLIContext(height), connectionid, true)
	if err != nil && strings.Contains(err.Error(), "not found") {
		return conntypes.NewQueryConnectionResponse(
			conntypes.NewConnectionEnd(
				conntypes.UNINITIALIZED,
				"client",
				conntypes.NewCounterparty(
					"client",
					"connection",
					committypes.NewMerklePrefix([]byte{}),
				),
				[]*conntypes.Version{},
				0,
			), []byte{}, clienttypes.NewHeight(0, 0)), nil
	} else if err != nil {
		return nil, err
	}
	return res, nil
}

// QueryConnections gets any connections on a chain
// TODO add pagination support
func (cp *CosmosProvider) QueryConnections() (conns []*conntypes.IdentifiedConnection, err error) {
	qc := conntypes.NewQueryClient(cp.CLIContext(0))
	res, err := qc.Connections(context.Background(), &conntypes.QueryConnectionsRequest{
		Pagination: DefaultPageRequest(),
	})
	return res.Connections, err
}

// QueryConnectionsUsingClient gets any connections that exist between chain and counterparty
// TODO add pagination support
func (cp *CosmosProvider) QueryConnectionsUsingClient(height int64, clientid string) (*conntypes.QueryConnectionsResponse, error) {
	qc := conntypes.NewQueryClient(cp.CLIContext(0))
	res, err := qc.Connections(context.Background(), &conntypes.QueryConnectionsRequest{
		Pagination: DefaultPageRequest(),
	})
	return res, err
}

// GenerateConnHandshakeProof generates all the proofs needed to prove the existence of the
// connection state on this chain. A counterparty should use these generated proofs.
func (cp *CosmosProvider) GenerateConnHandshakeProof(height int64, clientId, connId string) (clientState ibcexported.ClientState, clientStateProof []byte, consensusProof []byte, connectionProof []byte, connectionProofHeight ibcexported.Height, err error) {
	var (
		clientStateRes     *clienttypes.QueryClientStateResponse
		consensusStateRes  *clienttypes.QueryConsensusStateResponse
		connectionStateRes *conntypes.QueryConnectionResponse
		eg                 = new(errgroup.Group)
	)

	// query for the client state for the proof and get the height to query the consensus state at.
	clientStateRes, err = cp.QueryClientStateResponse(height, clientId)
	if err != nil {
		return nil, nil, nil, nil, clienttypes.Height{}, err
	}

	clientState, err = clienttypes.UnpackClientState(clientStateRes.ClientState)
	if err != nil {
		return nil, nil, nil, nil, clienttypes.Height{}, err
	}

	eg.Go(func() error {
		var err error
		consensusStateRes, err = cp.QueryClientConsensusState(height, clientId, clientState.GetLatestHeight())
		return err
	})
	eg.Go(func() error {
		var err error
		connectionStateRes, err = cp.QueryConnection(height, connId)
		return err
	})

	if err := eg.Wait(); err != nil {
		return nil, nil, nil, nil, clienttypes.Height{}, err
	}

	return clientState, clientStateRes.Proof, consensusStateRes.Proof, connectionStateRes.Proof, connectionStateRes.ProofHeight, nil
}

func (cp *CosmosProvider) QueryChannel(height int64, channelid, portid string) (chanRes *chantypes.QueryChannelResponse, err error) {
	return nil, nil
}

func (cp *CosmosProvider) QueryChannelClient(height int64, channelid, portid string) (*clienttypes.IdentifiedClientState, error) {
	return nil, nil
}

func (cp *CosmosProvider) QueryConnectionChannels(height int64, connectionid string) ([]*chantypes.IdentifiedChannel, error) {
	return nil, nil
}

func (cp *CosmosProvider) QueryChannels() ([]*chantypes.IdentifiedChannel, error) { return nil, nil }

func (cp *CosmosProvider) QueryPacketCommitments(height uint64, channelid, portid string) (commitments []*chantypes.PacketState, err error) {
	return nil, nil
}

func (cp *CosmosProvider) QueryPacketAcknowledgements(height uint64, channelid, portid string) (acknowledgements []*chantypes.PacketState, err error) {
	return nil, nil
}

func (cp *CosmosProvider) QueryUnreceivedPackets(height uint64, channelid, portid string, seqs []uint64) ([]uint64, error) {
	return nil, nil
}

func (cp *CosmosProvider) QueryUnreceivedAcknowledgements(height uint64, channelid, portid string, seqs []uint64) ([]uint64, error) {
	return nil, nil
}

func (cp *CosmosProvider) QueryNextSeqRecv(height int64, channelid, portid string) (recvRes *chantypes.QueryNextSequenceReceiveResponse, err error) {
	return nil, nil
}

func (cp *CosmosProvider) QueryPacketCommitment(height int64, channelid, portid string, seq uint64) (comRes *chantypes.QueryPacketCommitmentResponse, err error) {
	return nil, nil
}

func (cp *CosmosProvider) QueryPacketAcknowledgement(height int64, channelid, portid string, seq uint64) (ackRes *chantypes.QueryPacketAcknowledgementResponse, err error) {
	return nil, nil
}

func (cp *CosmosProvider) QueryPacketReceipt(height int64, channelid, portid string, seq uint64) (recRes *chantypes.QueryPacketReceiptResponse, err error) {
	return nil, nil
}

func (cp *CosmosProvider) QueryDenomTrace(denom string) (*transfertypes.DenomTrace, error) {
	return nil, nil
}

func (cp *CosmosProvider) QueryDenomTraces(offset, limit uint64, height int64) ([]*transfertypes.DenomTrace, error) {
	return nil, nil
}
