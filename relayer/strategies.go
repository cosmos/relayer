package relayer

import (
	"fmt"
	"time"
)

// StartRelayer starts the main relaying loop
func StartRelayer(src, dst *Chain, maxTxSize, maxMsgLength uint64) (func(), error) {
	doneChan := make(chan struct{})
	go func() {
		for {
			select {
			case <-doneChan:
				return
			default:
				// Fetch any unrelayed sequences depending on the channel order
				sp, err := UnrelayedSequences(src, dst)
				if err != nil {
					src.Log(fmt.Sprintf("unrelayed sequences error: %s", err))
				} else {
					if len(sp.Src) > 0 && src.debug {
						src.Log(fmt.Sprintf("[%s] unrelayed-packets-> %v", src.ChainID(), sp.Src))
					}
					if len(sp.Dst) > 0 && dst.debug {
						dst.Log(fmt.Sprintf("[%s] unrelayed-packets-> %v", dst.ChainID(), sp.Dst))
					}
					if !sp.Empty() {
						if err = RelayPackets(src, dst, sp, maxTxSize, maxMsgLength); err != nil {
							src.Log(fmt.Sprintf("relay packets error: %s", err))
						}
					}
				}

				// Fetch any unrelayed acks depending on the channel order
				ap, err := UnrelayedAcknowledgements(src, dst)
				if err != nil {
					src.Log(fmt.Sprintf("unrelayed acks error: %s", err))
				} else {
					if len(ap.Src) > 0 && src.debug {
						src.Log(fmt.Sprintf("[%s] unrelayed-acks-> %v", src.ChainID(), ap.Src))
					}
					if len(ap.Dst) > 0 && dst.debug {
						dst.Log(fmt.Sprintf("[%s] unrelayed-acks-> %v", dst.ChainID(), ap.Dst))
					}
					if !ap.Empty() {
						if err = RelayAcknowledgements(src, dst, ap, maxTxSize, maxMsgLength); err != nil && src.debug {
							src.Log(fmt.Sprintf("relay acks error: %s", err))
						}
					}
				}

				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
	return func() { doneChan <- struct{}{} }, nil
}
