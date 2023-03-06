package relayer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
