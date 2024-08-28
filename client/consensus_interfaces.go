package client

// Interfaces that the relayer uses to interact with CometBFT or Gordian

// interchaintest
// TxWithRetry(ctx context.Context, client client.RPCClient, hash []byte) (*coretypes.ResultTx, error) {

// Main:
/*
	_, err := cc.RPCClient.Status(ctx)
	s
blockRes, err = ccp.chainProvider.RPCClient.BlockResults(queryCtx, &sI)
			if err != nil && ccp.metrics != nil {
				ccp.metrics.IncBlockQueryFailure(chainID, "RPC Client")
			}


				resp, err := cc.RPCClient.ABCIQuery(ctx, queryPath, nil)
	if err != nil || resp.Response.Code != 0 {
		return "", err
	}

feeGrant uses WithClient(cc.RPCClient).
	future scope of work: http rpc wrapper


	resultBlock, err := cc.RPCClient.Block(ctx, &height)
	if err != nil {
		return time.Time{}, err
	}
	return resultBlock.Block.Time, nil


*it gets all messages from the block & then filters based on IBC events

res, err := cc.RPCClient.BlockSearch(ctx, query, &page, &limit, "")
		if err != nil {
			return err
		}

block, err := cc.RPCClient.BlockResults(ctx, &b.Block.Height)
				if err != nil {
					return err
				}

res, err := cc.RPCClient.TxSearch(ctx, query, true, &page, &limit, "")
		if err != nil {
			return err
		}



	resp, err := cc.RPCClient.Tx(ctx, hash, true)
	if err != nil {
		return nil, err
	}

	events := parseEventsFromResponseDeliverTx(resp.TxResult.Events)


	res, err := cc.RPCClient.TxSearch(ctx, strings.Join(events, " AND "), true, &page, &limit, "")
	if err != nil {
		return nil, err
	}

	res, err := cc.RPCClient.BroadcastTxAsync(ctx, txBytes)
	if res != nil {
		fmt.Printf("TX hash: %s\n", res.Hash)
	}

	res, err := cc.RPCClient.BroadcastTxSync(ctx, tx)
	isErr := err != nil
	isFailed := res != nil && res.Code != 0
	if isErr || isFailed {




	result, err := cc.RPCClient.ABCIQueryWithOptions(ctx, req.Path, req.Data, opts)
	if err != nil {
		return abci.ResponseQuery{}, err
	}

	if !result.Response.IsOK() {
		return abci.ResponseQuery{}, sdkErrorToGRPCError(result.Response)
	}

	// data from trusted node or subspace query doesn't need verification
	if !opts.Prove || !isQueryStoreWithProof(req.Path) {
		return result.Response, nil
	}
*/
