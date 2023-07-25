package icon

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/cosmos/relayer/v2/relayer/chains/icon/cryptoutils"
	"github.com/cosmos/relayer/v2/relayer/chains/icon/types"
	"github.com/cosmos/relayer/v2/relayer/common"
	"github.com/cosmos/relayer/v2/relayer/provider"

	"github.com/cosmos/gogoproto/proto"

	"github.com/icon-project/IBC-Integration/libraries/go/common/icon"
	icn "github.com/icon-project/IBC-Integration/libraries/go/common/icon"
	"github.com/icon-project/goloop/common/codec"
	"github.com/icon-project/goloop/common/crypto"
	"github.com/icon-project/goloop/common/db"
	"github.com/icon-project/goloop/common/trie/ompt"
)

var (
	ethAddressLen = 20
)

func MptProve(key types.HexInt, proofs [][]byte, hash []byte) ([]byte, error) {
	db := db.NewMapDB()
	defer db.Close()
	index, err := key.Value()
	if err != nil {
		return nil, err
	}
	indexKey, err := codec.RLP.MarshalToBytes(index)
	if err != nil {
		return nil, err
	}
	mpt := ompt.NewMPTForBytes(db, hash)
	trie, err1 := mpt.Prove(indexKey, proofs)
	if err1 != nil {
		return nil, err1

	}
	return trie, nil
}

func Base64ToData(encoded string, v interface{}) ([]byte, error) {
	if encoded == "" {
		return nil, fmt.Errorf("Encoded string is empty ")
	}

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}

	return codec.RLP.UnmarshalFromBytes(decoded, v)
}

func HexBytesToProtoUnmarshal(encoded types.HexBytes, v proto.Message) ([]byte, error) {
	inputBytes, err := encoded.Value()
	if err != nil {
		return nil, fmt.Errorf("Error unmarshalling HexByte")
	}

	if bytes.Equal(inputBytes, make([]byte, 0)) {
		return nil, fmt.Errorf("Encoded hexbyte is empty ")
	}

	if err := proto.Unmarshal(inputBytes, v); err != nil {
		return nil, err

	}
	return inputBytes, nil

}

func isHexString(s string) bool {
	s = strings.ToLower(s)
	if !strings.HasPrefix(s, "0x") {
		return false
	}

	s = s[2:]

	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func btpBlockNotPresent(err error) bool {
	if strings.Contains(err.Error(), "NotFound: E1005:fail to get a BTP block header") {
		return true
	}
	return false

}

func getCommitmentHash(key, msg []byte) []byte {
	msgHash := common.Sha3keccak256(msg)
	return common.Sha3keccak256(key, msgHash)
}

func VerifyProof(commitmentkey []byte, msgval []byte, root []byte, proof []byte) (bool, error) {
	leaf := getCommitmentHash(commitmentkey, msgval)
	var decodedProof icn.MerkleProofs
	if err := proto.Unmarshal(proof, &decodedProof); err != nil {
		return false, err
	}

	return cryptoutils.VerifyMerkleProof(root, leaf, decodedProof.Proofs), nil
}

func getSrcNetworkId(id int64) string {
	return fmt.Sprintf("%s.icon", types.NewHexInt(id))
}

func getIconPacketEncodedBytes(pkt provider.PacketInfo) ([]byte, error) {
	iconPkt := icon.Packet{
		Sequence:           pkt.Sequence,
		SourcePort:         pkt.SourcePort,
		SourceChannel:      pkt.SourceChannel,
		DestinationPort:    pkt.DestPort,
		DestinationChannel: pkt.DestChannel,
		TimeoutHeight: &icon.Height{
			RevisionNumber: pkt.TimeoutHeight.RevisionNumber,
			RevisionHeight: pkt.TimeoutHeight.RevisionHeight,
		},
		TimeoutTimestamp: pkt.TimeoutTimestamp,
		Data:             pkt.Data,
	}

	return proto.Marshal(&iconPkt)

}

func GetNetworkSectionRoot(header *types.BTPBlockHeader) []byte {
	networkSection := types.NewNetworkSection(header)
	return cryptoutils.CalculateRootFromProof(networkSection.Hash(), header.NetworkSectionToRoot)
}

func VerifyBtpProof(decision *types.NetworkTypeSectionDecision, proof [][]byte, listValidators [][]byte) (bool, error) {

	requiredVotes := (2 * len(listValidators)) / 3
	if requiredVotes < 1 {
		requiredVotes = 1
	}

	numVotes := 0
	validators := make(map[types.HexBytes]struct{})
	for _, val := range listValidators {
		validators[types.NewHexBytes(val)] = struct{}{}
	}

	for _, raw_sig := range proof {
		sig, err := crypto.ParseSignature(raw_sig)
		if err != nil {
			return false, err
		}
		pubkey, err := sig.RecoverPublicKey(decision.Hash())
		if err != nil {
			continue
		}

		address, err := newEthAddressFromPubKey(pubkey.SerializeCompressed())
		if err != nil {
			continue
		}
		if address == nil {
			continue
		}
		if _, ok := validators[types.NewHexBytes(address)]; !ok {
			continue
		}
		delete(validators, types.NewHexBytes(address))
		if numVotes++; numVotes >= requiredVotes {
			return true, nil
		}
	}

	return false, nil

}

func newEthAddressFromPubKey(pubKey []byte) ([]byte, error) {
	if len(pubKey) == crypto.PublicKeyLenCompressed {
		pk, err := crypto.ParsePublicKey(pubKey)
		if err != nil {
			return nil, err
		}
		pubKey = pk.SerializeUncompressed()
	}
	digest := common.Sha3keccak256(pubKey[1:])
	return digest[len(digest)-ethAddressLen:], nil
}

func isValidIconContractAddress(addr string) bool {
	if !strings.HasPrefix(addr, "cx") {
		return false
	}
	if len(addr) != 42 {
		return false
	}
	_, err := hex.DecodeString(addr[2:])
	if err != nil {
		return false
	}
	return true
}
