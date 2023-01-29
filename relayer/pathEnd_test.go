package relayer

import (
	"testing"

	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	"github.com/stretchr/testify/require"
)

func TestOrderFromString(t *testing.T) {
	const (
		ordered   = "ordered"
		unordered = "unordered"
		none      = ""
	)

	o := OrderFromString(ordered)
	require.Equal(t, chantypes.ORDERED, o)

	u := OrderFromString(unordered)
	require.Equal(t, chantypes.UNORDERED, u)

	empty := OrderFromString(none)
	require.Equal(t, chantypes.NONE, empty)
}

func TestStringFromOrder(t *testing.T) {
	const (
		ordered   = chantypes.ORDERED
		unordered = chantypes.UNORDERED
		none      = chantypes.NONE
	)

	o := StringFromOrder(ordered)
	require.Equal(t, "ordered", o)

	u := StringFromOrder(unordered)
	require.Equal(t, "unordered", u)

	empty := StringFromOrder(none)
	require.Equal(t, "", empty)
}
