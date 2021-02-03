package relayer

import (
	"encoding/json"
)

// SendToController is ...
var SendToController func(needReply bool, str string) (string, error)

// ControllerUpcall takes action interface type
func ControllerUpcall(action interface{}) (bool, error) {
	bz, err := json.Marshal(action)
	if err != nil {
		return false, err
	}

	ret, err := SendToController(true, string(bz))
	if err != nil {
		return false, err
	}
	var vi interface{}
	if err = json.Unmarshal([]byte(ret), &vi); err != nil {
		return false, err
	}

	truthy := vi != nil
	if truthy {
		switch v := vi.(type) {
		case bool:
			truthy = v
		case float64:
			truthy = v != 0
		case string:
			truthy = v != ""
		}
	}
	return truthy, nil
}
