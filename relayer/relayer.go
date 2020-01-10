package relayer

// Relay implements the algorithm described in ICS18 (https://github.com/cosmos/ics/tree/master/spec/ics-018-relayer-algorithms)
func Relay(strategy string, c []*Chain) error {
	for _, src := range c {
		for _, dstID := range src.Counterparties {
			if dstID != src.ChainID {
				dst, err := GetChain(dstID, c)
				if err != nil {
					return err
				}

				var msgs RelayMsgs

				// NOTE: This implemenation will allow for multiple strategies to be implemented
				// w/in this package and switched via config or flag
				if Strategy(strategy) != nil {
					msgs = Strategy(strategy)(src, dst)
				}

				// Submit the transactions to each chain
				err = src.SendMsgs(msgs.Src)
				if err != nil {
					return err
				}

				err = dst.SendMsgs(msgs.Dst)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
