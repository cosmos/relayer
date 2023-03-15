/*
 * Copyright 2021 ICON Foundation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package types

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	chanTypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	"github.com/gorilla/websocket"
	"github.com/icon-project/goloop/common"

	"github.com/icon-project/icon-bridge/common/intconv"
	"github.com/icon-project/icon-bridge/common/jsonrpc"
)

const (
	JsonrpcApiVersion                                = 3
	JsonrpcErrorCodeSystem         jsonrpc.ErrorCode = -31000
	JsonrpcErrorCodeTxPoolOverflow jsonrpc.ErrorCode = -31001
	JsonrpcErrorCodePending        jsonrpc.ErrorCode = -31002
	JsonrpcErrorCodeExecuting      jsonrpc.ErrorCode = -31003
	JsonrpcErrorCodeNotFound       jsonrpc.ErrorCode = -31004
	JsonrpcErrorLackOfResource     jsonrpc.ErrorCode = -31005
	JsonrpcErrorCodeTimeout        jsonrpc.ErrorCode = -31006
	JsonrpcErrorCodeSystemTimeout  jsonrpc.ErrorCode = -31007
	JsonrpcErrorCodeScore          jsonrpc.ErrorCode = -30000
)

const (
	DuplicateTransactionError = iota + 2000
	TransactionPoolOverflowError
	ExpiredTransactionError
	FutureTransactionError
	TransitionInterruptedError
	InvalidTransactionError
	InvalidQueryError
	InvalidResultError
	NoActiveContractError
	NotContractAddressError
	InvalidPatchDataError
	CommittedTransactionError
)

const (
	ResultStatusSuccess           = "0x1"
	ResultStatusFailureCodeRevert = 32
	ResultStatusFailureCodeEnd    = 99
)

const (
	BMCRelayMethod     = "handleRelayMessage"
	BMCGetStatusMethod = "getStatus"
)

type BlockHeader struct {
	Version                int
	Height                 int64
	Timestamp              int64
	Proposer               []byte
	PrevID                 []byte
	VotesHash              []byte
	NextValidatorsHash     []byte
	PatchTransactionsHash  []byte
	NormalTransactionsHash []byte
	LogsBloom              []byte
	Result                 []byte
}

// type EventLog struct {
// 	Addr    []byte
// 	Indexed [][]byte
// 	Data    [][]byte
// }

type EventLog struct {
	Addr    Address
	Indexed []string
	Data    []string
}

type TransactionResult struct {
	To                 Address `json:"to"`
	CumulativeStepUsed HexInt  `json:"cumulativeStepUsed"`
	StepUsed           HexInt  `json:"stepUsed"`
	StepPrice          HexInt  `json:"stepPrice"`
	EventLogs          []struct {
		Addr    Address  `json:"scoreAddress"`
		Indexed []string `json:"indexed"`
		Data    []string `json:"data"`
	} `json:"eventLogs"`
	LogsBloom HexBytes `json:"logsBloom"`
	Status    HexInt   `json:"status"`
	Failure   *struct {
		CodeValue    HexInt `json:"code"`
		MessageValue string `json:"message"`
	} `json:"failure,omitempty"`
	SCOREAddress Address  `json:"scoreAddress,omitempty"`
	BlockHash    HexBytes `json:"blockHash" validate:"required,t_hash"`
	BlockHeight  HexInt   `json:"blockHeight" validate:"required,t_int"`
	TxIndex      HexInt   `json:"txIndex" validate:"required,t_int"`
	TxHash       HexBytes `json:"txHash" validate:"required,t_int"`
}

type TransactionParam struct {
	Version     HexInt      `json:"version" validate:"required,t_int"`
	FromAddress Address     `json:"from" validate:"required,t_addr_eoa"`
	ToAddress   Address     `json:"to" validate:"required,t_addr"`
	Value       HexInt      `json:"value,omitempty" validate:"optional,t_int"`
	StepLimit   HexInt      `json:"stepLimit" validate:"required,t_int"`
	Timestamp   HexInt      `json:"timestamp" validate:"required,t_int"`
	NetworkID   HexInt      `json:"nid" validate:"required,t_int"`
	Nonce       HexInt      `json:"nonce,omitempty" validate:"optional,t_int"`
	Signature   string      `json:"signature" validate:"required,t_sig"`
	DataType    string      `json:"dataType,omitempty" validate:"optional,call|deploy|message"`
	Data        interface{} `json:"data,omitempty"`
	TxHash      HexBytes    `json:"-"`
}

type CallData struct {
	Method string      `json:"method"`
	Params interface{} `json:"params,omitempty"`
}

type BMCRelayMethodParams struct {
	Prev     string `json:"_prev"`
	Messages string `json:"_msg"`
}

type ClientStateParam struct {
	ClientID string `json:"client_id"`
	Height   string `json:"height"`
}

type ClientState struct {
	ChainId                      string
	TrustLevel                   Fraction
	TrustingPeriod               Duration
	UnbondingPeriod              Duration
	MaxClockDrift                Duration
	FrozenHeight                 Height
	LatestHeight                 Height
	ProofSpecs                   ProofSpec
	AllowUpdateAfterExpiry       bool
	AllowUpdateAfterMisbehaviour bool
}

type ConsensusState struct {
	Timestamp          Duration
	Root               MerkleRoot
	NextValidatorsHash []byte
}

type MerkleRoot struct {
	Hash []byte
}

type Duration struct {
	Seconds big.Int
	Nanos   big.Int
}

type Fraction struct {
	Numerator   big.Int
	Denominator big.Int
}

type ProofSpec struct {
}

type MsgCreateClient struct {
	ClientState    []byte `json:"clientState"`
	ConsensusState []byte `json:"consensusState"`
	ClientType     string `json:"clientType"`
}

type MsgUpdateClient struct {
	ClientId      string `json:"clientId"`
	ClientMessage []byte `json:"clientMessage"`
}

type MsgChannelCloseConfirm struct {
	PortId      string
	ChannelId   string
	ProofInit   []byte
	ProofHeight Height
}

type MsgChannelCloseInit struct {
	PortId    string
	ChannelId string
}

type MsgChannelOpenAck struct {
	PortId                string
	ChannelId             string
	CounterpartyVersion   string
	CounterpartyChannelId string
	ProofTry              []byte
	ProofHeight           Height
}

type MsgChannelOpenConfirm struct {
	PortId      string
	ChannelId   string
	ProofAck    []byte
	ProofHeight Height
}

type MsgChannelOpenInit struct {
	PortId  string
	Channel Channel
}

type MsgChannelOpenTry struct {
	PortId              string
	PreviousChannelId   string
	Channel             Channel
	CounterpartyVersion string
	ProofInit           []byte
	ProofHeight         Height
}

type MsgConnectionOpenAck struct {
	ConnectionId             string
	ClientStateBytes         []byte
	Version                  Version
	CounterpartyConnectionID string
	ProofTry                 []byte
	ProofClient              []byte
	ProofConsensus           []byte
	ProofHeight              Height
	ConsensusHeight          Height
}

type MsgConnectionOpenConfirm struct {
	ConnectionId string
	ProofAck     []byte
	ProofHeight  Height
}

type MsgConnectionOpenInit struct {
	ClientId     string
	Counterparty ConnectionCounterparty
	DelayPeriod  big.Int
}

type MsgConnectionOpenTry struct {
	PreviousConnectionId string
	Counterparty         ConnectionCounterparty
	DelayPeriod          big.Int
	ClientId             string
	ClientStateBytes     []byte
	CounterpartyVersions []Version
	ProofInit            []byte
	ProofClient          []byte
	ProofConsensus       []byte
	ProofHeight          Height
	ConsensusHeight      Height
}

type MsgPacketAcknowledgement struct {
	Packet          Packet
	Acknowledgement []byte
	Proof           []byte
	ProofHeight     Height
}

type MsgPacketRecv struct {
	Packet      Packet
	Proof       []byte
	ProofHeight Height
}

type Version struct {
	Identifier string
	Features   []string
}

type Channel struct {
	State          chanTypes.State
	Ordering       chanTypes.Order
	Counterparty   ChannelCounterparty
	ConnectionHops []string
	Version        string
}

type ChannelCounterparty struct {
	PortId    string
	ChannelId string
}

type ConnectionCounterparty struct {
	ClientId     string
	ConnectionId string
	Prefix       MerklePrefix
}

type MerklePrefix struct {
	KeyPrefix []byte `protobuf:"bytes,1,opt,name=key_prefix,json=keyPrefix,proto3" json:"key_prefix,omitempty" yaml:"key_prefix"`
}

func NewMerklePrefix(keyPrefix []byte) MerklePrefix {
	return MerklePrefix{
		KeyPrefix: keyPrefix,
	}
}

type CallParam struct {
	FromAddress Address   `json:"from" validate:"optional,t_addr_eoa"`
	ToAddress   Address   `json:"to" validate:"required,t_addr_score"`
	DataType    string    `json:"dataType" validate:"required,call"`
	Data        *CallData `json:"data"`
}

// Added to implement RelayerMessage interface
func (c *CallParam) Type() string {
	return c.DataType
}

func (c *CallParam) MsgBytes() ([]byte, error) {
	return nil, nil
}

type AddressParam struct {
	Address Address `json:"address" validate:"required,t_addr"`
	Height  HexInt  `json:"height,omitempty" validate:"optional,t_int"`
}

type BMCStatusParams struct {
	Target string `json:"_link"`
}

type BMCStatus struct {
	TxSeq            HexInt `json:"tx_seq"`
	RxSeq            HexInt `json:"rx_seq"`
	BMRIndex         HexInt `json:"relay_idx"`
	RotateHeight     HexInt `json:"rotate_height"`
	RotateTerm       HexInt `json:"rotate_term"`
	DelayLimit       HexInt `json:"delay_limit"`
	MaxAggregation   HexInt `json:"max_agg"`
	CurrentHeight    HexInt `json:"cur_height"`
	RxHeight         HexInt `json:"rx_height"`
	RxHeightSrc      HexInt `json:"rx_height_src"`
	BlockIntervalSrc HexInt `json:"block_interval_src"`
	BlockIntervalDst HexInt `json:"block_interval_dst"`
}

type TransactionHashParam struct {
	Hash HexBytes `json:"txHash" validate:"required,t_hash"`
}

type BlockHeightParam struct {
	Height HexInt `json:"height" validate:"required,t_int"`
}
type DataHashParam struct {
	Hash HexBytes `json:"hash" validate:"required,t_hash"`
}
type ProofResultParam struct {
	BlockHash HexBytes `json:"hash" validate:"required,t_hash"`
	Index     HexInt   `json:"index" validate:"required,t_int"`
}
type ProofEventsParam struct {
	BlockHash HexBytes `json:"hash" validate:"required,t_hash"`
	Index     HexInt   `json:"index" validate:"required,t_int"`
	Events    []HexInt `json:"events"`
}

type BlockRequest struct {
	Height       HexInt         `json:"height"`
	EventFilters []*EventFilter `json:"eventFilters,omitempty"`
}

type EventFilter struct {
	Addr      Address   `json:"addr,omitempty"`
	Signature string    `json:"event"`
	Indexed   []*string `json:"indexed,omitempty"`
	Data      []*string `json:"data,omitempty"`
}

type BlockNotification struct {
	Hash    HexBytes     `json:"hash"`
	Height  HexInt       `json:"height"`
	Indexes [][]HexInt   `json:"indexes,omitempty"`
	Events  [][][]HexInt `json:"events,omitempty"`
}

type EventRequest struct {
	EventFilter
	Height HexInt `json:"height"`
}

type EventNotification struct {
	Hash   HexBytes `json:"hash"`
	Height HexInt   `json:"height"`
	Index  HexInt   `json:"index"`
	Events []HexInt `json:"events,omitempty"`
}

type WSEvent string

const (
	WSEventInit WSEvent = "WSEventInit"
)

type WSResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
}

// T_BIN_DATA, T_HASH
type HexBytes string

func (hs HexBytes) Value() ([]byte, error) {
	if hs == "" {
		return nil, nil
	}
	return hex.DecodeString(string(hs[2:]))
}
func NewHexBytes(b []byte) HexBytes {
	return HexBytes("0x" + hex.EncodeToString(b))
}

// T_INT
type HexInt string

func (i HexInt) Value() (int64, error) {
	s := string(i)
	if strings.HasPrefix(s, "0x") {
		s = s[2:]
	}
	return strconv.ParseInt(s, 16, 64)
}

func (i HexInt) Int() (int, error) {
	s := string(i)
	if strings.HasPrefix(s, "0x") {
		s = s[2:]
	}
	v, err := strconv.ParseInt(s, 16, 32)
	return int(v), err
}

func (i HexInt) BigInt() (*big.Int, error) {
	bi := new(big.Int)
	if err := intconv.ParseBigInt(bi, string(i)); err != nil {
		return nil, err
	} else {
		return bi, nil
	}
}

func NewHexInt(v int64) HexInt {
	return HexInt("0x" + strconv.FormatInt(v, 16))
}

// T_ADDR_EOA, T_ADDR_SCORE
type Address string

func (a Address) Value() ([]byte, error) {
	var b [21]byte
	switch a[:2] {
	case "cx":
		b[0] = 1
	case "hx":
	default:
		return nil, fmt.Errorf("invalid prefix %s", a[:2])
	}
	n, err := hex.Decode(b[1:], []byte(a[2:]))
	if err != nil {
		return nil, err
	}
	if n != 20 {
		return nil, fmt.Errorf("invalid length %d", n)
	}
	return b[:], nil
}

func NewAddress(b []byte) Address {
	if len(b) != 21 {
		return ""
	}
	switch b[0] {
	case 1:
		return Address("cx" + hex.EncodeToString(b[1:]))
	case 0:
		return Address("hx" + hex.EncodeToString(b[1:]))
	default:
		return ""
	}
}

// T_SIG
type Signature string

type RelayMessage struct {
	ReceiptProofs [][]byte
	//
	height        int64
	eventSequence int64
	numberOfEvent int
}

type ReceiptProof struct {
	Index  int
	Events []byte
	Height int64
}

type Block struct {
	//BlockHash              HexBytes  `json:"block_hash" validate:"required,t_hash"`
	//Version                HexInt    `json:"version" validate:"required,t_int"`
	Height    int64 `json:"height" validate:"required,t_int"`
	Timestamp int64 `json:"time_stamp" validate:"required,t_int"`
	//Proposer               HexBytes  `json:"peer_id" validate:"optional,t_addr_eoa"`
	//PrevID                 HexBytes  `json:"prev_block_hash" validate:"required,t_hash"`
	//NormalTransactionsHash HexBytes  `json:"merkle_tree_root_hash" validate:"required,t_hash"`
	NormalTransactions []struct {
		TxHash HexBytes `json:"txHash"`
		//Version   HexInt   `json:"version"`
		From Address `json:"from"`
		To   Address `json:"to"`
		//Value     HexInt   `json:"value,omitempty" `
		//StepLimit HexInt   `json:"stepLimit"`
		//TimeStamp HexInt   `json:"timestamp"`
		//NID       HexInt   `json:"nid,omitempty"`
		//Nonce     HexInt   `json:"nonce,omitempty"`
		//Signature HexBytes `json:"signature"`
		DataType string          `json:"dataType,omitempty"`
		Data     json.RawMessage `json:"data,omitempty"`
	} `json:"confirmed_transaction_list"`
	//Signature              HexBytes  `json:"signature" validate:"optional,t_hash"`
}

type VerifierOptions struct {
	BlockHeight    uint64         `json:"blockHeight"`
	ValidatorsHash common.HexHash `json:"validatorsHash"`
}

type CommitVoteItem struct {
	Timestamp int64
	Signature common.Signature
}

type CommitVoteList struct {
	Round          int32
	BlockPartSetID *PartSetID
	Items          []CommitVoteItem
}

type PartSetID struct {
	Count uint16
	Hash  []byte
}

type HR struct {
	Height int64
	Round  int32
}

type VoteBase struct {
	HR
	Type           VoteType
	BlockID        []byte
	BlockPartSetID PartSetID
}

type Vote struct {
	VoteBase
	Timestamp int64
}

type VoteType byte

type WsReadCallback func(*websocket.Conn, interface{}) error

type BTPBlockParam struct {
	Height    HexInt `json:"height" validate:"required,t_int"`
	NetworkId HexInt `json:"networkID" validate:"required,t_int"`
}

type BTPNetworkInfoParam struct {
	Height HexInt `json:"height" validate:"optional,t_int"`
	Id     HexInt `json:"id" validate:"required,t_int"`
}

type BTPNotification struct {
	Header string `json:"header"`
	Proof  string `json:"proof,omitempty"`
}

type BTPRequest struct {
	Height    HexInt `json:"height"`
	NetworkID HexInt `json:"networkID"`
	ProofFlag HexInt `json:"proofFlag"`
}

type BTPNetworkInfo struct {
	StartHeight             HexInt   `json:"startHeight"`
	NetworkTypeID           HexInt   `json:"networkTypeID"`
	NetworkName             string   `json:"networkName"`
	Open                    HexInt   `json:"open"`
	Owner                   Address  `json:"owner"`
	NextMessageSN           HexInt   `json:"nextMessageSN"`
	NextProofContextChanged HexInt   `json:"nextProofContextChanged"`
	PrevNSHash              HexBytes `json:"prevNSHash"`
	LastNSHash              HexBytes `json:"lastNSHash"`
	NetworkID               HexInt   `json:"networkID"`
	NetworkTypeName         string   `json:"networkTypeName"`
}

type BTPBlockUpdate struct {
	BTPBlockHeader []byte
	BTPBlockProof  []byte
}

type BTPBlockHeader struct {
	MainHeight             int64
	Round                  int32
	NextProofContextHash   []byte
	NetworkSectionToRoot   [][]byte
	NetworkID              int64
	UpdateNumber           int64
	PrevNetworkSectionHash []byte
	MessageCount           int64
	MessagesRoot           []byte
	NextProofContext       []byte
}

type Packet struct {
	Sequence           big.Int `json:"sequence"`
	SourcePort         string  `json:"sourcePort"`
	SourceChannel      string  `json:"sourceChannel"`
	DestinationPort    string  `json:"destinationPort"`
	DestinationChannel string  `json:"destionationChannel"`
	Data               []byte  `json:"data"`
	TimeoutHeight      Height  `json:"timeoutHeight"`
	Timestamp          big.Int `json:"timestamp"`
}

type Height struct {
	RevisionNumber big.Int `json:"revisionNumber"`
	RevisionHeight big.Int `json:"revisionHeight"`
}
