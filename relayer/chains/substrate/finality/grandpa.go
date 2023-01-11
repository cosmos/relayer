package finality

import (
	"fmt"
	"github.com/ChainSafe/chaindb"
	rpcclient "github.com/ComposableFi/go-substrate-rpc-client/v4"
	codec "github.com/ComposableFi/go-substrate-rpc-client/v4/scale"
	rpctypes "github.com/ComposableFi/go-substrate-rpc-client/v4/types"
	ibcexported "github.com/cosmos/ibc-go/v5/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/chains/substrate/finality/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"unsafe"
)

/*
#cgo LDFLAGS: -L"${SRCDIR}/../../../../lib" -lgo_export
#include "../../../../lib/go-export.h"
*/
import "C"

// todo
//   - implement condition for when previouslyFinalizedHeight isn't passed as an argument
//   - implement finality methods
//   - construct grandpa consensus state
//   - implement parsing methods from grandpa to wasm client types.
//   - check if validation data can be used to fetch relaychain data from the parachain
//   - make justification subscription in chain processor generic. It only subscribes to beefy justifications currently
//   - write tests for grandpa construct methods
var _ FinalityGadget = &Grandpa{}

const GrandpaFinalityGadget = "grandpa"

type Grandpa struct {
	parachainClient  *rpcclient.SubstrateAPI
	relayChainClient *rpcclient.SubstrateAPI
	paraID           uint32
	relayChain       int32
	memDB            *chaindb.BadgerDB
	relayMetadata    *rpctypes.Metadata
	paraMetadata     *rpctypes.Metadata
	clientState      *types.ClientState
}

func NewGrandpa(
	parachainClient,
	relayChainClient *rpcclient.SubstrateAPI,
	paraID uint32,
	relayChain int32,
	memDB *chaindb.BadgerDB,
	clientState *types.ClientState,
) *Grandpa {
	paraMetadata, err := parachainClient.RPC.State.GetMetadataLatest()
	relayMetadata, err := relayChainClient.RPC.State.GetMetadataLatest()
	if err != nil {
		panic(err)
	}
	return &Grandpa{
		parachainClient,
		relayChainClient,
		paraID,
		relayChain,
		memDB,
		relayMetadata,
		paraMetadata,
		clientState,
	}
}

type GrandpaIBCHeader struct {
	height       uint64
	SignedHeader *ParachainHeadersWithFinalityProof
	//SignedHeader *types.Header
}

func (h GrandpaIBCHeader) Height() uint64 {
	return h.height
}

func (h GrandpaIBCHeader) ConsensusState() ibcexported.ConsensusState {
	// todo: should this be the first parachain header in the list?
	//parachainHeader := h.SignedHeader.ParachainHeaders[0].ParachainHeader
	//timestamp, err := decodeExtrinsicTimestamp(parachainHeader.Extrinsic)
	//if err != nil {
	//	panic(err)
	//}

	// todo: construct the grandpa consensus state and wrap it in a Wasm consensus
	//return types.ConsensusState{Timestamp: timestamppb.New(timestamp)}
	//return nil
	panic("[ConsensusState] implement me")
}

func (g *Grandpa) QueryLatestHeight() (paraHeight int64, relayChainHeight int64, err error) {
	block, err := g.parachainClient.RPC.Chain.GetBlockLatest()
	if err != nil {
		return 0, 0, err
	}
	paraHeight = int64(block.Block.Header.Number)

	relayBlock, err := g.relayChainClient.RPC.Chain.GetBlockLatest()
	if err != nil {
		return 0, 0, err
	}
	relayChainHeight = int64(relayBlock.Block.Header.Number)
	return paraHeight, relayChainHeight, nil
}

func (g *Grandpa) QueryHeaderAt(latestRelayChainHeight uint64) (header ibcexported.Header, err error) {
	fmt.Println("Querying header at height", latestRelayChainHeight)
	//hash, err := g.relayChainClient.RPC.Chain.GetBlockHash(latestRelayChainHeight)
	//header, err = g.relayChainClient.RPC.Chain.GetHeader(hash)
	//header, err = g.parachainClient.RPC.Chain.GetHeader(hash)
	header, err = g.queryFinalizedParachainHeadersWithProof(g.clientState, 1, nil)
	if err != nil {
		return nil, err
	}
	return header, nil
}

func (g *Grandpa) QueryHeaderOverBlocks(finalizedBlockHeight, previouslyFinalizedBlockHeight uint64) (ibcexported.Header, error) {
	fmt.Println("Querying header over blocks", finalizedBlockHeight, previouslyFinalizedBlockHeight)
	header, err := g.queryFinalizedParachainHeadersWithProof(g.clientState, uint32(previouslyFinalizedBlockHeight), nil)
	if err != nil {
		return nil, err
	}
	return header, nil
}

func (g *Grandpa) IBCHeader(header ibcexported.Header) provider.IBCHeader {
	fmt.Println("Converting header to IBC header")
	signedHeader := header.(*ParachainHeadersWithFinalityProof)
	return GrandpaIBCHeader{
		height:       signedHeader.GetHeight().GetRevisionHeight(),
		SignedHeader: signedHeader,
	}
}

func (g *Grandpa) ClientState(_ provider.IBCHeader) (ibcexported.ClientState, error) {
	fmt.Println("Querying client state")
	//grandpaHeader, ok := header.(GrandpaIBCHeader)
	//if !ok {
	//	return nil, fmt.Errorf("got data of type %T but wanted  finality.GrandpaIBCHeader \n", header)
	//}
	//
	//currentAuthorities, err := g.getCurrentAuthorities()
	//if err != nil {
	//	return nil, err
	//}
	//
	//blockHash, err := g.relayChainClient.RPC.Chain.GetBlockHash(grandpaHeader.height)
	//if err != nil {
	//	return nil, err
	//}
	//
	//currentSetId, err := g.getCurrentSetId(blockHash)
	//if err != nil {
	//	return nil, err
	//}
	//
	//latestRelayHash, err := g.relayChainClient.RPC.Chain.GetFinalizedHead()
	//if err != nil {
	//	return nil, err
	//}
	//
	//latestRelayheader, err := g.relayChainClient.RPC.Chain.GetHeader(latestRelayHash)
	//if err != nil {
	//	return nil, err
	//}
	//
	//paraHeader, err := g.getParachainHeader(latestRelayHash)
	//if err != nil {
	//	return nil, err
	//}
	//
	//var relayChain types.RelayChain
	//switch types.RelayChain(g.relayChain) {
	//case types.RelayChain_POLKADOT:
	//	relayChain = types.RelayChain_POLKADOT
	//case types.RelayChain_KUSAMA:
	//	relayChain = types.RelayChain_KUSAMA
	//case types.RelayChain_ROCOCO:
	//	relayChain = types.RelayChain_ROCOCO
	//}

	//return types.ClientState{
	//	ParaId:             g.paraID,
	//	CurrentSetId:       currentSetId,
	//	CurrentAuthorities: currentAuthorities,
	//	LatestRelayHash:    latestRelayHash[:],
	//	LatestRelayHeight:  uint32(latestRelayheader.Number),
	//	LatestParaHeight:   uint32(paraHeader.Number),
	//	RelayChain:         relayChain,
	//}, nil
	return nil, nil
}

func (g *Grandpa) ConsensusState() (types.ConsensusState, error) {
	fmt.Println("Querying consensus state")
	// todo
	return types.ConsensusState{}, nil
}

type FinalityProof struct {
	/// The hash of block F for which justification is provided.
	Block rpctypes.Hash
	/// Justification of the block F.
	Justification []byte
	/// The set of headers in the range (B; F] that we believe are unknown to the caller. Ordered.
	UnknownHeaders []rpctypes.Header
}

func (fp FinalityProof) Encode(encoder codec.Encoder) error {
	var err error
	err = encoder.Write(fp.Block[:])
	err = encoder.Encode(fp.Justification)
	err = encoder.Encode(fp.UnknownHeaders)

	if err != nil {
		return err
	}
	return nil
}

func (fp *FinalityProof) Decode(decoder *codec.Decoder) error {
	var err error
	err = decoder.Decode(&fp.Block)
	if err != nil {
		return err
	}
	err = decoder.Decode(&fp.Justification)
	if err != nil {
		return err
	}
	err = decoder.Decode(&fp.UnknownHeaders)
	if err != nil {
		return err
	}
	_, err = decoder.ReadOneByte()
	if err == nil {
		return fmt.Errorf("expected end of stream")
	}
	return nil
}

type GrandpaJustification struct {
	/// Current voting round number, monotonically increasing
	Round uint64
	/// Contains block hash & number that's being finalized and the signatures.
	Commit Commit
	/// Contains the path from a [`PreCommit`]'s target hash to the GHOST finalized block.
	VotesAncestries []rpctypes.Header
}

func (gj GrandpaJustification) Encode(encoder codec.Encoder) error {
	var err error
	err = encoder.Encode(gj.Round)
	err = encoder.Encode(gj.Commit)
	err = encoder.Encode(gj.VotesAncestries)
	if err != nil {
		return err
	}
	return nil
}

func (gj *GrandpaJustification) Decode(decoder *codec.Decoder) error {
	var err error
	err = decoder.Decode(&gj.Round)
	if err != nil {
		return err
	}
	err = decoder.Decode(&gj.Commit)
	if err != nil {
		return err
	}
	err = decoder.Decode(&gj.VotesAncestries)
	if err != nil {
		return err
	}
	_, err = decoder.ReadOneByte()
	if err == nil {
		return fmt.Errorf("expected end of stream")
	}
	return nil
}

type Commit struct {
	/// The target block's hash.
	TargetHash rpctypes.Hash
	/// The target block's number.
	TargetNumber uint32
	/// Precommits for target block or any block after it that justify this commit.
	Precommits []SignedPrecommit
}

func (c Commit) Encode(encoder codec.Encoder) error {
	var err error
	err = encoder.Write(c.TargetHash[:])
	err = encoder.Encode(c.TargetNumber)
	err = encoder.Encode(c.Precommits)
	if err != nil {
		return err
	}
	return nil
}

func (c *Commit) Decode(decoder *codec.Decoder) error {
	var err error
	err = decoder.Decode(&c.TargetHash)
	if err != nil {
		return err
	}
	err = decoder.Decode(&c.TargetNumber)
	if err != nil {
		return err
	}
	err = decoder.Decode(&c.Precommits)
	if err != nil {
		return err
	}
	_, err = decoder.ReadOneByte()
	if err == nil {
		return fmt.Errorf("expected end of stream")
	}
	return nil
}

type SignedPrecommit struct {
	/// The precommit message which has been signed.
	Precommit Precommit
	/// The signature on the message.
	Signature rpctypes.Signature
	/// The Id of the signer.
	Id rpctypes.Bytes32
}

func (sp SignedPrecommit) Encode(encoder codec.Encoder) error {
	var err error
	err = encoder.Encode(sp.Precommit)
	err = encoder.Encode(sp.Signature)
	err = encoder.Encode(sp.Id)
	if err != nil {
		return err
	}
	return nil
}

func (sp *SignedPrecommit) Decode(decoder *codec.Decoder) error {
	var err error
	err = decoder.Decode(&sp.Precommit)
	if err != nil {
		return err
	}
	err = decoder.Decode(&sp.Signature)
	if err != nil {
		return err
	}
	err = decoder.Decode(&sp.Id)
	if err != nil {
		return err
	}
	_, err = decoder.ReadOneByte()
	if err == nil {
		return fmt.Errorf("expected end of stream")
	}
	return nil
}

type Precommit struct {
	/// The target block's hash.
	TargetHash rpctypes.Hash
	/// The target block's number
	TargetNumber uint32
}

func (p Precommit) Encode(encoder codec.Encoder) error {
	var err error
	err = encoder.Write(p.TargetHash[:])
	err = encoder.Encode(p.TargetNumber)
	if err != nil {
		return err
	}
	return nil
}

func (p *Precommit) Decode(decoder *codec.Decoder) error {
	var err error
	err = decoder.Decode(&p.TargetHash)
	if err != nil {
		return err
	}
	err = decoder.Decode(&p.TargetNumber)
	if err != nil {
		return err
	}
	_, err = decoder.ReadOneByte()
	if err == nil {
		return fmt.Errorf("expected end of stream")
	}
	return nil
}

func (c *Commit) prettyPrint() {
	fmt.Println("Commit:")
	fmt.Println("  target hash: ", c.TargetHash.Hex())
	fmt.Println("  target number: ", c.TargetNumber)
	fmt.Println("  signatures: ")
	for _, sp := range c.Precommits {
		fmt.Println("    precommit.target_hash: ", sp.Precommit.TargetHash.Hex())
		fmt.Println("    precommit.target_number: ", sp.Precommit.TargetNumber)
		fmt.Println("    signature: ", sp.Signature.Hex())
		fmt.Println("    id: ", rpctypes.HexEncodeToString(sp.Id[:]))
	}
}
func (gj *GrandpaJustification) prettyPrint() {
	fmt.Printf("Round: %d \n", gj.Round)
	gj.Commit.prettyPrint()
	fmt.Printf("VotesAncestries: \n")
	for _, header := range gj.VotesAncestries {
		fmt.Printf("  %s \n", prettyPrintH(&header))
	}
}

func prettyPrintH(h *rpctypes.Header) string {
	var data string
	for _, d := range h.Digest {
		e, err := rpctypes.EncodeToHex(d)
		if err != nil {
			panic(err)
		}
		data += e
	}
	return fmt.Sprintf("Header: \n  parentHash: %s \n  number: %d \n  stateRoot: %s \n  extrinsicsRoot: %s \n  digest: %s \n", h.ParentHash.Hex(), h.Number, h.StateRoot.Hex(), h.ExtrinsicsRoot.Hex(), data)
}

type TimeStampExtWithProof struct {
	/// The timestamp inherent SCALE-encoded bytes. Decode with [`UncheckedExtrinsic`]
	Ext []byte
	/// Merkle-patricia trie existence proof for the extrinsic, this is generated by the relayer.
	Proof TrieProof
}

func (g *Grandpa) fetchTimestampExtrinsicWithProof(blockHash rpctypes.Hash) (TimeStampExtWithProof, error) {
	var extWithProof TimeStampExtWithProof
	var block *rpctypes.SignedBlock
	var err error
	block, err = g.parachainClient.RPC.Chain.GetBlock(blockHash)
	if err != nil {
		return extWithProof, err
	}
	if block == nil {
		return extWithProof, fmt.Errorf("block not found")
	}
	extrinsics := block.Block.Extrinsics
	if len(extrinsics) == 0 {
		return extWithProof, fmt.Errorf("block has no extrinsics")
	}
	extWithProof.Ext, err = rpctypes.Encode(&extrinsics[0])
	if err != nil {
		return extWithProof, err
	}

	db := NewTrieDBMut()
	defer db.Free()
	for i, ext := range extrinsics {
		key, err := rpctypes.Encode(rpctypes.NewUCompactFromUInt(uint64(i)))
		if err != nil {
			return extWithProof, err
		}
		extEncoded, err := rpctypes.Encode(&ext)
		if err != nil {
			return extWithProof, err
		}
		db.Insert(key, extEncoded)
	}
	root := db.Root()
	key, err := rpctypes.Encode(rpctypes.NewUCompactFromUInt(0))
	extWithProof.Proof = db.GenerateTrieProof(root, key)
	return extWithProof, nil
}

type TrieDBMut struct {
	db   unsafe.Pointer
	root unsafe.Pointer
	trie unsafe.Pointer
}

func NewTrieDBMut() *TrieDBMut {
	var db unsafe.Pointer
	var root unsafe.Pointer
	var trie unsafe.Pointer

	C.ext_TrieDBMut_LayoutV0_BlakeTwo256_new(&trie, &db, &root)
	return &TrieDBMut{
		db:   db,
		root: root,
		trie: trie,
	}
}

func (t *TrieDBMut) Free() {
	C.ext_TrieDBMut_LayoutV0_BlakeTwo256_free(t.trie, t.db, t.root)
}

func (t *TrieDBMut) Insert(key []byte, value []byte) {
	C.ext_TrieDBMut_LayoutV0_BlakeTwo256_insert(
		t.trie,
		(*C.uint8_t)(unsafe.Pointer(&key[0])),
		C.size_t(len(key)),
		(*C.uint8_t)(unsafe.Pointer(&value[0])),
		C.size_t(len(value)),
	)
}

func (t *TrieDBMut) Root() rpctypes.Hash {
	var root rpctypes.Hash
	C.ext_TrieDBMut_LayoutV0_BlakeTwo256_root(t.trie, unsafe.Pointer(&root))
	return root
}

type TrieProof struct {
	// The proof is built from `items` and `lengths` fields and points to the data in `items`
	Proof   [][]byte
	items   []*C.uchar
	lengths []C.size_t
}

// NewTrieProofFromRaw construct TrieProof
// # Safety
// TODO
// Ref: https://zchee.github.io/golang-wiki/cgo/#turning-c-arrays-into-go-slices
func NewTrieProofFromRaw(proofItemsAddr **C.uint8_t, proofLensAddr *C.size_t, proofLen uint) TrieProof {
	items := unsafe.Slice(&*proofItemsAddr, proofLen)
	lens := unsafe.Slice(&*proofLensAddr, proofLen)

	proof := make([][]byte, proofLen)
	for i := 0; i < int(proofLen); i++ {
		proof[i] = C.GoBytes(
			unsafe.Pointer(items[i]),
			C.int(lens[i]),
		)
	}

	return TrieProof{
		Proof:   proof,
		items:   items,
		lengths: lens,
	}
}

func NewTrieProof(proof [][]byte) TrieProof {
	items := make([]*C.uchar, len(proof))
	lengths := make([]C.size_t, len(proof))
	for i := 0; i < len(proof); i++ {
		items[i] = (*C.uchar)(unsafe.Pointer(&proof[i][0]))
		lengths[i] = C.size_t(len(proof[i]))
	}

	return TrieProof{
		Proof:   proof,
		items:   items,
		lengths: lengths,
	}
}

func (t *TrieProof) Free() {
	C.free(unsafe.Pointer(&t.items[0]))
	C.free(unsafe.Pointer(&t.lengths[0]))
}

func (t *TrieDBMut) GenerateTrieProof(root rpctypes.Hash, key []byte) TrieProof {
	var proofItemsAddr **C.uint8_t
	var proofLensAddr *C.size_t
	var proofLen C.size_t

	rootPtr := unsafe.Pointer(&root)
	C.ext_TrieDBMut_LayoutV0_BlakeTwo256_generate_trie_proof(
		t.db,
		rootPtr,
		(*C.uint8_t)(unsafe.Pointer(&key[0])),
		C.size_t(len(key)),
		&proofItemsAddr,
		&proofLensAddr,
		&proofLen,
	)

	return NewTrieProofFromRaw(proofItemsAddr, proofLensAddr, uint(proofLen))
}

// VerifyTrieProof Proof.items and Proof.lengths fields are used as pointers to proof data
func (t *TrieDBMut) VerifyTrieProof(
	root rpctypes.Hash,
	key []byte,
	value []byte,
	proof TrieProof,
) bool {
	if len(proof.items) != len(proof.lengths) {
		panic("proof items and lengths length mismatch")
	}
	rootPtr := unsafe.Pointer(&root)
	keyPtr := unsafe.Pointer(&key[0])
	valuePtr := unsafe.Pointer(&value[0])
	proofItemsAddr := unsafe.Pointer(&proof.items[0])
	proofLensAddr := unsafe.Pointer(&proof.lengths[0])
	proofLen := C.size_t(len(proof.items))

	return bool(C.ext_TrieDBMut_LayoutV0_BlakeTwo256_verify_trie_proof(
		rootPtr,
		(*C.uint8_t)(keyPtr),
		C.size_t(len(key)),
		(*C.uint8_t)(valuePtr),
		C.size_t(len(value)),
		(**C.uint8_t)(proofItemsAddr),
		(*C.size_t)(proofLensAddr),
		proofLen,
	))
}

func ReadProofCheck(root rpctypes.Hash, proof TrieProof, key []byte) ([]byte, bool) {
	var value *C.uint8_t
	var valueLen C.size_t

	res := bool(C.ext_TrieDBMut_LayoutV0_BlakeTwo256_read_proof_check(
		unsafe.Pointer(&root),
		(*C.uint8_t)(unsafe.Pointer(&key[0])),
		C.size_t(len(key)),
		(**C.uint8_t)(unsafe.Pointer(&proof.items[0])),
		(*C.size_t)(unsafe.Pointer(&proof.lengths[0])),
		C.size_t(len(proof.items)),
		&value,
		&valueLen,
	))
	if !res {
		return nil, false
	}
	return C.GoBytes(unsafe.Pointer(value), C.int(valueLen)), true
}
