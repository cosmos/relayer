package relayer

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clientTypes "github.com/cosmos/cosmos-sdk/x/ibc/02-client/types"
)

// CreateClients creates clients for src on dst and dst on src given the configured paths
func (c *Chain) CreateClients(dst *Chain) (err error) {
	clients := &RelayMsgs{Src: []sdk.Msg{}, Dst: []sdk.Msg{}}

	// Create client for the destination chain on the source chain if it doesn't exist
	var srcCs, dstCs *clientTypes.QueryClientStateResponse
	if srcCs, err = c.QueryClientState(); err != nil && srcCs == nil {
		dstH, err := dst.UpdateLiteWithHeader()
		if err != nil {
			return err
		}
		if c.debug {
			c.logCreateClient(dst, MustGetHeight(dstH.GetHeight()))
		}
		ubdPeriod, err := dst.QueryUnbondingPeriod()
		if err != nil {
			return err
		}
		clients.Src = append(clients.Src, c.PathEnd.CreateClient(dstH, dst.GetTrustingPeriod(), ubdPeriod, c.MustGetAddress()))
	}

	// Create client for the source chain on destination chain if it doesn't exist
	if dstCs, err = dst.QueryClientState(); err != nil && dstCs == nil {
		srcH, err := c.UpdateLiteWithHeader()
		if err != nil {
			return err
		}
		if dst.debug {
			dst.logCreateClient(c, MustGetHeight(srcH.GetHeight()))
		}
		ubdPeriod, err := c.QueryUnbondingPeriod()
		if err != nil {
			return err
		}

		clients.Dst = append(clients.Dst, dst.PathEnd.CreateClient(srcH, c.GetTrustingPeriod(), ubdPeriod, dst.MustGetAddress()))
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
