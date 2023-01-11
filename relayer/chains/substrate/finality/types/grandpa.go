package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	clienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
	types1 "github.com/cosmos/ibc-go/v5/modules/core/23-commitment/types"
	ibcexported "github.com/cosmos/ibc-go/v5/modules/core/exported"
)

var _ ibcexported.ConsensusState = (*ConsensusState)(nil)
var _ ibcexported.ClientState = (*ClientState)(nil)
var _ ibcexported.Header = (*Header)(nil)

const ClientType = "ics10-grandpa"

// ClientType returns Beefy
func (ConsensusState) ClientType() string {
	return ClientType
}

// GetRoot returns the commitment Root for the specific
func (cs ConsensusState) GetRoot() ibcexported.Root {
	return types1.MerkleRoot{Hash: cs.Root}
}

// GetTimestamp returns block time in nanoseconds of the header that created consensus state
func (cs ConsensusState) GetTimestamp() uint64 {
	return uint64(cs.Timestamp.GetNanos())
}

// ValidateBasic defines a basic validation for the beefy consensus state.
func (cs ConsensusState) ValidateBasic() error {
	if len(cs.Root) == 0 {
		return sdkerrors.Wrap(clienttypes.ErrInvalidConsensus, "root cannot be empty")
	}

	if cs.GetTimestamp() <= 0 {
		return sdkerrors.Wrap(clienttypes.ErrInvalidConsensus, "timestamp must be a positive Unix time")
	}
	return nil
}

func (m *ClientState) ClientType() string {
	return ClientType
}

func (m *ClientState) GetLatestHeight() ibcexported.Height {
	//TODO implement me
	panic("implement me return latest height")
}

func (m *ClientState) Validate() error {
	//TODO implement me
	panic("implement me validate client state")
}

func (m *ClientState) Initialize(context sdk.Context, codec codec.BinaryCodec, store sdk.KVStore, state ibcexported.ConsensusState) error {
	//TODO implement me
	panic("implement me initialize client state")
}

func (m *ClientState) Status(ctx sdk.Context, clientStore sdk.KVStore, cdc codec.BinaryCodec) ibcexported.Status {
	//TODO implement me
	panic("implement me status")
}

func (m *ClientState) ExportMetadata(store sdk.KVStore) []ibcexported.GenesisMetadata {
	//TODO implement me
	panic("implement me export metadata")
}

func (m *ClientState) CheckHeaderAndUpdateState(context sdk.Context, codec codec.BinaryCodec, store sdk.KVStore, header ibcexported.Header) (ibcexported.ClientState, ibcexported.ConsensusState, error) {
	//TODO implement me
	panic("implement me check header and update state")
}

func (m *ClientState) CheckMisbehaviourAndUpdateState(context sdk.Context, codec codec.BinaryCodec, store sdk.KVStore, misbehaviour ibcexported.Misbehaviour) (ibcexported.ClientState, error) {
	//TODO implement me
	panic("implement me check misbehaviour and update state")
}

func (m *ClientState) CheckSubstituteAndUpdateState(ctx sdk.Context, cdc codec.BinaryCodec, subjectClientStore, substituteClientStore sdk.KVStore, substituteClient ibcexported.ClientState) (ibcexported.ClientState, error) {
	//TODO implement me
	panic("implement me check substitute and update state")
}

func (m *ClientState) VerifyUpgradeAndUpdateState(ctx sdk.Context, cdc codec.BinaryCodec, store sdk.KVStore, newClient ibcexported.ClientState, newConsState ibcexported.ConsensusState, proofUpgradeClient, proofUpgradeConsState []byte) (ibcexported.ClientState, ibcexported.ConsensusState, error) {
	//TODO implement me
	panic("implement me verify upgrade and update state")
}

func (m *ClientState) ZeroCustomFields() ibcexported.ClientState {
	//TODO implement me
	panic("implement me zero custom fields")
}

func (m *ClientState) VerifyClientState(store sdk.KVStore, cdc codec.BinaryCodec, height ibcexported.Height, prefix ibcexported.Prefix, counterpartyClientIdentifier string, proof []byte, clientState ibcexported.ClientState) error {
	//TODO implement me
	panic("implement me verify client state")
}

func (m *ClientState) VerifyClientConsensusState(store sdk.KVStore, cdc codec.BinaryCodec, height ibcexported.Height, counterpartyClientIdentifier string, consensusHeight ibcexported.Height, prefix ibcexported.Prefix, proof []byte, consensusState ibcexported.ConsensusState) error {
	//TODO implement me
	panic("implement me verify client consensus state")
}

func (m *ClientState) VerifyConnectionState(store sdk.KVStore, cdc codec.BinaryCodec, height ibcexported.Height, prefix ibcexported.Prefix, proof []byte, connectionID string, connectionEnd ibcexported.ConnectionI) error {
	//TODO implement me
	panic("implement me verify connection state")
}

func (m *ClientState) VerifyChannelState(store sdk.KVStore, cdc codec.BinaryCodec, height ibcexported.Height, prefix ibcexported.Prefix, proof []byte, portID, channelID string, channel ibcexported.ChannelI) error {
	//TODO implement me
	panic("implement me verify channel state")
}

func (m *ClientState) VerifyPacketCommitment(ctx sdk.Context, store sdk.KVStore, cdc codec.BinaryCodec, height ibcexported.Height, delayTimePeriod uint64, delayBlockPeriod uint64, prefix ibcexported.Prefix, proof []byte, portID, channelID string, sequence uint64, commitmentBytes []byte) error {
	//TODO implement me
	panic("implement me verify packet commitment")
}

func (m *ClientState) VerifyPacketAcknowledgement(ctx sdk.Context, store sdk.KVStore, cdc codec.BinaryCodec, height ibcexported.Height, delayTimePeriod uint64, delayBlockPeriod uint64, prefix ibcexported.Prefix, proof []byte, portID, channelID string, sequence uint64, acknowledgement []byte) error {
	//TODO implement me
	panic("implement me verify packet acknowledgement")
}

func (m *ClientState) VerifyPacketReceiptAbsence(ctx sdk.Context, store sdk.KVStore, cdc codec.BinaryCodec, height ibcexported.Height, delayTimePeriod uint64, delayBlockPeriod uint64, prefix ibcexported.Prefix, proof []byte, portID, channelID string, sequence uint64) error {
	//TODO implement me
	panic("implement me verify packet receipt absence")
}

func (m *ClientState) VerifyNextSequenceRecv(ctx sdk.Context, store sdk.KVStore, cdc codec.BinaryCodec, height ibcexported.Height, delayTimePeriod uint64, delayBlockPeriod uint64, prefix ibcexported.Prefix, proof []byte, portID, channelID string, nextSequenceRecv uint64) error {
	//TODO implement me
	panic("implement me verify next sequence recv")
}

func (m *Header) ClientType() string {
	return ClientType
}

func (m *Header) GetHeight() ibcexported.Height {
	//TODO implement me
	panic("implement me GetHeight")
}

func (m *Header) ValidateBasic() error {
	//TODO implement me
	panic("implement me ValidateBasic")
}
