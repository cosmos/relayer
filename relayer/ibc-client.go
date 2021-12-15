package relayer

import (
	clienttypes "github.com/cosmos/ibc-go/v2/modules/core/02-client/types"
	ibcexported "github.com/cosmos/ibc-go/v2/modules/core/exported"
	"golang.org/x/sync/errgroup"
)

func GetLightSignedHeadersAtHeights(src, dst *Chain, srch, dsth int64) (ibcexported.Header, ibcexported.Header, error) {
	var (
		eg                               = new(errgroup.Group)
		srcUpdateHeader, dstUpdateHeader ibcexported.Header
	)
	eg.Go(func() error {
		var err error
		srcUpdateHeader, err = src.ChainProvider.GetLightSignedHeaderAtHeight(srch)
		return err
	})
	eg.Go(func() error {
		var err error
		dstUpdateHeader, err = dst.ChainProvider.GetLightSignedHeaderAtHeight(dsth)
		return err
	})
	if err := eg.Wait(); err != nil {
		return nil, nil, err
	}
	return srcUpdateHeader, dstUpdateHeader, nil
}

// MustGetHeight takes the height inteface and returns the actual height
func MustGetHeight(h ibcexported.Height) clienttypes.Height {
	height, ok := h.(clienttypes.Height)
	if !ok {
		panic("height is not an instance of height!")
	}
	return height
}
