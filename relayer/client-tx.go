package relayer

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"golang.org/x/sync/errgroup"
)

// CreateClients creates clients for src on dst and dst on src given the configured paths
func (c *Chain) CreateClients(dst *Chain) (err error) {
	var (
		clients = &RelayMsgs{Src: []sdk.Msg{}, Dst: []sdk.Msg{}}
		eg      = new(errgroup.Group)
	)

	srcH, dstH, err := UpdatesWithHeaders(c, dst)
	if err != nil {
		return err
	}

	// Create client for the destination chain on the source chain if it doesn't exist
	eg.Go(func() error {
		if srcCs, err := c.QueryClientState(srcH.Header.Height); err != nil && srcCs == nil {
			if c.debug {
				c.logCreateClient(dst, dstH.Header.Height)
			}
			ubdPeriod, err := dst.QueryUnbondingPeriod()
			if err != nil {
				return err
			}
			consensusParams, err := dst.QueryConsensusParams()
			if err != nil {
				return err
			}
			clients.Src = append(
				clients.Src,
				c.PathEnd.CreateClient(
					dstH,
					dst.GetTrustingPeriod(),
					ubdPeriod,
					consensusParams,
					c.MustGetAddress(),
				))
		}
		return nil
	})

	eg.Go(func() error {
		// Create client for the source chain on destination chain if it doesn't exist
		if dstCs, err := dst.QueryClientState(dstH.Header.Height); err != nil && dstCs == nil {
			if dst.debug {
				dst.logCreateClient(c, srcH.Header.Height)
			}
			ubdPeriod, err := c.QueryUnbondingPeriod()
			if err != nil {
				return err
			}
			consensusParams, err := c.QueryConsensusParams()
			if err != nil {
				return err
			}
			clients.Dst = append(
				clients.Dst,
				dst.PathEnd.CreateClient(
					srcH,
					c.GetTrustingPeriod(),
					ubdPeriod,
					consensusParams,
					dst.MustGetAddress(),
				))
		}
		return nil
	})

	if err := eg.Wait(); err != nil {
		return err
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

// UpdateClients updates clients for src on dst and dst on src given the configured paths
func (c *Chain) UpdateClients(dst *Chain) (err error) {
	clients := &RelayMsgs{Src: []sdk.Msg{}, Dst: []sdk.Msg{}}

	sh, err := NewSyncHeaders(c, dst)
	if err != nil {
		return err
	}

	srcUH, dstUH, err := sh.GetTrustedHeaders(c, dst)
	if err != nil {
		return err
	}

	clients.Src = append(clients.Src, c.PathEnd.UpdateClient(dstUH, c.MustGetAddress()))
	clients.Dst = append(clients.Dst, dst.PathEnd.UpdateClient(srcUH, dst.MustGetAddress()))

	// Send msgs to both chains
	if clients.Ready() {
		if clients.Send(c, dst); clients.success {
			c.Log(fmt.Sprintf("★ Clients updated: [%s]client(%s) {%d}->{%d} and [%s]client(%s) {%d}->{%d}",
				c.ChainID,
				c.PathEnd.ClientID,
				MustGetHeight(srcUH.TrustedHeight),
				srcUH.Header.Height,
				dst.ChainID,
				dst.PathEnd.ClientID,
				MustGetHeight(dstUH.TrustedHeight),
				dstUH.Header.Height,
			),
			)
		}
	}

	return nil
}
