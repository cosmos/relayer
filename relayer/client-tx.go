package relayer

import (
	"fmt"
	"reflect"
	"time"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	clientutils "github.com/cosmos/cosmos-sdk/x/ibc/core/02-client/client/utils"
	clienttypes "github.com/cosmos/cosmos-sdk/x/ibc/core/02-client/types"
	commitmenttypes "github.com/cosmos/cosmos-sdk/x/ibc/core/23-commitment/types"
	ibctmtypes "github.com/cosmos/cosmos-sdk/x/ibc/light-clients/07-tendermint/types"
	"github.com/tendermint/tendermint/light"
)

// CreateClients creates clients for src on dst and dst on src if the client ids are unspecified.
// TODO: de-duplicate code
func (c *Chain) CreateClients(dst *Chain) (modified bool, err error) {
	// Handle off chain light clients
	if err := c.ValidateLightInitialized(); err != nil {
		return false, err
	}

	if err = dst.ValidateLightInitialized(); err != nil {
		return false, err
	}

	srcH, dstH, err := UpdatesWithHeaders(c, dst)
	if err != nil {
		return false, err
	}

	// Create client for the destination chain on the source chain if client id is unspecified
	if c.PathEnd.ClientID == "" {
		if c.debug {
			c.logCreateClient(dst, dstH.Header.Height)
		}
		ubdPeriod, err := dst.QueryUnbondingPeriod()
		if err != nil {
			return modified, err
		}

		// Create the ClientState we want on 'c' tracking 'dst'
		clientState := ibctmtypes.NewClientState(
			dstH.GetHeader().GetChainID(),
			ibctmtypes.NewFractionFromTm(light.DefaultTrustLevel),
			dst.GetTrustingPeriod(),
			ubdPeriod,
			time.Minute*10,
			dstH.GetHeight().(clienttypes.Height),
			commitmenttypes.GetSDKSpecs(),
			DefaultUpgradePath,
			false,
			false,
		)

		// Check if an identical light client already exists
		clientID, found := FindMatchingClient(c, dst, clientState)
		if !found {
			msgs := []sdk.Msg{
				c.CreateClient(
					clientState,
					dstH,
				),
			}

			// if a matching client does not exist, create one
			res, success, err := c.SendMsgs(msgs)
			if err != nil {
				return modified, err
			}
			if !success {
				return modified, fmt.Errorf("tx failed: %s", res.RawLog)
			}

			// update the client identifier
			// use index 0, the transaction only has one message
			clientID, err = ParseClientIDFromEvents(res.Logs[0].Events)
			if err != nil {
				return modified, err
			}
		}

		c.PathEnd.ClientID = clientID
		modified = true

	} else {
		// Ensure client exists in the event of user inputted identifiers
		_, err := c.QueryClientState(srcH.Header.Height)
		if err != nil {
			return false, fmt.Errorf("please ensure provided on-chain client (%s) exists on the chain (%s): %v",
				c.PathEnd.ClientID, c.ChainID, err)
		}
	}

	// Create client for the source chain on destination chain if client id is unspecified
	if dst.PathEnd.ClientID == "" {
		if dst.debug {
			dst.logCreateClient(c, srcH.Header.Height)
		}
		ubdPeriod, err := c.QueryUnbondingPeriod()
		if err != nil {
			return modified, err
		}
		// Create the ClientState we want on 'dst' tracking 'c'
		clientState := ibctmtypes.NewClientState(
			srcH.GetHeader().GetChainID(),
			ibctmtypes.NewFractionFromTm(light.DefaultTrustLevel),
			c.GetTrustingPeriod(),
			ubdPeriod,
			time.Minute*10,
			srcH.GetHeight().(clienttypes.Height),
			commitmenttypes.GetSDKSpecs(),
			DefaultUpgradePath,
			false,
			false,
		)

		// Check if an identical light client already exists
		// NOTE: we pass in 'dst' as the source and 'c' as the
		// counterparty.
		clientID, found := FindMatchingClient(dst, c, clientState)
		if !found {
			msgs := []sdk.Msg{
				dst.CreateClient(
					clientState,
					srcH,
				),
			}

			// if a matching client does not exist, create one
			res, success, err := dst.SendMsgs(msgs)
			if err != nil {
				return modified, err
			}
			if !success {
				return modified, fmt.Errorf("tx failed: %s", res.RawLog)
			}

			// update client identifier
			clientID, err = ParseClientIDFromEvents(res.Logs[0].Events)
			if err != nil {
				return modified, err
			}
		}
		dst.PathEnd.ClientID = clientID
		modified = true

	} else {
		// Ensure client exists in the event of user inputted identifiers
		_, err := dst.QueryClientState(dstH.Header.Height)
		if err != nil {
			return false, fmt.Errorf("please ensure provided on-chain client (%s) exists on the chain (%s): %v",
				dst.PathEnd.ClientID, dst.ChainID, err)
		}

	}

	c.Log(fmt.Sprintf("★ Clients created: client(%s) on chain[%s] and client(%s) on chain[%s]",
		c.PathEnd.ClientID, c.ChainID, dst.PathEnd.ClientID, dst.ChainID))

	return modified, nil
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

	clients.Src = append(clients.Src, c.UpdateClient(dstUH))
	clients.Dst = append(clients.Dst, dst.UpdateClient(srcUH))

	// Send msgs to both chains
	if clients.Ready() {
		if clients.Send(c, dst); clients.Success() {
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

// UpgradeClients upgrades the client on src after dst chain has undergone an upgrade.
func (c *Chain) UpgradeClients(dst *Chain, height int64) error {
	sh, err := NewSyncHeaders(c, dst)
	if err != nil {
		return err
	}
	if err := sh.Updates(c, dst); err != nil {
		return err
	}

	if height == 0 {
		height = int64(sh.GetHeight(dst.ChainID))
	}

	// TODO: construct method of only attempting to get dst header
	// Note: we explicitly do not check the error since the source
	// trusted header will fail
	_, dstUpdateHeader, _ := sh.GetTrustedHeaders(c, dst)

	// query proofs on counterparty
	clientState, proofUpgradeClient, _, err := dst.QueryUpgradedClient(height)
	if err != nil {
		return err
	}

	consensusState, proofUpgradeConsensusState, _, err := dst.QueryUpgradedConsState(height)
	if err != nil {
		return err
	}

	upgradeMsg := &clienttypes.MsgUpgradeClient{ClientId: c.PathEnd.ClientID, ClientState: clientState,
		ConsensusState: consensusState, ProofUpgradeClient: proofUpgradeClient,
		ProofUpgradeConsensusState: proofUpgradeConsensusState, Signer: c.MustGetAddress().String()}

	msgs := []sdk.Msg{
		c.UpdateClient(dstUpdateHeader),
		upgradeMsg,
	}

	_, _, err = c.SendMsgs(msgs)
	if err != nil {
		return err
	}

	return nil
}

// FindMatchingClient will determine if there exists a client with identical client and consensus states
// to the client which would have been created. Source is the chain that would be adding a client
// which would track the counterparty. Therefore we query source for the existing clients
// and check if any match the counterparty. The counterparty must have a matching consensus state
// to the latest consensus state of a potential match. The provided client state is the client
// state that will be created if there exist no matches.
func FindMatchingClient(source, counterparty *Chain, clientState *ibctmtypes.ClientState) (string, bool) {
	// TODO: add appropriate offset and limits, along with retries
	clientsResp, err := source.QueryClients(0, 1000)
	if err != nil {
		if source.debug {
			source.Log(fmt.Sprintf("Error: querying clients on %s failed: %v", source.PathEnd.ChainID, err))
		}
		return "", false
	}

	for _, identifiedClientState := range clientsResp.ClientStates {
		// unpack any into ibc tendermint client state
		clientStateExported, err := clienttypes.UnpackClientState(identifiedClientState.ClientState)
		if err != nil {
			return "", false
		}

		// cast from interface to concrete type
		existingClientState, ok := clientStateExported.(*ibctmtypes.ClientState)
		if !ok {
			return "", false
		}

		// check if the client states match
		if IsMatchingClient(*clientState, *existingClientState) {

			// query the latest consensus state of the potential matching client
			consensusStateResp, err := clientutils.QueryConsensusStateABCI(source.CLIContext(0),
				identifiedClientState.ClientId, existingClientState.GetLatestHeight())
			if err != nil {
				if source.debug {
					source.Log(fmt.Sprintf("Error: failed to query latest consensus state for existing client on chain %s: %v",
						source.PathEnd.ChainID, err))
				}
				continue
			}

			header, err := counterparty.QueryHeaderAtHeight(int64(existingClientState.GetLatestHeight().GetRevisionHeight()))
			if err != nil {
				if source.debug {
					source.Log(fmt.Sprintf("Error: failed to query header for chain %s at height %d: %v",
						counterparty.PathEnd.ChainID, existingClientState.GetLatestHeight().GetRevisionHeight(), err))
				}
				continue
			}

			if IsMatchingConsensusState(consensusStateResp.ConsensusState, header.ConsensusState()) {
				// matching client found
				return identifiedClientState.ClientId, true
			}
		}
	}

	return "", false
}

// IsMatchingClient determines if the two provided clients match in all fields
// except latest height. They are assumed to be IBC tendermint light clients.
// NOTE: we don't pass in a pointer so upstream references don't have a modified
// latest height set to zero.
func IsMatchingClient(clientStateA, clientStateB ibctmtypes.ClientState) bool {
	// zero out latest client height since this is determined and incremented
	// by on-chain updates. Changing the latest height does not fundamentally
	// change the client. The associated consensus state at the latest height
	// determines this last check
	clientStateA.LatestHeight = clienttypes.ZeroHeight()
	clientStateB.LatestHeight = clienttypes.ZeroHeight()

	return reflect.DeepEqual(clientStateA, clientStateB)
}

// IsMatchingConsensusState determines if the two provided consensus states are
// identical. They are assumed to be IBC tendermint light clients.
func IsMatchingConsensusState(anyConsState *codectypes.Any, consensusStateB *ibctmtypes.ConsensusState) bool {
	exportedConsState, err := clienttypes.UnpackConsensusState(anyConsState)
	if err != nil {
		return false
	}
	consensusStateA, ok := exportedConsState.(*ibctmtypes.ConsensusState)
	if !ok {
		return false
	}

	return reflect.DeepEqual(*consensusStateA, *consensusStateB)
}
