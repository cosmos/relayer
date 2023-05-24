package archway

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
)

func getKey(data string) string {
	length := uint16(len(data)) // Assuming the string length fits within 32 bits

	// Convert the length to big endian format
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, length)
	return fmt.Sprintf("%x%s", buf, data)
}

func byteToInt(b []byte) (int, error) {
	return strconv.Atoi(string(b))

}

func ProcessContractResponse(p wasmtypes.QuerySmartContractStateResponse) ([]byte, error) {
	data := string(p.Data.Bytes())
	trimmedData := strings.ReplaceAll(data, `"`, "")
	return hex.DecodeString(trimmedData)
}
