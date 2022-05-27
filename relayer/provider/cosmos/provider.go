package cosmos

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/types/module"
	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v3/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	commitmenttypes "github.com/cosmos/ibc-go/v3/modules/core/23-commitment/types"
	host "github.com/cosmos/ibc-go/v3/modules/core/24-host"
	ibcexported "github.com/cosmos/ibc-go/v3/modules/core/exported"
	tmclient "github.com/cosmos/ibc-go/v3/modules/light-clients/07-tendermint/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/gogo/protobuf/proto"
	lens "github.com/strangelove-ventures/lens/client"
	tmtypes "github.com/tendermint/tendermint/types"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	_ provider.ChainProvider = &CosmosProvider{}
	_ provider.KeyProvider   = &CosmosProvider{}
	_ provider.QueryProvider = &CosmosProvider{}

	// Default IBC settings
	defaultChainPrefix = commitmenttypes.NewMerklePrefix([]byte("ibc"))
	defaultDelayPeriod = uint64(0)

	// Variables used for retries
	RtyAttNum = uint(5)
	RtyAtt    = retry.Attempts(RtyAttNum)
	RtyDel    = retry.Delay(time.Millisecond * 400)
	RtyErr    = retry.LastErrorOnly(true)

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
		fmt.Printf("got data of type %T but wanted provider.CosmosMessage \n", val)
		return nil
	} else {
		return val.Msg
	}
}

func CosmosMsgs(rm ...provider.RelayerMessage) []sdk.Msg {
	sdkMsgs := make([]sdk.Msg, 0)
	for _, rMsg := range rm {
		if val, ok := rMsg.(CosmosMessage); !ok {
			fmt.Printf("got data of type %T but wanted provider.CosmosMessage \n", val)
			return nil
		} else {
			sdkMsgs = append(sdkMsgs, val.Msg)
		}
	}
	return sdkMsgs
}

func (cm CosmosMessage) Type() string {
	return sdk.MsgTypeURL(cm.Msg)
}

func (cm CosmosMessage) MsgBytes() ([]byte, error) {
	return proto.Marshal(cm.Msg)
}

// MarshalLogObject is used to encode cm to a zap logger with the zap.Object field type.
func (cm CosmosMessage) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	// Using plain json.Marshal or calling cm.Msg.String() both fail miserably here.
	// There is probably a better way to encode the message than this.
	j, err := codec.NewLegacyAmino().MarshalJSON(cm.Msg)
	if err != nil {
		return err
	}
	enc.AddByteString("msg_json", j)
	return nil
}

type CosmosProviderConfig struct {
	Key            string  `json:"key" yaml:"key"`
	ChainID        string  `json:"chain-id" yaml:"chain-id"`
	RPCAddr        string  `json:"rpc-addr" yaml:"rpc-addr"`
	AccountPrefix  string  `json:"account-prefix" yaml:"account-prefix"`
	KeyringBackend string  `json:"keyring-backend" yaml:"keyring-backend"`
	GasAdjustment  float64 `json:"gas-adjustment" yaml:"gas-adjustment"`
	GasPrices      string  `json:"gas-prices" yaml:"gas-prices"`
	Debug          bool    `json:"debug" yaml:"debug"`
	Timeout        string  `json:"timeout" yaml:"timeout"`
	OutputFormat   string  `json:"output-format" yaml:"output-format"`
	SignModeStr    string  `json:"sign-mode" yaml:"sign-mode"`
}

func (pc CosmosProviderConfig) Validate() error {
	if _, err := time.ParseDuration(pc.Timeout); err != nil {
		return fmt.Errorf("invalid Timeout: %w", err)
	}
	return nil
}

// NewProvider validates the CosmosProviderConfig, instantiates a ChainClient and then instantiates a CosmosProvider
func (pc CosmosProviderConfig) NewProvider(log *zap.Logger, homepath string, debug bool) (provider.ChainProvider, error) {
	if err := pc.Validate(); err != nil {
		return nil, err
	}
	cc, err := lens.NewChainClient(
		log.With(zap.String("sys", "chain_client")),
		ChainClientConfig(&pc),
		homepath,
		os.Stdin,
		os.Stdout,
	)
	if err != nil {
		return nil, err
	}
	return &CosmosProvider{
		log: log,

		ChainClient: *cc,
		PCfg:        pc,
	}, nil
}

// ChainClientConfig builds a ChainClientConfig struct from a CosmosProviderConfig, this is used
// to instantiate an instance of ChainClient from lens which is how we build the CosmosProvider
func ChainClientConfig(pcfg *CosmosProviderConfig) *lens.ChainClientConfig {
	return &lens.ChainClientConfig{
		Key:            pcfg.Key,
		ChainID:        pcfg.ChainID,
		RPCAddr:        pcfg.RPCAddr,
		AccountPrefix:  pcfg.AccountPrefix,
		KeyringBackend: pcfg.KeyringBackend,
		GasAdjustment:  pcfg.GasAdjustment,
		GasPrices:      pcfg.GasPrices,
		Debug:          pcfg.Debug,
		Timeout:        pcfg.Timeout,
		OutputFormat:   pcfg.OutputFormat,
		SignModeStr:    pcfg.SignModeStr,
		Modules:        append([]module.AppModuleBasic{}, lens.ModuleBasics...),
	}
}

type CosmosProvider struct {
	log *zap.Logger

	lens.ChainClient
	PCfg CosmosProviderConfig
}

func (cc *CosmosProvider) ProviderConfig() provider.ProviderConfig {
	return cc.PCfg
}

func (cc *CosmosProvider) ChainId() string {
	return cc.PCfg.ChainID
}

func (cc *CosmosProvider) Type() string {
	return "cosmos"
}

func (cc *CosmosProvider) Key() string {
	return cc.PCfg.Key
}

func (cc *CosmosProvider) Timeout() string {
	return cc.PCfg.Timeout
}

func (cc *CosmosProvider) AddKey(name string, coinType uint32) (*provider.KeyOutput, error) {
	// The lens client returns an equivalent KeyOutput type,
	// but that type is declared in the lens module,
	// and relayer's KeyProvider interface references the relayer KeyOutput.
	//
	// Translate the lens KeyOutput to a relayer KeyOutput here to satisfy the interface.

	ko, err := cc.ChainClient.AddKey(name, coinType)
	if err != nil {
		return nil, err
	}
	return &provider.KeyOutput{
		Mnemonic: ko.Mnemonic,
		Address:  ko.Address,
	}, nil
}

// Address returns the chains configured address as a string
func (cc *CosmosProvider) Address() (string, error) {
	var (
		err  error
		info keyring.Info
	)
	info, err = cc.Keybase.Key(cc.PCfg.Key)
	if err != nil {
		return "", err
	}
	out, err := cc.EncodeBech32AccAddr(info.GetAddress())
	if err != nil {
		return "", err
	}

	return out, err
}

func (cc *CosmosProvider) TrustingPeriod(ctx context.Context) (time.Duration, error) {
	res, err := cc.QueryStakingParams(ctx)
	if err != nil {
		return 0, err
	}

	// We want the trusting period to be 85% of the unbonding time.
	// Go mentions that the time.Duration type can track approximately 290 years.
	// We don't want to lose precision if the duration is a very long duration
	// by converting int64 to float64.
	// Use integer math the whole time, first reducing by a factor of 100
	// and then re-growing by 85x.
	tp := res.UnbondingTime / 100 * 85

	// And we only want the trusting period to be whole hours.
	return tp.Truncate(time.Hour), nil
}

// Sprint returns the json representation of the specified proto message.
func (cc *CosmosProvider) Sprint(toPrint proto.Message) (string, error) {
	out, err := cc.Codec.Marshaler.MarshalJSON(toPrint)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// CreateClient creates an sdk.Msg to update the client on src with consensus state from dst
func (cc *CosmosProvider) CreateClient(clientState ibcexported.ClientState, dstHeader ibcexported.Header) (provider.RelayerMessage, error) {
	var (
		acc string
		err error
	)

	tmHeader, ok := dstHeader.(*tmclient.Header)
	if !ok {
		return nil, fmt.Errorf("got data of type %T but wanted tmclient.Header", dstHeader)
	}

	if acc, err = cc.Address(); err != nil {
		return nil, err
	}

	anyClientState, err := clienttypes.PackClientState(clientState)
	if err != nil {
		return nil, err
	}

	anyConsensusState, err := clienttypes.PackConsensusState(tmHeader.ConsensusState())
	if err != nil {
		return nil, err
	}

	msg := &clienttypes.MsgCreateClient{
		ClientState:    anyClientState,
		ConsensusState: anyConsensusState,
		Signer:         acc,
	}
	if err != nil {
		return nil, err
	}

	return NewCosmosMessage(msg), nil
}

func (cc *CosmosProvider) SubmitMisbehavior( /*TBD*/ ) (provider.RelayerMessage, error) {
	return nil, nil
}

func (cc *CosmosProvider) UpdateClient(srcClientId string, dstHeader ibcexported.Header) (provider.RelayerMessage, error) {
	acc, err := cc.Address()
	if err != nil {
		return nil, err
	}

	anyHeader, err := clienttypes.PackHeader(dstHeader)
	if err != nil {
		return nil, err
	}

	msg := &clienttypes.MsgUpdateClient{
		ClientId: srcClientId,
		Header:   anyHeader,
		Signer:   acc,
	}

	return NewCosmosMessage(msg), nil
}

func (cc *CosmosProvider) ConnectionOpenInit(srcClientId, dstClientId string, dstHeader ibcexported.Header) ([]provider.RelayerMessage, error) {
	var (
		acc     string
		err     error
		version *conntypes.Version
	)
	updateMsg, err := cc.UpdateClient(srcClientId, dstHeader)
	if err != nil {
		return nil, err
	}

	if acc, err = cc.Address(); err != nil {
		return nil, err
	}

	counterparty := conntypes.Counterparty{
		ClientId:     dstClientId,
		ConnectionId: "",
		Prefix:       defaultChainPrefix,
	}
	msg := &conntypes.MsgConnectionOpenInit{
		ClientId:     srcClientId,
		Counterparty: counterparty,
		Version:      version,
		DelayPeriod:  defaultDelayPeriod,
		Signer:       acc,
	}

	return []provider.RelayerMessage{updateMsg, NewCosmosMessage(msg)}, nil
}

func (cc *CosmosProvider) ConnectionOpenTry(ctx context.Context, dstQueryProvider provider.QueryProvider, dstHeader ibcexported.Header, srcClientId, dstClientId, srcConnId, dstConnId string) ([]provider.RelayerMessage, error) {
	var (
		acc string
		err error
	)
	updateMsg, err := cc.UpdateClient(srcClientId, dstHeader)
	if err != nil {
		return nil, err
	}

	cph, err := dstQueryProvider.QueryLatestHeight(ctx)
	if err != nil {
		return nil, err
	}

	clientState, clientStateProof, consensusStateProof, connStateProof, proofHeight, err := dstQueryProvider.GenerateConnHandshakeProof(ctx, cph, dstClientId, dstConnId)
	if err != nil {
		return nil, err
	}

	if len(connStateProof) == 0 {
		// It is possible that we have asked for a proof too early.
		// If the connection state proof is empty, there is no point in returning the MsgConnectionOpenTry.
		// We are not using (*conntypes.MsgConnectionOpenTry).ValidateBasic here because
		// that chokes on cross-chain bech32 details in ibc-go.
		return nil, fmt.Errorf("received invalid zero-length connection state proof")
	}

	if acc, err = cc.Address(); err != nil {
		return nil, err
	}

	csAny, err := clienttypes.PackClientState(clientState)
	if err != nil {
		return nil, err
	}

	counterparty := conntypes.Counterparty{
		ClientId:     dstClientId,
		ConnectionId: dstConnId,
		Prefix:       defaultChainPrefix,
	}

	// TODO: Get DelayPeriod from counterparty connection rather than using default value
	msg := &conntypes.MsgConnectionOpenTry{
		ClientId:             srcClientId,
		PreviousConnectionId: srcConnId,
		ClientState:          csAny,
		Counterparty:         counterparty,
		DelayPeriod:          defaultDelayPeriod,
		CounterpartyVersions: conntypes.ExportedVersionsToProto(conntypes.GetCompatibleVersions()),
		ProofHeight: clienttypes.Height{
			RevisionNumber: proofHeight.GetRevisionNumber(),
			RevisionHeight: proofHeight.GetRevisionHeight(),
		},
		ProofInit:       connStateProof,
		ProofClient:     clientStateProof,
		ProofConsensus:  consensusStateProof,
		ConsensusHeight: clientState.GetLatestHeight().(clienttypes.Height),
		Signer:          acc,
	}

	return []provider.RelayerMessage{updateMsg, NewCosmosMessage(msg)}, nil
}

func (cc *CosmosProvider) ConnectionOpenAck(ctx context.Context, dstQueryProvider provider.QueryProvider, dstHeader ibcexported.Header, srcClientId, srcConnId, dstClientId, dstConnId string) ([]provider.RelayerMessage, error) {
	var (
		acc string
		err error
	)

	updateMsg, err := cc.UpdateClient(srcClientId, dstHeader)
	if err != nil {
		return nil, err
	}
	cph, err := dstQueryProvider.QueryLatestHeight(ctx)
	if err != nil {
		return nil, err
	}

	clientState, clientStateProof, consensusStateProof, connStateProof,
		proofHeight, err := dstQueryProvider.GenerateConnHandshakeProof(ctx, cph, dstClientId, dstConnId)
	if err != nil {
		return nil, err
	}

	if acc, err = cc.Address(); err != nil {
		return nil, err
	}

	csAny, err := clienttypes.PackClientState(clientState)
	if err != nil {
		return nil, err
	}

	msg := &conntypes.MsgConnectionOpenAck{
		ConnectionId:             srcConnId,
		CounterpartyConnectionId: dstConnId,
		Version:                  conntypes.DefaultIBCVersion,
		ClientState:              csAny,
		ProofHeight: clienttypes.Height{
			RevisionNumber: proofHeight.GetRevisionNumber(),
			RevisionHeight: proofHeight.GetRevisionHeight(),
		},
		ProofTry:        connStateProof,
		ProofClient:     clientStateProof,
		ProofConsensus:  consensusStateProof,
		ConsensusHeight: clientState.GetLatestHeight().(clienttypes.Height),
		Signer:          acc,
	}

	return []provider.RelayerMessage{updateMsg, NewCosmosMessage(msg)}, nil
}

func (cc *CosmosProvider) ConnectionOpenConfirm(ctx context.Context, dstQueryProvider provider.QueryProvider, dstHeader ibcexported.Header, dstConnId, srcClientId, srcConnId string) ([]provider.RelayerMessage, error) {
	var (
		acc string
		err error
	)
	updateMsg, err := cc.UpdateClient(srcClientId, dstHeader)
	if err != nil {
		return nil, err
	}

	cph, err := dstQueryProvider.QueryLatestHeight(ctx)
	if err != nil {
		return nil, err
	}
	counterpartyConnState, err := dstQueryProvider.QueryConnection(ctx, cph, dstConnId)
	if err != nil {
		return nil, err
	}

	if acc, err = cc.Address(); err != nil {
		return nil, err
	}

	msg := &conntypes.MsgConnectionOpenConfirm{
		ConnectionId: srcConnId,
		ProofAck:     counterpartyConnState.Proof,
		ProofHeight:  counterpartyConnState.ProofHeight,
		Signer:       acc,
	}

	return []provider.RelayerMessage{updateMsg, NewCosmosMessage(msg)}, nil
}

func (cc *CosmosProvider) ChannelOpenInit(srcClientId, srcConnId, srcPortId, srcVersion, dstPortId string, order chantypes.Order, dstHeader ibcexported.Header) ([]provider.RelayerMessage, error) {
	var (
		acc string
		err error
	)
	updateMsg, err := cc.UpdateClient(srcClientId, dstHeader)
	if err != nil {
		return nil, err
	}

	if acc, err = cc.Address(); err != nil {
		return nil, err
	}

	msg := &chantypes.MsgChannelOpenInit{
		PortId: srcPortId,
		Channel: chantypes.Channel{
			State:    chantypes.INIT,
			Ordering: order,
			Counterparty: chantypes.Counterparty{
				PortId:    dstPortId,
				ChannelId: "",
			},
			ConnectionHops: []string{srcConnId},
			Version:        srcVersion,
		},
		Signer: acc,
	}

	return []provider.RelayerMessage{updateMsg, NewCosmosMessage(msg)}, nil
}

func (cc *CosmosProvider) ChannelOpenTry(ctx context.Context, dstQueryProvider provider.QueryProvider, dstHeader ibcexported.Header, srcPortId, dstPortId, srcChanId, dstChanId, srcVersion, srcConnectionId, srcClientId string) ([]provider.RelayerMessage, error) {
	var (
		acc string
		err error
	)
	updateMsg, err := cc.UpdateClient(srcClientId, dstHeader)
	if err != nil {
		return nil, err
	}
	cph, err := dstQueryProvider.QueryLatestHeight(ctx)
	if err != nil {
		return nil, err
	}

	counterpartyChannelRes, err := dstQueryProvider.QueryChannel(ctx, cph, dstChanId, dstPortId)
	if err != nil {
		return nil, err
	}

	if len(counterpartyChannelRes.Proof) == 0 {
		// It is possible that we have asked for a proof too early.
		// If the connection state proof is empty, there is no point in returning the MsgChannelOpenTry.
		// We are not using (*conntypes.MsgChannelOpenTry).ValidateBasic here because
		// that chokes on cross-chain bech32 details in ibc-go.
		return nil, fmt.Errorf("received invalid zero-length channel state proof")
	}

	if acc, err = cc.Address(); err != nil {
		return nil, err
	}

	msg := &chantypes.MsgChannelOpenTry{
		PortId:            srcPortId,
		PreviousChannelId: srcChanId,
		Channel: chantypes.Channel{
			State:    chantypes.TRYOPEN,
			Ordering: counterpartyChannelRes.Channel.Ordering,
			Counterparty: chantypes.Counterparty{
				PortId:    dstPortId,
				ChannelId: dstChanId,
			},
			ConnectionHops: []string{srcConnectionId},
			Version:        srcVersion,
		},
		CounterpartyVersion: counterpartyChannelRes.Channel.Version,
		ProofInit:           counterpartyChannelRes.Proof,
		ProofHeight:         counterpartyChannelRes.ProofHeight,
		Signer:              acc,
	}

	return []provider.RelayerMessage{updateMsg, NewCosmosMessage(msg)}, nil
}

func (cc *CosmosProvider) ChannelOpenAck(ctx context.Context, dstQueryProvider provider.QueryProvider, dstHeader ibcexported.Header, srcClientId, srcPortId, srcChanId, dstChanId, dstPortId string) ([]provider.RelayerMessage, error) {
	var (
		acc string
		err error
	)
	updateMsg, err := cc.UpdateClient(srcClientId, dstHeader)
	if err != nil {
		return nil, err
	}

	cph, err := dstQueryProvider.QueryLatestHeight(ctx)
	if err != nil {
		return nil, err
	}

	counterpartyChannelRes, err := dstQueryProvider.QueryChannel(ctx, cph, dstChanId, dstPortId)
	if err != nil {
		return nil, err
	}

	if acc, err = cc.Address(); err != nil {
		return nil, err
	}

	msg := &chantypes.MsgChannelOpenAck{
		PortId:                srcPortId,
		ChannelId:             srcChanId,
		CounterpartyChannelId: dstChanId,
		CounterpartyVersion:   counterpartyChannelRes.Channel.Version,
		ProofTry:              counterpartyChannelRes.Proof,
		ProofHeight:           counterpartyChannelRes.ProofHeight,
		Signer:                acc,
	}

	return []provider.RelayerMessage{updateMsg, NewCosmosMessage(msg)}, nil
}

func (cc *CosmosProvider) ChannelOpenConfirm(ctx context.Context, dstQueryProvider provider.QueryProvider, dstHeader ibcexported.Header, srcClientId, srcPortId, srcChanId, dstPortId, dstChanId string) ([]provider.RelayerMessage, error) {
	var (
		acc string
		err error
	)
	updateMsg, err := cc.UpdateClient(srcClientId, dstHeader)
	if err != nil {
		return nil, err
	}
	cph, err := dstQueryProvider.QueryLatestHeight(ctx)
	if err != nil {
		return nil, err
	}

	counterpartyChanState, err := dstQueryProvider.QueryChannel(ctx, cph, dstChanId, dstPortId)
	if err != nil {
		return nil, err
	}

	if acc, err = cc.Address(); err != nil {
		return nil, err
	}

	msg := &chantypes.MsgChannelOpenConfirm{
		PortId:      srcPortId,
		ChannelId:   srcChanId,
		ProofAck:    counterpartyChanState.Proof,
		ProofHeight: counterpartyChanState.ProofHeight,
		Signer:      acc,
	}

	return []provider.RelayerMessage{updateMsg, NewCosmosMessage(msg)}, nil
}

func (cc *CosmosProvider) ChannelCloseInit(srcPortId, srcChanId string) (provider.RelayerMessage, error) {
	var (
		acc string
		err error
	)
	if acc, err = cc.Address(); err != nil {
		return nil, err
	}

	msg := &chantypes.MsgChannelCloseInit{
		PortId:    srcPortId,
		ChannelId: srcChanId,
		Signer:    acc,
	}

	return NewCosmosMessage(msg), nil
}

func (cc *CosmosProvider) ChannelCloseConfirm(ctx context.Context, dstQueryProvider provider.QueryProvider, dsth int64, dstChanId, dstPortId, srcPortId, srcChanId string) (provider.RelayerMessage, error) {
	var (
		acc string
		err error
	)
	dstChanResp, err := dstQueryProvider.QueryChannel(ctx, dsth, dstChanId, dstPortId)
	if err != nil {
		return nil, err
	}

	if acc, err = cc.Address(); err != nil {
		return nil, err
	}

	msg := &chantypes.MsgChannelCloseConfirm{
		PortId:      srcPortId,
		ChannelId:   srcChanId,
		ProofInit:   dstChanResp.Proof,
		ProofHeight: dstChanResp.ProofHeight,
		Signer:      acc,
	}

	return NewCosmosMessage(msg), nil
}

// GetIBCUpdateHeader updates the off chain tendermint light client and
// returns an IBC Update Header which can be used to update an on chain
// light client on the destination chain. The source is used to construct
// the header data.
func (cc *CosmosProvider) GetIBCUpdateHeader(ctx context.Context, srch int64, dst provider.ChainProvider, dstClientId string) (ibcexported.Header, error) {
	// Construct header data from light client representing source.
	h, err := cc.GetLightSignedHeaderAtHeight(ctx, srch)
	if err != nil {
		return nil, err
	}

	// Inject trusted fields based on previous header data from source
	return cc.InjectTrustedFields(ctx, h, dst, dstClientId)
}

func (cc *CosmosProvider) GetLightSignedHeaderAtHeight(ctx context.Context, h int64) (ibcexported.Header, error) {
	if h == 0 {
		return nil, errors.New("height cannot be 0")
	}

	lightBlock, err := cc.LightProvider.LightBlock(ctx, h)
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

// InjectTrustedFields injects the necessary trusted fields for a header to update a light
// client stored on the destination chain, using the information provided by the source
// chain.
// TrustedHeight is the latest height of the IBC client on dst
// TrustedValidators is the validator set of srcChain at the TrustedHeight
// InjectTrustedFields returns a copy of the header with TrustedFields modified
func (cc *CosmosProvider) InjectTrustedFields(ctx context.Context, header ibcexported.Header, dst provider.ChainProvider, dstClientId string) (ibcexported.Header, error) {
	// make copy of header stored in mop
	h, ok := header.(*tmclient.Header)
	if !ok {
		return nil, fmt.Errorf("trying to inject fields into non-tendermint headers")
	}

	// retrieve dst client from src chain
	// this is the client that will be updated
	cs, err := dst.QueryClientState(ctx, int64(h.TrustedHeight.RevisionHeight), dstClientId)
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
		tmpHeader, err := cc.GetLightSignedHeaderAtHeight(ctx, int64(h.TrustedHeight.RevisionHeight+1))
		if err != nil {
			return err
		}

		th, ok := tmpHeader.(*tmclient.Header)
		if !ok {
			err = errors.New("non-tm client header")
		}

		trustedHeader = th
		return err
	}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr); err != nil {
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
func (cc *CosmosProvider) MsgRelayAcknowledgement(ctx context.Context, dst provider.ChainProvider, dstChanId, dstPortId, srcChanId, srcPortId string, dsth int64, packet provider.RelayPacket) (provider.RelayerMessage, error) {
	var (
		acc string
		err error
	)
	msgPacketAck, ok := packet.(*relayMsgPacketAck)
	if !ok {
		return nil, fmt.Errorf("got data of type %T but wanted relayMsgPacketAck", packet)
	}

	if acc, err = cc.Address(); err != nil {
		return nil, err
	}

	ackRes, err := dst.QueryPacketAcknowledgement(ctx, dsth, dstChanId, dstPortId, packet.Seq())
	switch {
	case err != nil:
		return nil, err
	case ackRes.Proof == nil || ackRes.Acknowledgement == nil:
		return nil, fmt.Errorf("ack packet acknowledgement query seq(%d) is nil", packet.Seq())
	case ackRes == nil:
		return nil, fmt.Errorf("ack packet [%s]seq{%d} has no associated proofs", dst.ChainId(), packet.Seq())
	default:
		msg := &chantypes.MsgAcknowledgement{
			Packet: chantypes.Packet{
				Sequence:           packet.Seq(),
				SourcePort:         srcPortId,
				SourceChannel:      srcChanId,
				DestinationPort:    dstPortId,
				DestinationChannel: dstChanId,
				Data:               packet.Data(),
				TimeoutHeight:      packet.Timeout(),
				TimeoutTimestamp:   packet.TimeoutStamp(),
			},
			Acknowledgement: msgPacketAck.ack,
			ProofAcked:      ackRes.Proof,
			ProofHeight:     ackRes.ProofHeight,
			Signer:          acc,
		}

		return NewCosmosMessage(msg), nil
	}
}

// MsgTransfer creates a new transfer message
func (cc *CosmosProvider) MsgTransfer(amount sdk.Coin, dstChainId, dstAddr, srcPortId, srcChanId string, timeoutHeight, timeoutTimestamp uint64) (provider.RelayerMessage, error) {
	var (
		acc string
		err error
		msg sdk.Msg
	)
	if acc, err = cc.Address(); err != nil {
		return nil, err
	}

	// If the timeoutHeight is 0 then we don't need to explicitly set it on the MsgTransfer
	if timeoutHeight == 0 {
		msg = &transfertypes.MsgTransfer{
			SourcePort:       srcPortId,
			SourceChannel:    srcChanId,
			Token:            amount,
			Sender:           acc,
			Receiver:         dstAddr,
			TimeoutTimestamp: timeoutTimestamp,
		}
	} else {
		version := clienttypes.ParseChainID(dstChainId)

		msg = &transfertypes.MsgTransfer{
			SourcePort:    srcPortId,
			SourceChannel: srcChanId,
			Token:         amount,
			Sender:        acc,
			Receiver:      dstAddr,
			TimeoutHeight: clienttypes.Height{
				RevisionNumber: version,
				RevisionHeight: timeoutHeight,
			},
			TimeoutTimestamp: timeoutTimestamp,
		}
	}

	return NewCosmosMessage(msg), nil
}

// MsgRelayTimeout constructs the MsgTimeout which is to be sent to the sending chain.
// The counterparty represents the receiving chain where the receipts would have been
// stored.
func (cc *CosmosProvider) MsgRelayTimeout(
	ctx context.Context,
	dst provider.ChainProvider,
	dsth int64,
	packet provider.RelayPacket,
	dstChanId, dstPortId, srcChanId, srcPortId string,
	order chantypes.Order,
) (provider.RelayerMessage, error) {
	var (
		acc string
		err error
		msg provider.RelayerMessage
	)
	if acc, err = cc.Address(); err != nil {
		return nil, err
	}

	switch order {
	case chantypes.UNORDERED:
		msg, err = cc.unorderedChannelTimeoutMsg(ctx, dst, dsth, packet, acc, dstChanId, dstPortId, srcChanId, srcPortId)
		if err != nil {
			return nil, err
		}
	case chantypes.ORDERED:
		msg, err = cc.orderedChannelTimeoutMsg(ctx, dst, dsth, packet, acc, dstChanId, dstPortId, srcChanId, srcPortId)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("invalid order type %s, order should be %s or %s",
			order, chantypes.ORDERED, chantypes.UNORDERED)
	}

	return msg, nil
}

func (cc *CosmosProvider) orderedChannelTimeoutMsg(
	ctx context.Context,
	dst provider.ChainProvider,
	dsth int64,
	packet provider.RelayPacket,
	acc, dstChanId, dstPortId, srcChanId, srcPortId string,
) (provider.RelayerMessage, error) {
	seqRes, err := dst.QueryNextSeqRecv(ctx, dsth, dstChanId, dstPortId)
	if err != nil {
		return nil, err
	}

	if seqRes == nil {
		return nil, fmt.Errorf("timeout packet [%s]seq{%d} has no associated proofs", cc.PCfg.ChainID, packet.Seq())
	}

	if seqRes.Proof == nil {
		return nil, fmt.Errorf("timeout packet next sequence received proof seq(%d) is nil", packet.Seq())
	}

	msg := &chantypes.MsgTimeout{
		Packet: chantypes.Packet{
			Sequence:           packet.Seq(),
			SourcePort:         srcPortId,
			SourceChannel:      srcChanId,
			DestinationPort:    dstPortId,
			DestinationChannel: dstChanId,
			Data:               packet.Data(),
			TimeoutHeight:      packet.Timeout(),
			TimeoutTimestamp:   packet.TimeoutStamp(),
		},
		ProofUnreceived:  seqRes.Proof,
		ProofHeight:      seqRes.ProofHeight,
		NextSequenceRecv: packet.Seq(),
		Signer:           acc,
	}

	return NewCosmosMessage(msg), nil
}

func (cc *CosmosProvider) unorderedChannelTimeoutMsg(
	ctx context.Context,
	dst provider.ChainProvider,
	dsth int64,
	packet provider.RelayPacket,
	acc, dstChanId, dstPortId, srcChanId, srcPortId string,
) (provider.RelayerMessage, error) {
	recvRes, err := dst.QueryPacketReceipt(ctx, dsth, dstChanId, dstPortId, packet.Seq())
	if err != nil {
		return nil, err
	}

	if recvRes == nil {
		return nil, fmt.Errorf("timeout packet [%s]seq{%d} has no associated proofs", cc.PCfg.ChainID, packet.Seq())
	}

	if recvRes.Proof == nil {
		return nil, fmt.Errorf("timeout packet receipt proof seq(%d) is nil", packet.Seq())
	}

	msg := &chantypes.MsgTimeout{
		Packet: chantypes.Packet{
			Sequence:           packet.Seq(),
			SourcePort:         srcPortId,
			SourceChannel:      srcChanId,
			DestinationPort:    dstPortId,
			DestinationChannel: dstChanId,
			Data:               packet.Data(),
			TimeoutHeight:      packet.Timeout(),
			TimeoutTimestamp:   packet.TimeoutStamp(),
		},
		ProofUnreceived:  recvRes.Proof,
		ProofHeight:      recvRes.ProofHeight,
		NextSequenceRecv: packet.Seq(),
		Signer:           acc,
	}
	return NewCosmosMessage(msg), nil
}

// MsgRelayRecvPacket constructs the MsgRecvPacket which is to be sent to the receiving chain.
// The counterparty represents the sending chain where the packet commitment would be stored.
func (cc *CosmosProvider) MsgRelayRecvPacket(ctx context.Context, dst provider.ChainProvider, dsth int64, packet provider.RelayPacket, dstChanId, dstPortId, srcChanId, srcPortId string) (provider.RelayerMessage, error) {
	var (
		acc string
		err error
	)
	if acc, err = cc.Address(); err != nil {
		return nil, err
	}

	comRes, err := dst.QueryPacketCommitment(ctx, dsth, dstChanId, dstPortId, packet.Seq())
	switch {
	case err != nil:
		return nil, err
	case comRes.Proof == nil || comRes.Commitment == nil:
		return nil, fmt.Errorf("recv packet commitment query seq(%d) is nil", packet.Seq())
	case comRes == nil:
		return nil, fmt.Errorf("receive packet [%s]seq{%d} has no associated proofs", cc.PCfg.ChainID, packet.Seq())
	default:
		msg := &chantypes.MsgRecvPacket{
			Packet: chantypes.Packet{
				Sequence:           packet.Seq(),
				SourcePort:         dstPortId,
				SourceChannel:      dstChanId,
				DestinationPort:    srcPortId,
				DestinationChannel: srcChanId,
				Data:               packet.Data(),
				TimeoutHeight:      packet.Timeout(),
				TimeoutTimestamp:   packet.TimeoutStamp(),
			},
			ProofCommitment: comRes.Proof,
			ProofHeight:     comRes.ProofHeight,
			Signer:          acc,
		}

		return NewCosmosMessage(msg), nil
	}
}

// RelayPacketFromSequence relays a packet with a given seq on src and returns recvPacket msgs, timeoutPacketmsgs and error
func (cc *CosmosProvider) RelayPacketFromSequence(
	ctx context.Context,
	src, dst provider.ChainProvider,
	srch, dsth, seq uint64,
	dstChanId, dstPortId, dstClientId, srcChanId, srcPortId, srcClientId string,
	order chantypes.Order,
) (provider.RelayerMessage, provider.RelayerMessage, error) {
	txs, err := cc.QueryTxs(ctx, 1, 1000, rcvPacketQuery(srcChanId, int(seq)))
	switch {
	case err != nil:
		return nil, nil, err
	case len(txs) == 0:
		return nil, nil, fmt.Errorf("no transactions returned with query")
	case len(txs) > 1:
		return nil, nil, fmt.Errorf("more than one transaction returned with query")
	}

	rcvPackets, timeoutPackets, err := cc.relayPacketsFromResultTx(ctx, src, dst, int64(dsth), txs[0], dstChanId, dstPortId, dstClientId, srcChanId, srcPortId, srcClientId)
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

		packet, err := dst.MsgRelayRecvPacket(ctx, src, int64(srch), pkt, srcChanId, srcPortId, dstChanId, dstPortId)
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

		timeout, err := src.MsgRelayTimeout(ctx, dst, int64(dsth), pkt, dstChanId, dstPortId, srcChanId, srcPortId, order)
		if err != nil {
			return nil, nil, err
		}
		return nil, timeout, nil
	}

	return nil, nil, fmt.Errorf("should have errored before here")
}

// AcknowledgementFromSequence relays an acknowledgement with a given seq on src, source is the sending chain, destination is the receiving chain
func (cc *CosmosProvider) AcknowledgementFromSequence(ctx context.Context, dst provider.ChainProvider, dsth, seq uint64, dstChanId, dstPortId, srcChanId, srcPortId string) (provider.RelayerMessage, error) {
	txs, err := dst.QueryTxs(ctx, 1, 1000, ackPacketQuery(dstChanId, int(seq)))
	switch {
	case err != nil:
		return nil, err
	case len(txs) == 0:
		return nil, fmt.Errorf("no transactions returned with query")
	case len(txs) > 1:
		return nil, fmt.Errorf("more than one transaction returned with query")
	}

	acks, err := cc.acknowledgementsFromResultTx(dstChanId, dstPortId, srcChanId, srcPortId, txs[0])
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
		msg, err := cc.MsgRelayAcknowledgement(ctx, dst, dstChanId, dstPortId, srcChanId, srcPortId, int64(dsth), ack)
		if err != nil {
			return nil, err
		}
		out = msg
	}
	return out, nil
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
func (cc *CosmosProvider) relayPacketsFromResultTx(ctx context.Context, src, dst provider.ChainProvider, dsth int64, resp *provider.RelayerTxResponse, dstChanId, dstPortId, dstClientId, srcChanId, srcPortId, srcClientId string) ([]provider.RelayPacket, []provider.RelayPacket, error) {
	var (
		rcvPackets     []provider.RelayPacket
		timeoutPackets []provider.RelayPacket
	)

EventLoop:
	for _, event := range resp.Events {
		rp := &relayMsgRecvPacket{}

		if event.EventType != spTag {
			continue
		}

		for attributeKey, attributeValue := range event.Attributes {
			switch attributeKey {
			case srcChanTag:
				if attributeValue != srcChanId {
					continue EventLoop
				}
			case dstChanTag:
				if attributeValue != dstChanId {
					continue EventLoop
				}
			case srcPortTag:
				if attributeValue != srcPortId {
					continue EventLoop
				}
			case dstPortTag:
				if attributeValue != dstPortId {
					continue EventLoop
				}
			case dataTag:
				rp.packetData = []byte(attributeValue)
			case toHeightTag:
				timeout, err := clienttypes.ParseHeight(attributeValue)
				if err != nil {
					cc.log.Warn("error parsing height timeout",
						zap.String("chain_id", cc.ChainId()),
						zap.Uint64("sequence", rp.seq),
						zap.Error(err),
					)
					continue EventLoop
				}
				rp.timeout = timeout
			case toTSTag:
				timeout, err := strconv.ParseUint(attributeValue, 10, 64)
				if err != nil {
					cc.log.Warn("error parsing timestamp timeout",
						zap.String("chain_id", cc.ChainId()),
						zap.Uint64("sequence", rp.seq),
						zap.Error(err),
					)
					continue EventLoop
				}
				rp.timeoutStamp = timeout
			case seqTag:
				seq, err := strconv.ParseUint(attributeValue, 10, 64)
				if err != nil {
					cc.log.Warn("error parsing packet sequence",
						zap.String("chain_id", cc.ChainId()),
						zap.Error(err),
					)
					continue EventLoop
				}
				rp.seq = seq
			}
		}

		// If packet data is nil or sequence number is 0 keep parsing events,
		// also check that at least the block height or timestamp is set.
		if rp.packetData == nil || rp.seq == 0 || (rp.timeout.IsZero() && rp.timeoutStamp == 0) {
			continue
		}

		// fetch the header which represents a block produced on destination
		block, err := dst.GetIBCUpdateHeader(ctx, dsth, src, srcClientId)
		if err != nil {
			return nil, nil, err
		}

		// if the timestamp is set on the packet, we need to retrieve the current block time from dst
		var dstBlockTime int64
		if rp.timeoutStamp > 0 {
			dstBlockTime, err = dst.BlockTime(ctx, dsth)
			if err != nil {
				return nil, nil, err
			}
		}

		switch {
		// If the packet has a timeout time, and it has been reached, return a timeout packet
		case rp.timeoutStamp > 0 && uint64(dstBlockTime) > rp.timeoutStamp:
			timeoutPackets = append(timeoutPackets, rp.timeoutPacket())
		// If the packet has a timeout height, and it has been reached, return a timeout packet
		case !rp.timeout.IsZero() && block.GetHeight().GTE(rp.timeout):
			timeoutPackets = append(timeoutPackets, rp.timeoutPacket())
		// If the packet matches the relay constraints relay it as a MsgReceivePacket
		default:
			rcvPackets = append(rcvPackets, rp)
		}
	}

	// If there is a relayPacket, return it
	if len(rcvPackets) > 0 || len(timeoutPackets) > 0 {
		return rcvPackets, timeoutPackets, nil
	}

	return nil, nil, fmt.Errorf("no packet data found")
}

// acknowledgementsFromResultTx looks through the events in a *ctypes.ResultTx and returns
// relayPackets with the appropriate data
func (cc *CosmosProvider) acknowledgementsFromResultTx(dstChanId, dstPortId, srcChanId, srcPortId string, resp *provider.RelayerTxResponse) ([]provider.RelayPacket, error) {
	var ackPackets []provider.RelayPacket

EventLoop:
	for _, event := range resp.Events {
		rp := &relayMsgPacketAck{}

		if event.EventType != waTag {
			continue
		}

		for attributeKey, attributeValue := range event.Attributes {

			switch attributeKey {
			case srcChanTag:
				if attributeValue != srcChanId {
					continue EventLoop
				}
			case dstChanTag:
				if attributeValue != dstChanId {
					continue EventLoop
				}
			case srcPortTag:
				if attributeValue != srcPortId {
					continue EventLoop
				}
			case dstPortTag:
				if attributeValue != dstPortId {
					continue EventLoop
				}
			case ackTag:
				rp.ack = []byte(attributeValue)
			case dataTag:
				rp.packetData = []byte(attributeValue)
			case toHeightTag:
				timeout, err := clienttypes.ParseHeight(attributeValue)
				if err != nil {
					cc.log.Warn("error parsing height timeout",
						zap.String("chain_id", cc.ChainId()),
						zap.Uint64("sequence", rp.seq),
						zap.Error(err),
					)
					continue EventLoop
				}
				rp.timeout = timeout
			case toTSTag:
				timeout, err := strconv.ParseUint(attributeValue, 10, 64)
				if err != nil {
					cc.log.Warn("error parsing timestamp timeout",
						zap.String("chain_id", cc.ChainId()),
						zap.Uint64("sequence", rp.seq),
						zap.Error(err),
					)
					continue EventLoop
				}
				rp.timeoutStamp = timeout
			case seqTag:
				seq, err := strconv.ParseUint(attributeValue, 10, 64)
				if err != nil {
					cc.log.Warn("error parsing packet sequence",
						zap.String("chain_id", cc.ChainId()),
						zap.Error(err),
					)
					continue EventLoop
				}
				rp.seq = seq
			}
		}

		// If packet data is nil or sequence number is 0 keep parsing events,
		// also check that at least the block height or timestamp is set.
		if rp.ack == nil || rp.packetData == nil || rp.seq == 0 || (rp.timeout.IsZero() && rp.timeoutStamp == 0) {
			continue
		}

		ackPackets = append(ackPackets, rp)

	}

	// If there is a relayPacket, return it
	if len(ackPackets) > 0 {
		return ackPackets, nil
	}

	return nil, fmt.Errorf("no packet data found")
}

func (cc *CosmosProvider) MsgUpgradeClient(srcClientId string, consRes *clienttypes.QueryConsensusStateResponse, clientRes *clienttypes.QueryClientStateResponse) (provider.RelayerMessage, error) {
	var (
		acc string
		err error
	)
	if acc, err = cc.Address(); err != nil {
		return nil, err
	}
	return NewCosmosMessage(&clienttypes.MsgUpgradeClient{ClientId: srcClientId, ClientState: clientRes.ClientState,
		ConsensusState: consRes.ConsensusState, ProofUpgradeClient: consRes.GetProof(),
		ProofUpgradeConsensusState: consRes.ConsensusState.Value, Signer: acc}), nil
}

// AutoUpdateClient update client automatically to prevent expiry
func (cc *CosmosProvider) AutoUpdateClient(ctx context.Context, dst provider.ChainProvider, thresholdTime time.Duration, srcClientId, dstClientId string) (time.Duration, error) {
	srch, err := cc.QueryLatestHeight(ctx)
	if err != nil {
		return 0, err
	}
	dsth, err := dst.QueryLatestHeight(ctx)
	if err != nil {
		return 0, err
	}

	clientState, err := cc.queryTMClientState(ctx, srch, srcClientId)
	if err != nil {
		return 0, err
	}

	if clientState.TrustingPeriod <= thresholdTime {
		return 0, fmt.Errorf("client (%s) trusting period time is less than or equal to threshold time", srcClientId)
	}

	// query the latest consensus state of the potential matching client
	var consensusStateResp *clienttypes.QueryConsensusStateResponse
	if err = retry.Do(func() error {
		if clientState == nil {
			return fmt.Errorf("client state for chain (%s) at height (%d) cannot be nil", cc.ChainId(), srch)
		}
		consensusStateResp, err = cc.QueryConsensusStateABCI(ctx, srcClientId, clientState.GetLatestHeight())
		return err
	}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
		cc.log.Info(
			"Error querying consensus state ABCI",
			zap.String("chain_id", cc.PCfg.ChainID),
			zap.Uint("attempt", n+1),
			zap.Uint("max_attempts", RtyAttNum),
			zap.Error(err),
		)
		clientState, err = cc.queryTMClientState(ctx, srch, srcClientId)
		if err != nil {
			clientState = nil
			cc.log.Info(
				"Failed to refresh tendermint client state in order to re-query consensus state ABCI",
				zap.String("chain_id", cc.PCfg.ChainID),
				zap.Error(err),
			)
		}
	})); err != nil {
		return 0, err
	}

	exportedConsState, err := clienttypes.UnpackConsensusState(consensusStateResp.ConsensusState)
	if err != nil {
		return 0, err
	}

	consensusState, ok := exportedConsState.(*tmclient.ConsensusState)
	if !ok {
		return 0, fmt.Errorf("consensus state with clientID %s from chain %s is not IBC tendermint type",
			srcClientId, cc.PCfg.ChainID)
	}

	expirationTime := consensusState.Timestamp.Add(clientState.TrustingPeriod)

	timeToExpiry := time.Until(expirationTime)

	if timeToExpiry > thresholdTime {
		return timeToExpiry, nil
	}

	if clientState.IsExpired(consensusState.Timestamp, time.Now()) {
		return 0, fmt.Errorf("client (%s) is already expired on chain: %s", srcClientId, cc.PCfg.ChainID)
	}

	srcUpdateHeader, err := cc.GetIBCUpdateHeader(ctx, srch, dst, dstClientId)
	if err != nil {
		return 0, err
	}

	dstUpdateHeader, err := dst.GetIBCUpdateHeader(ctx, dsth, cc, srcClientId)
	if err != nil {
		return 0, err
	}

	updateMsg, err := cc.UpdateClient(srcClientId, dstUpdateHeader)
	if err != nil {
		return 0, err
	}

	msgs := []provider.RelayerMessage{updateMsg}

	res, success, err := cc.SendMessages(ctx, msgs)
	if err != nil {
		// cp.LogFailedTx(res, err, CosmosMsgs(msgs...))
		return 0, err
	}
	if !success {
		return 0, fmt.Errorf("tx failed: %s", res.Data)
	}
	cc.log.Info(
		"Client updated",
		zap.String("provider_chain_id", cc.PCfg.ChainID),
		zap.String("src_client_id", srcClientId),
		zap.Uint64("prev_height", MustGetHeight(srcUpdateHeader.GetHeight()).GetRevisionHeight()),
		zap.Uint64("cur_height", srcUpdateHeader.GetHeight().GetRevisionHeight()),
	)

	return clientState.TrustingPeriod, nil
}

// FindMatchingClient will determine if there exists a client with identical client and consensus states
// to the client which would have been created. Source is the chain that would be adding a client
// which would track the counterparty. Therefore we query source for the existing clients
// and check if any match the counterparty. The counterparty must have a matching consensus state
// to the latest consensus state of a potential match. The provided client state is the client
// state that will be created if there exist no matches.
func (cc *CosmosProvider) FindMatchingClient(ctx context.Context, counterparty provider.ChainProvider, clientState ibcexported.ClientState) (string, bool) {
	// TODO: add appropriate offset and limits
	var (
		clientsResp clienttypes.IdentifiedClientStates
		err         error
	)

	if err = retry.Do(func() error {
		clientsResp, err = cc.QueryClients(ctx)
		if err != nil {
			return err
		}
		return nil
	}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
		cc.log.Info(
			"Failed to query clients",
			zap.String("chain_id", cc.PCfg.ChainID),
			zap.Uint("attempt", n+1),
			zap.Uint("max_attempts", RtyAttNum),
			zap.Error(err),
		)
	})); err != nil {
		return "", false
	}

	for _, identifiedClientState := range clientsResp {
		// unpack any into ibc tendermint client state
		existingClientState, err := castClientStateToTMType(identifiedClientState.ClientState)
		if err != nil {
			return "", false
		}

		tmClientState, ok := clientState.(*tmclient.ClientState)
		if !ok {
			cc.log.Info(
				"Failed to convert value to *tmclient.ClientState",
				zap.Stringer("client_state_type", reflect.TypeOf(clientState)),
			)
			return "", false
		}

		// check if the client states match
		// NOTE: FrozenHeight.IsZero() is a sanity check, the client to be created should always
		// have a zero frozen height and therefore should never match with a frozen client
		if isMatchingClient(*tmClientState, *existingClientState) && existingClientState.FrozenHeight.IsZero() {

			// query the latest consensus state of the potential matching client
			consensusStateResp, err := cc.QueryConsensusStateABCI(ctx, identifiedClientState.ClientId, existingClientState.GetLatestHeight())
			if err != nil {
				cc.log.Info(
					"Failed to query latest consensus state for existing client on chain",
					zap.String("chain_id", cc.PCfg.ChainID),
					zap.Error(err),
				)
				continue
			}

			//nolint:lll
			header, err := counterparty.GetLightSignedHeaderAtHeight(ctx, int64(existingClientState.GetLatestHeight().GetRevisionHeight()))
			if err != nil {
				cc.log.Info(
					"Failed to query header",
					zap.String("chain_id", counterparty.ChainId()),
					zap.Uint64("height", existingClientState.GetLatestHeight().GetRevisionHeight()),
					zap.Error(err),
				)
				continue
			}

			exportedConsState, err := clienttypes.UnpackConsensusState(consensusStateResp.ConsensusState)
			if err != nil {
				cc.log.Info(
					"Failed to unpack consensus state",
					zap.String("chain", counterparty.ChainId()),
					zap.Error(err),
				)
				continue
			}
			existingConsensusState, ok := exportedConsState.(*tmclient.ConsensusState)
			if !ok {
				cc.log.Info(
					"Cannot convert consensus state to *tmclient.ConsensusState",
					zap.String("chain_id", counterparty.ChainId()),
					zap.Stringer("consensus_state_type", reflect.TypeOf(exportedConsState)),
				)
				continue
			}

			if existingClientState.IsExpired(existingConsensusState.Timestamp, time.Now()) {
				continue
			}

			tmHeader, ok := header.(*tmclient.Header)
			if !ok {
				cc.log.Info(
					"Failed to convert value to *tmclient.Header",
					zap.Stringer("header_type", reflect.TypeOf(header)),
				)
				return "", false
			}

			if isMatchingConsensusState(existingConsensusState, tmHeader.ConsensusState()) {
				// found matching client
				return identifiedClientState.ClientId, true
			}
		}
	}
	return "", false
}

func (cc *CosmosProvider) QueryConsensusStateABCI(ctx context.Context, clientID string, height ibcexported.Height) (*clienttypes.QueryConsensusStateResponse, error) {
	key := host.FullConsensusStateKey(clientID, height)

	value, proofBz, proofHeight, err := cc.QueryTendermintProof(ctx, int64(height.GetRevisionHeight()), key)
	if err != nil {
		return nil, err
	}

	// check if consensus state exists
	if len(value) == 0 {
		return nil, sdkerrors.Wrap(clienttypes.ErrConsensusStateNotFound, clientID)
	}

	// TODO do we really want to create a new codec? ChainClient exposes proto.Marshaler
	cdc := codec.NewProtoCodec(cc.Codec.InterfaceRegistry)

	cs, err := clienttypes.UnmarshalConsensusState(cdc, value)
	if err != nil {
		return nil, err
	}

	anyConsensusState, err := clienttypes.PackConsensusState(cs)
	if err != nil {
		return nil, err
	}

	return &clienttypes.QueryConsensusStateResponse{
		ConsensusState: anyConsensusState,
		Proof:          proofBz,
		ProofHeight:    proofHeight,
	}, nil
}

// isMatchingClient determines if the two provided clients match in all fields
// except latest height. They are assumed to be IBC tendermint light clients.
// NOTE: we don't pass in a pointer so upstream references don't have a modified
// latest height set to zero.
func isMatchingClient(clientStateA, clientStateB tmclient.ClientState) bool {
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

// queryTMClientState retrieves the latest consensus state for a client in state at a given height
// and unpacks/cast it to tendermint clientstate
func (cc *CosmosProvider) queryTMClientState(ctx context.Context, srch int64, srcClientId string) (*tmclient.ClientState, error) {
	clientStateRes, err := cc.QueryClientStateResponse(ctx, srch, srcClientId)
	if err != nil {
		return &tmclient.ClientState{}, err
	}

	return castClientStateToTMType(clientStateRes.ClientState)
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

// WaitForNBlocks blocks until the next block on a given chain
func (cc *CosmosProvider) WaitForNBlocks(ctx context.Context, n int64) error {
	var initial int64
	h, err := cc.RPCClient.Status(ctx)
	if err != nil {
		return err
	}
	if h.SyncInfo.CatchingUp {
		return fmt.Errorf("chain catching up")
	}
	initial = h.SyncInfo.LatestBlockHeight
	for {
		h, err = cc.RPCClient.Status(ctx)
		if err != nil {
			return err
		}
		if h.SyncInfo.LatestBlockHeight > initial+n {
			return nil
		}
		select {
		case <-time.After(10 * time.Millisecond):
			// Nothing to do.
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// MustGetHeight takes the height inteface and returns the actual height
func MustGetHeight(h ibcexported.Height) clienttypes.Height {
	height, ok := h.(clienttypes.Height)
	if !ok {
		panic("height is not an instance of height!")
	}
	return height
}

// SendMessage attempts to sign, encode & send a RelayerMessage
// This is used extensively in the relayer as an extension of the Provider interface
func (cc *CosmosProvider) SendMessage(ctx context.Context, msg provider.RelayerMessage) (*provider.RelayerTxResponse, bool, error) {
	return cc.SendMessages(ctx, []provider.RelayerMessage{msg})
}

// SendMessages attempts to sign, encode, & send a slice of RelayerMessages
// This is used extensively in the relayer as an extension of the Provider interface
//
// NOTE: An error is returned if there was an issue sending the transaction. A successfully sent, but failed
// transaction will not return an error. If a transaction is successfully sent, the result of the execution
// of that transaction will be logged. A boolean indicating if a transaction was successfully
// sent and executed successfully is returned.
func (cc *CosmosProvider) SendMessages(ctx context.Context, msgs []provider.RelayerMessage) (*provider.RelayerTxResponse, bool, error) {
	var resp *sdk.TxResponse

	if err := retry.Do(func() error {
		txBytes, err := cc.buildMessages(ctx, msgs)
		if err != nil {
			errMsg := err.Error()

			// Occasionally the client will be out of date,
			// and we will receive an RPC error like:
			//     rpc error: code = InvalidArgument desc = failed to execute message; message index: 1: channel handshake open try failed: failed channel state verification for client (07-tendermint-0): client state height < proof height ({0 58} < {0 59}), please ensure the client has been updated: invalid height: invalid request
			// or
			//     rpc error: code = InvalidArgument desc = failed to execute message; message index: 1: receive packet verification failed: couldn't verify counterparty packet commitment: failed packet commitment verification for client (07-tendermint-0): client state height < proof height ({0 142} < {0 143}), please ensure the client has been updated: invalid height: invalid request
			//
			// No amount of retrying will fix this. The client needs to be updated.
			// Unfortunately, the entirety of that error message originates on the server,
			// so there is not an obvious way to access a more structured error value.
			//
			// If this logic should ever fail due to the string values of the error messages on the server
			// changing from the client's version of the library,
			// at worst this will run more unnecessary retries.
			if strings.Contains(errMsg, sdkerrors.ErrInvalidHeight.Error()) {
				cc.log.Info(
					"Skipping retry due to invalid height error",
					zap.Error(err),
				)
				return retry.Unrecoverable(err)
			}

			// On a fast retry, it is possible to have an invalid connection state.
			// Retrying that message also won't fix the underlying state mismatch,
			// so log it and mark it as unrecoverable.
			if strings.Contains(errMsg, conntypes.ErrInvalidConnectionState.Error()) {
				cc.log.Info(
					"Skipping retry due to invalid connection state",
					zap.Error(err),
				)
				return retry.Unrecoverable(err)
			}

			// Also possible to have an invalid channel state on a fast retry.
			if strings.Contains(errMsg, chantypes.ErrInvalidChannelState.Error()) {
				cc.log.Info(
					"Skipping retry due to invalid channel state",
					zap.Error(err),
				)
				return retry.Unrecoverable(err)
			}

			// If the message reported an invalid proof, back off.
			// NOTE: this error string ("invalid proof") will match other errors too,
			// but presumably it is safe to stop retrying in those cases as well.
			if strings.Contains(errMsg, commitmenttypes.ErrInvalidProof.Error()) {
				cc.log.Info(
					"Skipping retry due to invalid proof",
					zap.Error(err),
				)
				return retry.Unrecoverable(err)
			}

			// Invalid packets should not be retried either.
			if strings.Contains(errMsg, chantypes.ErrInvalidPacket.Error()) {
				cc.log.Info(
					"Skipping retry due to invalid packet",
					zap.Error(err),
				)
				return retry.Unrecoverable(err)
			}

			return err
		}

		resp, err = cc.BroadcastTx(ctx, txBytes)
		if err != nil {
			if err == sdkerrors.ErrWrongSequence {
				// Allow retrying if we got an invalid sequence error when attempting to broadcast this tx.
				return err
			}

			// Don't retry if BroadcastTx resulted in any other error.
			// (This was the previous behavior. Unclear if that is still desired.)
			return retry.Unrecoverable(err)
		}

		return nil
	}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
		cc.log.Info(
			"Error building or broadcasting transaction",
			zap.String("chain_id", cc.PCfg.ChainID),
			zap.Uint("attempt", n+1),
			zap.Uint("max_attempts", RtyAttNum),
			zap.Error(err),
		)
	})); err != nil || resp == nil {
		return nil, false, err
	}

	rlyResp := &provider.RelayerTxResponse{
		Height: resp.Height,
		TxHash: resp.TxHash,
		Code:   resp.Code,
		Data:   resp.Data,
		Events: parseEventsFromTxResponse(resp),
	}

	// transaction was executed, log the success or failure using the tx response code
	// NOTE: error is nil, logic should use the returned error to determine if the
	// transaction was successfully executed.
	if rlyResp.Code != 0 {
		cc.LogFailedTx(rlyResp, nil, msgs)
		return rlyResp, false, fmt.Errorf("transaction failed with code: %d", resp.Code)
	}

	cc.LogSuccessTx(resp, msgs)
	return rlyResp, true, nil
}

func parseEventsFromTxResponse(resp *sdk.TxResponse) []provider.RelayerEvent {
	var events []provider.RelayerEvent

	if resp == nil {
		return events
	}

	for _, logs := range resp.Logs {
		for _, event := range logs.Events {
			attributes := make(map[string]string)
			for _, attribute := range event.Attributes {
				attributes[attribute.Key] = attribute.Value
			}
			events = append(events, provider.RelayerEvent{
				EventType:  event.Type,
				Attributes: attributes,
			})
		}
	}
	return events
}

func (cc *CosmosProvider) buildMessages(ctx context.Context, msgs []provider.RelayerMessage) ([]byte, error) {
	// Query account details
	txf, err := cc.PrepareFactory(cc.TxFactory())
	if err != nil {
		return nil, err
	}

	// TODO: Make this work with new CalculateGas method
	// TODO: This is related to GRPC client stuff?
	// https://github.com/cosmos/cosmos-sdk/blob/5725659684fc93790a63981c653feee33ecf3225/client/tx/tx.go#L297
	// If users pass gas adjustment, then calculate gas
	_, adjusted, err := cc.CalculateGas(ctx, txf, CosmosMsgs(msgs...)...)
	if err != nil {
		return nil, err
	}

	// Set the gas amount on the transaction factory
	txf = txf.WithGas(adjusted)

	var txb client.TxBuilder
	// Build the transaction builder & retry on failures
	if err := retry.Do(func() error {
		txb, err = tx.BuildUnsignedTx(txf, CosmosMsgs(msgs...)...)
		if err != nil {
			return err
		}
		return nil
	}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr); err != nil {
		return nil, err
	}

	// Attach the signature to the transaction
	// Force encoding in the chain specific address
	for _, msg := range msgs {
		cc.Codec.Marshaler.MustMarshalJSON(CosmosMsg(msg))
	}

	done := cc.SetSDKContext()

	if err := retry.Do(func() error {
		if err := tx.Sign(txf, cc.PCfg.Key, txb, false); err != nil {
			return err
		}
		return nil
	}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr); err != nil {
		return nil, err
	}

	done()

	var txBytes []byte
	// Generate the transaction bytes
	if err := retry.Do(func() error {
		var err error
		txBytes, err = cc.Codec.TxConfig.TxEncoder()(txb.GetTx())
		if err != nil {
			return err
		}
		return nil
	}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr); err != nil {
		return nil, err
	}

	return txBytes, nil
}

func (cc *CosmosProvider) BlockTime(ctx context.Context, height int64) (int64, error) {
	resultBlock, err := cc.RPCClient.Block(ctx, &height)
	if err != nil {
		return 0, err
	}
	return resultBlock.Block.Time.UnixNano(), nil
}
