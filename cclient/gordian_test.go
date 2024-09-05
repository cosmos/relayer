package cclient_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cosmos/relayer/v2/cclient"
	"github.com/stretchr/testify/require"
)

// cat example-tx-signed.json
const tx = `{"body":{"messages":[{"@type":"/cosmos.bank.v1beta1.MsgSend","from_address":"cosmos1r5v5srda7xfth3hn2s26txvrcrntldjumt8mhl","to_address":"cosmos10r39fueph9fq7a6lgswu4zdsg8t3gxlqvvvyvn","amount":[{"denom":"stake","amount":"1"}]}],"memo":"","timeout_height":"0","unordered":false,"timeout_timestamp":"0001-01-01T00:00:00Z","extension_options":[],"non_critical_extension_options":[]},"auth_info":{"signer_infos":[{"public_key":{"@type":"/cosmos.crypto.secp256k1.PubKey","key":"ArpmqEz3g5rxcqE+f8n15wCMuLyhWF+PO6+zA57aPB/d"},"mode_info":{"single":{"mode":"SIGN_MODE_DIRECT"}},"sequence":"1"}],"fee":{"amount":[],"gas_limit":"200000","payer":"cosmos1r5v5srda7xfth3hn2s26txvrcrntldjumt8mhl","granter":""},"tip":null},"signatures":["CeyHZH8itZikoY8mWtfCzM46qZfOLkncHRe8CxludOUpgvxklTcy4+EetVN++OzBgxxXUMG/B5DIuJAFQ4G6cg=="]}`

// const tx = `{"tx":"eyJib2R5Ijp7Im1lc3NhZ2VzIjpbeyJAdHlwZSI6Ii9jb3Ntb3MuYmFuay52MWJldGExLk1zZ1NlbmQiLCJmcm9tX2FkZHJlc3MiOiJjb3Ntb3MxcjV2NXNyZGE3eGZ0aDNobjJzMjZ0eHZyY3JudGxkanVtdDhtaGwiLCJ0b19hZGRyZXNzIjoiY29zbW9zMTByMzlmdWVwaDlmcTdhNmxnc3d1NHpkc2c4dDNneGxxdnZ2eXZuIiwiYW1vdW50IjpbeyJkZW5vbSI6InN0YWtlIiwiYW1vdW50IjoiMSJ9XX1dLCJtZW1vIjoiIiwidGltZW91dF9oZWlnaHQiOiIwIiwidW5vcmRlcmVkIjpmYWxzZSwidGltZW91dF90aW1lc3RhbXAiOiIwMDAxLTAxLTAxVDAwOjAwOjAwWiIsImV4dGVuc2lvbl9vcHRpb25zIjpbXSwibm9uX2NyaXRpY2FsX2V4dGVuc2lvbl9vcHRpb25zIjpbXX0sImF1dGhfaW5mbyI6eyJzaWduZXJfaW5mb3MiOlt7InB1YmxpY19rZXkiOnsiQHR5cGUiOiIvY29zbW9zLmNyeXB0by5zZWNwMjU2azEuUHViS2V5Iiwia2V5IjoiQXJwbXFFejNnNXJ4Y3FFK2Y4bjE1d0NNdUx5aFdGK1BPNit6QTU3YVBCL2QifSwibW9kZV9pbmZvIjp7InNpbmdsZSI6eyJtb2RlIjoiU0lHTl9NT0RFX0RJUkVDVCJ9fSwic2VxdWVuY2UiOiIxIn1dLCJmZWUiOnsiYW1vdW50IjpbXSwiZ2FzX2xpbWl0IjoiMjAwMDAwIiwicGF5ZXIiOiJjb3Ntb3MxcjV2NXNyZGE3eGZ0aDNobjJzMjZ0eHZyY3JudGxkanVtdDhtaGwiLCJncmFudGVyIjoiIn0sInRpcCI6bnVsbH0sInNpZ25hdHVyZXMiOlsiQ2V5SFpIOGl0Wmlrb1k4bVd0ZkN6TTQ2cVpmT0xrbmNIUmU4Q3hsdWRPVXBndnhrbFRjeTQrRWV0Vk4rK096Qmd4eFhVTUcvQjVESXVKQUZRNEc2Y2c9PSJdfQo="}`

func TestGordian(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	gc := cclient.NewGordianConsensus("http://127.0.0.1:26657")

	s, err := gc.GetStatus(ctx)
	require.NoError(t, err)
	t.Log(s)

	bt, err := gc.GetBlockTime(ctx, 2)
	require.NoError(t, err)
	t.Log(bt)

	resp, err := gc.DoBroadcastTxSync(ctx, []byte(tx))
	fmt.Println("resp", resp)
	require.NoError(t, err)
	t.Log(resp)

	tx, err := gc.GetTx(ctx, []byte("D8FF0A405957A3D090A485CA3C997A25E2964F2E7840DDBCBFE805EC97192651"), false)
	require.NoError(t, err, "tx hash not found, make sure to submit one.")
	t.Log(tx)

	bh := int64(s.LatestBlockHeight)
	vals, err := gc.GetValidators(ctx, &bh, nil, nil)
	require.NoError(t, err)
	t.Log("vals", vals)

}
