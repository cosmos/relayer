package relayer_test

import (
	"encoding/json"
	"fmt"
	"github.com/cosmos/relayer/v2/relayer"
	"github.com/cosmos/relayer/v2/relayer/chains/cosmos"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNewPathHops(t *testing.T) {
	srcChainID := "juno"
	hopChainID := "cosmoshub"
	dstChainID := "osmosis"
	hopClientIDs := []string{"07-tendermint-439", "07-tendermint-259"}
	hopConnectionIDs := []string{"connection-372", "connection-257"}
	ibcDataStr := fmt.Sprintf(`{
  "$schema": "../ibc_data.schema.json",
  "chain_1": {
    "chain_name": "%s",
    "client_id": "07-tendermint-3",
    "connection_id": "connection-2"
  },
  "chain_2": {
    "chain_name": "%s",
    "client_id": "07-tendermint-1",
    "connection_id": "connection-1"
  },
  "hops": [
    {
      "chain_name": "cosmoshub",
      "client_ids": [
        "%s",
        "%s"
      ],
      "connection_ids": [
        "%s",
        "%s"
      ]
    }
  ],
  "channels": [
    {
      "chain_1": {
        "channel_id": "channel-998",
        "port_id": "transfer"
      },
      "chain_2": {
        "channel_id": "channel-999",
        "port_id": "transfer"
      },
      "ordering": "unordered",
      "version": "ics20-1",
      "tags": {
        "status": "live",
        "preferred": true,
        "dex": "osmosis"
      }
    }
  ]
}`, srcChainID, dstChainID, hopClientIDs[0], hopClientIDs[1], hopConnectionIDs[0], hopConnectionIDs[1])
	var ibc relayer.IBCdata
	require.NoError(t, json.Unmarshal([]byte(ibcDataStr), &ibc))
	srcPathEnd := &relayer.PathEnd{
		ChainID:      srcChainID,
		ClientID:     ibc.Chain1.ClientID,
		ConnectionID: ibc.Chain1.ConnectionID,
	}
	dstPathEnd := &relayer.PathEnd{
		ChainID:      dstChainID,
		ClientID:     ibc.Chain2.ClientID,
		ConnectionID: ibc.Chain2.ConnectionID,
	}
	chainsMap := map[string]*relayer.Chain{}
	for _, chainID := range []string{srcChainID, hopChainID, dstChainID} {
		chainsMap[chainID] = &relayer.Chain{
			Chainid: chainID,
			ChainProvider: &cosmos.CosmosProvider{
				PCfg: cosmos.CosmosProviderConfig{
					ChainID: chainID,
				},
			},
		}
	}
	chains := relayer.Chains(chainsMap)
	pathHops, err := relayer.NewPathHops(&ibc, srcPathEnd, dstPathEnd, &chains)
	require.NoError(t, err)
	require.Len(t, pathHops, 1)
	require.Equal(t, "cosmoshub", pathHops[0].ChainID)
	require.Equal(t, "juno", pathHops[0].PathEnds[0].ChainID)
	require.Equal(t, "osmosis", pathHops[0].PathEnds[1].ChainID)
	require.Equal(t, hopClientIDs[0], pathHops[0].PathEnds[0].ClientID)
	require.Equal(t, hopClientIDs[1], pathHops[0].PathEnds[1].ClientID)
	require.Equal(t, hopConnectionIDs[0], pathHops[0].PathEnds[0].ConnectionID)
	require.Equal(t, hopConnectionIDs[1], pathHops[0].PathEnds[1].ConnectionID)
}
