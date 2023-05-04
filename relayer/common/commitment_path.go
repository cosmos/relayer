package common

import (
	"fmt"
	"math/big"
)

func encodePacked(parts ...interface{}) []byte {
	var result string
	for _, part := range parts {
		switch v := part.(type) {
		case string:
			result += v
		case *big.Int:
			result += v.String()
		default:
			panic(fmt.Errorf("unsupported type: %T", v))
		}
	}
	return []byte(result)
}

func GetClientStatePath(clientId string) []byte {
	return encodePacked("clients/", clientId, "/clientState")
}

func GetConsensusStatePath(clientId string, revisionNumber, revisionHeight *big.Int) []byte {
	return encodePacked("clients/", clientId, "/consensusStates/", revisionNumber, "-", revisionHeight)
}

func GetConnectionPath(connectionId string) []byte {
	return encodePacked("connections", connectionId)
}

func GetChannelPath(portId, channelId string) []byte {
	return encodePacked("channelEnds/ports/", portId, "/channels/", channelId)
}

func GetPacketCommitmentPath(portId, channelId string, sequence *big.Int) []byte {
	return encodePacked("commitments/ports/", portId, "/channels/", channelId, "/sequences/", sequence)
}

func GetPacketAcknowledgementCommitmentPath(portId, channelId string, sequence *big.Int) []byte {
	return encodePacked("acks/ports/", portId, "/channels/", channelId, "/sequences/", sequence)
}

func GetPacketReceiptCommitmentPath(portId, channelId string, sequence *big.Int) []byte {
	return encodePacked("receipts/ports/", portId, "/channels/", channelId, "/sequences/", sequence)
}

func GetNextSequenceRecvCommitmentPath(portId, channelId string) []byte {
	return encodePacked("nextSequenceRecv/ports/", portId, "/channels/", channelId)
}

func GetClientStateCommitmentKey(clientId string) []byte {
	return Sha3keccak256(GetClientStatePath(clientId))
}

func GetConsensusStateCommitmentKey(clientId string, revisionNumber, revisionHeight *big.Int) []byte {
	return Sha3keccak256(GetConsensusStatePath(clientId, revisionNumber, revisionHeight))
}

func GetConnectionCommitmentKey(connectionId string) []byte {
	return Sha3keccak256(GetConnectionPath(connectionId))
}

func GetChannelCommitmentKey(portId, channelId string) []byte {
	return Sha3keccak256(GetChannelPath(portId, channelId))
}

func GetPacketCommitmentKey(portId, channelId string, sequence *big.Int) []byte {
	return Sha3keccak256(GetPacketCommitmentPath(portId, channelId, sequence))
}

func GetPacketAcknowledgementCommitmentKey(portId, channelId string, sequence *big.Int) []byte {
	return Sha3keccak256(GetPacketAcknowledgementCommitmentPath(portId, channelId, sequence))
}

func GetPacketReceiptCommitmentKey(portId, channelId string, sequence *big.Int) []byte {
	return Sha3keccak256(GetPacketReceiptCommitmentPath(portId, channelId, sequence))
}

func GetNextSequenceRecvCommitmentKey(portId, channelId string) []byte {
	return Sha3keccak256(GetNextSequenceRecvCommitmentPath(portId, channelId))
}
