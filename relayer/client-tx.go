package relayer

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clientTypes "github.com/cosmos/cosmos-sdk/x/ibc/02-client/types"
)

// CreateClients creates clients for src on dst and dst on src given the configured paths
func (c *Chain) CreateClients(dst *Chain) (err error) {
	clients := &RelayMsgs{Src: []sdk.Msg{}, Dst: []sdk.Msg{}}

	// Create client for dst on c if it doesn't exist
	var srcCs, dstCs *clientTypes.StateResponse
	if srcCs, err = c.QueryClientState(); err != nil {
		return err
	} else if srcCs == nil {
		dstH, err := dst.UpdateLiteWithHeader()
		if err != nil {
			return err
		}
		if c.debug {
			c.logCreateClient(dst, dstH.GetHeight())
		}
		clients.Src = append(clients.Src, c.PathEnd.CreateClient(dstH, dst.GetTrustingPeriod(), c.MustGetAddress()))
	}

	// Create client for c on dst if it doesn't exist
	if dstCs, err = dst.QueryClientState(); err != nil {
		return err
	} else if dstCs == nil {
		srcH, err := c.UpdateLiteWithHeader()
		if err != nil {
			return err
		}
		if dst.debug {
			dst.logCreateClient(c, srcH.GetHeight())
		}
		clients.Dst = append(clients.Dst, dst.PathEnd.CreateClient(srcH, c.GetTrustingPeriod(), dst.MustGetAddress()))
	}

	// Send msgs to both chains
	if clients.Ready() {
		if clients.Send(c, dst); clients.success {
			c.Log(fmt.Sprintf("★ Clients created: [%s]client(%s) and [%s]client(%s)",
				c.ChainID, c.PathEnd.ClientID, dst.ChainID, dst.PathEnd.ClientID))
		}
	}

	return nil
}
