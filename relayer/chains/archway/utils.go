package archway

import (
	"encoding/binary"
	"fmt"
	"strconv"
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
