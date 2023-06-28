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

	"github.com/gorilla/websocket"
	"github.com/icon-project/IBC-Integration/libraries/go/common/icon"
	"github.com/icon-project/goloop/common"
	"github.com/icon-project/goloop/common/codec"

	relayer_common "github.com/cosmos/relayer/v2/relayer/common"
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

type EventLog struct {
	Addr    Address
	Indexed [][]byte
	Data    [][]byte
}

type EventLogStr struct {
	Addr    Address  `json:"scoreAddress"`
	Indexed []string `json:"indexed"`
	Data    []string `json:"data"`
}

type TransactionResult struct {
	To                 Address       `json:"to"`
	CumulativeStepUsed HexInt        `json:"cumulativeStepUsed"`
	StepUsed           HexInt        `json:"stepUsed"`
	StepPrice          HexInt        `json:"stepPrice"`
	EventLogs          []EventLogStr `json:"eventLogs"`
	LogsBloom          HexBytes      `json:"logsBloom"`
	Status             HexInt        `json:"status"`
	Failure            *struct {
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
	Version     HexInt   `json:"version" validate:"required,t_int"`
	FromAddress Address  `json:"from" validate:"required,t_addr_eoa"`
	ToAddress   Address  `json:"to" validate:"required,t_addr"`
	Value       HexInt   `json:"value,omitempty" validate:"optional,t_int"`
	StepLimit   HexInt   `json:"stepLimit" validate:"required,t_int"`
	Timestamp   HexInt   `json:"timestamp" validate:"required,t_int"`
	NetworkID   HexInt   `json:"nid" validate:"required,t_int"`
	Nonce       HexInt   `json:"nonce,omitempty" validate:"optional,t_int"`
	Signature   string   `json:"signature" validate:"required,t_sig"`
	DataType    string   `json:"dataType,omitempty" validate:"optional,call|deploy|message"`
	Data        CallData `json:"data,omitempty"`
	TxHash      HexBytes `json:"-"`
}

type BlockHeaderResult struct {
	StateHash        []byte
	PatchReceiptHash []byte
	ReceiptHash      common.HexBytes
	ExtensionData    []byte
}
type TxResult struct {
	Status             int64
	To                 []byte
	CumulativeStepUsed []byte
	StepUsed           []byte
	StepPrice          []byte
	LogsBloom          []byte
	EventLogs          []EventLog
	ScoreAddress       []byte
	EventLogsHash      common.HexBytes
	TxIndex            HexInt
	BlockHeight        HexInt
}

type CallData struct {
	Method string      `json:"method"`
	Params interface{} `json:"params,omitempty"`
}

type GenericClientParams[T MsgCreateClient | MsgUpdateClient] struct {
	Msg T `json:"msg"`
}

type MsgCreateClient struct {
	ClientState    HexBytes `json:"clientState"`
	ConsensusState HexBytes `json:"consensusState"`
	ClientType     string   `json:"clientType"`
	BtpNetworkId   HexInt   `json:"btpNetworkId"`
}

type MsgUpdateClient struct {
	ClientId      string   `json:"clientId"`
	ClientMessage HexBytes `json:"clientMessage"`
}

type GenericChannelParam[T MsgChannelOpenInit | MsgChannelOpenTry | MsgChannelOpenAck | MsgChannelOpenConfirm | MsgChannelCloseInit | MsgChannelCloseConfirm] struct {
	Msg T `json:"msg"`
}

type MsgChannelCloseConfirm struct {
	PortId      string   `json:"portId"`
	ChannelId   string   `json:"channelId"`
	ProofInit   HexBytes `json:"proofInit"`
	ProofHeight HexBytes `json:"proofHeight"`
}

type MsgChannelCloseInit struct {
	PortId    string `json:"portId"`
	ChannelId string `json:"channelId"`
}

type MsgChannelOpenAck struct {
	PortId                string   `json:"portId"`
	ChannelId             string   `json:"channelId"`
	CounterpartyVersion   string   `json:"counterpartyVersion"`
	CounterpartyChannelId string   `json:"counterpartyChannelId"`
	ProofTry              HexBytes `json:"proofTry"`
	ProofHeight           HexBytes `json:"proofHeight"`
}

type MsgChannelOpenConfirm struct {
	PortId      string   `json:"portId"`
	ChannelId   string   `json:"channelId"`
	ProofAck    HexBytes `json:"proofAck"`
	ProofHeight HexBytes `json:"proofHeight"`
}

type MsgChannelOpenInit struct {
	PortId  string   `json:"portId"`
	Channel HexBytes `json:"channel"` // HexBytes
}

type MsgChannelOpenTry struct {
	PortId              string   `json:"portId"`
	PreviousChannelId   string   `json:"previousChannelId"`
	Channel             HexBytes `json:"channel"`
	CounterpartyVersion string   `json:"counterpartyVersion"`
	ProofInit           HexBytes `json:"proofInit"`
	ProofHeight         HexBytes `json:"proofHeight"`
}

type GenericConnectionParam[T MsgConnectionOpenInit | MsgConnectionOpenTry | MsgConnectionOpenAck | MsgConnectionOpenConfirm] struct {
	Msg T `json:"msg"`
}

type MsgConnectionOpenAck struct {
	ConnectionId             string   `json:"connectionId"`
	ClientStateBytes         HexBytes `json:"clientStateBytes"`
	Version                  HexBytes `json:"version"`
	CounterpartyConnectionID string   `json:"counterpartyConnectionID"`
	ProofTry                 HexBytes `json:"proofTry"`
	ProofClient              HexBytes `json:"proofClient"`
	ProofConsensus           HexBytes `json:"proofConsensus"`
	ProofHeight              HexBytes `json:"proofHeight"`
	ConsensusHeight          HexBytes `json:"consensusHeight"`
}

type MsgConnectionOpenConfirm struct {
	ConnectionId string   `json:"connectionId"`
	ProofAck     HexBytes `json:"proofAck"`
	ProofHeight  HexBytes `json:"proofHeight"`
}

type MsgConnectionOpenInit struct {
	ClientId     string   `json:"clientId"`
	Counterparty HexBytes `json:"counterparty"`
	DelayPeriod  HexInt   `json:"delayPeriod"`
}

type MsgConnectionOpenTry struct {
	PreviousConnectionId string     `json:"previousConnectionId"`
	Counterparty         HexBytes   `json:"counterparty"`
	DelayPeriod          HexInt     `json:"delayPeriod"`
	ClientId             string     `json:"clientId"`
	ClientStateBytes     HexBytes   `json:"clientStateBytes"`
	CounterpartyVersions []HexBytes `json:"counterpartyVersions"`
	ProofInit            HexBytes   `json:"proofInit"`
	ProofClient          HexBytes   `json:"proofClient"`
	ProofConsensus       HexBytes   `json:"proofConsensus"`
	ProofHeight          HexBytes   `json:"proofHeight"`
	ConsensusHeight      HexBytes   `json:"consensusHeight"`
}

type GenericPacketParams[T MsgPacketRecv | MsgPacketAcknowledgement | MsgTimeoutPacket | MsgRequestTimeout] struct {
	Msg T `json:"msg"`
}

type MsgPacketAcknowledgement struct {
	Packet          HexBytes `json:"packet"`
	Acknowledgement HexBytes `json:"acknowledgement"`
	Proof           HexBytes `json:"proof"`
	ProofHeight     HexBytes `json:"proofHeight"`
}

type MsgPacketRecv struct {
	Packet      HexBytes `json:"packet"`
	Proof       HexBytes `json:"proof"`
	ProofHeight HexBytes `json:"proofHeight"`
}

type MsgTimeoutPacket struct {
	Packet           HexBytes `json:"packet"`
	Proof            HexBytes `json:"proof"`
	ProofHeight      HexBytes `json:"proofHeight"`
	NextSequenceRecv HexInt   `json:"nextSequenceRecv"`
}

type MsgRequestTimeout struct {
	Packet      HexBytes `json:"packet"`
	Proof       HexBytes `json:"proof"`
	ProofHeight HexBytes `json:"proofHeight"`
}

type CallParam struct {
	FromAddress Address   `json:"from" validate:"optional,t_addr_eoa"`
	ToAddress   Address   `json:"to" validate:"required,t_addr_score"`
	DataType    string    `json:"dataType" validate:"required,call"`
	Data        *CallData `json:"data"`
	Height      HexInt    `json:"height,omitempty"`
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

type WsReadCallback func(*websocket.Conn, interface{}) error

// BTP Related
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

type BTPBlockHeader struct {
	MainHeight             uint64
	Round                  int32
	NextProofContextHash   []byte
	NetworkSectionToRoot   []*icon.MerkleNode
	NetworkID              uint64
	UpdateNumber           uint64
	PrevNetworkSectionHash []byte
	MessageCount           uint64
	MessageRoot            []byte
	NextProofContext       []byte
}

type Dir int

const (
	DirLeft = Dir(iota)
	DirRight
)

type BTPQueryParam struct {
	Height HexInt `json:"height,omitempty" validate:"optional,t_int"`
	Id     HexInt `json:"id" validate:"required,t_int"`
}

type BTPNetworkTypeInfo struct {
	NetworkTypeName  string   `json:"networkTypeName"`
	NextProofContext HexBytes `json:"nextProofContext"`
	OpenNetworkIDs   []HexInt `json:"openNetworkIDs"`
	NetworkTypeID    HexInt   `json:"networkTypeID"`
}

type ValidatorList struct {
	Validators [][]byte `json:"validators"`
}

type ValidatorSignatures struct {
	Signatures [][]byte `json:"signatures"`
}

type NetworkSection struct {
	Nid          int64
	UpdateNumber int64
	Prev         []byte
	MessageCount int64
	MessageRoot  []byte
}

func NewNetworkSection(
	header *BTPBlockHeader,
) *NetworkSection {
	return &NetworkSection{
		Nid:          int64(header.NetworkID),
		UpdateNumber: int64(header.UpdateNumber),
		Prev:         header.PrevNetworkSectionHash,
		MessageCount: int64(header.MessageCount),
		MessageRoot:  header.MessageRoot,
	}
}

func (h *NetworkSection) Hash() []byte {
	return relayer_common.Sha3keccak256(codec.RLP.MustMarshalToBytes(h))

}
