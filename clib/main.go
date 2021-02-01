// This is the entry point for a shared library built around the relayer.
// It depends on cgo, unlike the rly binary.
//
// This library was used as the basis for the Agoric Smart Relay:
// https://github.com/Agoric/agoric-sdk/tree/goz-smart-relay/packages/smart-relay

package main

// /* These comments before the import "C" are included in the C output. */
// #include <stdlib.h>
// typedef const char* Body;
// typedef int (*sendFunc)(int, int, Body);
// inline int invokeSendFunc(sendFunc send, int port, int reply, Body str) {
//    return send(port, reply, str);
// }
import "C"
import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/cosmos/relayer/cmd"
	"github.com/cosmos/relayer/relayer"
)

type goReturn = struct {
	str string
	err error
}

var clibPort = 0
var replies = map[int]chan goReturn{}
var lastReply = 0

//nolint:golint
//export RunClib
func RunClib(nodePort C.int, toNode C.sendFunc, clibArgs []*C.char) C.int {
	if relayer.SendToController == nil {
		relayer.SendToController = func(needReply bool, str string) (string, error) {
			var rPort int
			if needReply {
				lastReply++
				rPort = lastReply
				replies[rPort] = make(chan goReturn)
			}
			// Send the message.
			C.invokeSendFunc(toNode, nodePort, C.int(rPort), C.CString(str))
			if !needReply {
				// Return immediately
				return "<no-reply-requested>", nil
			}

			// Block the sending goroutine while we wait for the reply
			ret := <-replies[rPort]
			delete(replies, rPort)
			return ret.str, ret.err
		}
	}

	args := make([]string, len(clibArgs))
	for i, s := range clibArgs {
		args[i] = C.GoString(s)
	}
	// fmt.Println("Starting relayer with args", args)
	go func() {
		os.Args = args
		cmd.Execute()
		// fmt.Printf("exiting with nodePort %d\n", nodePort)
		if nodePort == 0 {
			os.Exit(0)
		}
	}()

	clibPort++
	return C.int(clibPort)
}

//nolint:golint
//export ReplyToClib
func ReplyToClib(replyPort C.int, isError C.int, str C.Body) C.int {
	goStr := C.GoString(str)
	returnCh := replies[int(replyPort)]
	if returnCh == nil {
		return C.int(0)
	}
	ret := goReturn{}
	if int(isError) == 0 {
		ret.str = goStr
	} else {
		ret.err = errors.New(goStr)
	}
	returnCh <- ret
	return C.int(0)
}

//nolint:golint
//export SendToClib
func SendToClib(port C.int, str C.Body) C.Body {
	goStr := C.GoString(str)
	var action relayer.DeliverMsgsAction
	err := json.Unmarshal([]byte(goStr), &action)
	if err == nil {
		switch action.Type {
		case "RELAYER_SEND":
			src := relayer.UnmarshalChain(action.Src)
			dst := relayer.UnmarshalChain(action.Dst)
			if src == nil || dst == nil {
				return C.CString("false")
			}
			rm := relayer.RelayMsgs{
				Succeeded: action.Succeeded,
				Last:      action.Last,
			}
			rm.Src = relayer.DecodeMsgs(src, action.SrcMsgs)
			rm.Dst = relayer.DecodeMsgs(dst, action.DstMsgs)

			rm.SendWithController(src, dst, false)
			if !rm.Succeeded {
				return C.CString("0")
			}
			return C.CString(fmt.Sprintf("%d", len(rm.Src)+len(rm.Dst)))
		default:
			fmt.Printf("failed action.Type %s\n", action.Type)
		}
	} else {
		fmt.Printf("failed unmarshalling %s\n", err)
	}
	return C.CString("false")
}

// Do nothing in main.
func main() {}
