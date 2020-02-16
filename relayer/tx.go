package relayer

import (
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clientExported "github.com/cosmos/cosmos-sdk/x/ibc/02-client/exported"
	clientTypes "github.com/cosmos/cosmos-sdk/x/ibc/02-client/types"
	connState "github.com/cosmos/cosmos-sdk/x/ibc/03-connection/exported"
	connTypes "github.com/cosmos/cosmos-sdk/x/ibc/03-connection/types"
	chanState "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/exported"
	chanTypes "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/types"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint"
	commitment "github.com/cosmos/cosmos-sdk/x/ibc/23-commitment"
)

var (
	defaultChainPrefix = commitment.NewPrefix([]byte("ibc"))
	defaultIBCVersion  = "1.0.0"
	defaultIBCVersions = []string{defaultIBCVersion}
)

// CreateConnection creates a connection between two chains given src and dst client IDs
func (src *Chain) CreateConnection(dst *Chain, srcClientID, dstClientID, srcConnectionID, dstConnectionID string, timeout time.Duration) error {
	ticker := time.NewTicker(timeout)
	for ; true; <-ticker.C {
		msgs, err := src.CreateConnectionStep(dst, srcClientID, dstClientID, srcConnectionID, dstConnectionID)
		if err != nil {
			return err
		}

		if len(msgs.Dst) == 0 && len(msgs.Src) == 0 {
			break
		}

		// Submit the transactions to src chain
		srcRes, err := src.SendMsgs(msgs.Src)
		if err != nil {
			return err
		}
		src.logger.Info(srcRes.String())

		// Submit the transactions to dst chain
		dstRes, err := dst.SendMsgs(msgs.Dst)
		if err != nil {
			return err
		}
		src.logger.Info(dstRes.String())
	}

	return nil
}

// CreateConnectionStep returns the next set of messags for creating a channel
// with the given identifier between chains src and dst. If handshake hasn't started,
// CreateConnetionStep will start the handshake on src
func (src *Chain) CreateConnectionStep(dst *Chain,
	srcClientID, dstClientID,
	srcConnectionID, dstConnectionID string) (*RelayMsgs, error) {
	out := &RelayMsgs{Src: []sdk.Msg{}, Dst: []sdk.Msg{}}

	err := UpdateLiteDBsToLatestHeaders(src, dst)
	if err != nil {
		return nil, err
	}

	hs, err := GetLatestHeaders(src, dst)
	if err != nil {
		return nil, err
	}

	srcEnd, err := src.QueryConnection(srcConnectionID, hs[src.ChainID].Height)
	if err != nil {
		return nil, err
	}

	dstEnd, err := dst.QueryConnection(dstConnectionID, hs[src.ChainID].Height)
	if err != nil {
		return nil, err
	}

	switch {
	// Handshake hasn't been started on src or dst, relay `connOpenInit` to src
	case srcEnd.Connection.State == connState.UNINITIALIZED && dstEnd.Connection.State == connState.UNINITIALIZED:
		out.Src = append(out.Src, src.ConnInit(srcConnectionID, srcClientID, dstConnectionID, dstClientID))

	// Handshake has started on dst (1 stepdone), relay `connOpenTry` and `updateClient` on src
	case srcEnd.Connection.State == connState.UNINITIALIZED && dstEnd.Connection.State == connState.INIT:
		out.Src = append(out.Src, src.UpdateClient(srcClientID, hs[dst.ChainID]),
			src.ConnTry(srcClientID, dstClientID, srcConnectionID, dstConnectionID, dstEnd, hs[dst.ChainID].Height))

	// Handshake has started on src (1 step done), relay `connOpenTry` and `updateClient` on dst
	case srcEnd.Connection.State == connState.INIT && dstEnd.Connection.State == connState.UNINITIALIZED:
		out.Dst = append(out.Dst, dst.UpdateClient(dstClientID, hs[src.ChainID]),
			dst.ConnTry(dstClientID, srcClientID, dstConnectionID, srcConnectionID, srcEnd, hs[src.ChainID].Height))

	// Handshake has started on src end (2 steps done), relay `connOpenAck` and `updateClient` to dst end
	case srcEnd.Connection.State == connState.TRYOPEN && dstEnd.Connection.State == connState.INIT:
		out.Dst = append(out.Dst, dst.UpdateClient(dstClientID, hs[src.ChainID]),
			dst.ConnAck(dstConnectionID, srcEnd, hs[dst.ChainID].Height))

	// Handshake has started on dst end (2 steps done), relay `connOpenAck` and `updateClient` to src end
	case srcEnd.Connection.State == connState.INIT && dstEnd.Connection.State == connState.TRYOPEN:
		out.Src = append(out.Src, src.UpdateClient(srcClientID, hs[dst.ChainID]),
			src.ConnAck(srcConnectionID, dstEnd, hs[src.ChainID].Height))

	// Handshake has confirmed on dst (3 steps done), relay `connOpenConfirm` and `updateClient` to src end
	case srcEnd.Connection.State == connState.TRYOPEN && dstEnd.Connection.State == connState.OPEN:
		out.Src = append(out.Src, src.UpdateClient(srcClientID, hs[dst.ChainID]),
			src.ConnConfirm(srcConnectionID, dstEnd, hs[src.ChainID].Height))

	// Handshake has confirmed on src (3 steps done), relay `connOpenConfirm` and `updateClient` to dst end
	case srcEnd.Connection.State == connState.OPEN && dstEnd.Connection.State == connState.TRYOPEN:
		out.Dst = append(out.Dst, dst.UpdateClient(dstClientID, hs[src.ChainID]),
			dst.ConnConfirm(dstConnectionID, srcEnd, hs[dst.ChainID].Height))
	}

	return out, nil
}

// CreateChannel creates a connection between two chains given src and dst client IDs
func (src *Chain) CreateChannel(dst *Chain, srcClientID, dstClientID, srcConnectionID, dstConnectionID,
	srcChannelID, dstChannelID, srcPortID, dstPortID string, timeout time.Duration, ordering chanState.Order) error {
	ticker := time.NewTicker(timeout)
	for ; true; <-ticker.C {
		msgs, err := src.CreateChannelStep(dst, srcClientID, dstClientID, srcConnectionID, dstConnectionID,
			srcChannelID, dstChannelID, srcPortID, dstPortID, ordering)
		if err != nil {
			return err
		}

		if len(msgs.Dst) == 0 && len(msgs.Src) == 0 {
			break
		}

		// Submit the transactions to src chain
		srcRes, err := src.SendMsgs(msgs.Src)
		if err != nil {
			return err
		}
		src.logger.Info(srcRes.String())

		// Submit the transactions to dst chain
		dstRes, err := dst.SendMsgs(msgs.Dst)
		if err != nil {
			return err
		}
		src.logger.Info(dstRes.String())
	}

	return nil
}

// CreateChannelStep returns the next set of messages for creating a channel with given
// identifiers between chains src and dst. If the handshake hasn't started, then CreateChannelStep
// will begin the handshake on the src chain
func (src *Chain) CreateChannelStep(dst *Chain,
	srcClientID, dstClientID,
	srcConnectionID, dstConnectionID,
	srcChannelID, dstChannelID,
	srcPortID, dstPortID string, ordering chanState.Order) (*RelayMsgs, error) {
	out := &RelayMsgs{Src: []sdk.Msg{}, Dst: []sdk.Msg{}}

	err := UpdateLiteDBsToLatestHeaders(src, dst)
	if err != nil {
		return nil, err
	}

	hs, err := GetLatestHeaders(src, dst)
	if err != nil {
		return nil, err
	}

	dstEnd, err := dst.QueryChannel(dstConnectionID, dstPortID, hs[dst.ChainID].Height)
	if err != nil {
		return nil, err
	}

	srcEnd, err := src.QueryChannel(srcConnectionID, srcPortID, hs[src.ChainID].Height)
	if err != nil {
		return nil, err
	}

	switch {
	// Handshake hasn't been started on src or dst, relay `chanOpenInit` to src
	case srcEnd.Channel.State == chanState.UNINITIALIZED && dstEnd.Channel.State == chanState.UNINITIALIZED:
		out.Src = append(out.Src, src.ChanInit(srcConnectionID, srcChannelID, dstChannelID, srcPortID, dstPortID, ordering))

	// Handshake has started on dst (1 step done), relay `chanOpenTry` and `updateClient` to src
	case srcEnd.Channel.State == chanState.UNINITIALIZED && dstEnd.Channel.State == chanState.INIT:
		out.Src = append(out.Src, src.UpdateClient(srcClientID, hs[dst.ChainID]),
			src.ChanTry(srcChannelID, dstChannelID, srcPortID, dstPortID, dstEnd))

	// Handshake has started on src (1 step done), relay `chanOpenTry` and `updateClient` to dst
	case srcEnd.Channel.State == chanState.INIT && dstEnd.Channel.State == chanState.UNINITIALIZED:
		out.Dst = append(out.Dst, dst.UpdateClient(dstClientID, hs[src.ChainID]),
			dst.ChanTry(dstChannelID, srcChannelID, dstPortID, dstPortID, dstEnd))

	// Handshake has started on src (2 steps done), relay `chanOpenAck` and `updateClient` to dst
	case srcEnd.Channel.State == chanState.TRYOPEN && dstEnd.Channel.State == chanState.INIT:
		out.Dst = append(out.Dst, dst.UpdateClient(dstClientID, hs[src.ChainID]),
			dst.ChanAck(dstChannelID, dstPortID, srcEnd))

	// Handshake has started on dst (2 steps done), relay `chanOpenAck` and `updateClient` to src
	case srcEnd.Channel.State == chanState.INIT && dstEnd.Channel.State == chanState.TRYOPEN:
		out.Src = append(out.Src, src.UpdateClient(srcClientID, hs[dst.ChainID]),
			src.ChanAck(srcChannelID, srcPortID, dstEnd))

	// Handshake has confirmed on dst (3 steps done), relay `chanOpenConfirm` and `updateClient` to src
	case srcEnd.Channel.State == chanState.TRYOPEN && dstEnd.Channel.State == chanState.OPEN:
		out.Src = append(out.Src, src.UpdateClient(srcClientID, hs[dst.ChainID]),
			src.ChanConfirm(srcChannelID, srcPortID, dstEnd))

	// Handshake has confirmed on src (3 steps done), relay `chanOpenConfirm` and `updateClient` to dst
	case srcEnd.Channel.State == chanState.OPEN && dstEnd.Channel.State == chanState.TRYOPEN:
		out.Dst = append(out.Dst, dst.UpdateClient(dstClientID, hs[src.ChainID]),
			dst.ChanConfirm(dstChannelID, dstPortID, srcEnd))
	}

	return out, nil
}

// UpdateClient creates an sdk.Msg to update the client on c with data pulled from cp
func (c *Chain) UpdateClient(srcClientID string, dstHeader *tmclient.Header) clientTypes.MsgUpdateClient {
	return clientTypes.NewMsgUpdateClient(srcClientID, dstHeader, c.MustGetAddress())
}

// CreateClient creates an sdk.Msg to update the client on src with consensus state from dst
func (c *Chain) CreateClient(srcClientID string, dstHeader *tmclient.Header) clientTypes.MsgCreateClient {
	return clientTypes.NewMsgCreateClient(srcClientID, clientExported.ClientTypeTendermint, dstHeader.ConsensusState(), c.MustGetAddress())
}

// ConnInit creates a MsgConnectionOpenInit
func (c *Chain) ConnInit(srcConnID, srcClientID, dstConnId, dstClientID string) sdk.Msg {
	return connTypes.NewMsgConnectionOpenInit(srcConnID, srcClientID, dstConnId, dstClientID, defaultChainPrefix, c.MustGetAddress())
}

// ConnTry creates a MsgConnectionOpenTry
func (c *Chain) ConnTry(srcClientID, dstClientID, srcConnID, dstConnID string, dstConnState connTypes.ConnectionResponse, srcHeight int64) sdk.Msg {
	return connTypes.NewMsgConnectionOpenTry(srcConnID, srcClientID, dstConnID, dstClientID, defaultChainPrefix, defaultIBCVersions, dstConnState.Proof, dstConnState.Proof, dstConnState.ProofHeight, uint64(srcHeight), c.MustGetAddress())
}

// ConnAck creates a MsgConnectionOpenAck
func (c *Chain) ConnAck(srcConnID string, dstConnState connTypes.ConnectionResponse, srcHeight int64) sdk.Msg {
	return connTypes.NewMsgConnectionOpenAck(srcConnID, dstConnState.Proof, dstConnState.Proof, dstConnState.ProofHeight, uint64(srcHeight), defaultIBCVersion, c.MustGetAddress())
}

// ConnConfirm creates a MsgConnectionOpenAck
func (c *Chain) ConnConfirm(srcConnID string, dstConnState connTypes.ConnectionResponse, srcHeight int64) sdk.Msg {
	return connTypes.NewMsgConnectionOpenAck(srcConnID, dstConnState.Proof, dstConnState.Proof, dstConnState.ProofHeight, uint64(srcHeight), defaultIBCVersion, c.MustGetAddress())
}

// ChanInit creates a MsgChannelOpenInit
func (c *Chain) ChanInit(srcConnID, srcChanID, dstChanID, srcPortID, dstPortID string, ordering chanState.Order) sdk.Msg {
	return chanTypes.NewMsgChannelOpenInit(srcPortID, srcChanID, defaultIBCVersion, ordering, []string{srcConnID}, dstPortID, dstChanID, c.MustGetAddress())
}

// ChanTry creates a MsgChannelOpenTry
func (c *Chain) ChanTry(srcChanID, dstChanID, srcPortID, dstPortID string, dstChanState chanTypes.ChannelResponse) sdk.Msg {
	return chanTypes.NewMsgChannelOpenTry(srcPortID, srcChanID, defaultIBCVersion, dstChanState.Channel.Ordering, dstChanState.Channel.ConnectionHops,
		dstPortID, dstChanID, defaultIBCVersion, dstChanState.Proof, dstChanState.ProofHeight, c.MustGetAddress())
}

// ChanAck creates a MsgChannelOpenAck
func (c *Chain) ChanAck(srcChanID, srcPortID string, dstChanState chanTypes.ChannelResponse) sdk.Msg {
	return chanTypes.NewMsgChannelOpenAck(srcPortID, srcChanID, dstChanState.Channel.GetVersion(), dstChanState.Proof, dstChanState.ProofHeight, c.MustGetAddress())
}

// ChanConfirm creates a MsgChannelOpenConfirm
func (c *Chain) ChanConfirm(srcChanID, srcPortID string, dstChanState chanTypes.ChannelResponse) sdk.Msg {
	return chanTypes.NewMsgChannelOpenConfirm(srcPortID, srcChanID, dstChanState.Proof, dstChanState.ProofHeight, c.MustGetAddress())
}

// ChanCloseInit creates a MsgChannelCloseInit
func (c *Chain) ChanCloseInit(srcChanID, srcPortID string) sdk.Msg {
	return chanTypes.NewMsgChannelCloseInit(srcPortID, srcChanID, c.MustGetAddress())
}

// ChanCloseConfirm creates a MsgChannelCloseConfirm
func (c *Chain) ChanCloseConfirm(srcChanID, srcPortID string, dstChanState chanTypes.ChannelResponse) sdk.Msg {
	return chanTypes.NewMsgChannelCloseConfirm(srcPortID, srcChanID, dstChanState.Proof, dstChanState.ProofHeight, c.MustGetAddress())
}

// SendMsg wraps the msg in a stdtx, signs and sends it
func (c *Chain) SendMsg(datagram sdk.Msg) (sdk.TxResponse, error) {
	return c.SendMsgs([]sdk.Msg{datagram})
}

// SendMsgs wraps the msgs in a stdtx, signs and sends it
func (c *Chain) SendMsgs(datagrams []sdk.Msg) (sdk.TxResponse, error) {

	txBytes, err := c.BuildAndSignTx(datagrams)
	if err != nil {
		return sdk.TxResponse{}, err
	}

	return c.BroadcastTxCommit(txBytes)
}
