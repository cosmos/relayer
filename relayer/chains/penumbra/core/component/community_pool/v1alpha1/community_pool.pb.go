// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: penumbra/core/component/community_pool/v1alpha1/community_pool.proto

package community_poolv1alpha1

import (
	context "context"
	fmt "fmt"
	grpc1 "github.com/cosmos/gogoproto/grpc"
	proto "github.com/cosmos/gogoproto/proto"
	v1alpha1 "github.com/cosmos/relayer/v2/relayer/chains/penumbra/core/asset/v1alpha1"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	io "io"
	math "math"
	math_bits "math/bits"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

// CommunityPool parameter data.
type CommunityPoolParameters struct {
	// Whether Community Pool spend proposals are enabled.
	CommunityPoolSpendProposalsEnabled bool `protobuf:"varint,1,opt,name=community_pool_spend_proposals_enabled,json=communityPoolSpendProposalsEnabled,proto3" json:"community_pool_spend_proposals_enabled,omitempty"`
}

func (m *CommunityPoolParameters) Reset()         { *m = CommunityPoolParameters{} }
func (m *CommunityPoolParameters) String() string { return proto.CompactTextString(m) }
func (*CommunityPoolParameters) ProtoMessage()    {}
func (*CommunityPoolParameters) Descriptor() ([]byte, []int) {
	return fileDescriptor_cbd96600126596ed, []int{0}
}
func (m *CommunityPoolParameters) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *CommunityPoolParameters) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_CommunityPoolParameters.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *CommunityPoolParameters) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CommunityPoolParameters.Merge(m, src)
}
func (m *CommunityPoolParameters) XXX_Size() int {
	return m.Size()
}
func (m *CommunityPoolParameters) XXX_DiscardUnknown() {
	xxx_messageInfo_CommunityPoolParameters.DiscardUnknown(m)
}

var xxx_messageInfo_CommunityPoolParameters proto.InternalMessageInfo

func (m *CommunityPoolParameters) GetCommunityPoolSpendProposalsEnabled() bool {
	if m != nil {
		return m.CommunityPoolSpendProposalsEnabled
	}
	return false
}

// CommunityPool genesis state.
type GenesisContent struct {
	// CommunityPool parameters.
	CommunityPoolParams *CommunityPoolParameters `protobuf:"bytes,1,opt,name=community_pool_params,json=communityPoolParams,proto3" json:"community_pool_params,omitempty"`
}

func (m *GenesisContent) Reset()         { *m = GenesisContent{} }
func (m *GenesisContent) String() string { return proto.CompactTextString(m) }
func (*GenesisContent) ProtoMessage()    {}
func (*GenesisContent) Descriptor() ([]byte, []int) {
	return fileDescriptor_cbd96600126596ed, []int{1}
}
func (m *GenesisContent) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *GenesisContent) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_GenesisContent.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *GenesisContent) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GenesisContent.Merge(m, src)
}
func (m *GenesisContent) XXX_Size() int {
	return m.Size()
}
func (m *GenesisContent) XXX_DiscardUnknown() {
	xxx_messageInfo_GenesisContent.DiscardUnknown(m)
}

var xxx_messageInfo_GenesisContent proto.InternalMessageInfo

func (m *GenesisContent) GetCommunityPoolParams() *CommunityPoolParameters {
	if m != nil {
		return m.CommunityPoolParams
	}
	return nil
}

// Requests the list of all asset balances associated with the Community Pool.
type CommunityPoolAssetBalancesRequest struct {
	// The expected chain id (empty string if no expectation).
	ChainId string `protobuf:"bytes,1,opt,name=chain_id,json=chainId,proto3" json:"chain_id,omitempty"`
	// (Optional): The specific asset balances to retrieve, if excluded all will be returned.
	AssetIds []*v1alpha1.AssetId `protobuf:"bytes,2,rep,name=asset_ids,json=assetIds,proto3" json:"asset_ids,omitempty"`
}

func (m *CommunityPoolAssetBalancesRequest) Reset()         { *m = CommunityPoolAssetBalancesRequest{} }
func (m *CommunityPoolAssetBalancesRequest) String() string { return proto.CompactTextString(m) }
func (*CommunityPoolAssetBalancesRequest) ProtoMessage()    {}
func (*CommunityPoolAssetBalancesRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_cbd96600126596ed, []int{2}
}
func (m *CommunityPoolAssetBalancesRequest) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *CommunityPoolAssetBalancesRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_CommunityPoolAssetBalancesRequest.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *CommunityPoolAssetBalancesRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CommunityPoolAssetBalancesRequest.Merge(m, src)
}
func (m *CommunityPoolAssetBalancesRequest) XXX_Size() int {
	return m.Size()
}
func (m *CommunityPoolAssetBalancesRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_CommunityPoolAssetBalancesRequest.DiscardUnknown(m)
}

var xxx_messageInfo_CommunityPoolAssetBalancesRequest proto.InternalMessageInfo

func (m *CommunityPoolAssetBalancesRequest) GetChainId() string {
	if m != nil {
		return m.ChainId
	}
	return ""
}

func (m *CommunityPoolAssetBalancesRequest) GetAssetIds() []*v1alpha1.AssetId {
	if m != nil {
		return m.AssetIds
	}
	return nil
}

// The Community Pool's balance of a single asset.
type CommunityPoolAssetBalancesResponse struct {
	// The balance for a single asset.
	Balance *v1alpha1.Value `protobuf:"bytes,1,opt,name=balance,proto3" json:"balance,omitempty"`
}

func (m *CommunityPoolAssetBalancesResponse) Reset()         { *m = CommunityPoolAssetBalancesResponse{} }
func (m *CommunityPoolAssetBalancesResponse) String() string { return proto.CompactTextString(m) }
func (*CommunityPoolAssetBalancesResponse) ProtoMessage()    {}
func (*CommunityPoolAssetBalancesResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_cbd96600126596ed, []int{3}
}
func (m *CommunityPoolAssetBalancesResponse) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *CommunityPoolAssetBalancesResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_CommunityPoolAssetBalancesResponse.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *CommunityPoolAssetBalancesResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CommunityPoolAssetBalancesResponse.Merge(m, src)
}
func (m *CommunityPoolAssetBalancesResponse) XXX_Size() int {
	return m.Size()
}
func (m *CommunityPoolAssetBalancesResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_CommunityPoolAssetBalancesResponse.DiscardUnknown(m)
}

var xxx_messageInfo_CommunityPoolAssetBalancesResponse proto.InternalMessageInfo

func (m *CommunityPoolAssetBalancesResponse) GetBalance() *v1alpha1.Value {
	if m != nil {
		return m.Balance
	}
	return nil
}

func init() {
	proto.RegisterType((*CommunityPoolParameters)(nil), "penumbra.core.component.community_pool.v1alpha1.CommunityPoolParameters")
	proto.RegisterType((*GenesisContent)(nil), "penumbra.core.component.community_pool.v1alpha1.GenesisContent")
	proto.RegisterType((*CommunityPoolAssetBalancesRequest)(nil), "penumbra.core.component.community_pool.v1alpha1.CommunityPoolAssetBalancesRequest")
	proto.RegisterType((*CommunityPoolAssetBalancesResponse)(nil), "penumbra.core.component.community_pool.v1alpha1.CommunityPoolAssetBalancesResponse")
}

func init() {
	proto.RegisterFile("penumbra/core/component/community_pool/v1alpha1/community_pool.proto", fileDescriptor_cbd96600126596ed)
}

var fileDescriptor_cbd96600126596ed = []byte{
	// 531 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xac, 0x94, 0x41, 0x8b, 0xd3, 0x40,
	0x14, 0xc7, 0x3b, 0x5d, 0x71, 0xbb, 0xb3, 0xe2, 0x21, 0x22, 0xae, 0x3d, 0x84, 0x35, 0xa2, 0xf4,
	0x94, 0xb8, 0xdd, 0x5b, 0xc5, 0x83, 0x8d, 0xb2, 0xee, 0x41, 0x88, 0x29, 0xec, 0x41, 0x0a, 0x61,
	0x9a, 0x3c, 0x6c, 0x20, 0x99, 0x19, 0x67, 0x26, 0x85, 0x82, 0x27, 0x3f, 0x80, 0xf8, 0x19, 0x3c,
	0x7a, 0xf4, 0x4b, 0x28, 0x9e, 0xf6, 0xb8, 0x47, 0x69, 0x6f, 0x7e, 0x0a, 0x99, 0xa4, 0x93, 0x9a,
	0xea, 0x2e, 0x14, 0xbd, 0x94, 0x37, 0xaf, 0xef, 0xf7, 0x7f, 0xef, 0xfd, 0x99, 0x09, 0x7e, 0xc6,
	0x81, 0x16, 0xf9, 0x44, 0x10, 0x2f, 0x66, 0x02, 0xbc, 0x98, 0xe5, 0x9c, 0x51, 0xa0, 0x4a, 0x47,
	0x79, 0x41, 0x53, 0x35, 0x8f, 0x38, 0x63, 0x99, 0x37, 0x3b, 0x22, 0x19, 0x9f, 0x92, 0xa3, 0x8d,
	0xbc, 0xcb, 0x05, 0x53, 0xcc, 0xf2, 0x8c, 0x8a, 0xab, 0x55, 0xdc, 0x5a, 0xc5, 0xdd, 0xa8, 0x36,
	0x2a, 0xdd, 0x5e, 0xb3, 0x2d, 0x91, 0x12, 0xd4, 0xba, 0x47, 0x79, 0xac, 0xa4, 0x9d, 0x1c, 0xdf,
	0xf1, 0x8d, 0x48, 0xc0, 0x58, 0x16, 0x10, 0x41, 0x72, 0x50, 0x20, 0xa4, 0x15, 0xe2, 0x87, 0x4d,
	0xfd, 0x48, 0x72, 0xa0, 0x49, 0xc4, 0x05, 0xe3, 0x4c, 0x92, 0x4c, 0x46, 0x40, 0xc9, 0x24, 0x83,
	0xe4, 0x00, 0x1d, 0xa2, 0x5e, 0x27, 0x74, 0xe2, 0xdf, 0x85, 0x46, 0xba, 0x36, 0x30, 0xa5, 0xcf,
	0xab, 0x4a, 0xe7, 0x03, 0xc2, 0x37, 0x4f, 0x80, 0x82, 0x4c, 0xa5, 0xcf, 0xa8, 0x02, 0xaa, 0xac,
	0x77, 0xf8, 0xf6, 0x46, 0x1b, 0xae, 0x67, 0x90, 0xa5, 0xea, 0x7e, 0xff, 0x85, 0xbb, 0xe5, 0xf2,
	0xee, 0x25, 0xfb, 0x84, 0xb7, 0xe2, 0x3f, 0xfe, 0x90, 0xce, 0x7b, 0x84, 0xef, 0x35, 0x80, 0xa7,
	0xda, 0x9c, 0x21, 0xc9, 0x08, 0x8d, 0x41, 0x86, 0xf0, 0xb6, 0x00, 0xa9, 0xac, 0xbb, 0xb8, 0x13,
	0x4f, 0x49, 0x4a, 0xa3, 0xb4, 0x5a, 0x76, 0x2f, 0xdc, 0x2d, 0xcf, 0xa7, 0x89, 0x35, 0xc4, 0x7b,
	0xa5, 0x9f, 0x51, 0x9a, 0xc8, 0x83, 0xf6, 0xe1, 0x4e, 0x6f, 0xbf, 0xff, 0x60, 0x63, 0xe4, 0xca,
	0xef, 0x7a, 0xbe, 0xb2, 0xc3, 0x69, 0x12, 0x76, 0x48, 0x15, 0x48, 0x27, 0xc6, 0xce, 0x55, 0x33,
	0x48, 0xce, 0xa8, 0x04, 0xeb, 0x09, 0xde, 0x9d, 0x54, 0xb9, 0x95, 0x35, 0xf7, 0xaf, 0xee, 0x73,
	0x46, 0xb2, 0x02, 0x42, 0xc3, 0xf4, 0x2f, 0x10, 0xbe, 0xf1, 0xaa, 0x00, 0x31, 0x1f, 0x81, 0x98,
	0xa5, 0x31, 0x58, 0x5f, 0x11, 0xee, 0x5e, 0xde, 0xd6, 0x0a, 0xff, 0xcd, 0xf8, 0xbf, 0xf9, 0xd8,
	0x1d, 0xfd, 0x57, 0xcd, 0xca, 0x97, 0x47, 0x68, 0xf8, 0x65, 0xe7, 0xdb, 0xc2, 0x46, 0xe7, 0x0b,
	0x1b, 0xfd, 0x58, 0xd8, 0xe8, 0xe3, 0xd2, 0x6e, 0x9d, 0x2f, 0xed, 0xd6, 0xc5, 0xd2, 0x6e, 0xe1,
	0xe3, 0x98, 0xe5, 0xdb, 0x36, 0x1d, 0x5a, 0xcd, 0x2b, 0xa4, 0x1f, 0x4a, 0x80, 0x5e, 0x8b, 0x37,
	0xa9, 0x9a, 0x16, 0x13, 0x4d, 0x79, 0x31, 0x93, 0x39, 0x93, 0x9e, 0x80, 0x8c, 0xcc, 0x41, 0x78,
	0xb3, 0x7e, 0x1d, 0x96, 0xf7, 0x42, 0x7a, 0x5b, 0xbe, 0xfb, 0xc7, 0xcd, 0xbc, 0x49, 0x7f, 0x6a,
	0x5f, 0x0b, 0x7c, 0xdf, 0xff, 0xdc, 0x76, 0x03, 0xb3, 0x82, 0xaf, 0x57, 0xf0, 0xeb, 0x15, 0x1a,
	0x93, 0xba, 0x67, 0x2b, 0xee, 0xfb, 0x1a, 0x18, 0x6b, 0x60, 0x5c, 0x03, 0xe3, 0x06, 0x30, 0x36,
	0xc0, 0xa2, 0x3d, 0xd8, 0x0e, 0x18, 0x9f, 0x04, 0xc3, 0x97, 0xa0, 0x48, 0x42, 0x14, 0xf9, 0xd9,
	0xee, 0x1b, 0x78, 0x30, 0xd0, 0xb4, 0xfe, 0x5d, 0xe1, 0x65, 0xb8, 0xe6, 0x07, 0x03, 0x23, 0x30,
	0xb9, 0x5e, 0x7e, 0x80, 0x8e, 0x7f, 0x05, 0x00, 0x00, 0xff, 0xff, 0xea, 0x31, 0xe6, 0x39, 0x23,
	0x05, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// QueryServiceClient is the client API for QueryService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type QueryServiceClient interface {
	CommunityPoolAssetBalances(ctx context.Context, in *CommunityPoolAssetBalancesRequest, opts ...grpc.CallOption) (QueryService_CommunityPoolAssetBalancesClient, error)
}

type queryServiceClient struct {
	cc grpc1.ClientConn
}

func NewQueryServiceClient(cc grpc1.ClientConn) QueryServiceClient {
	return &queryServiceClient{cc}
}

func (c *queryServiceClient) CommunityPoolAssetBalances(ctx context.Context, in *CommunityPoolAssetBalancesRequest, opts ...grpc.CallOption) (QueryService_CommunityPoolAssetBalancesClient, error) {
	stream, err := c.cc.NewStream(ctx, &_QueryService_serviceDesc.Streams[0], "/penumbra.core.component.community_pool.v1alpha1.QueryService/CommunityPoolAssetBalances", opts...)
	if err != nil {
		return nil, err
	}
	x := &queryServiceCommunityPoolAssetBalancesClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type QueryService_CommunityPoolAssetBalancesClient interface {
	Recv() (*CommunityPoolAssetBalancesResponse, error)
	grpc.ClientStream
}

type queryServiceCommunityPoolAssetBalancesClient struct {
	grpc.ClientStream
}

func (x *queryServiceCommunityPoolAssetBalancesClient) Recv() (*CommunityPoolAssetBalancesResponse, error) {
	m := new(CommunityPoolAssetBalancesResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// QueryServiceServer is the server API for QueryService service.
type QueryServiceServer interface {
	CommunityPoolAssetBalances(*CommunityPoolAssetBalancesRequest, QueryService_CommunityPoolAssetBalancesServer) error
}

// UnimplementedQueryServiceServer can be embedded to have forward compatible implementations.
type UnimplementedQueryServiceServer struct {
}

func (*UnimplementedQueryServiceServer) CommunityPoolAssetBalances(req *CommunityPoolAssetBalancesRequest, srv QueryService_CommunityPoolAssetBalancesServer) error {
	return status.Errorf(codes.Unimplemented, "method CommunityPoolAssetBalances not implemented")
}

func RegisterQueryServiceServer(s grpc1.Server, srv QueryServiceServer) {
	s.RegisterService(&_QueryService_serviceDesc, srv)
}

func _QueryService_CommunityPoolAssetBalances_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(CommunityPoolAssetBalancesRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(QueryServiceServer).CommunityPoolAssetBalances(m, &queryServiceCommunityPoolAssetBalancesServer{stream})
}

type QueryService_CommunityPoolAssetBalancesServer interface {
	Send(*CommunityPoolAssetBalancesResponse) error
	grpc.ServerStream
}

type queryServiceCommunityPoolAssetBalancesServer struct {
	grpc.ServerStream
}

func (x *queryServiceCommunityPoolAssetBalancesServer) Send(m *CommunityPoolAssetBalancesResponse) error {
	return x.ServerStream.SendMsg(m)
}

var _QueryService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "penumbra.core.component.community_pool.v1alpha1.QueryService",
	HandlerType: (*QueryServiceServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "CommunityPoolAssetBalances",
			Handler:       _QueryService_CommunityPoolAssetBalances_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "penumbra/core/component/community_pool/v1alpha1/community_pool.proto",
}

func (m *CommunityPoolParameters) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *CommunityPoolParameters) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *CommunityPoolParameters) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.CommunityPoolSpendProposalsEnabled {
		i--
		if m.CommunityPoolSpendProposalsEnabled {
			dAtA[i] = 1
		} else {
			dAtA[i] = 0
		}
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func (m *GenesisContent) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *GenesisContent) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *GenesisContent) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.CommunityPoolParams != nil {
		{
			size, err := m.CommunityPoolParams.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintCommunityPool(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *CommunityPoolAssetBalancesRequest) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *CommunityPoolAssetBalancesRequest) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *CommunityPoolAssetBalancesRequest) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.AssetIds) > 0 {
		for iNdEx := len(m.AssetIds) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.AssetIds[iNdEx].MarshalToSizedBuffer(dAtA[:i])
				if err != nil {
					return 0, err
				}
				i -= size
				i = encodeVarintCommunityPool(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0x12
		}
	}
	if len(m.ChainId) > 0 {
		i -= len(m.ChainId)
		copy(dAtA[i:], m.ChainId)
		i = encodeVarintCommunityPool(dAtA, i, uint64(len(m.ChainId)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *CommunityPoolAssetBalancesResponse) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *CommunityPoolAssetBalancesResponse) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *CommunityPoolAssetBalancesResponse) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.Balance != nil {
		{
			size, err := m.Balance.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintCommunityPool(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func encodeVarintCommunityPool(dAtA []byte, offset int, v uint64) int {
	offset -= sovCommunityPool(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *CommunityPoolParameters) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.CommunityPoolSpendProposalsEnabled {
		n += 2
	}
	return n
}

func (m *GenesisContent) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.CommunityPoolParams != nil {
		l = m.CommunityPoolParams.Size()
		n += 1 + l + sovCommunityPool(uint64(l))
	}
	return n
}

func (m *CommunityPoolAssetBalancesRequest) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.ChainId)
	if l > 0 {
		n += 1 + l + sovCommunityPool(uint64(l))
	}
	if len(m.AssetIds) > 0 {
		for _, e := range m.AssetIds {
			l = e.Size()
			n += 1 + l + sovCommunityPool(uint64(l))
		}
	}
	return n
}

func (m *CommunityPoolAssetBalancesResponse) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Balance != nil {
		l = m.Balance.Size()
		n += 1 + l + sovCommunityPool(uint64(l))
	}
	return n
}

func sovCommunityPool(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozCommunityPool(x uint64) (n int) {
	return sovCommunityPool(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *CommunityPoolParameters) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowCommunityPool
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: CommunityPoolParameters: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: CommunityPoolParameters: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field CommunityPoolSpendProposalsEnabled", wireType)
			}
			var v int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowCommunityPool
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				v |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			m.CommunityPoolSpendProposalsEnabled = bool(v != 0)
		default:
			iNdEx = preIndex
			skippy, err := skipCommunityPool(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthCommunityPool
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *GenesisContent) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowCommunityPool
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: GenesisContent: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: GenesisContent: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field CommunityPoolParams", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowCommunityPool
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthCommunityPool
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthCommunityPool
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.CommunityPoolParams == nil {
				m.CommunityPoolParams = &CommunityPoolParameters{}
			}
			if err := m.CommunityPoolParams.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipCommunityPool(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthCommunityPool
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *CommunityPoolAssetBalancesRequest) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowCommunityPool
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: CommunityPoolAssetBalancesRequest: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: CommunityPoolAssetBalancesRequest: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field ChainId", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowCommunityPool
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthCommunityPool
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthCommunityPool
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.ChainId = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field AssetIds", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowCommunityPool
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthCommunityPool
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthCommunityPool
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.AssetIds = append(m.AssetIds, &v1alpha1.AssetId{})
			if err := m.AssetIds[len(m.AssetIds)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipCommunityPool(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthCommunityPool
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *CommunityPoolAssetBalancesResponse) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowCommunityPool
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: CommunityPoolAssetBalancesResponse: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: CommunityPoolAssetBalancesResponse: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Balance", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowCommunityPool
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthCommunityPool
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthCommunityPool
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Balance == nil {
				m.Balance = &v1alpha1.Value{}
			}
			if err := m.Balance.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipCommunityPool(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthCommunityPool
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipCommunityPool(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowCommunityPool
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowCommunityPool
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
		case 1:
			iNdEx += 8
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowCommunityPool
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLengthCommunityPool
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupCommunityPool
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthCommunityPool
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthCommunityPool        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowCommunityPool          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupCommunityPool = fmt.Errorf("proto: unexpected end of group")
)
