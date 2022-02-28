package substrate

import (
	"bytes"
	"fmt"
	"io"
	"time"

	"github.com/ComposableFi/go-substrate-rpc-client/scale"
	rpcClient "github.com/ComposableFi/go-substrate-rpc-client/v4"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	chantypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	"github.com/cosmos/relayer/relayer/provider"
	"github.com/cosmos/relayer/relayer/provider/substrate/keystore"
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

// (ccc *ChainClientConfig, homepath string, input io.Reader, output io.Writer, kro ...keyring.Option) (*ChainClient, error) {
func NewSubstrateProvider(spc *SubstrateProviderConfig, homepath string) (*SubstrateProvider, error) {
	sp := &SubstrateProvider{
		Config: spc,
	}
	err := sp.Init()
	if err != nil {
		return nil, err
	}

	return sp, nil
}

type SubstrateProviderConfig struct {
	Timeout        string `json:"timeout" yaml:"timeout"`
	RPCAddr        string `json:"rpc-addr" yaml:"rpc-addr"`
	ChainID        string `json:"chain-id" yaml:"chain-id"`
	Key            string `json:"key" yaml:"key"`
	KeyringBackend string `json:"keyring-backend" yaml:"keyring-backend"`
	KeyDirectory   string `json:"key-directory" yaml:"key-directory"`
}

func (spc *SubstrateProviderConfig) NewProvider(homepath string, debug bool) (provider.ChainProvider, error) {
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
func (srp *SubstrateRelayPacket) FetchCommitResponse(dst provider.ChainProvider, queryHeight uint64, dstChanId, dstPortId string) error {
	dstRecvRes, err := dst.QueryPacketReceipt(int64(queryHeight)-1, dstChanId, dstPortId, srp.seq)
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
