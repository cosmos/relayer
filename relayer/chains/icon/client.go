package icon

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cosmos/relayer/v2/relayer/chains/icon/types"
	"go.uber.org/zap"

	"github.com/gorilla/websocket"
	"github.com/icon-project/goloop/client"
	"github.com/icon-project/goloop/common"
	"github.com/icon-project/goloop/common/codec"
	"github.com/icon-project/goloop/common/crypto"
	"github.com/icon-project/goloop/module"
	"github.com/icon-project/goloop/server/jsonrpc"
	"github.com/pkg/errors"
)

const (
	DefaultSendTransactionRetryInterval        = 3 * time.Second        //3sec
	DefaultGetTransactionResultPollingInterval = 500 * time.Millisecond //1.5sec
)

type Wallet interface {
	Sign(data []byte) ([]byte, error)
	Address() string
}

type IClient interface {
	Call(p *types.CallParam, r interface{}) error
	GetBalance(param *types.AddressParam) (*big.Int, error)
	GetBlockByHeight(p *types.BlockHeightParam) (*types.Block, error)
	GetBlockHeaderBytesByHeight(p *types.BlockHeightParam) ([]byte, error)
	GetVotesByHeight(p *types.BlockHeightParam) ([]byte, error)
	GetDataByHash(p *types.DataHashParam) ([]byte, error)
	GetProofForResult(p *types.ProofResultParam) ([][]byte, error)
	GetProofForEvents(p *types.ProofEventsParam) ([][][]byte, error)

	GetBTPHeader(p *types.BTPBlockParam) (string, error)
	GetBTPMessage(p *types.BTPBlockParam) ([]string, error)
	GetBTPProof(p *types.BTPBlockParam) (string, error)
	GetBTPNetworkInfo(p *types.BTPNetworkInfoParam) (*types.BTPNetworkInfo, error)

	MonitorBlock(ctx context.Context, p *types.BlockRequest, cb func(conn *websocket.Conn, v *types.BlockNotification) error, scb func(conn *websocket.Conn), errCb func(*websocket.Conn, error)) error
	MonitorBTP(ctx context.Context, p *types.BTPRequest, cb func(conn *websocket.Conn, v *types.BTPNotification) error, scb func(conn *websocket.Conn), errCb func(*websocket.Conn, error)) error
	MonitorEvent(ctx context.Context, p *types.EventRequest, cb func(conn *websocket.Conn, v *types.EventNotification) error, errCb func(*websocket.Conn, error)) error
	Monitor(ctx context.Context, reqUrl string, reqPtr, respPtr interface{}, cb types.WsReadCallback) error
	CloseAllMonitor()
	CloseMonitor(conn *websocket.Conn)

	GetLastBlock() (*types.Block, error)
	GetBlockHeaderByHeight(height int64) (*types.BlockHeader, error)
	GetValidatorsByHash(hash common.HexHash) ([]common.Address, error)
}

type Client struct {
	*client.JsonRpcClient
	DebugEndPoint string
	conns         map[string]*websocket.Conn
	log           *zap.Logger
	mtx           sync.Mutex
}

var txSerializeExcludes = map[string]bool{"signature": true}

func (c *Client) SignTransaction(w module.Wallet, p *types.TransactionParam) error {
	p.Timestamp = types.NewHexInt(time.Now().UnixNano() / int64(time.Microsecond))
	js, err := json.Marshal(p)
	if err != nil {
		return err
	}

	bs, err := SerializeJSON(js, nil, txSerializeExcludes)
	if err != nil {
		return err
	}
	bs = append([]byte("icx_sendTransaction."), bs...)
	txHash := crypto.SHA3Sum256(bs)
	p.TxHash = types.NewHexBytes(txHash)
	sig, err := w.Sign(txHash)
	if err != nil {
		return err
	}
	p.Signature = base64.StdEncoding.EncodeToString(sig)
	return nil
}

func (c *Client) SendTransaction(p *types.TransactionParam) (*types.HexBytes, error) {
	var result types.HexBytes
	if _, err := c.Do("icx_sendTransaction", p, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) SendTransactionAndWait(p *types.TransactionParam) (*types.HexBytes, error) {
	var result types.HexBytes
	if _, err := c.Do("icx_sendTransactionAndWait", p, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetTransactionResult(p *types.TransactionHashParam) (*types.TransactionResult, error) {
	tr := &types.TransactionResult{}
	if _, err := c.Do("icx_getTransactionResult", p, tr); err != nil {
		return nil, err
	}
	return tr, nil
}

func (c *Client) WaitTransactionResult(p *types.TransactionHashParam) (*types.TransactionResult, error) {
	tr := &types.TransactionResult{}
	if _, err := c.Do("icx_waitTransactionResult", p, tr); err != nil {
		return nil, err
	}
	return tr, nil
}

func (c *Client) Call(p *types.CallParam, r interface{}) error {
	_, err := c.Do("icx_call", p, r)
	return err
}

func (c *Client) WaitForResults(ctx context.Context, thp *types.TransactionHashParam) (txh *types.HexBytes, txr *types.TransactionResult, err error) {
	ticker := time.NewTicker(time.Duration(DefaultGetTransactionResultPollingInterval) * time.Nanosecond)
	retryLimit := 20
	retryCounter := 0
	txh = &thp.Hash
	for {
		defer ticker.Stop()
		select {
		case <-ctx.Done():
			err = errors.New("Context Cancelled ReceiptWait Exiting ")
			return
		case <-ticker.C:
			if retryCounter >= retryLimit {
				err = errors.New("Retry Limit Exceeded while waiting for results of transaction")
				return
			}
			retryCounter++
			txr, err = c.GetTransactionResult(thp)
			if err != nil {
				switch re := err.(type) {
				case *jsonrpc.Error:
					switch re.Code {
					case types.JsonrpcErrorCodePending, types.JsonrpcErrorCodeNotFound, types.JsonrpcErrorCodeExecuting:
						continue
					}
				}
			}
			return
		}
	}
}

func (c *Client) GetLastBlock() (*types.Block, error) {
	result := &types.Block{}
	if _, err := c.Do("icx_getLastBlock", struct{}{}, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) GetBlockByHeight(p *types.BlockHeightParam) (*types.Block, error) {
	result := &types.Block{}
	if _, err := c.Do("icx_getBlockByHeight", p, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) GetBlockHeaderBytesByHeight(p *types.BlockHeightParam) ([]byte, error) {
	var result []byte
	if _, err := c.Do("icx_getBlockHeaderByHeight", p, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) GetVotesByHeight(p *types.BlockHeightParam) ([]byte, error) {
	var result []byte
	if _, err := c.Do("icx_getVotesByHeight", p, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) GetDataByHash(p *types.DataHashParam) ([]byte, error) {
	var result []byte
	_, err := c.Do("icx_getDataByHash", p, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) GetProofForResult(p *types.ProofResultParam) ([][]byte, error) {
	var result [][]byte
	if _, err := c.Do("icx_getProofForResult", p, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) GetProofForEvents(p *types.ProofEventsParam) ([][][]byte, error) {
	var result [][][]byte
	if _, err := c.Do("icx_getProofForEvents", p, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) GetBTPHeader(p *types.BTPBlockParam) (string, error) {
	var header string
	if _, err := c.Do("btp_getHeader", p, &header); err != nil {
		return "", err
	}
	return header, nil
}

func (c *Client) GetBTPMessage(p *types.BTPBlockParam) ([]string, error) {
	var result []string
	if _, err := c.Do("btp_getMessages", p, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) GetBTPProof(p *types.BTPBlockParam) (string, error) {
	var result string
	if _, err := c.Do("btp_getProof", p, &result); err != nil {
		return "", err
	}
	return result, nil
}

func (c *Client) GetBTPNetworkInfo(p *types.BTPNetworkInfoParam) (*types.BTPNetworkInfo, error) {
	result := &types.BTPNetworkInfo{}
	if _, err := c.Do("btp_getNetworkInfo", p, &result); err != nil {
		return nil, err
	}
	return result, nil
}
func (c *Client) GetNetworkTypeInfo(height int64, networkId int64) (*types.BTPNetworkTypeInfo, error) {
	nti := &types.BTPNetworkTypeInfo{}
	param := &types.BTPQueryParam{
		Height: types.NewHexInt(height),
		Id:     types.NewHexInt(networkId),
	}
	if _, err := c.Do("btp_getNetworkTypeInfo", param, nti); err != nil {
		return nil, err
	}
	return nti, nil
}

func (c *Client) MonitorBlock(ctx context.Context, p *types.BlockRequest, cb func(conn *websocket.Conn, v *types.BlockNotification) error, scb func(conn *websocket.Conn), errCb func(*websocket.Conn, error)) error {
	resp := &types.BlockNotification{}
	return c.Monitor(ctx, "/block", p, resp, func(conn *websocket.Conn, v interface{}) error {
		switch t := v.(type) {
		case *types.BlockNotification:
			if err := cb(conn, t); err != nil {
				// c.log.Debugf("MonitorBlock callback return err:%+v", err)
				return err
			}
		case types.WSEvent:
			switch t {
			case types.WSEventInit:
				if scb != nil {
					scb(conn)
				} else {
					return errors.New("Second Callback function (scb) is nil ")
				}
			}
		case error:
			errCb(conn, t)
			return t
		default:
			errCb(conn, fmt.Errorf("not supported type %T", t))
			return errors.New("Not supported type")
		}
		return nil
	})
}

func (c *Client) MonitorEvent(ctx context.Context, p *types.EventRequest, cb func(conn *websocket.Conn, v *types.EventNotification) error, errCb func(*websocket.Conn, error)) error {
	resp := &types.EventNotification{}
	return c.Monitor(ctx, "/event", p, resp, func(conn *websocket.Conn, v interface{}) error {
		switch t := v.(type) {
		case *types.EventNotification:
			if err := cb(conn, t); err != nil {
				c.log.Debug(fmt.Sprintf("MonitorEvent callback return err:%+v", err))
			}
		case error:
			errCb(conn, t)
		default:
			errCb(conn, fmt.Errorf("not supported type %T", t))
		}
		return nil
	})
}

func (c *Client) Monitor(ctx context.Context, reqUrl string, reqPtr, respPtr interface{}, cb types.WsReadCallback) error {
	if cb == nil {
		return fmt.Errorf("callback function cannot be nil")
	}
	conn, err := c.wsConnect(reqUrl, nil)
	if err != nil {
		return err
	}
	defer func() {
		c.log.Debug(fmt.Sprintf("Monitor finish %s", conn.LocalAddr().String()))
		c.wsClose(conn)
	}()
	if err = c.wsRequest(conn, reqPtr); err != nil {
		return err
	}
	if err := cb(conn, types.WSEventInit); err != nil {
		return err
	}
	return c.wsReadJSONLoop(ctx, conn, respPtr, cb)
}

func (c *Client) CloseMonitor(conn *websocket.Conn) {
	c.log.Debug(fmt.Sprintf("CloseMonitor %s", conn.LocalAddr().String()))
	c.wsClose(conn)
}

func (c *Client) CloseAllMonitor() {
	for _, conn := range c.conns {
		c.log.Debug(fmt.Sprintf("CloseAllMonitor %s", conn.LocalAddr().String()))
		c.wsClose(conn)
	}
}

func (c *Client) _addWsConn(conn *websocket.Conn) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	la := conn.LocalAddr().String()
	c.conns[la] = conn
}

func (c *Client) _removeWsConn(conn *websocket.Conn) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	la := conn.LocalAddr().String()
	_, ok := c.conns[la]
	if ok {
		delete(c.conns, la)
	}
}

type wsConnectError struct {
	error
	httpResp *http.Response
}

func (c *Client) wsConnect(reqUrl string, reqHeader http.Header) (*websocket.Conn, error) {
	wsEndpoint := strings.Replace(c.Endpoint, "http", "ws", 1)
	conn, httpResp, err := websocket.DefaultDialer.Dial(wsEndpoint+reqUrl, reqHeader)
	if err != nil {
		wsErr := wsConnectError{error: err}
		wsErr.httpResp = httpResp
		return nil, wsErr
	}
	c._addWsConn(conn)
	return conn, nil
}

type wsRequestError struct {
	error
	wsResp *types.WSResponse
}

func (c *Client) wsRequest(conn *websocket.Conn, reqPtr interface{}) error {
	if reqPtr == nil {
		log.Panicf("reqPtr cannot be nil")
	}
	var err error
	wsResp := &types.WSResponse{}
	if err = conn.WriteJSON(reqPtr); err != nil {
		return wsRequestError{fmt.Errorf("fail to WriteJSON err:%+v", err), nil}
	}

	if err = conn.ReadJSON(wsResp); err != nil {
		return wsRequestError{fmt.Errorf("fail to ReadJSON err:%+v", err), nil}
	}

	if wsResp.Code != 0 {
		return wsRequestError{
			fmt.Errorf("invalid WSResponse code:%d, message:%s", wsResp.Code, wsResp.Message),
			wsResp}
	}
	return nil
}

func (c *Client) wsClose(conn *websocket.Conn) {
	c._removeWsConn(conn)
	if err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
		c.log.Debug(fmt.Sprintf("fail to WriteMessage CloseNormalClosure err:%+v", err))
	}
	if err := conn.Close(); err != nil {
		c.log.Debug(fmt.Sprintf("fail to Close err:%+v", err))
	}
}

func (c *Client) wsRead(conn *websocket.Conn, respPtr interface{}) error {
	mt, r, err := conn.NextReader()
	if err != nil {
		return err
	}
	if mt == websocket.CloseMessage {
		return io.EOF
	}
	return json.NewDecoder(r).Decode(respPtr)
}

func (c *Client) wsReadJSONLoop(ctx context.Context, conn *websocket.Conn, respPtr interface{}, cb types.WsReadCallback) error {
	elem := reflect.ValueOf(respPtr).Elem()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			v := reflect.New(elem.Type())
			ptr := v.Interface()
			if _, ok := c.conns[conn.LocalAddr().String()]; !ok {
				c.log.Debug(fmt.Sprintf("wsReadJSONLoop c.conns[%s] is nil", conn.LocalAddr().String()))
				return errors.New("wsReadJSONLoop c.conns is nil")
			}
			if err := c.wsRead(conn, ptr); err != nil {
				c.log.Debug(fmt.Sprintf("wsReadJSONLoop c.conns[%s] ReadJSON err:%+v", conn.LocalAddr().String(), err))
				if cErr, ok := err.(*websocket.CloseError); !ok || cErr.Code != websocket.CloseNormalClosure {
					cb(conn, err)
				}
				return err
			}
			if err := cb(conn, ptr); err != nil {
				return err
			}
		}

	}
}

func (c *Client) GetBlockHeaderByHeight(height int64) (*types.BlockHeader, error) {
	p := &types.BlockHeightParam{Height: types.NewHexInt(height)}
	b, err := c.GetBlockHeaderBytesByHeight(p)
	if err != nil {
		return nil, err
	}
	var blockHeader types.BlockHeader
	_, err = codec.RLP.UnmarshalFromBytes(b, &blockHeader)
	if err != nil {
		return nil, err
	}
	return &blockHeader, nil
}

func (c *Client) GetValidatorsByHash(hash common.HexHash) ([]common.Address, error) {
	data, err := c.GetDataByHash(&types.DataHashParam{Hash: types.NewHexBytes(hash.Bytes())})
	if err != nil {
		return nil, errors.Wrapf(err, "GetDataByHash; %v", err)
	}
	if !bytes.Equal(hash, crypto.SHA3Sum256(data)) {
		return nil, errors.Errorf(
			"invalid data: hash=%v, data=%v", hash, common.HexBytes(data))
	}
	var validators []common.Address
	_, err = codec.BC.UnmarshalFromBytes(data, &validators)
	if err != nil {
		return nil, errors.Wrapf(err, "Unmarshal Validators: %v", err)
	}
	return validators, nil
}

func (c *Client) GetBalance(param *types.AddressParam) (*big.Int, error) {
	var result types.HexInt
	_, err := c.Do("icx_getBalance", param, &result)
	if err != nil {
		return nil, err
	}
	bInt, err := result.BigInt()
	if err != nil {
		return nil, err
	}
	return bInt, nil
}

const (
	HeaderKeyIconOptions = "Icon-Options"
	IconOptionsDebug     = "debug"
	IconOptionsTimeout   = "timeout"
)

type IconOptions map[string]string

func (opts IconOptions) Set(key, value string) {
	opts[key] = value
}

func (opts IconOptions) Get(key string) string {
	if opts == nil {
		return ""
	}
	v := opts[key]
	if len(v) == 0 {
		return ""
	}
	return v
}

func (opts IconOptions) Del(key string) {
	delete(opts, key)
}

func (opts IconOptions) SetBool(key string, value bool) {
	opts.Set(key, strconv.FormatBool(value))
}

func (opts IconOptions) GetBool(key string) (bool, error) {
	return strconv.ParseBool(opts.Get(key))
}

func (opts IconOptions) SetInt(key string, v int64) {
	opts.Set(key, strconv.FormatInt(v, 10))
}

func (opts IconOptions) GetInt(key string) (int64, error) {
	return strconv.ParseInt(opts.Get(key), 10, 64)
}

func (opts IconOptions) ToHeaderValue() string {
	if opts == nil {
		return ""
	}
	strs := make([]string, len(opts))
	i := 0
	for k, v := range opts {
		strs[i] = fmt.Sprintf("%s=%s", k, v)
		i++
	}
	return strings.Join(strs, ",")
}

func NewIconOptionsByHeader(h http.Header) IconOptions {
	s := h.Get(HeaderKeyIconOptions)
	if s != "" {
		kvs := strings.Split(s, ",")
		m := make(map[string]string)
		for _, kv := range kvs {
			if kv != "" {
				idx := strings.Index(kv, "=")
				if idx > 0 {
					m[kv[:idx]] = kv[(idx + 1):]
				} else {
					m[kv] = ""
				}
			}
		}
		return m
	}
	return nil
}

func (c *Client) MonitorBTP(ctx context.Context, p *types.BTPRequest, cb func(conn *websocket.Conn, v *types.BTPNotification) error, scb func(conn *websocket.Conn), errCb func(*websocket.Conn, error)) error {
	resp := &types.BTPNotification{}
	return c.Monitor(ctx, "/btp", p, resp, func(conn *websocket.Conn, v interface{}) error {
		switch t := v.(type) {
		case *types.BTPNotification:
			if err := cb(conn, t); err != nil {
				// c.log.Debug(fmt.Sprintf("MonitorBTP callback return err:%+v", err))
				return err
			}
		case types.WSEvent:
			c.log.Debug(fmt.Sprintf("MonitorBTP WSEvent %s %+v", conn.LocalAddr().String(), t))
			switch t {
			case types.WSEventInit:
				if scb != nil {
					scb(conn)
				} else {
					return errors.New("Second Callback function (scb) is nil ")
				}
			}
		case error:
			errCb(conn, t)
			return t
		default:
			errCb(conn, fmt.Errorf("not supported type %T", t))
			return errors.New("Not supported type")
		}
		return nil
	})
}

func (c *Client) EstimateStep(param *types.TransactionParamForEstimate) (*types.HexInt, error) {
	if len(c.DebugEndPoint) == 0 {
		return nil, errors.New("UnavailableDebugEndPoint")
	}
	currTime := time.Now().UnixNano() / time.Hour.Microseconds()
	param.Timestamp = types.NewHexInt(currTime)
	var result types.HexInt
	if _, err := c.DoURL(c.DebugEndPoint,
		"debug_estimateStep", param, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func NewClient(uri string, l *zap.Logger) *Client {
	//TODO options {MaxRetrySendTx, MaxRetryGetResult, MaxIdleConnsPerHost, Debug, Dump}
	tr := &http.Transport{MaxIdleConnsPerHost: 1000}
	cl := &http.Client{Transport: tr}
	apiClient := client.NewJsonRpcClient(cl, uri)
	c := &Client{
		JsonRpcClient: apiClient,
		DebugEndPoint: guessDebugEndpoint(uri),
		conns:         make(map[string]*websocket.Conn),
		log:           l,
	}
	opts := IconOptions{}
	opts.SetBool(IconOptionsDebug, true)
	c.CustomHeader[HeaderKeyIconOptions] = opts.ToHeaderValue()
	return c
}
