package wasm

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/cosmos/relayer/v2/relayer/common"
)

func getKey(data string) string {
	return fmt.Sprintf("%s%x", getKeyLength(data), []byte(data))
}

func getKeyLength(data string) string {
	length := uint16(len(data)) // Assuming the string length fits within 32 bits

	// Convert the length to big endian format
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, length)
	return fmt.Sprintf("%x", buf)
}

func byteToInt(b []byte) (int, error) {
	return strconv.Atoi(string(b))

}

func ProcessContractResponse(p *wasmtypes.QuerySmartContractStateResponse) ([]byte, error) {
	var output string
	if err := json.Unmarshal(p.Data.Bytes(), &output); err != nil {
		return nil, err
	}
	return hex.DecodeString(output)
}

func getStorageKeyFromPath(path []byte) []byte {
	return common.MustHexStrToBytes(fmt.Sprintf("%s%x", getKey(STORAGEKEY__Commitments), path))
}
