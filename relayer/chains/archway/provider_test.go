package archway

// import (
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"testing"
// 	"time"

// 	"github.com/stretchr/testify/assert"

// 	"github.com/CosmWasm/wasmd/x/wasm/types"
// 	"github.com/cosmos/cosmos-sdk/client"
// 	// "github.com/tendermint/tendermint/rpc/client"
// )

// func TestArchwayClient(t *testing.T) {

// 	// addr := "http://localhost:26657"
// 	addr := "https://rpc.constantine-2.archway.tech:443 "
// 	client, _ := NewRPCClient(addr, 20*time.Second)

// 	ctx := context.Background()
// 	var height int64 = 20

// 	output, err := client.Block(ctx, &height)
// 	assert.NoError(t, err)

// 	fmt.Println("the output is:", output)

// }

// func TestArchwayQueryContract(t *testing.T) {

// 	// cl, _ := client.NewClientFromNode("http://localhost:26657")
// 	cl, _ := client.NewClientFromNode("https://rpc.constantine-1.archway.tech:443")
// 	ctx := context.Background()

// 	contractAddr := "archway15f3c0m82kp5fmqvfjh08l5dav0epkrs4ll6huhjv0zhfqyzpak7sf3g0dw"

// 	cliCtx := client.Context{}.WithClient(cl)
// 	queryCLient := types.NewQueryClient(cliCtx)

// 	// cl.BroadcastTxSync(ctx, )

// 	// contractInfo, err := queryCLient.ContractInfo(ctx, &types.QueryContractInfoRequest{
// 	// 	Address: "archway14hj2tavq8fpesdwxxcu44rty3hh90vhujrvcmstl4zr3txmfvw9sy85n2u",
// 	// })

// 	// if err != nil {
// 	// 	fmt.Println("errorr occured", err)
// 	// 	return
// 	// }

// 	type Msg struct {
// 		Count int    `json:"count"`
// 		Owner string `json:"owner"`
// 	}
// 	// fmt.Println("contractInfo", contractInfo)
// 	route := fmt.Sprintf("custom/%s/%s/%s/%s", types.QuerierRoute,
// 		"contract-state", contractAddr,
// 		"smart")
// 	data := []byte(`{"get_count":{}}`)
// 	clientState, _, _ := cliCtx.QueryWithData(route, data)

// 	var m Msg
// 	_ = json.Unmarshal(clientState, &m)

// 	fmt.Println("Count is :  ", m.Count)

// 	contractState, _ := queryCLient.SmartContractState(ctx, &types.QuerySmartContractStateRequest{
// 		Address:   "archway15f3c0m82kp5fmqvfjh08l5dav0epkrs4ll6huhjv0zhfqyzpak7sf3g0dw",
// 		QueryData: []byte(`{get_count:{}}`),
// 	})

// 	// contractState, _ := queryCLient.RawContractState(ctx, &types.QueryRawContractStateRequest{
// 	// 	Address:   contractAddr,
// 	// 	QueryData: []byte("state"),
// 	// })

// 	// contractState, _ := queryCLient.SmartContractState(ctx, &types.QuerySmartContractStateRequest{
// 	// 	Address:   "archway14hj2tavq8fpesdwxxcu44rty3hh90vhujrvcmstl4zr3txmfvw9sy85n2u",
// 	// 	QueryData: []byte("state"),
// 	// })

// 	fmt.Println("contractState:", contractState)

// }
