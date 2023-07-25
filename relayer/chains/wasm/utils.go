package wasm

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

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
	data := string(p.Data.Bytes())
	trimmedData := strings.ReplaceAll(data, `"`, "")
	return hex.DecodeString(trimmedData)
}

func getStorageKeyFromPath(path []byte) []byte {
	return common.MustHexStrToBytes(fmt.Sprintf("%s%x", getKey(STORAGEKEY__Commitments), path))
}
