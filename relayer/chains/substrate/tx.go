package substrate

import (
	"context"
	"fmt"
	"time"

	"github.com/ChainSafe/gossamer/lib/common"

	rpcclienttypes "github.com/ComposableFi/go-substrate-rpc-client/v4/types"

	clienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
	ibcexported "github.com/cosmos/ibc-go/v5/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/gogo/protobuf/proto"
)

func (sp *SubstrateProvider) NewClientState(
	dstChainID string,
	dstIBCHeader provider.IBCHeader,
	dstTrustingPeriod,
	dstUbdPeriod time.Duration,
	allowUpdateAfterExpiry,
	allowUpdateAfterMisbehaviour bool,
) (ibcexported.ClientState, error) {
	substrateHeader, ok := dstIBCHeader.(SubstrateIBCHeader)
	if !ok {
		return nil, fmt.Errorf("got data of type %T but wanted  substrate.SubstrateIBCHeader \n", dstIBCHeader)
	}

	// TODO: this won't work because we need the height passed to GetBlockHash to be the previously finalized beefy height
	// from the relayer. However, the height from substrate.Height() is the height of the first parachain from the beefy header.
	blockHash, err := sp.RelayerRPCClient.RPC.Chain.GetBlockHash(substrateHeader.Height())
	if err != nil {
		return nil, err
	}

	commitment, err := signedCommitment(sp.RelayerRPCClient, blockHash)
	if err != nil {
		return nil, err
	}

	cs, err := sp.clientState(sp.RelayerRPCClient, commitment)
	if err != nil {
		return nil, err
	}

	return cs, nil
}

func (sp *SubstrateProvider) SendMessage(ctx context.Context, msg provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {
	return sp.SendMessages(ctx, []provider.RelayerMessage{msg}, memo)
}

type Any struct {
	TypeUrl []byte `json:"type_url,omitempty"`
	Value   []byte `json:"value,omitempty"`
}

func (sp *SubstrateProvider) buildCallParams(msgs []provider.RelayerMessage) (call string, anyMsgs []Any, err error) {
	var msgTypeCall = func(msgType string) string {
		switch msgType {
		case "/" + proto.MessageName(&clienttypes.MsgCreateClient{}):
			return "Ibc.deliver_permissioned"
		default:
			return "Ibc.deliver"
		}
	}

	call = msgTypeCall(msgs[0].Type())
	for i := 0; i < len(msgs); i++ {
		if call != msgTypeCall(msgs[i].Type()) {
			return "", nil,
				fmt.Errorf("permissioned and non permissioned calls can't be mixed")
		}

		msgBytes, err := msgs[i].MsgBytes()
		if err != nil {
			return "", nil, err
		}

		anyMsgs = append(anyMsgs, Any{
			TypeUrl: []byte(msgs[i].Type()),
			Value:   msgBytes,
		})
	}

	return
}

func (sp *SubstrateProvider) fetchAndBuildEvents(
	ctx context.Context,
	blockHash rpcclienttypes.Hash,
	extHash rpcclienttypes.Hash,
) (relayerEvents []provider.RelayerEvent, err error) {
	// TODO: this should query the substrate block for ibc events and return
	// the RelayerEvent type
	client, err := sp.RPCClient.RPC.IBC.QueryNewlyCreatedClient(ctx, blockHash, extHash)
	if err != nil {
		return nil, err
	}

	attributes := make(map[string]string)
	attributes[clienttypes.AttributeKeyClientID] = client.ClientId
	return []provider.RelayerEvent{
		{
			EventType:  clienttypes.EventTypeCreateClient,
			Attributes: attributes,
		},
	}, nil
}

func (sp *SubstrateProvider) SendMessages(ctx context.Context, msgs []provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {
	meta, err := sp.RPCClient.RPC.State.GetMetadataLatest()
	if err != nil {
		return nil, false, err
	}

	call, anyMsgs, err := sp.buildCallParams(msgs)
	if err != nil {
		return nil, false, err
	}

	c, err := rpcclienttypes.NewCall(meta, call, anyMsgs)
	if err != nil {
		return nil, false, err
	}

	sc, err := rpcclienttypes.NewCall(meta, "Sudo.sudo", c)
	if err != nil {
		return nil, false, err
	}

	// Create the extrinsic
	ext := rpcclienttypes.NewExtrinsic(sc)

	genesisHash, err := sp.RPCClient.RPC.Chain.GetBlockHash(0)
	if err != nil {
		return nil, false, err
	}

	rv, err := sp.RPCClient.RPC.State.GetRuntimeVersionLatest()
	if err != nil {
		return nil, false, err
	}

	info, err := sp.Keybase.Key(sp.Key())
	if err != nil {
		return nil, false, err
	}

	key, err := rpcclienttypes.CreateStorageKey(meta, "System", "Account", info.GetPublicKey(), nil)
	if err != nil {
		return nil, false, err
	}

	var accountInfo rpcclienttypes.AccountInfo
	ok, err := sp.RPCClient.RPC.State.GetStorageLatest(key, &accountInfo)
	if err != nil || !ok {
		return nil, false, err
	}

	nonce := uint32(accountInfo.Nonce)

	o := rpcclienttypes.SignatureOptions{
		BlockHash:          genesisHash,
		Era:                rpcclienttypes.ExtrinsicEra{IsMortalEra: false},
		GenesisHash:        genesisHash,
		Nonce:              rpcclienttypes.NewUCompactFromUInt(uint64(nonce)),
		SpecVersion:        rv.SpecVersion,
		Tip:                rpcclienttypes.NewUCompactFromUInt(0),
		TransactionVersion: rv.TransactionVersion,
	}

	err = ext.Sign(info.GetKeyringPair(), o)
	if err != nil {
		return nil, false, err
	}

	// Send the extrinsic
	sub, err := sp.RPCClient.RPC.Author.SubmitAndWatchExtrinsic(ext)
	if err != nil {
		fmt.Printf("Extrinsic Error: %v \n", err.Error())
		return nil, false, err
	}

	var status rpcclienttypes.ExtrinsicStatus
	defer sub.Unsubscribe()
	for {
		status = <-sub.Chan()
		// TODO: add zap log for waiting on transaction
		if status.IsInBlock {
			break
		}
	}

	encodedExt, err := rpcclienttypes.Encode(ext)
	if err != nil {
		return nil, false, err
	}

	var extHash [32]byte
	extHash, err = common.Blake2bHash(encodedExt)
	if err != nil {
		return nil, false, err
	}

	events, err := sp.fetchAndBuildEvents(ctx, status.AsInBlock, extHash)
	if err != nil {
		return nil, false, err
	}

	rlyRes := &provider.RelayerTxResponse{
		// TODO: pass in a proper block height
		TxHash: fmt.Sprintf("0x%x", extHash[:]),
		Events: events,
	}

	return rlyRes, true, nil
}
