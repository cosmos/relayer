package cosmos

//import (
//	abci "github.com/cometbft/cometbft/abci/types"
//	"github.com/cosmos/relayer/v2/relayer/chains"
//)

//func (ccp *CosmosChainProcessor) ibcMessagesFromBlockEvents(
//	events []abci.Event,
//	height uint64, base64Encoded bool,
//) (res []chains.IbcMessage) {
//	chainID := ccp.chainProvider.ChainId()
//	res = append(res, chains.IbcMessagesFromEvents(ccp.log, events, chainID, height, base64Encoded)...)
//	return res
//}
