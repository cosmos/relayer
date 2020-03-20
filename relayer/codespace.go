package relayer

import "fmt"

// GetCodespace returns the configuration for a given path
func GetCodespace(codespace string, code int) (msg string, err error) {
	if cs, ok := codespaces[codespace]; ok {
		if val, ok := cs[code]; ok {
			msg = val
		}
	} else {
		err = fmt.Errorf("codespace for %s(%d) not found in map", codespace, code)
	}
	return
}

var codespaces = map[string]map[int]string{
	"client": {
		1:  "light client already exists",
		2:  "light client not found",
		3:  "light client is frozen due to misbehaviour",
		4:  "consensus state not found",
		5:  "invalid consensus state",
		6:  "client type not found",
		7:  "invalid client type",
		8:  "commitment root not found",
		9:  "invalid block header",
		10: "invalid light client misbehaviour evidence",
		13: "client consensus state verification failed",
		14: "connection state verification failed",
		15: "channel state verification failed",
		16: "packet commitment verification failed",
		17: "packet acknowledgement verification failed",
		18: "packet acknowledgement absence verification failed",
		19: "next sequence receive verification failed",
		20: "self consensus state not found",
	},
	"connection": {
		1: "connection already exists",
		2: "connection not found",
		3: "light client connection paths not found",
		4: "connection path is not associated to the given light client",
		5: "invalid connection state",
		6: "invalid counterparty connection",
		7: "invalid connection",
	},
	"channels": {
		1:  "channel already exists",
		2:  "channel not found",
		3:  "invalid channel",
		4:  "invalid channel state",
		5:  "invalid channel ordering",
		6:  "invalid counterparty channel",
		7:  "channel capability not found",
		8:  "sequence send not found",
		9:  "sequence receive not found",
		10: "invalid packet",
		11: "packet timeout",
		12: "too many connection hops",
		13: "acknowledgement too long",
	},
	"tendermint": {
		1: "invalid trusting period",
		2: "invalid unbonding period",
		3: "invalid header",
	},
	"transfer": {
		1: "invalid packet timeout",
	},
	"commitment": {
		1: "invalid proof",
		2: "invalid prefix",
	},
	"ibc": {
		1: "invalid height",
		2: "invalid version",
	},
	"sdk": {
		2:  "tx parse error",
		3:  "invalid sequence",
		4:  "unauthorized",
		5:  "insufficient funds",
		6:  "unknown request",
		7:  "invalid address",
		8:  "invalid pubkey",
		9:  "unknown address",
		10: "invalid coins",
		11: "out of gas",
		12: "memo too large",
		13: "insufficient fee",
		14: "maximum number of signatures exceeded",
		15: "no signatures supplied",
		16: "failed to marshal JSON bytes",
		17: "failed to unmarshal JSON bytes",
		18: "invalid request",
		19: "tx already in mempool",
		20: "mempool is full",
		21: "tx too large",
	},
}
