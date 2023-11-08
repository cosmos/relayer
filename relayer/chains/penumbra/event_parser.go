package penumbra

import (
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/relayer/v2/relayer/chains"
)

func (ccp *PenumbraChainProcessor) ibcMessagesFromBlockEvents(
	beginBlockEvents, endBlockEvents []abci.Event,
	height uint64, base64Encoded bool,
) (res []chains.IbcMessage) {
	chainID := ccp.chainProvider.ChainId()
	res = append(res, chains.IbcMessagesFromEvents(ccp.log, beginBlockEvents, chainID, height, base64Encoded)...)
	res = append(res, chains.IbcMessagesFromEvents(ccp.log, endBlockEvents, chainID, height, base64Encoded)...)
	return res
}
