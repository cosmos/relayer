package types

import (
	"github.com/cosmos/ibc-go/v7/modules/core/exported"
)

var _ exported.ConsensusState = (*ConsensusState)(nil)

func (m ConsensusState) ClientType() string {
	return ""
}

func (m ConsensusState) GetTimestamp() uint64 {
	return m.Timestamp
}

func (m ConsensusState) ValidateBasic() error {
	return nil
}
