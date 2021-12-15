package cosmos

import (
	"context"
	"errors"
	"fmt"
	"github.com/tendermint/tendermint/light"
	"net/url"
	"os"
	"time"

	"github.com/avast/retry-go"
	sdkTx "github.com/cosmos/cosmos-sdk/client/tx"
	keys "github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/simapp/params"
	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v2/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v2/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v2/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v2/modules/core/04-channel/types"
	commitmenttypes "github.com/cosmos/ibc-go/v2/modules/core/23-commitment/types"
	ibcexported "github.com/cosmos/ibc-go/v2/modules/core/exported"
	tmclient "github.com/cosmos/ibc-go/v2/modules/light-clients/07-tendermint/types"
	"github.com/cosmos/relayer/relayer/provider"
	"github.com/tendermint/tendermint/libs/log"
	provtypes "github.com/tendermint/tendermint/light/provider"
	prov "github.com/tendermint/tendermint/light/provider/http"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	tmtypes "github.com/tendermint/tendermint/types"
)

var (
	_                  provider.QueryProvider  = &CosmosProvider{}
	_                  provider.ChainProvider  = &CosmosProvider{}
	_                  provider.KeyProvider    = &CosmosProvider{}
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

func (cp *CosmosProviderConfig) NewProvider(homepath string, debug bool) (provider.ChainProvider, error) {
	err := cp.Validate()
	if err != nil {
		return nil, err
	}
	p, err := NewCosmosProvider(cp, homepath, debug)
	if err != nil {
		return nil, err
	}
	return p, err
}

func (cp *CosmosProviderConfig) Validate() error {
	if cp.Key == "" {
		return fmt.Errorf("must set default key in the comsos provider config")
	}
	if cp.ChainID == "" {
		return fmt.Errorf("must set chain id in the cosmos provider config")
	}
	if cp.AccountPrefix == "" {
		return fmt.Errorf("must set account prefix in the cosmos provider config")
	}
	if _, err := sdk.ParseDecCoins(cp.GasPrices); err != nil {
		return err
	}
	if _, err := time.ParseDuration(cp.TrustingPeriod); err != nil {
		return err
	}
	if _, err := time.ParseDuration(cp.Timeout); err != nil {
		return err
	}
	if _, err := url.ParseRequestURI(cp.RPCAddr); err != nil {
		return err
	}
	return nil
}

type CosmosMessage struct {
	Msg sdk.Msg
}

func NewCosmosMessage(msg sdk.Msg) provider.RelayerMessage {
	return CosmosMessage{
		Msg: msg,
	}
}

func (cm CosmosMessage) Type() string {
	return sdk.MsgTypeURL(cm.Msg)
}

func SdkMsgFromRelayerMessage(p provider.RelayerMessage) sdk.Msg {
	msg, _ := p.(CosmosMessage)
	return msg.Msg
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

func (cp *CosmosProvider) ProviderConfig() provider.ProviderConfig {
	return cp.Config
}

func (cp *CosmosProvider) ChainId() string {
	return cp.Config.ChainID
}

func (cp *CosmosProvider) Type() string {
	return "cosmos"
}

func (cp *CosmosProvider) Key() string {
	return cp.Config.Key
}

// Address returns the address of the configured ChainProvider as a string
func (cp *CosmosProvider) Address() string {
	return cp.MustGetAddress()
}

func (cp *CosmosProvider) Init() error {
	if err := cp.CreateKeystore(cp.HomePath); err != nil {
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

	cp.Client = client
	cp.Encoding = encodingConfig
	cp.logger = log.NewTMLogger(log.NewSyncWriter(os.Stdout)) // switch to json logging? add option for json logging?
	cp.Provider = liteprovider
	return nil
}

// NOTE: we explicitly call 'MustGetAddress' before 'NewMsg...'
// to ensure the correct config file is being used to generate
// the account prefix. 'NewMsg...' functions take an AccAddress
// rather than a string. The 'address.String()' function uses
// the currently set config file. Querying a counterparty would
// swap the config file. 'MustGetAddress' sets the config file
// correctly. Do not change this ordering until the SDK config
// file handling has been refactored.
// https://github.com/cosmos/cosmos-sdk/issues/8332

// CreateClient creates an sdk.Msg to update the client on src with consensus state from dst
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

	return NewCosmosMessage(msg), msg.ValidateBasic()
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
	return NewCosmosMessage(msg), msg.ValidateBasic()
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

	return []provider.RelayerMessage{updateMsg, NewCosmosMessage(msg)}, msg.ValidateBasic()
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

	return []provider.RelayerMessage{updateMsg, NewCosmosMessage(msg)}, msg.ValidateBasic()
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

	return []provider.RelayerMessage{updateMsg, NewCosmosMessage(msg)}, msg.ValidateBasic()
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

	return []provider.RelayerMessage{updateMsg, NewCosmosMessage(msg)}, msg.ValidateBasic()
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

	return []provider.RelayerMessage{updateMsg, NewCosmosMessage(msg)}, msg.ValidateBasic()
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

	return []provider.RelayerMessage{updateMsg, NewCosmosMessage(msg)}, msg.ValidateBasic()
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

	return []provider.RelayerMessage{updateMsg, NewCosmosMessage(msg)}, msg.ValidateBasic()
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

	return []provider.RelayerMessage{updateMsg, NewCosmosMessage(msg)}, msg.ValidateBasic()
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

	// helper/wrapper function for TxResponse->RelayerTxResponse?
	rlyRes := &provider.RelayerTxResponse{
		Height: res.Height,
		TxHash: res.TxHash,
		Code:   res.Code,
		Data:   res.Data,
	}

	// transaction was executed, log the success or failure using the tx response code
	// NOTE: error is nil, logic should use the returned error to determine if the
	// transaction was successfully executed.
	if rlyRes.Code != 0 {
		cp.LogFailedTx(res, err, CosmosMsgs(msgs...))
		return rlyRes, false, nil
	}

	cp.LogSuccessTx(res, CosmosMsgs(msgs...))
	return rlyRes, true, nil
}

func (cp *CosmosProvider) GetLightSignedHeaderAtHeight(h int64) (ibcexported.Header, error) {
	lightBlock, err := cp.Provider.LightBlock(context.Background(), h)
	if err != nil {
		return nil, err
	}
	protoVal, err := tmtypes.NewValidatorSet(lightBlock.ValidatorSet.Validators).ToProto()
	if err != nil {
		return nil, err
	}

	return &tmclient.Header{
		SignedHeader: lightBlock.SignedHeader.ToProto(),
		ValidatorSet: protoVal,
	}, nil
}

// GetIBCUpdateHeader updates the off chain tendermint light client and
// returns an IBC Update Header which can be used to update an on chain
// light client on the destination chain. The source is used to construct
// the header data.
func (cp *CosmosProvider) GetIBCUpdateHeader(srch int64, dst provider.ChainProvider, dstClientId string) (ibcexported.Header, error) {
	// Construct header data from light client representing source.
	h, err := cp.GetLightSignedHeaderAtHeight(srch)
	if err != nil {
		return nil, err
	}

	// Inject trusted fields based on previous header data from source
	return cp.InjectTrustedFields(h, dst, dstClientId)
}

// InjectTrustedFields injects the necessary trusted fields for a header to update a light
// client stored on the destination chain, using the information provided by the source
// chain.
// TrustedHeight is the latest height of the IBC client on dst
// TrustedValidators is the validator set of srcChain at the TrustedHeight
// InjectTrustedFields returns a copy of the header with TrustedFields modified
func (cp *CosmosProvider) InjectTrustedFields(header ibcexported.Header, dst provider.ChainProvider, dstClientId string) (ibcexported.Header, error) {
	// make copy of header stored in mop
	h, ok := header.(*tmclient.Header)
	if !ok {
		return nil, fmt.Errorf("trying to inject fields into non-tendermint headers")
	}

	// retrieve dst client from src chain
	// this is the client that will updated
	cs, err := dst.QueryClientState(0, dstClientId)
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
	var trustedHeader *tmclient.Header
	if err := retry.Do(func() error {
		tmpHeader, err := cp.GetLightSignedHeaderAtHeight(int64(h.TrustedHeight.RevisionHeight) + 1)
		th, ok := tmpHeader.(*tmclient.Header)
		if !ok {
			err = errors.New("non-tm client header")
		}
		trustedHeader = th
		return err
	}, provider.RtyAtt, provider.RtyDel, provider.RtyErr); err != nil {
		return nil, fmt.Errorf(
			"failed to get trusted header, please ensure header at the height %d has not been pruned by the connected node: %w",
			h.TrustedHeight.RevisionHeight, err,
		)
	}

	// inject TrustedValidators into header
	h.TrustedValidators = trustedHeader.ValidatorSet
	return h, nil
}

// MsgRelayAcknowledgement constructs the MsgAcknowledgement which is to be sent to the sending chain.
// The counterparty represents the receiving chain where the acknowledgement would be stored.
func (cp *CosmosProvider) MsgRelayAcknowledgement(dst provider.ChainProvider, dstChanId, dstPortId, srcChanId, srcPortId string, dsth int64, packet provider.RelayPacket) (provider.RelayerMessage, error) {
	msgPacketAck, ok := packet.(*relayMsgPacketAck)
	if !ok {
		return nil, fmt.Errorf("got data of type %T but wanted relayMsgPacketAck \n", packet)
	}
	ackRes, err := dst.QueryPacketAcknowledgement(dsth, dstChanId, dstPortId, packet.Seq())
	switch {
	case err != nil:
		return nil, err
	case ackRes.Proof == nil || ackRes.Acknowledgement == nil:
		return nil, fmt.Errorf("ack packet acknowledgement query seq(%d) is nil", packet.Seq())
	case ackRes == nil:
		return nil, fmt.Errorf("ack packet [%s]seq{%d} has no associated proofs", dst.ChainId(), packet.Seq())
	default:
		return NewCosmosMessage(chantypes.NewMsgAcknowledgement(
			chantypes.NewPacket(
				packet.Data(),
				packet.Seq(),
				srcPortId,
				srcChanId,
				dstPortId,
				dstChanId,
				packet.Timeout(),
				packet.TimeoutStamp(),
			),
			msgPacketAck.ack,
			ackRes.Proof,
			ackRes.ProofHeight,
			cp.address.String())), nil
	}
}

// MsgTransfer creates a new transfer message
func (cp *CosmosProvider) MsgTransfer(amount sdk.Coin, dstChainId, dstAddr, srcPortId, srcChanId string, timeoutHeight, timeoutTimestamp uint64) provider.RelayerMessage {
	version := clienttypes.ParseChainID(dstChainId)
	return NewCosmosMessage(transfertypes.NewMsgTransfer(
		srcPortId,
		srcChanId,
		amount,
		cp.address.String(),
		dstAddr,
		clienttypes.NewHeight(version, timeoutHeight),
		timeoutTimestamp,
	))
}

// MsgRelayTimeout constructs the MsgTimeout which is to be sent to the sending chain.
// The counterparty represents the receiving chain where the receipts would have been
// stored.
func (cp *CosmosProvider) MsgRelayTimeout(dst provider.ChainProvider, dsth int64, packet provider.RelayPacket, dstChanId, dstPortId, srcChanId, srcPortId string) (provider.RelayerMessage, error) {
	recvRes, err := dst.QueryPacketReceipt(dsth, dstChanId, dstPortId, packet.Seq())
	switch {
	case err != nil:
		return nil, err
	case recvRes.Proof == nil:
		return nil, fmt.Errorf("timeout packet receipt proof seq(%d) is nil", packet.Seq())
	case recvRes == nil:
		return nil, fmt.Errorf("timeout packet [%s]seq{%d} has no associated proofs", cp.Config.ChainID, packet.Seq())
	default:
		return NewCosmosMessage(chantypes.NewMsgTimeout(
			chantypes.NewPacket(
				packet.Data(),
				packet.Seq(),
				srcPortId,
				srcChanId,
				dstPortId,
				dstChanId,
				packet.Timeout(),
				packet.TimeoutStamp(),
			),
			packet.Seq(),
			recvRes.Proof,
			recvRes.ProofHeight,
			cp.address.String(),
		)), nil
	}
}

// MsgRelayRecvPacket constructs the MsgRecvPacket which is to be sent to the receiving chain.
// The counterparty represents the sending chain where the packet commitment would be stored.
func (cp *CosmosProvider) MsgRelayRecvPacket(dst provider.ChainProvider, dsth int64, packet provider.RelayPacket, dstChanId, dstPortId, srcChanId, srcPortId string) (provider.RelayerMessage, error) {
	comRes, err := dst.QueryPacketCommitment(dsth, dstChanId, dstPortId, packet.Seq())
	switch {
	case err != nil:
		return nil, err
	case comRes.Proof == nil || comRes.Commitment == nil:
		return nil, fmt.Errorf("recv packet commitment query seq(%d) is nil", packet.Seq())
	case comRes == nil:
		return nil, fmt.Errorf("receive packet [%s]seq{%d} has no associated proofs", cp.Config.ChainID, packet.Seq())
	default:
		return NewCosmosMessage(chantypes.NewMsgRecvPacket(
			chantypes.NewPacket(
				packet.Data(),
				packet.Seq(),
				dstPortId,
				dstChanId,
				srcPortId,
				srcChanId,
				packet.Timeout(),
				packet.TimeoutStamp(),
			),
			comRes.Proof,
			comRes.ProofHeight,
			cp.address.String(),
		)), nil
	}
}

// RelayPacketFromSequence relays a packet with a given seq on src and returns recvPacket msgs, timeoutPacketmsgs and error
func (cp *CosmosProvider) RelayPacketFromSequence(src, dst provider.ChainProvider, srch, dsth, seq uint64, dstChanId, dstPortId, srcChanId, srcPortId, srcClientId string) (provider.RelayerMessage, provider.RelayerMessage, error) {
	txs, err := cp.QueryTxs(1, 1000, rcvPacketQuery(srcChanId, int(seq)))
	switch {
	case err != nil:
		return nil, nil, err
	case len(txs) == 0:
		return nil, nil, fmt.Errorf("no transactions returned with query")
	case len(txs) > 1:
		return nil, nil, fmt.Errorf("more than one transaction returned with query")
	}

	rcvPackets, timeoutPackets, err := relayPacketsFromResultTx(src, dst, int64(dsth), txs[0], dstChanId, dstPortId, srcChanId, srcPortId, srcClientId)
	switch {
	case err != nil:
		return nil, nil, err
	case len(rcvPackets) == 0 && len(timeoutPackets) == 0:
		return nil, nil, fmt.Errorf("no relay msgs created from query response")
	case len(rcvPackets)+len(timeoutPackets) > 1:
		return nil, nil, fmt.Errorf("more than one relay msg found in tx query")
	}

	if len(rcvPackets) == 1 {
		pkt := rcvPackets[0]
		if seq != pkt.Seq() {
			return nil, nil, fmt.Errorf("wrong sequence: expected(%d) got(%d)", seq, pkt.Seq())
		}

		packet, err := dst.MsgRelayRecvPacket(src, int64(srch), pkt, srcChanId, srcPortId, dstChanId, dstPortId)
		if err != nil {
			return nil, nil, err
		}

		return packet, nil, nil
	}

	if len(timeoutPackets) == 1 {
		pkt := timeoutPackets[0]
		if seq != pkt.Seq() {
			return nil, nil, fmt.Errorf("wrong sequence: expected(%d) got(%d)", seq, pkt.Seq())
		}

		timeout, err := src.MsgRelayTimeout(dst, int64(dsth), pkt, dstChanId, dstPortId, srcChanId, srcPortId)
		if err != nil {
			return nil, nil, err
		}
		return nil, timeout, nil
	}

	return nil, nil, fmt.Errorf("should have errored before here")
}

// AcknowledgementFromSequence relays an acknowledgement with a given seq on src, source is the sending chain, destination is the receiving chain
func (cp *CosmosProvider) AcknowledgementFromSequence(dst provider.ChainProvider, dsth, seq uint64, dstChanId, dstPortId, srcChanId, srcPortId string) (provider.RelayerMessage, error) {
	txs, err := dst.QueryTxs(1, 1000, ackPacketQuery(dstChanId, int(seq)))
	switch {
	case err != nil:
		return nil, err
	case len(txs) == 0:
		return nil, fmt.Errorf("no transactions returned with query")
	case len(txs) > 1:
		return nil, fmt.Errorf("more than one transaction returned with query")
	}

	acks, err := acknowledgementsFromResultTx(dstChanId, dstPortId, srcChanId, srcPortId, txs[0])
	switch {
	case err != nil:
		return nil, err
	case len(acks) == 0:
		return nil, fmt.Errorf("no ack msgs created from query response")
	}

	var out provider.RelayerMessage
	for _, ack := range acks {
		if seq != ack.Seq() {
			continue
		}
		msg, err := cp.MsgRelayAcknowledgement(dst, dstChanId, dstPortId, srcChanId, srcPortId, int64(dsth), ack)
		if err != nil {
			return nil, err
		}
		out = msg
	}
	return out, nil
}

// TODO revisit this and ensure this works
// DefaultUpgradePath is the default IBC upgrade path set for an on-chain light client
var defaultUpgradePath = []string{"upgrade", "upgradedIBCState"}

func (cp *CosmosProvider) NewClientState(dstUpdateHeader ibcexported.Header, dstTrustingPeriod, dstUbdPeriod time.Duration, allowUpdateAfterExpiry, allowUpdateAfterMisbehaviour bool) (ibcexported.ClientState, error) {
	dstTmHeader, ok := dstUpdateHeader.(*tmclient.Header)
	if !ok {
		return nil, fmt.Errorf("got data of type %T but wanted  tmclient.Header \n", dstUpdateHeader)
	}
	// Create the ClientState we want on 'c' tracking 'dst'
	return tmclient.NewClientState(
		dstTmHeader.GetHeader().GetChainID(),
		tmclient.NewFractionFromTm(light.DefaultTrustLevel),
		dstTrustingPeriod,
		dstUbdPeriod,
		time.Minute*10,
		dstUpdateHeader.GetHeight().(clienttypes.Height),
		commitmenttypes.GetSDKSpecs(),
		defaultUpgradePath,
		allowUpdateAfterExpiry,
		allowUpdateAfterMisbehaviour,
	), nil
}
