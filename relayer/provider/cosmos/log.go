package cosmos

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/gogo/protobuf/proto"
)

// Print fmt.Printlns the json or yaml representation of whatever is passed in
// CONTRACT: The cmd calling this function needs to have the "json" and "indent" flags set
// TODO: better "text" printing here would be a nice to have
// TODO: fix indenting all over the code base
func (cp *CosmosProvider) Print(toPrint proto.Message, text, indent bool) error {
	var (
		out []byte
		err error
	)

	switch {
	case indent && text:
		return fmt.Errorf("must pass either indent or text, not both")
	case text:
		// TODO: This isn't really a good option,
		out = []byte(fmt.Sprintf("%v", toPrint))
	default:
		out, err = cp.Encoding.Marshaler.MarshalJSON(toPrint)
	}

	if err != nil {
		return err
	}

	fmt.Println(string(out))
	return nil
}

// Log takes a string and logs the data
func (cp *CosmosProvider) Log(s string) {
	cp.logger.Info(s)
}

// LogFailedTx takes the transaction and the messages to create it and logs the appropriate data
func (cp *CosmosProvider) LogFailedTx(res *sdk.TxResponse, err error, msgs []sdk.Msg) {
	if cp.debug {
		cp.Log(fmt.Sprintf("- [%s] -> failed sending transaction:", cp.Config.ChainID))
		for _, msg := range msgs {
			cp.Print(msg, false, false)
		}
	}

	if err != nil {
		cp.logger.Error(fmt.Errorf("- [%s] -> err(%v)", cp.Config.ChainID, err).Error())
		if res == nil {
			return
		}
	}

	if res.Code != 0 && res.Codespace != "" {
		cp.logger.Info(fmt.Sprintf("✘ [%s]@{%d} - msg(%s) err(%s:%d:%s)",
			cp.Config.ChainID, res.Height, getMsgAction(msgs), res.Codespace, res.Code, res.RawLog))
	}

	if cp.debug && !res.Empty() {
		cp.Log("- transaction response:")
		cp.Print(res, false, false)
	}
}

// LogSuccessTx take the transaction and the messages to create it and logs the appropriate data
func (cp *CosmosProvider) LogSuccessTx(res *sdk.TxResponse, msgs []sdk.Msg) {
	cp.logger.Info(fmt.Sprintf("✔ [%s]@{%d} - msg(%s) hash(%s)", cp.Config.ChainID, res.Height, getMsgAction(msgs), res.TxHash))
}
