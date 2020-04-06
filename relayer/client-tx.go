package relayer

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clientTypes "github.com/cosmos/cosmos-sdk/x/ibc/02-client/types"
)

// CreateClients creates clients for src on dst and dst on src given the configured paths
func (src *Chain) CreateClients(dst *Chain) (err error) {
	clients := &RelayMsgs{Src: []sdk.Msg{}, Dst: []sdk.Msg{}}

	// Create client for dst on src if it doesn't exist
	var srcCs, dstCs *clientTypes.StateResponse
	if srcCs, err = src.QueryClientState(); err != nil {
		return err
	} else if srcCs == nil {
		dstH, err := dst.UpdateLiteWithHeader()
		if err != nil {
			return err
		}
		if src.debug {
			src.logCreateClient(dst, dstH.GetHeight())
		}
		clients.Src = append(clients.Src, src.PathEnd.CreateClient(dstH, src.GetTrustingPeriod(), src.MustGetAddress()))
	}
	// TODO: maybe log something here that the client has been created?

	// Create client for src on dst if it doesn't exist
	if dstCs, err = dst.QueryClientState(); err != nil {
		return err
	} else if dstCs == nil {
		srcH, err := src.UpdateLiteWithHeader()
		if err != nil {
			return err
		}
		if dst.debug {
			dst.logCreateClient(src, srcH.GetHeight())
		}
		clients.Dst = append(clients.Dst, dst.PathEnd.CreateClient(srcH, dst.GetTrustingPeriod(), dst.MustGetAddress()))
	}
	// TODO: maybe log something here that the client has been created?

	// Send msgs to both chains
	if clients.Ready() {
		if clients.Send(src, dst); clients.success {
			src.Log(fmt.Sprintf("★ Clients created: [%s]client(%s) and [%s]client(%s)",
				src.ChainID, src.PathEnd.ClientID, dst.ChainID, dst.PathEnd.ClientID))
		}
	}

	return nil
}
