// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: penumbra/core/component/chain/v1alpha1/chain.proto

package chainv1alpha1

import (
	context "context"
	fmt "fmt"
	grpc1 "github.com/cosmos/gogoproto/grpc"
	proto "github.com/cosmos/gogoproto/proto"
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

// Global chain configuration data, such as chain ID, epoch duration, etc.
type ChainParameters struct {
	// The identifier of the chain.
	ChainId string `protobuf:"bytes,1,opt,name=chain_id,json=chainId,proto3" json:"chain_id,omitempty"`
	// The duration of each epoch, in number of blocks.
	EpochDuration uint64 `protobuf:"varint,2,opt,name=epoch_duration,json=epochDuration,proto3" json:"epoch_duration,omitempty"`
}

func (m *ChainParameters) Reset()         { *m = ChainParameters{} }
func (m *ChainParameters) String() string { return proto.CompactTextString(m) }
func (*ChainParameters) ProtoMessage()    {}
func (*ChainParameters) Descriptor() ([]byte, []int) {
	return fileDescriptor_aca0b6fc4499b003, []int{0}
}
func (m *ChainParameters) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *ChainParameters) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_ChainParameters.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *ChainParameters) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ChainParameters.Merge(m, src)
}
func (m *ChainParameters) XXX_Size() int {
	return m.Size()
}
func (m *ChainParameters) XXX_DiscardUnknown() {
	xxx_messageInfo_ChainParameters.DiscardUnknown(m)
}

var xxx_messageInfo_ChainParameters proto.InternalMessageInfo

func (m *ChainParameters) GetChainId() string {
	if m != nil {
		return m.ChainId
	}
	return ""
}

func (m *ChainParameters) GetEpochDuration() uint64 {
	if m != nil {
		return m.EpochDuration
	}
	return 0
}

// The ratio between two numbers, used in governance to describe vote thresholds and quorums.
type Ratio struct {
	// The numerator.
	Numerator uint64 `protobuf:"varint,1,opt,name=numerator,proto3" json:"numerator,omitempty"`
	// The denominator.
	Denominator uint64 `protobuf:"varint,2,opt,name=denominator,proto3" json:"denominator,omitempty"`
}

func (m *Ratio) Reset()         { *m = Ratio{} }
func (m *Ratio) String() string { return proto.CompactTextString(m) }
func (*Ratio) ProtoMessage()    {}
func (*Ratio) Descriptor() ([]byte, []int) {
	return fileDescriptor_aca0b6fc4499b003, []int{1}
}
func (m *Ratio) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Ratio) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Ratio.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Ratio) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Ratio.Merge(m, src)
}
func (m *Ratio) XXX_Size() int {
	return m.Size()
}
func (m *Ratio) XXX_DiscardUnknown() {
	xxx_messageInfo_Ratio.DiscardUnknown(m)
}

var xxx_messageInfo_Ratio proto.InternalMessageInfo

func (m *Ratio) GetNumerator() uint64 {
	if m != nil {
		return m.Numerator
	}
	return 0
}

func (m *Ratio) GetDenominator() uint64 {
	if m != nil {
		return m.Denominator
	}
	return 0
}

// Parameters for Fuzzy Message Detection
type FmdParameters struct {
	PrecisionBits   uint32 `protobuf:"varint,1,opt,name=precision_bits,json=precisionBits,proto3" json:"precision_bits,omitempty"`
	AsOfBlockHeight uint64 `protobuf:"varint,2,opt,name=as_of_block_height,json=asOfBlockHeight,proto3" json:"as_of_block_height,omitempty"`
}

func (m *FmdParameters) Reset()         { *m = FmdParameters{} }
func (m *FmdParameters) String() string { return proto.CompactTextString(m) }
func (*FmdParameters) ProtoMessage()    {}
func (*FmdParameters) Descriptor() ([]byte, []int) {
	return fileDescriptor_aca0b6fc4499b003, []int{2}
}
func (m *FmdParameters) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *FmdParameters) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_FmdParameters.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *FmdParameters) XXX_Merge(src proto.Message) {
	xxx_messageInfo_FmdParameters.Merge(m, src)
}
func (m *FmdParameters) XXX_Size() int {
	return m.Size()
}
func (m *FmdParameters) XXX_DiscardUnknown() {
	xxx_messageInfo_FmdParameters.DiscardUnknown(m)
}

var xxx_messageInfo_FmdParameters proto.InternalMessageInfo

func (m *FmdParameters) GetPrecisionBits() uint32 {
	if m != nil {
		return m.PrecisionBits
	}
	return 0
}

func (m *FmdParameters) GetAsOfBlockHeight() uint64 {
	if m != nil {
		return m.AsOfBlockHeight
	}
	return 0
}

// Chain-specific genesis content.
type GenesisContent struct {
	// The ChainParameters present at genesis.
	ChainParams *ChainParameters `protobuf:"bytes,1,opt,name=chain_params,json=chainParams,proto3" json:"chain_params,omitempty"`
}

func (m *GenesisContent) Reset()         { *m = GenesisContent{} }
func (m *GenesisContent) String() string { return proto.CompactTextString(m) }
func (*GenesisContent) ProtoMessage()    {}
func (*GenesisContent) Descriptor() ([]byte, []int) {
	return fileDescriptor_aca0b6fc4499b003, []int{3}
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

func (m *GenesisContent) GetChainParams() *ChainParameters {
	if m != nil {
		return m.ChainParams
	}
	return nil
}

type Epoch struct {
	Index       uint64 `protobuf:"varint,1,opt,name=index,proto3" json:"index,omitempty"`
	StartHeight uint64 `protobuf:"varint,2,opt,name=start_height,json=startHeight,proto3" json:"start_height,omitempty"`
}

func (m *Epoch) Reset()         { *m = Epoch{} }
func (m *Epoch) String() string { return proto.CompactTextString(m) }
func (*Epoch) ProtoMessage()    {}
func (*Epoch) Descriptor() ([]byte, []int) {
	return fileDescriptor_aca0b6fc4499b003, []int{4}
}
func (m *Epoch) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Epoch) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Epoch.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Epoch) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Epoch.Merge(m, src)
}
func (m *Epoch) XXX_Size() int {
	return m.Size()
}
func (m *Epoch) XXX_DiscardUnknown() {
	xxx_messageInfo_Epoch.DiscardUnknown(m)
}

var xxx_messageInfo_Epoch proto.InternalMessageInfo

func (m *Epoch) GetIndex() uint64 {
	if m != nil {
		return m.Index
	}
	return 0
}

func (m *Epoch) GetStartHeight() uint64 {
	if m != nil {
		return m.StartHeight
	}
	return 0
}

type EpochByHeightRequest struct {
	Height uint64 `protobuf:"varint,1,opt,name=height,proto3" json:"height,omitempty"`
}

func (m *EpochByHeightRequest) Reset()         { *m = EpochByHeightRequest{} }
func (m *EpochByHeightRequest) String() string { return proto.CompactTextString(m) }
func (*EpochByHeightRequest) ProtoMessage()    {}
func (*EpochByHeightRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_aca0b6fc4499b003, []int{5}
}
func (m *EpochByHeightRequest) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *EpochByHeightRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_EpochByHeightRequest.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *EpochByHeightRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_EpochByHeightRequest.Merge(m, src)
}
func (m *EpochByHeightRequest) XXX_Size() int {
	return m.Size()
}
func (m *EpochByHeightRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_EpochByHeightRequest.DiscardUnknown(m)
}

var xxx_messageInfo_EpochByHeightRequest proto.InternalMessageInfo

func (m *EpochByHeightRequest) GetHeight() uint64 {
	if m != nil {
		return m.Height
	}
	return 0
}

type EpochByHeightResponse struct {
	Epoch *Epoch `protobuf:"bytes,1,opt,name=epoch,proto3" json:"epoch,omitempty"`
}

func (m *EpochByHeightResponse) Reset()         { *m = EpochByHeightResponse{} }
func (m *EpochByHeightResponse) String() string { return proto.CompactTextString(m) }
func (*EpochByHeightResponse) ProtoMessage()    {}
func (*EpochByHeightResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_aca0b6fc4499b003, []int{6}
}
func (m *EpochByHeightResponse) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *EpochByHeightResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_EpochByHeightResponse.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *EpochByHeightResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_EpochByHeightResponse.Merge(m, src)
}
func (m *EpochByHeightResponse) XXX_Size() int {
	return m.Size()
}
func (m *EpochByHeightResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_EpochByHeightResponse.DiscardUnknown(m)
}

var xxx_messageInfo_EpochByHeightResponse proto.InternalMessageInfo

func (m *EpochByHeightResponse) GetEpoch() *Epoch {
	if m != nil {
		return m.Epoch
	}
	return nil
}

func init() {
	proto.RegisterType((*ChainParameters)(nil), "penumbra.core.component.chain.v1alpha1.ChainParameters")
	proto.RegisterType((*Ratio)(nil), "penumbra.core.component.chain.v1alpha1.Ratio")
	proto.RegisterType((*FmdParameters)(nil), "penumbra.core.component.chain.v1alpha1.FmdParameters")
	proto.RegisterType((*GenesisContent)(nil), "penumbra.core.component.chain.v1alpha1.GenesisContent")
	proto.RegisterType((*Epoch)(nil), "penumbra.core.component.chain.v1alpha1.Epoch")
	proto.RegisterType((*EpochByHeightRequest)(nil), "penumbra.core.component.chain.v1alpha1.EpochByHeightRequest")
	proto.RegisterType((*EpochByHeightResponse)(nil), "penumbra.core.component.chain.v1alpha1.EpochByHeightResponse")
}

func init() {
	proto.RegisterFile("penumbra/core/component/chain/v1alpha1/chain.proto", fileDescriptor_aca0b6fc4499b003)
}

var fileDescriptor_aca0b6fc4499b003 = []byte{
	// 581 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x9c, 0x54, 0x4f, 0x6b, 0xd4, 0x4e,
	0x18, 0x6e, 0x42, 0xb7, 0xbf, 0x5f, 0x67, 0xbb, 0x2d, 0x0c, 0x55, 0xaa, 0x48, 0xa8, 0x81, 0x96,
	0x52, 0x31, 0xa1, 0xeb, 0x41, 0x88, 0x0a, 0x92, 0xa8, 0xab, 0x07, 0x71, 0x4d, 0xc1, 0x43, 0x09,
	0xc4, 0xd9, 0x64, 0xda, 0x0c, 0x6e, 0x66, 0xe2, 0xcc, 0x64, 0x71, 0xbf, 0x83, 0x07, 0xbf, 0x80,
	0x17, 0x8f, 0x7e, 0x12, 0xf1, 0xd4, 0xa3, 0x47, 0xd9, 0xbd, 0x79, 0xf1, 0x2b, 0xc8, 0x4c, 0xfe,
	0xb4, 0xdd, 0xd3, 0xea, 0x65, 0x99, 0xf7, 0xd9, 0xe7, 0x79, 0xde, 0xe7, 0x7d, 0x87, 0x09, 0xe8,
	0x17, 0x98, 0x96, 0xf9, 0x88, 0x23, 0x37, 0x61, 0x1c, 0xbb, 0x09, 0xcb, 0x0b, 0x46, 0x31, 0x95,
	0x6e, 0x92, 0x21, 0x42, 0xdd, 0xc9, 0x11, 0x1a, 0x17, 0x19, 0x3a, 0xaa, 0x4a, 0xa7, 0xe0, 0x4c,
	0x32, 0xb8, 0xdf, 0x68, 0x1c, 0xa5, 0x71, 0x5a, 0x8d, 0x53, 0x91, 0x1a, 0x8d, 0x7d, 0x0c, 0xb6,
	0x02, 0x85, 0x0c, 0x11, 0x47, 0x39, 0x96, 0x98, 0x0b, 0x78, 0x03, 0xfc, 0xaf, 0x49, 0x31, 0x49,
	0x77, 0x8c, 0x5d, 0xe3, 0x60, 0x3d, 0xfc, 0x4f, 0xd7, 0x2f, 0x52, 0xb8, 0x07, 0x36, 0x71, 0xc1,
	0x92, 0x2c, 0x4e, 0x4b, 0x8e, 0x24, 0x61, 0x74, 0xc7, 0xdc, 0x35, 0x0e, 0x56, 0xc3, 0x9e, 0x46,
	0x9f, 0xd4, 0xa0, 0x3d, 0x00, 0x9d, 0x50, 0x9d, 0xe0, 0x2d, 0xb0, 0x4e, 0xcb, 0x1c, 0x73, 0x24,
	0x19, 0xd7, 0x5e, 0xab, 0xe1, 0x05, 0x00, 0x77, 0x41, 0x37, 0xc5, 0x94, 0xe5, 0x84, 0xea, 0xff,
	0x2b, 0xab, 0xcb, 0x90, 0x9d, 0x80, 0xde, 0xb3, 0x3c, 0xbd, 0x94, 0x6d, 0x0f, 0x6c, 0x16, 0x1c,
	0x27, 0x44, 0x10, 0x46, 0xe3, 0x11, 0x91, 0x42, 0xbb, 0xf6, 0xc2, 0x5e, 0x8b, 0xfa, 0x44, 0x0a,
	0x78, 0x07, 0x40, 0x24, 0x62, 0x76, 0x1a, 0x8f, 0xc6, 0x2c, 0x79, 0x17, 0x67, 0x98, 0x9c, 0x65,
	0xb2, 0x6e, 0xb0, 0x85, 0xc4, 0xab, 0x53, 0x5f, 0xe1, 0xcf, 0x35, 0x6c, 0x8f, 0xc1, 0xe6, 0x00,
	0x53, 0x2c, 0x88, 0x08, 0x18, 0x95, 0x98, 0x4a, 0x78, 0x02, 0x36, 0xaa, 0x0d, 0x14, 0xaa, 0x73,
	0xd5, 0xa3, 0xdb, 0xbf, 0xef, 0x2c, 0xb7, 0x53, 0x67, 0x61, 0xa1, 0x61, 0x37, 0x69, 0x01, 0x61,
	0x3f, 0x06, 0x9d, 0xa7, 0x6a, 0x59, 0x70, 0x1b, 0x74, 0x08, 0x4d, 0xf1, 0x87, 0x7a, 0x2f, 0x55,
	0x01, 0x6f, 0x83, 0x0d, 0x21, 0x11, 0x97, 0x57, 0x33, 0x77, 0x35, 0x56, 0xe7, 0x75, 0xc0, 0xb6,
	0x76, 0xf0, 0xa7, 0x15, 0x10, 0xe2, 0xf7, 0x25, 0x16, 0x12, 0x5e, 0x07, 0x6b, 0xb5, 0xa8, 0x72,
	0xac, 0x2b, 0x3b, 0x02, 0xd7, 0x16, 0xf8, 0xa2, 0x60, 0x54, 0x60, 0x18, 0x80, 0x8e, 0xbe, 0xb7,
	0x7a, 0xbe, 0xbb, 0xcb, 0xce, 0xa7, 0xdd, 0xc2, 0x4a, 0xdb, 0xff, 0x6c, 0x80, 0x8d, 0xd7, 0x25,
	0xe6, 0xd3, 0x63, 0xcc, 0x27, 0x24, 0xc1, 0xf0, 0xa3, 0x01, 0x7a, 0x57, 0xfa, 0xc1, 0x87, 0x7f,
	0x65, 0xbc, 0x30, 0xd6, 0xcd, 0x47, 0xff, 0xa8, 0xae, 0x86, 0xf4, 0x7f, 0x9b, 0xdf, 0x66, 0x96,
	0x71, 0x3e, 0xb3, 0x8c, 0x9f, 0x33, 0xcb, 0xf8, 0x34, 0xb7, 0x56, 0xce, 0xe7, 0xd6, 0xca, 0x8f,
	0xb9, 0xb5, 0x02, 0x0e, 0x13, 0x96, 0x2f, 0x69, 0xee, 0x83, 0xea, 0x52, 0xd5, 0xdb, 0x1a, 0x1a,
	0x27, 0x6f, 0xcf, 0x88, 0xcc, 0xca, 0x91, 0xa2, 0xbb, 0x09, 0x13, 0x39, 0x13, 0x2e, 0xc7, 0x63,
	0x34, 0xc5, 0xdc, 0x9d, 0xf4, 0xdb, 0xa3, 0xb6, 0x10, 0xee, 0x72, 0xaf, 0xf7, 0x81, 0x2e, 0x9b,
	0xea, 0x8b, 0xb9, 0x3a, 0x0c, 0x82, 0xe0, 0xab, 0xb9, 0x3f, 0x6c, 0xf2, 0x05, 0x2a, 0x5f, 0xd0,
	0xe6, 0xd3, 0x79, 0x9c, 0x37, 0x35, 0xff, 0xfb, 0x05, 0x31, 0x52, 0xc4, 0xa8, 0x25, 0x46, 0x9a,
	0x18, 0x35, 0xc4, 0x99, 0xd9, 0x5f, 0x8e, 0x18, 0x0d, 0x86, 0xfe, 0x4b, 0x2c, 0x51, 0x8a, 0x24,
	0xfa, 0x65, 0x1e, 0x36, 0x22, 0xcf, 0x53, 0x2a, 0xf5, 0x5b, 0xcb, 0x3c, 0x4f, 0xeb, 0x3c, 0xaf,
	0x11, 0x8e, 0xd6, 0xf4, 0x17, 0xe8, 0xde, 0x9f, 0x00, 0x00, 0x00, 0xff, 0xff, 0xb6, 0xbd, 0x1b,
	0x4f, 0xb7, 0x04, 0x00, 0x00,
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
	// TODO: move to SCT cf sct/src/component/view.rs:9 "make epoch management the responsibility of this component"
	EpochByHeight(ctx context.Context, in *EpochByHeightRequest, opts ...grpc.CallOption) (*EpochByHeightResponse, error)
}

type queryServiceClient struct {
	cc grpc1.ClientConn
}

func NewQueryServiceClient(cc grpc1.ClientConn) QueryServiceClient {
	return &queryServiceClient{cc}
}

func (c *queryServiceClient) EpochByHeight(ctx context.Context, in *EpochByHeightRequest, opts ...grpc.CallOption) (*EpochByHeightResponse, error) {
	out := new(EpochByHeightResponse)
	err := c.cc.Invoke(ctx, "/penumbra.core.component.chain.v1alpha1.QueryService/EpochByHeight", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// QueryServiceServer is the server API for QueryService service.
type QueryServiceServer interface {
	// TODO: move to SCT cf sct/src/component/view.rs:9 "make epoch management the responsibility of this component"
	EpochByHeight(context.Context, *EpochByHeightRequest) (*EpochByHeightResponse, error)
}

// UnimplementedQueryServiceServer can be embedded to have forward compatible implementations.
type UnimplementedQueryServiceServer struct {
}

func (*UnimplementedQueryServiceServer) EpochByHeight(ctx context.Context, req *EpochByHeightRequest) (*EpochByHeightResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method EpochByHeight not implemented")
}

func RegisterQueryServiceServer(s grpc1.Server, srv QueryServiceServer) {
	s.RegisterService(&_QueryService_serviceDesc, srv)
}

func _QueryService_EpochByHeight_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(EpochByHeightRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServiceServer).EpochByHeight(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/penumbra.core.component.chain.v1alpha1.QueryService/EpochByHeight",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServiceServer).EpochByHeight(ctx, req.(*EpochByHeightRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _QueryService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "penumbra.core.component.chain.v1alpha1.QueryService",
	HandlerType: (*QueryServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "EpochByHeight",
			Handler:    _QueryService_EpochByHeight_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "penumbra/core/component/chain/v1alpha1/chain.proto",
}

func (m *ChainParameters) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *ChainParameters) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *ChainParameters) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.EpochDuration != 0 {
		i = encodeVarintChain(dAtA, i, uint64(m.EpochDuration))
		i--
		dAtA[i] = 0x10
	}
	if len(m.ChainId) > 0 {
		i -= len(m.ChainId)
		copy(dAtA[i:], m.ChainId)
		i = encodeVarintChain(dAtA, i, uint64(len(m.ChainId)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *Ratio) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Ratio) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Ratio) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.Denominator != 0 {
		i = encodeVarintChain(dAtA, i, uint64(m.Denominator))
		i--
		dAtA[i] = 0x10
	}
	if m.Numerator != 0 {
		i = encodeVarintChain(dAtA, i, uint64(m.Numerator))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func (m *FmdParameters) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *FmdParameters) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *FmdParameters) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.AsOfBlockHeight != 0 {
		i = encodeVarintChain(dAtA, i, uint64(m.AsOfBlockHeight))
		i--
		dAtA[i] = 0x10
	}
	if m.PrecisionBits != 0 {
		i = encodeVarintChain(dAtA, i, uint64(m.PrecisionBits))
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
	if m.ChainParams != nil {
		{
			size, err := m.ChainParams.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintChain(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *Epoch) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Epoch) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Epoch) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.StartHeight != 0 {
		i = encodeVarintChain(dAtA, i, uint64(m.StartHeight))
		i--
		dAtA[i] = 0x10
	}
	if m.Index != 0 {
		i = encodeVarintChain(dAtA, i, uint64(m.Index))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func (m *EpochByHeightRequest) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *EpochByHeightRequest) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *EpochByHeightRequest) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.Height != 0 {
		i = encodeVarintChain(dAtA, i, uint64(m.Height))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func (m *EpochByHeightResponse) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *EpochByHeightResponse) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *EpochByHeightResponse) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.Epoch != nil {
		{
			size, err := m.Epoch.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintChain(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func encodeVarintChain(dAtA []byte, offset int, v uint64) int {
	offset -= sovChain(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *ChainParameters) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.ChainId)
	if l > 0 {
		n += 1 + l + sovChain(uint64(l))
	}
	if m.EpochDuration != 0 {
		n += 1 + sovChain(uint64(m.EpochDuration))
	}
	return n
}

func (m *Ratio) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Numerator != 0 {
		n += 1 + sovChain(uint64(m.Numerator))
	}
	if m.Denominator != 0 {
		n += 1 + sovChain(uint64(m.Denominator))
	}
	return n
}

func (m *FmdParameters) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.PrecisionBits != 0 {
		n += 1 + sovChain(uint64(m.PrecisionBits))
	}
	if m.AsOfBlockHeight != 0 {
		n += 1 + sovChain(uint64(m.AsOfBlockHeight))
	}
	return n
}

func (m *GenesisContent) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.ChainParams != nil {
		l = m.ChainParams.Size()
		n += 1 + l + sovChain(uint64(l))
	}
	return n
}

func (m *Epoch) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Index != 0 {
		n += 1 + sovChain(uint64(m.Index))
	}
	if m.StartHeight != 0 {
		n += 1 + sovChain(uint64(m.StartHeight))
	}
	return n
}

func (m *EpochByHeightRequest) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Height != 0 {
		n += 1 + sovChain(uint64(m.Height))
	}
	return n
}

func (m *EpochByHeightResponse) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Epoch != nil {
		l = m.Epoch.Size()
		n += 1 + l + sovChain(uint64(l))
	}
	return n
}

func sovChain(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozChain(x uint64) (n int) {
	return sovChain(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *ChainParameters) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowChain
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
			return fmt.Errorf("proto: ChainParameters: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: ChainParameters: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field ChainId", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowChain
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
				return ErrInvalidLengthChain
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthChain
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.ChainId = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field EpochDuration", wireType)
			}
			m.EpochDuration = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowChain
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.EpochDuration |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipChain(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthChain
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
func (m *Ratio) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowChain
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
			return fmt.Errorf("proto: Ratio: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Ratio: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Numerator", wireType)
			}
			m.Numerator = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowChain
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Numerator |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Denominator", wireType)
			}
			m.Denominator = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowChain
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Denominator |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipChain(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthChain
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
func (m *FmdParameters) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowChain
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
			return fmt.Errorf("proto: FmdParameters: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: FmdParameters: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field PrecisionBits", wireType)
			}
			m.PrecisionBits = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowChain
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.PrecisionBits |= uint32(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field AsOfBlockHeight", wireType)
			}
			m.AsOfBlockHeight = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowChain
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.AsOfBlockHeight |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipChain(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthChain
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
				return ErrIntOverflowChain
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
				return fmt.Errorf("proto: wrong wireType = %d for field ChainParams", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowChain
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
				return ErrInvalidLengthChain
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthChain
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.ChainParams == nil {
				m.ChainParams = &ChainParameters{}
			}
			if err := m.ChainParams.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipChain(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthChain
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
func (m *Epoch) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowChain
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
			return fmt.Errorf("proto: Epoch: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Epoch: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Index", wireType)
			}
			m.Index = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowChain
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Index |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field StartHeight", wireType)
			}
			m.StartHeight = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowChain
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.StartHeight |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipChain(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthChain
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
func (m *EpochByHeightRequest) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowChain
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
			return fmt.Errorf("proto: EpochByHeightRequest: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: EpochByHeightRequest: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Height", wireType)
			}
			m.Height = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowChain
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Height |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipChain(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthChain
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
func (m *EpochByHeightResponse) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowChain
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
			return fmt.Errorf("proto: EpochByHeightResponse: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: EpochByHeightResponse: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Epoch", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowChain
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
				return ErrInvalidLengthChain
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthChain
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Epoch == nil {
				m.Epoch = &Epoch{}
			}
			if err := m.Epoch.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipChain(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthChain
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
func skipChain(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowChain
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
					return 0, ErrIntOverflowChain
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
					return 0, ErrIntOverflowChain
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
				return 0, ErrInvalidLengthChain
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupChain
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthChain
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthChain        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowChain          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupChain = fmt.Errorf("proto: unexpected end of group")
)
