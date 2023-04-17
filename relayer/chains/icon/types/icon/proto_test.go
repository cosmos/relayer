package icon

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	proto "github.com/cosmos/gogoproto/proto"
)

func TestRegistry(t *testing.T) {

	type IBtpHeader interface {
		proto.Message
	}

	interfaceRegistry := types.NewInterfaceRegistry()

	interfaceRegistry.RegisterInterface("icon.types.v1.BTPHeaderI", (*IBtpHeader)(nil))
	interfaceRegistry.RegisterImplementations((*IBtpHeader)(nil), &BTPHeader{})

	marshaler := codec.NewProtoCodec(interfaceRegistry)

	ifaces := marshaler.InterfaceRegistry().ListAllInterfaces()
	impls := interfaceRegistry.ListImplementations("icon.types.v1.BTPHeaderI")
	if len(ifaces) != 1 && len(impls) != 1 {
		t.Errorf("Error on registering interface and implementation ")
	}
	if ifaces[0] != "icon.types.v1.BTPHeaderI" {
		t.Errorf("Interface list not matching")
	}

	if impls[0] != "/icon.types.v1.BTPHeader" {
		t.Errorf("Implement list is not matching ")
	}

}
