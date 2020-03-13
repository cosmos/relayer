package relayer

import "fmt"

// Relay implements the algorithm described in ICS18 (https://github.com/cosmos/ics/tree/master/spec/ics-018-relayer-algorithms)
func Relay(strategy string, c Chains, paths []Path) error {
	for _, src := range c {
		for _, path := range paths {
			if path.Src.ChainID != src.ChainID {
				dst, err := c.GetChain(path.Dst.ChainID)
				if err != nil {
					return err
				}

				err = src.SetPath(path.Src, FULLPATH)
				if err != nil {
					return err
				}

				err = dst.SetPath(path.Dst, FULLPATH)
				if err != nil {
					return err
				}

				// NOTE: This implemenation will allow for multiple strategies to be implemented
				// w/in this package and switched via config or flag
				if Strategy(strategy) == nil {
					return fmt.Errorf("Must pick a configurable relaying strategy")
				}

				msgs, err := Strategy(strategy)(src, dst)
				if err != nil {
					return err
				}

				// Submit the transactions to src chain
				srcRes, err := src.SendMsgs(msgs.Src)
				if err != nil {
					return err
				}
				fmt.Println(srcRes)

				// Submit the transactions to dst chain
				dstRes, err := dst.SendMsgs(msgs.Dst)
				if err != nil {
					return err
				}
				fmt.Println(dstRes)
			}
		}
	}
	return nil
}
