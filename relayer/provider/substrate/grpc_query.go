package substrate

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/types/tx"
	gogogrpc "github.com/gogo/protobuf/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"
	"google.golang.org/grpc/metadata"
)

var _ gogogrpc.ClientConn = &SubstrateProvider{}

var protoCodec = encoding.GetCodec(proto.Name)

// Invoke implements the grpc ClientConn.Invoke method
func (sp *SubstrateProvider) Invoke(ctx context.Context, method string, req, reply interface{}, opts ...grpc.CallOption) (err error) {
	// Two things can happen here:
	// 1. either we're broadcasting a Tx, in which call we call Tendermint's broadcast endpoint directly,
	// 2. or we are querying for state, in which case we call ABCI's Query.

	// In both cases, we don't allow empty request req (it will panic unexpectedly).
	if reflect.ValueOf(req).IsNil() {
		return errors.New("request cannot be nil")
	}

	// Case 1. Broadcasting a Tx.
	if reqProto, ok := req.(*tx.BroadcastTxRequest); ok {
		if !ok {
			return fmt.Errorf("expected %T, got %T", (*tx.BroadcastTxRequest)(nil), req)

		}
		resProto, ok := reply.(*tx.BroadcastTxResponse)
		if !ok {
			return fmt.Errorf("expected %T, got %T", (*tx.BroadcastTxResponse)(nil), req)
		}

		broadcastRes, err := sp.TxServiceBroadcast(ctx, reqProto)
		if err != nil {
			return err
		}
		*resProto = *broadcastRes
		return err
	}

	// Case 2. Querying state.
	inMd, _ := metadata.FromOutgoingContext(ctx)
	abciRes, outMd, err := sp.RunGRPCQuery(ctx, method, req, inMd)
	if err != nil {
		return err
	}

	if err = protoCodec.Unmarshal(abciRes.Value, reply); err != nil {
		return err
	}

	for _, callOpt := range opts {
		header, ok := callOpt.(grpc.HeaderCallOption)
		if !ok {
			continue
		}

		*header.HeaderAddr = outMd
	}

	if sp.Codec.InterfaceRegistry != nil {
		return types.UnpackInterfaces(reply, sp.Codec.Marshaler)
	}

	return nil
}

// NewStream implements the grpc ClientConn.NewStream method
func (sp *SubstrateProvider) NewStream(context.Context, *grpc.StreamDesc, string, ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, fmt.Errorf("streaming rpc not supported")
}

// TxServiceBroadcast is a helper function to broadcast a Tx with the correct gRPC types
// from the tx service. Calls `clientCtx.BroadcastTx` under the hood.
func (sp *SubstrateProvider) TxServiceBroadcast(ctx context.Context, req *tx.BroadcastTxRequest) (*tx.BroadcastTxResponse, error) {
	if req == nil || req.TxBytes == nil {
		return nil, fmt.Errorf("invalid empty tx")
	}

	resp, err := sp.BroadcastTx(context.Background(), req.TxBytes)
	if err != nil {
		return nil, err
	}

	return &tx.BroadcastTxResponse{
		TxResponse: resp,
	}, nil
}
