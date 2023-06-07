package archway

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
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

func jsonDumpDataFile(filename string, bufs interface{}) {
	// Marshal the slice of structs to JSON format
	jsonData, err := json.MarshalIndent(bufs, "", "  ")
	if err != nil {
		fmt.Println("Error marshaling slice of structs to JSON:", err)
		os.Exit(1)
	}

	// Write JSON data to file
	err = ioutil.WriteFile(filename, jsonData, 0644)
	if err != nil {
		fmt.Println("Error writing JSON to file:", err)
		os.Exit(1)
	}

	fmt.Println("Successfully created or appended JSON in headerDump.json")
}

func readExistingData(filename string, opPointer interface{}) error {

	// Check if the JSON file exists
	if _, err := os.Stat(filename); !os.IsNotExist(err) {
		// Read existing JSON data from file
		jsonData, err := ioutil.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("Error reading JSON from file: %v", err)
		}

		// Unmarshal JSON data into a slice of structs
		err = json.Unmarshal(jsonData, opPointer)
		if err != nil {
			return fmt.Errorf("Error unmarshaling JSON data: %v", err)
		}
	}

	return nil
}
