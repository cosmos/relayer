package relayer

import (
	"testing"

	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	"github.com/stretchr/testify/require"
)

func TestApplyChannelFilterAllowRule(t *testing.T) {
	channels := []*chantypes.IdentifiedChannel{
		{
			ChannelId: "channel-0",
		},
		{
			ChannelId: "channel-1",
		},
		{
			ChannelId: "channel-2",
		},
	}

	filter := ChannelFilter{
		Rule:        "allowlist",
		ChannelList: []string{"channel-0", "channel-2"},
	}

	filteredChans := applyChannelFilterRule(filter, channels)

	require.Equal(t, 2, len(filteredChans))
}

func TestApplyChannelFilterDenyRule(t *testing.T) {
	channels := []*chantypes.IdentifiedChannel{
		{
			ChannelId: "channel-0",
		},
		{
			ChannelId: "channel-1",
		},
		{
			ChannelId: "channel-2",
		},
	}

	filter := ChannelFilter{
		Rule:        "denylist",
		ChannelList: []string{"channel-0", "channel-2"},
	}

	filteredChans := applyChannelFilterRule(filter, channels)

	require.Equal(t, 1, len(filteredChans))
}

func TestApplyChannelFilterNoRule(t *testing.T) {
	channels := []*chantypes.IdentifiedChannel{
		{
			ChannelId: "channel-0",
		},
		{
			ChannelId: "channel-1",
		},
		{
			ChannelId: "channel-2",
		},
	}

	filter := ChannelFilter{}

	filteredChans := applyChannelFilterRule(filter, channels)

	require.Equal(t, 3, len(filteredChans))
}

func TestValidateChannelFilterRule(t *testing.T) {
	p := &Path{
		Filter: ChannelFilter{
			Rule: "allowlist",
		},
	}
	require.NoError(t, p.ValidateChannelFilterRule())

	p = &Path{
		Filter: ChannelFilter{
			Rule: "denylist",
		},
	}
	require.NoError(t, p.ValidateChannelFilterRule())

	p = &Path{Filter: ChannelFilter{}}
	require.NoError(t, p.ValidateChannelFilterRule())

	p = &Path{
		Filter: ChannelFilter{
			Rule: "invalid",
		},
	}
	require.Error(t, p.ValidateChannelFilterRule())
}
