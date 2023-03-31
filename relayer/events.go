package relayer

import (
	"fmt"

	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	connectiontypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	channeltypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
)

// ParseClientIDFromEvents parses events emitted from a MsgCreateClient and returns the
// client identifier.
func ParseClientIDFromEvents(events []provider.RelayerEvent) (string, error) {
	for _, event := range events {
		if event.EventType == clienttypes.EventTypeCreateClient {
			for attributeKey, attributeValue := range event.Attributes {
				if attributeKey == clienttypes.AttributeKeyClientID {
					return attributeValue, nil
				}
			}
		}
	}
	return "", fmt.Errorf("client identifier event attribute not found")
}

// ParseConnectionIDFromEvents parses events emitted from a MsgConnectionOpenInit or
// MsgConnectionOpenTry and returns the connection identifier.
func ParseConnectionIDFromEvents(events []provider.RelayerEvent) (string, error) {
	for _, event := range events {
		if event.EventType == connectiontypes.EventTypeConnectionOpenInit || event.EventType == connectiontypes.EventTypeConnectionOpenTry {
			for attributeKey, attributeValue := range event.Attributes {
				if attributeKey == connectiontypes.AttributeKeyConnectionID {
					return attributeValue, nil
				}
			}
		}
	}
	return "", fmt.Errorf("connection identifier event attribute not found")
}

// ParseChannelIDFromEvents parses events emitted from a MsgChannelOpenInit or
// MsgChannelOpenTry and returns the channel identifier.
func ParseChannelIDFromEvents(events []provider.RelayerEvent) (string, error) {
	for _, event := range events {
		if event.EventType == channeltypes.EventTypeChannelOpenInit || event.EventType == channeltypes.EventTypeChannelOpenTry {
			for attributeKey, attributeValue := range event.Attributes {
				if attributeKey == channeltypes.AttributeKeyChannelID {
					return attributeValue, nil
				}
			}
		}
	}
	return "", fmt.Errorf("channel identifier event attribute not found")
}
