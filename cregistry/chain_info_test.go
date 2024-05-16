package cregistry

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetAllRPCEndpoints(t *testing.T) {
	testCases := map[string]struct {
		chainInfo         ChainInfo
		expectedEndpoints []string
		expectedError     error
	}{
		"endpoint with TLS": {
			chainInfo:         ChainInfoWithRPCEndpoint("https://test.com"),
			expectedEndpoints: []string{"https://test.com:443"},
			expectedError:     nil,
		},
		"endpoint without TLS": {
			chainInfo:         ChainInfoWithRPCEndpoint("http://test.com:26657"),
			expectedEndpoints: []string{"http://test.com:26657"},
			expectedError:     nil,
		},
		"endpoint with TLS and with path": {
			chainInfo:         ChainInfoWithRPCEndpoint("https://test.com/rpc"),
			expectedEndpoints: []string{"https://test.com:443/rpc"},
			expectedError:     nil,
		},
		"endpoint with TLS and non-standard port": {
			chainInfo:         ChainInfoWithRPCEndpoint("https://test.com:8443"),
			expectedEndpoints: []string{"https://test.com:8443"},
			expectedError:     nil,
		},
		"proxied endpoint with TLS and non-standard port": {
			chainInfo:         ChainInfoWithRPCEndpoint("https://test.com:8443/rpc"),
			expectedEndpoints: []string{"https://test.com:8443/rpc"},
			expectedError:     nil,
		},
		"proxied endpoint without TLS and without path": {
			chainInfo:         ChainInfoWithRPCEndpoint("http://test.com"),
			expectedEndpoints: []string{"http://test.com:80"},
			expectedError:     nil,
		},
		"proxied endpoint without TLS and with path": {
			chainInfo:         ChainInfoWithRPCEndpoint("http://test.com/rpc"),
			expectedEndpoints: []string{"http://test.com:80/rpc"},
			expectedError:     nil,
		},
		"unsupported or invalid url scheme error": {
			chainInfo:         ChainInfoWithRPCEndpoint("ftp://test.com/rpc"),
			expectedEndpoints: nil,
			expectedError:     fmt.Errorf("invalid or unsupported url scheme: ftp"),
		},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			endpoints, err := tc.chainInfo.GetAllRPCEndpoints()
			require.Equal(t, tc.expectedError, err)
			require.Equal(t, tc.expectedEndpoints, endpoints)
		})
	}
}

func ChainInfoWithRPCEndpoint(endpoint string) ChainInfo {
	return ChainInfo{
		Apis: struct {
			RPC []struct {
				Address  string `json:"address"`
				Provider string `json:"provider"`
			} `json:"rpc"`
			Rest []struct {
				Address  string `json:"address"`
				Provider string `json:"provider"`
			} `json:"rest"`
		}{
			RPC: []struct {
				Address  string `json:"address"`
				Provider string `json:"provider"`
			}{
				{
					Address:  endpoint,
					Provider: "test",
				},
			},
		},
	}
}
