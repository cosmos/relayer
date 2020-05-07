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
		2:  "light client already exists",
		3:  "light client not found",
		4:  "light client is frozen due to misbehaviour",
		5:  "consensus state not found",
		6:  "invalid consensus state",
		7:  "client type not found",
		8:  "invalid client type",
		9:  "commitment root not found",
		10: "invalid block header",
		11: "invalid light client misbehaviour evidence",
		12: "client consensus state verification failed",
		13: "connection state verification failed",
		14: "channel state verification failed",
		15: "packet commitment verification failed",
		16: "packet acknowledgement verification failed",
		17: "packet acknowledgement absence verification failed",
		18: "next sequence receive verification failed",
		19: "self consensus state not found",
	},
	"connection": {
		2: "connection already exists",
		3: "connection not found",
		4: "light client connection paths not found",
		5: "connection path is not associated to the given light client",
		6: "invalid connection state",
		7: "invalid counterparty connection",
		8: "invalid connection",
	},
	"channels": {
		2:  "channel already exists",
		3:  "channel not found",
		4:  "invalid channel",
		5:  "invalid channel state",
		6:  "invalid channel ordering",
		7:  "invalid counterparty channel",
		8:  "invalid channel capability",
		9:  "channel capability not found",
		10: "sequence send not found",
		11: "sequence receive not found",
		12: "invalid packet",
		13: "packet timeout",
		14: "too many connection hops",
		15: "acknowledgement too long",
	},
	"port": {
		2: "port is already binded",
		3: "port not found",
		4: "invalid port",
		5: "route not found",
	},
	"tendermint": {
		2: "invalid trusting period",
		3: "invalid unbonding period",
		4: "invalid header",
	},
	"transfer": {
		2: "invalid packet timeout",
		3: "only one denom allowed",
		4: "invalid denomination for cross-chain transfer",
	},
	"commitment": {
		2: "invalid proof",
		3: "invalid prefix",
	},
	"host": {
		2: "invalid identifier",
		3: "invalid path",
		4: "invalid packet",
	},
	"ibc": {
		2: "invalid height",
		3: "invalid version",
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
		22: "key not found",
		23: "invalid account password",
		24: "tx intended signer does not match the given signer",
		25: "invalid gas adjustment",
	},
	"undefined": {
		1:      "internal",
		111222: "panic",
	},
	"bank": {
		2: "no inputs to send transaction",
		3: "no outputs to send transaction",
		4: "sum inputs != sum outputs",
		5: "send transactions are disabled",
	},
	"capability": {
		2: "capability name already taken",
		3: "given owner already claimed capability",
		4: "capability not owned by module",
	},
}
