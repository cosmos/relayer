package types

import (
	exported "github.com/cosmos/ibc-go/v7/modules/core/exported"
)

var (
	_ exported.ClientMessage = &Misbehaviour{}
)

func (m Misbehaviour) ClientType() string {
	return ""
}

func (m Misbehaviour) ValidateBasic() error {
	return nil
}
