package relayer

import (
	"encoding/hex"
	"fmt"

	clienttypes "github.com/cosmos/cosmos-sdk/x/ibc/core/02-client/types"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/light-clients/07-tendermint/types"
)

var (
	// strings for parsing events
	updateCliTag = "update_client"
	headerTag    = "header"
	clientIDTag  = "client_id"
)

// checkAndSubmitMisbehaviour check headers from update_client tx events
// and submit misbehaviour if found any
func checkAndSubmitMisbehaviour(src *Chain, events map[string][]string) error {
	if hdrs, ok := events[fmt.Sprintf("%s.%s", updateCliTag, headerTag)]; ok {
		for i, hdr := range hdrs {
			clientIDs := events[fmt.Sprintf("%s.%s", updateCliTag, clientIDTag)]
			emittedClientID := clientIDs[i]

			if src.PathEnd.ClientID == emittedClientID {
				hdrBytes, err := hex.DecodeString(hdr)
				if err != nil {
					return err
				}

				exportedHeader, err := clienttypes.UnmarshalHeader(src.Encoding.Marshaler, hdrBytes)
				if err != nil {
					return err
				}

				emittedHeader, ok := exportedHeader.(*tmclient.Header)
				if !ok {
					return fmt.Errorf("emitted header is not tendermint type")
				}

				trustedHeader, err := src.GetLightSignedHeaderAtHeight(emittedHeader.Header.Height)
				if err != nil {
					return err
				}

				trustedHeader.TrustedValidators = emittedHeader.TrustedValidators
				trustedHeader.TrustedHeight = emittedHeader.TrustedHeight

				if !IsMatchingConsensusState(emittedHeader.ConsensusState(), trustedHeader.ConsensusState()) {
					misbehaviour := tmclient.NewMisbehaviour(emittedClientID, emittedHeader, trustedHeader)
					msg, err := clienttypes.NewMsgSubmitMisbehaviour(emittedClientID, misbehaviour, src.MustGetAddress())
					if err != nil {
						return err
					}
					res, success, err := src.SendMsg(msg)
					if err != nil {
						return err
					}
					if !success {
						return fmt.Errorf("submit misbehaviour tx failed: %s", res.RawLog)
					}
					src.Log(fmt.Sprintf("Submitted misbehaviour for emitted header with height: %d",
						emittedHeader.Header.Height))
				}
			}
		}
	}

	return nil
}
