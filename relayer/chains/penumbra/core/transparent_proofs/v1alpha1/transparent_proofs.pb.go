// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: penumbra/core/transparent_proofs/v1alpha1/transparent_proofs.proto

package transparent_proofsv1alpha1

import (
	fmt "fmt"
	proto "github.com/cosmos/gogoproto/proto"
	v1alpha1 "github.com/cosmos/relayer/v2/relayer/chains/penumbra/core/crypto/v1alpha1"
	v1alpha11 "github.com/cosmos/relayer/v2/relayer/chains/penumbra/core/dex/v1alpha1"
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

// A Penumbra transparent Spend Proof.
type SpendProof struct {
	// Auxiliary inputs
	StateCommitmentProof *v1alpha1.StateCommitmentProof `protobuf:"bytes,1,opt,name=state_commitment_proof,json=stateCommitmentProof,proto3" json:"state_commitment_proof,omitempty"`
	//
	// @exclude
	// From the note being spent
	Note                *v1alpha1.Note `protobuf:"bytes,2,opt,name=note,proto3" json:"note,omitempty"`
	VBlinding           []byte         `protobuf:"bytes,6,opt,name=v_blinding,json=vBlinding,proto3" json:"v_blinding,omitempty"`
	SpendAuthRandomizer []byte         `protobuf:"bytes,9,opt,name=spend_auth_randomizer,json=spendAuthRandomizer,proto3" json:"spend_auth_randomizer,omitempty"`
	Ak                  []byte         `protobuf:"bytes,10,opt,name=ak,proto3" json:"ak,omitempty"`
	Nk                  []byte         `protobuf:"bytes,11,opt,name=nk,proto3" json:"nk,omitempty"`
}

func (m *SpendProof) Reset()         { *m = SpendProof{} }
func (m *SpendProof) String() string { return proto.CompactTextString(m) }
func (*SpendProof) ProtoMessage()    {}
func (*SpendProof) Descriptor() ([]byte, []int) {
	return fileDescriptor_1536b20e10cd99e5, []int{0}
}
func (m *SpendProof) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *SpendProof) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_SpendProof.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *SpendProof) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SpendProof.Merge(m, src)
}
func (m *SpendProof) XXX_Size() int {
	return m.Size()
}
func (m *SpendProof) XXX_DiscardUnknown() {
	xxx_messageInfo_SpendProof.DiscardUnknown(m)
}

var xxx_messageInfo_SpendProof proto.InternalMessageInfo

func (m *SpendProof) GetStateCommitmentProof() *v1alpha1.StateCommitmentProof {
	if m != nil {
		return m.StateCommitmentProof
	}
	return nil
}

func (m *SpendProof) GetNote() *v1alpha1.Note {
	if m != nil {
		return m.Note
	}
	return nil
}

func (m *SpendProof) GetVBlinding() []byte {
	if m != nil {
		return m.VBlinding
	}
	return nil
}

func (m *SpendProof) GetSpendAuthRandomizer() []byte {
	if m != nil {
		return m.SpendAuthRandomizer
	}
	return nil
}

func (m *SpendProof) GetAk() []byte {
	if m != nil {
		return m.Ak
	}
	return nil
}

func (m *SpendProof) GetNk() []byte {
	if m != nil {
		return m.Nk
	}
	return nil
}

// A Penumbra transparent SwapClaimProof.
type SwapClaimProof struct {
	// The swap being claimed
	SwapPlaintext *v1alpha11.SwapPlaintext `protobuf:"bytes,1,opt,name=swap_plaintext,json=swapPlaintext,proto3" json:"swap_plaintext,omitempty"`
	// Inclusion proof for the swap commitment
	SwapCommitmentProof *v1alpha1.StateCommitmentProof `protobuf:"bytes,4,opt,name=swap_commitment_proof,json=swapCommitmentProof,proto3" json:"swap_commitment_proof,omitempty"`
	// The nullifier key used to derive the swap nullifier
	Nk []byte `protobuf:"bytes,6,opt,name=nk,proto3" json:"nk,omitempty"`
	//
	// @exclude
	// Describes output amounts
	Lambda_1I *v1alpha1.Amount `protobuf:"bytes,20,opt,name=lambda_1_i,json=lambda1I,proto3" json:"lambda_1_i,omitempty"`
	Lambda_2I *v1alpha1.Amount `protobuf:"bytes,21,opt,name=lambda_2_i,json=lambda2I,proto3" json:"lambda_2_i,omitempty"`
}

func (m *SwapClaimProof) Reset()         { *m = SwapClaimProof{} }
func (m *SwapClaimProof) String() string { return proto.CompactTextString(m) }
func (*SwapClaimProof) ProtoMessage()    {}
func (*SwapClaimProof) Descriptor() ([]byte, []int) {
	return fileDescriptor_1536b20e10cd99e5, []int{1}
}
func (m *SwapClaimProof) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *SwapClaimProof) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_SwapClaimProof.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *SwapClaimProof) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SwapClaimProof.Merge(m, src)
}
func (m *SwapClaimProof) XXX_Size() int {
	return m.Size()
}
func (m *SwapClaimProof) XXX_DiscardUnknown() {
	xxx_messageInfo_SwapClaimProof.DiscardUnknown(m)
}

var xxx_messageInfo_SwapClaimProof proto.InternalMessageInfo

func (m *SwapClaimProof) GetSwapPlaintext() *v1alpha11.SwapPlaintext {
	if m != nil {
		return m.SwapPlaintext
	}
	return nil
}

func (m *SwapClaimProof) GetSwapCommitmentProof() *v1alpha1.StateCommitmentProof {
	if m != nil {
		return m.SwapCommitmentProof
	}
	return nil
}

func (m *SwapClaimProof) GetNk() []byte {
	if m != nil {
		return m.Nk
	}
	return nil
}

func (m *SwapClaimProof) GetLambda_1I() *v1alpha1.Amount {
	if m != nil {
		return m.Lambda_1I
	}
	return nil
}

func (m *SwapClaimProof) GetLambda_2I() *v1alpha1.Amount {
	if m != nil {
		return m.Lambda_2I
	}
	return nil
}

type UndelegateClaimProof struct {
	UnbondingAmount *v1alpha1.Amount `protobuf:"bytes,1,opt,name=unbonding_amount,json=unbondingAmount,proto3" json:"unbonding_amount,omitempty"`
	BalanceBlinding []byte           `protobuf:"bytes,2,opt,name=balance_blinding,json=balanceBlinding,proto3" json:"balance_blinding,omitempty"`
}

func (m *UndelegateClaimProof) Reset()         { *m = UndelegateClaimProof{} }
func (m *UndelegateClaimProof) String() string { return proto.CompactTextString(m) }
func (*UndelegateClaimProof) ProtoMessage()    {}
func (*UndelegateClaimProof) Descriptor() ([]byte, []int) {
	return fileDescriptor_1536b20e10cd99e5, []int{2}
}
func (m *UndelegateClaimProof) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *UndelegateClaimProof) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_UndelegateClaimProof.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *UndelegateClaimProof) XXX_Merge(src proto.Message) {
	xxx_messageInfo_UndelegateClaimProof.Merge(m, src)
}
func (m *UndelegateClaimProof) XXX_Size() int {
	return m.Size()
}
func (m *UndelegateClaimProof) XXX_DiscardUnknown() {
	xxx_messageInfo_UndelegateClaimProof.DiscardUnknown(m)
}

var xxx_messageInfo_UndelegateClaimProof proto.InternalMessageInfo

func (m *UndelegateClaimProof) GetUnbondingAmount() *v1alpha1.Amount {
	if m != nil {
		return m.UnbondingAmount
	}
	return nil
}

func (m *UndelegateClaimProof) GetBalanceBlinding() []byte {
	if m != nil {
		return m.BalanceBlinding
	}
	return nil
}

func init() {
	proto.RegisterType((*SpendProof)(nil), "penumbra.core.transparent_proofs.v1alpha1.SpendProof")
	proto.RegisterType((*SwapClaimProof)(nil), "penumbra.core.transparent_proofs.v1alpha1.SwapClaimProof")
	proto.RegisterType((*UndelegateClaimProof)(nil), "penumbra.core.transparent_proofs.v1alpha1.UndelegateClaimProof")
}

func init() {
	proto.RegisterFile("penumbra/core/transparent_proofs/v1alpha1/transparent_proofs.proto", fileDescriptor_1536b20e10cd99e5)
}

var fileDescriptor_1536b20e10cd99e5 = []byte{
	// 609 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x9c, 0x54, 0x4f, 0x6b, 0xd4, 0x40,
	0x14, 0x6f, 0xd2, 0x52, 0xec, 0x54, 0xdb, 0x92, 0xfe, 0x21, 0x14, 0x0c, 0xa5, 0x2a, 0xb4, 0x8a,
	0x09, 0xbb, 0x15, 0x84, 0x78, 0xea, 0xee, 0x41, 0x7a, 0x50, 0x42, 0x5a, 0x3d, 0xc8, 0x42, 0x78,
	0x49, 0xc6, 0xdd, 0xb0, 0xc9, 0xcc, 0x90, 0x4c, 0xb6, 0xad, 0x9f, 0x42, 0xd0, 0x4f, 0xa0, 0x37,
	0x3f, 0x89, 0x78, 0xea, 0xd1, 0xa3, 0x6c, 0xf1, 0xe2, 0xa7, 0x90, 0x99, 0x24, 0x9b, 0xdd, 0xee,
	0x42, 0x97, 0xde, 0xf2, 0xde, 0xfb, 0xfd, 0x7e, 0xef, 0xbd, 0x5f, 0x66, 0x06, 0xb5, 0x18, 0x26,
	0x79, 0xe2, 0xa7, 0x60, 0x05, 0x34, 0xc5, 0x16, 0x4f, 0x81, 0x64, 0x0c, 0x52, 0x4c, 0xb8, 0xc7,
	0x52, 0x4a, 0x3f, 0x66, 0xd6, 0xa0, 0x01, 0x31, 0xeb, 0x41, 0x63, 0x46, 0xcd, 0x64, 0x29, 0xe5,
	0x54, 0x3b, 0xac, 0x34, 0x4c, 0xa1, 0x61, 0xce, 0xc0, 0x55, 0x1a, 0xbb, 0x4f, 0x27, 0xdb, 0x05,
	0xe9, 0x25, 0xe3, 0xb4, 0x6e, 0x51, 0xc4, 0x85, 0xec, 0xee, 0xe3, 0x49, 0x6c, 0x88, 0x2f, 0x6a,
	0x60, 0x88, 0x2f, 0x0a, 0xd4, 0xfe, 0x77, 0x15, 0xa1, 0x53, 0x86, 0x49, 0xe8, 0x88, 0x56, 0x5a,
	0x84, 0x76, 0x32, 0x0e, 0x1c, 0x7b, 0x01, 0x4d, 0x92, 0x88, 0x27, 0xa3, 0x21, 0x74, 0x65, 0x4f,
	0x39, 0x58, 0x6d, 0x1e, 0x99, 0x93, 0xc3, 0x96, 0x1d, 0x2b, 0x61, 0xf3, 0x54, 0x90, 0xdb, 0x23,
	0xae, 0x14, 0x75, 0xb7, 0xb2, 0x19, 0x59, 0xed, 0x25, 0x5a, 0x22, 0x94, 0x63, 0x5d, 0x95, 0xc2,
	0x8f, 0x6e, 0x11, 0x7e, 0x4b, 0x39, 0x76, 0x25, 0x41, 0x7b, 0x88, 0xd0, 0xc0, 0xf3, 0xe3, 0x88,
	0x84, 0x11, 0xe9, 0xea, 0xcb, 0x7b, 0xca, 0xc1, 0x7d, 0x77, 0x65, 0xd0, 0x2a, 0x13, 0x5a, 0x13,
	0x6d, 0x67, 0x62, 0x21, 0x0f, 0x72, 0xde, 0xf3, 0x52, 0x20, 0x21, 0x4d, 0xa2, 0x4f, 0x38, 0xd5,
	0x57, 0x24, 0x72, 0x53, 0x16, 0x8f, 0x73, 0xde, 0x73, 0x47, 0x25, 0x6d, 0x0d, 0xa9, 0xd0, 0xd7,
	0x91, 0x04, 0xa8, 0xd0, 0x17, 0x31, 0xe9, 0xeb, 0xab, 0x45, 0x4c, 0xfa, 0xfb, 0x7f, 0x55, 0xb4,
	0x76, 0x7a, 0x0e, 0xac, 0x1d, 0x43, 0x94, 0x14, 0xe3, 0x3b, 0x68, 0x2d, 0x3b, 0x07, 0xe6, 0xb1,
	0x18, 0x22, 0xc2, 0xf1, 0x05, 0x2f, 0x1d, 0x3a, 0xbc, 0xb1, 0x88, 0xb0, 0xba, 0xb6, 0xe7, 0x1c,
	0x98, 0x53, 0x11, 0xdc, 0x07, 0xd9, 0x78, 0xa8, 0x75, 0xd1, 0xb6, 0x54, 0x9c, 0xb2, 0x7e, 0xe9,
	0xee, 0xd6, 0x6f, 0x0a, 0xc5, 0x9b, 0xce, 0x17, 0xdb, 0x2d, 0x57, 0xdb, 0x69, 0x6d, 0x84, 0x62,
	0x48, 0xfc, 0x10, 0xbc, 0x86, 0x17, 0xe9, 0x5b, 0xb2, 0xdb, 0x93, 0x5b, 0xba, 0x1d, 0x27, 0x34,
	0x27, 0xdc, 0xbd, 0x57, 0x10, 0x1b, 0x27, 0x63, 0x22, 0x4d, 0x2f, 0xd2, 0xb7, 0xef, 0x20, 0xd2,
	0x3c, 0xd9, 0xff, 0xa2, 0xa0, 0xad, 0x77, 0x24, 0xc4, 0x31, 0xee, 0x8a, 0x65, 0xc6, 0xdd, 0xde,
	0xc8, 0x89, 0x4f, 0xe5, 0x1f, 0xf6, 0x40, 0xd2, 0x4a, 0xbf, 0xe7, 0xec, 0xb1, 0x3e, 0xa2, 0x17,
	0x09, 0xed, 0x10, 0x6d, 0xf8, 0x10, 0x03, 0x09, 0x70, 0x7d, 0x96, 0x54, 0x69, 0xc9, 0x7a, 0x99,
	0xaf, 0x4e, 0x54, 0xeb, 0xeb, 0xe2, 0xcf, 0xa1, 0xa1, 0x5c, 0x0d, 0x0d, 0xe5, 0xcf, 0xd0, 0x50,
	0x3e, 0x5f, 0x1b, 0x0b, 0x57, 0xd7, 0xc6, 0xc2, 0xef, 0x6b, 0x63, 0x01, 0x3d, 0x0f, 0x68, 0x62,
	0xce, 0x7d, 0x7f, 0x5b, 0x3b, 0x67, 0x75, 0x51, 0x2e, 0x96, 0x39, 0xe2, 0x16, 0x3a, 0xca, 0x07,
	0xd6, 0x8d, 0x78, 0x2f, 0xf7, 0xcd, 0x80, 0x26, 0x56, 0x40, 0xb3, 0x84, 0x66, 0x56, 0x8a, 0x63,
	0xb8, 0xc4, 0xa9, 0x35, 0x68, 0x8e, 0x3e, 0x83, 0x1e, 0x44, 0x24, 0xb3, 0xe6, 0x7e, 0x74, 0x5e,
	0x4d, 0xd7, 0xaa, 0xd2, 0x37, 0x75, 0xd1, 0x69, 0x9f, 0xfd, 0x50, 0x0f, 0x9c, 0x6a, 0xfa, 0xb6,
	0x98, 0x7e, 0x6a, 0x40, 0xf3, 0x7d, 0x49, 0xf8, 0x55, 0x43, 0x3b, 0x02, 0xda, 0x99, 0x82, 0x76,
	0x2a, 0xe8, 0x50, 0x7d, 0x31, 0x2f, 0xb4, 0xf3, 0xda, 0x69, 0xbd, 0xc1, 0x1c, 0x42, 0xe0, 0xf0,
	0x4f, 0x7d, 0x56, 0xd1, 0x6c, 0x5b, 0xf0, 0x6c, 0x7b, 0x8a, 0x68, 0xdb, 0x15, 0xd3, 0x5f, 0x96,
	0x2f, 0xd8, 0xd1, 0xff, 0x00, 0x00, 0x00, 0xff, 0xff, 0x85, 0x1d, 0x0f, 0xfc, 0x84, 0x05, 0x00,
	0x00,
}

func (m *SpendProof) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *SpendProof) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *SpendProof) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.Nk) > 0 {
		i -= len(m.Nk)
		copy(dAtA[i:], m.Nk)
		i = encodeVarintTransparentProofs(dAtA, i, uint64(len(m.Nk)))
		i--
		dAtA[i] = 0x5a
	}
	if len(m.Ak) > 0 {
		i -= len(m.Ak)
		copy(dAtA[i:], m.Ak)
		i = encodeVarintTransparentProofs(dAtA, i, uint64(len(m.Ak)))
		i--
		dAtA[i] = 0x52
	}
	if len(m.SpendAuthRandomizer) > 0 {
		i -= len(m.SpendAuthRandomizer)
		copy(dAtA[i:], m.SpendAuthRandomizer)
		i = encodeVarintTransparentProofs(dAtA, i, uint64(len(m.SpendAuthRandomizer)))
		i--
		dAtA[i] = 0x4a
	}
	if len(m.VBlinding) > 0 {
		i -= len(m.VBlinding)
		copy(dAtA[i:], m.VBlinding)
		i = encodeVarintTransparentProofs(dAtA, i, uint64(len(m.VBlinding)))
		i--
		dAtA[i] = 0x32
	}
	if m.Note != nil {
		{
			size, err := m.Note.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintTransparentProofs(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0x12
	}
	if m.StateCommitmentProof != nil {
		{
			size, err := m.StateCommitmentProof.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintTransparentProofs(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *SwapClaimProof) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *SwapClaimProof) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *SwapClaimProof) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.Lambda_2I != nil {
		{
			size, err := m.Lambda_2I.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintTransparentProofs(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0x1
		i--
		dAtA[i] = 0xaa
	}
	if m.Lambda_1I != nil {
		{
			size, err := m.Lambda_1I.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintTransparentProofs(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0x1
		i--
		dAtA[i] = 0xa2
	}
	if len(m.Nk) > 0 {
		i -= len(m.Nk)
		copy(dAtA[i:], m.Nk)
		i = encodeVarintTransparentProofs(dAtA, i, uint64(len(m.Nk)))
		i--
		dAtA[i] = 0x32
	}
	if m.SwapCommitmentProof != nil {
		{
			size, err := m.SwapCommitmentProof.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintTransparentProofs(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0x22
	}
	if m.SwapPlaintext != nil {
		{
			size, err := m.SwapPlaintext.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintTransparentProofs(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *UndelegateClaimProof) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *UndelegateClaimProof) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *UndelegateClaimProof) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.BalanceBlinding) > 0 {
		i -= len(m.BalanceBlinding)
		copy(dAtA[i:], m.BalanceBlinding)
		i = encodeVarintTransparentProofs(dAtA, i, uint64(len(m.BalanceBlinding)))
		i--
		dAtA[i] = 0x12
	}
	if m.UnbondingAmount != nil {
		{
			size, err := m.UnbondingAmount.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintTransparentProofs(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func encodeVarintTransparentProofs(dAtA []byte, offset int, v uint64) int {
	offset -= sovTransparentProofs(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *SpendProof) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.StateCommitmentProof != nil {
		l = m.StateCommitmentProof.Size()
		n += 1 + l + sovTransparentProofs(uint64(l))
	}
	if m.Note != nil {
		l = m.Note.Size()
		n += 1 + l + sovTransparentProofs(uint64(l))
	}
	l = len(m.VBlinding)
	if l > 0 {
		n += 1 + l + sovTransparentProofs(uint64(l))
	}
	l = len(m.SpendAuthRandomizer)
	if l > 0 {
		n += 1 + l + sovTransparentProofs(uint64(l))
	}
	l = len(m.Ak)
	if l > 0 {
		n += 1 + l + sovTransparentProofs(uint64(l))
	}
	l = len(m.Nk)
	if l > 0 {
		n += 1 + l + sovTransparentProofs(uint64(l))
	}
	return n
}

func (m *SwapClaimProof) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.SwapPlaintext != nil {
		l = m.SwapPlaintext.Size()
		n += 1 + l + sovTransparentProofs(uint64(l))
	}
	if m.SwapCommitmentProof != nil {
		l = m.SwapCommitmentProof.Size()
		n += 1 + l + sovTransparentProofs(uint64(l))
	}
	l = len(m.Nk)
	if l > 0 {
		n += 1 + l + sovTransparentProofs(uint64(l))
	}
	if m.Lambda_1I != nil {
		l = m.Lambda_1I.Size()
		n += 2 + l + sovTransparentProofs(uint64(l))
	}
	if m.Lambda_2I != nil {
		l = m.Lambda_2I.Size()
		n += 2 + l + sovTransparentProofs(uint64(l))
	}
	return n
}

func (m *UndelegateClaimProof) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.UnbondingAmount != nil {
		l = m.UnbondingAmount.Size()
		n += 1 + l + sovTransparentProofs(uint64(l))
	}
	l = len(m.BalanceBlinding)
	if l > 0 {
		n += 1 + l + sovTransparentProofs(uint64(l))
	}
	return n
}

func sovTransparentProofs(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozTransparentProofs(x uint64) (n int) {
	return sovTransparentProofs(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *SpendProof) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowTransparentProofs
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
			return fmt.Errorf("proto: SpendProof: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: SpendProof: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field StateCommitmentProof", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTransparentProofs
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
				return ErrInvalidLengthTransparentProofs
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthTransparentProofs
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.StateCommitmentProof == nil {
				m.StateCommitmentProof = &v1alpha1.StateCommitmentProof{}
			}
			if err := m.StateCommitmentProof.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Note", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTransparentProofs
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
				return ErrInvalidLengthTransparentProofs
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthTransparentProofs
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Note == nil {
				m.Note = &v1alpha1.Note{}
			}
			if err := m.Note.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 6:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field VBlinding", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTransparentProofs
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthTransparentProofs
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthTransparentProofs
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.VBlinding = append(m.VBlinding[:0], dAtA[iNdEx:postIndex]...)
			if m.VBlinding == nil {
				m.VBlinding = []byte{}
			}
			iNdEx = postIndex
		case 9:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field SpendAuthRandomizer", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTransparentProofs
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthTransparentProofs
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthTransparentProofs
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.SpendAuthRandomizer = append(m.SpendAuthRandomizer[:0], dAtA[iNdEx:postIndex]...)
			if m.SpendAuthRandomizer == nil {
				m.SpendAuthRandomizer = []byte{}
			}
			iNdEx = postIndex
		case 10:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Ak", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTransparentProofs
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthTransparentProofs
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthTransparentProofs
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Ak = append(m.Ak[:0], dAtA[iNdEx:postIndex]...)
			if m.Ak == nil {
				m.Ak = []byte{}
			}
			iNdEx = postIndex
		case 11:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Nk", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTransparentProofs
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthTransparentProofs
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthTransparentProofs
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Nk = append(m.Nk[:0], dAtA[iNdEx:postIndex]...)
			if m.Nk == nil {
				m.Nk = []byte{}
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipTransparentProofs(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthTransparentProofs
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
func (m *SwapClaimProof) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowTransparentProofs
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
			return fmt.Errorf("proto: SwapClaimProof: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: SwapClaimProof: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field SwapPlaintext", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTransparentProofs
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
				return ErrInvalidLengthTransparentProofs
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthTransparentProofs
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.SwapPlaintext == nil {
				m.SwapPlaintext = &v1alpha11.SwapPlaintext{}
			}
			if err := m.SwapPlaintext.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field SwapCommitmentProof", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTransparentProofs
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
				return ErrInvalidLengthTransparentProofs
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthTransparentProofs
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.SwapCommitmentProof == nil {
				m.SwapCommitmentProof = &v1alpha1.StateCommitmentProof{}
			}
			if err := m.SwapCommitmentProof.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 6:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Nk", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTransparentProofs
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthTransparentProofs
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthTransparentProofs
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Nk = append(m.Nk[:0], dAtA[iNdEx:postIndex]...)
			if m.Nk == nil {
				m.Nk = []byte{}
			}
			iNdEx = postIndex
		case 20:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Lambda_1I", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTransparentProofs
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
				return ErrInvalidLengthTransparentProofs
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthTransparentProofs
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Lambda_1I == nil {
				m.Lambda_1I = &v1alpha1.Amount{}
			}
			if err := m.Lambda_1I.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 21:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Lambda_2I", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTransparentProofs
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
				return ErrInvalidLengthTransparentProofs
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthTransparentProofs
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Lambda_2I == nil {
				m.Lambda_2I = &v1alpha1.Amount{}
			}
			if err := m.Lambda_2I.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipTransparentProofs(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthTransparentProofs
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
func (m *UndelegateClaimProof) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowTransparentProofs
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
			return fmt.Errorf("proto: UndelegateClaimProof: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: UndelegateClaimProof: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field UnbondingAmount", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTransparentProofs
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
				return ErrInvalidLengthTransparentProofs
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthTransparentProofs
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.UnbondingAmount == nil {
				m.UnbondingAmount = &v1alpha1.Amount{}
			}
			if err := m.UnbondingAmount.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field BalanceBlinding", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTransparentProofs
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthTransparentProofs
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthTransparentProofs
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.BalanceBlinding = append(m.BalanceBlinding[:0], dAtA[iNdEx:postIndex]...)
			if m.BalanceBlinding == nil {
				m.BalanceBlinding = []byte{}
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipTransparentProofs(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthTransparentProofs
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
func skipTransparentProofs(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowTransparentProofs
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
					return 0, ErrIntOverflowTransparentProofs
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
					return 0, ErrIntOverflowTransparentProofs
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
				return 0, ErrInvalidLengthTransparentProofs
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupTransparentProofs
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthTransparentProofs
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthTransparentProofs        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowTransparentProofs          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupTransparentProofs = fmt.Errorf("proto: unexpected end of group")
)