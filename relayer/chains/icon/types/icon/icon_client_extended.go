package icon

import (
	"time"

	ics23 "github.com/confio/ics23/go"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	"github.com/cosmos/ibc-go/v7/modules/core/exported"
	tmclient "github.com/cosmos/ibc-go/v7/modules/light-clients/07-tendermint"
)

var _ exported.ClientState = (*ClientState)(nil)

func NewClientState(
	TrustingPeriod uint64,
	FrozenHeight uint64,
	MaxClockDrift uint64,
	LatestHeight uint64,
	NetworkSectionHash []byte,
	Validators [][]byte,
) *ClientState {
	return &ClientState{
		TrustingPeriod,
		FrozenHeight,
		MaxClockDrift,
		LatestHeight,
		NetworkSectionHash,
		Validators,
	}
}

// GetChainID returns the chain-id
func (cs ClientState) GetChainID() string {
	return "icon"
}

// ClientType is tendermint.
func (cs ClientState) ClientType() string {
	return "07-icon"
}

func (cs ClientState) GetLatestHeight() exported.Height {
	return types.Height{
		RevisionHeight: uint64(cs.LatestHeight),
	}
}

// GetTimestampAtHeight returns the timestamp in nanoseconds of the consensus state at the given height.
func (cs ClientState) GetTimestampAtHeight(
	ctx sdk.Context,
	clientStore sdk.KVStore,
	cdc codec.BinaryCodec,
	height exported.Height,
) (uint64, error) {
	panic("Icon Light Client: Do not use")
}

func (cs ClientState) Status(
	ctx sdk.Context,
	clientStore sdk.KVStore,
	cdc codec.BinaryCodec,
) exported.Status {
	panic("Icon Light Client: Do not use")

}

func (cs ClientState) IsExpired(latestTimestamp, now time.Time) bool {
	panic("Icon Light Client: Do not use")
}

func (cs ClientState) Validate() error {
	panic("Icon Light Client: Do not use")

}

func (cs ClientState) GetProofSpecs() []*ics23.ProofSpec {
	panic("Icon Light Client: Do not use")
}

func (cs ClientState) ZeroCustomFields() exported.ClientState {
	panic("Icon Light Client: Do not use")

}

func (cs ClientState) Initialize(ctx sdk.Context, cdc codec.BinaryCodec, clientStore sdk.KVStore, consState exported.ConsensusState) error {
	panic("Icon Light Client: Do not use")

}

func (cs ClientState) VerifyMembership(
	ctx sdk.Context,
	clientStore sdk.KVStore,
	cdc codec.BinaryCodec,
	height exported.Height,
	delayTimePeriod uint64,
	delayBlockPeriod uint64,
	proof []byte,
	path exported.Path,
	value []byte,
) error {
	panic("Icon Light Client: Do not use")
}

func (cs ClientState) VerifyNonMembership(
	ctx sdk.Context,
	clientStore sdk.KVStore,
	cdc codec.BinaryCodec,
	height exported.Height,
	delayTimePeriod uint64,
	delayBlockPeriod uint64,
	proof []byte,
	path exported.Path,
) error {
	panic("Icon Light Client: Do not use")
}

func (cs *ClientState) verifyMisbehaviour(ctx sdk.Context, clientStore sdk.KVStore, cdc codec.BinaryCodec, misbehaviour *tmclient.Misbehaviour) error {
	panic("Icon Light Client: Do not use")
}

func checkMisbehaviourHeader(
	clientState *ClientState, consState *ConsensusState, header *tmclient.Header, currentTimestamp time.Time,
) error {
	panic("Icon Light Client: Do not use")
}

func (cs ClientState) ExportMetadata(store sdk.KVStore) []exported.GenesisMetadata {
	panic("Icon Light Client: Do not use")
}

func (cs ClientState) VerifyClientMessage(ctx sdk.Context, cdc codec.BinaryCodec, clientStore sdk.KVStore, clientMsg exported.ClientMessage) error {
	panic("Icon Light Client: Do not use")
}

func (cs ClientState) CheckForMisbehaviour(ctx sdk.Context, cdc codec.BinaryCodec, clientStore sdk.KVStore, clientMsg exported.ClientMessage) bool {
	panic("Icon Light Client: Do not use")
}

func (cs ClientState) UpdateStateOnMisbehaviour(ctx sdk.Context, cdc codec.BinaryCodec, clientStore sdk.KVStore, clientMsg exported.ClientMessage) {
	panic("Icon Light Client: Do not use")
}
func (cs ClientState) UpdateState(ctx sdk.Context, cdc codec.BinaryCodec, clientStore sdk.KVStore, clientMsg exported.ClientMessage) []exported.Height {
	panic("Icon Light Client: Do not use")
}

func (cs ClientState) CheckSubstituteAndUpdateState(ctx sdk.Context, cdc codec.BinaryCodec, subjectClientStore, substituteClientStore sdk.KVStore, substituteClient exported.ClientState) error {
	panic("Icon Light Client: Do not use")
}

func (cs ClientState) VerifyUpgradeAndUpdateState(
	ctx sdk.Context,
	cdc codec.BinaryCodec,
	store sdk.KVStore,
	newClient exported.ClientState,
	newConsState exported.ConsensusState,
	proofUpgradeClient,
	proofUpgradeConsState []byte,
) error {

	panic("Icon Light Client: Do not use")
}
