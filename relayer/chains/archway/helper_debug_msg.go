package archway

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/cosmos/relayer/v2/relayer/chains/icon/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
)

var ArchwayDebugMessagePath = filepath.Join(os.Getenv("HOME"), ".relayer", "debug_archway_msg_data.json")

// for saving data in particular format
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

	fmt.Printf("Successfully created or appended JSON in %s", filename)
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

func SaveMsgToFile(filename string, msgs []provider.RelayerMessage) {
	type DataFormat struct {
		Step    string         `json:"step"`
		Update  types.HexBytes `json:"update"`
		Message types.HexBytes `json:"message"`
	}

	if len(msgs) == 0 {
		return
	}

	var d []DataFormat
	err := readExistingData(filename, &d)
	if err != nil {
		fmt.Println("error savetoFile ")
		return
	}

	var update types.HexBytes
	// update on msg n will be added to n+1 message
	for _, m := range msgs {
		if m == nil {
			continue
		}
		b, _ := m.MsgBytes()
		if m.Type() == "update_client" {
			update = types.NewHexBytes(b)
			continue
		}
		d = append(d, DataFormat{Step: m.Type(), Update: update, Message: types.NewHexBytes(b)})
		// resetting update
		update = ""
	}
	jsonDumpDataFile(filename, d)
}
