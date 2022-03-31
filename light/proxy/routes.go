package proxy

import (
	"fmt"

	"github.com/tendermint/tendermint/libs/bytes"
	lrpc "github.com/tendermint/tendermint/light/rpc"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpcserver "github.com/tendermint/tendermint/rpc/jsonrpc/server"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
	"github.com/tendermint/tendermint/types"
)

func RPCRoutes(c *lrpc.Client) map[string]*rpcserver.RPCFunc {
	return map[string]*rpcserver.RPCFunc{
		// Subscribe/unsubscribe are reserved for websocket events.
		"subscribe":       rpcserver.NewWSRPCFunc(makeSubscribeFunc(c), "query"),
		"unsubscribe":     rpcserver.NewWSRPCFunc(c.UnsubscribeWS, "query"),
		"unsubscribe_all": rpcserver.NewWSRPCFunc(c.UnsubscribeAllWS, ""),

		// info API
		"health":               rpcserver.NewRPCFunc(makeHealthFunc(c), ""),
		"status":               rpcserver.NewRPCFunc(makeStatusFunc(c), ""),
		"net_info":             rpcserver.NewRPCFunc(makeNetInfoFunc(c), "minHeight,maxHeight"),
		"blockchain":           rpcserver.NewRPCFunc(makeBlockchainInfoFunc(c), "minHeight,maxHeight"),
		"genesis":              rpcserver.NewRPCFunc(makeGenesisFunc(c), ""),
		"genesis_chunked":      rpcserver.NewRPCFunc(makeGenesisChunkedFunc(c), ""),
		"block":                rpcserver.NewRPCFunc(makeBlockFunc(c), "height"),
		"block_by_hash":        rpcserver.NewRPCFunc(makeBlockByHashFunc(c), "hash"),
		"block_results":        rpcserver.NewRPCFunc(makeBlockResultsFunc(c), "height"),
		"commit":               rpcserver.NewRPCFunc(makeCommitFunc(c), "height"),
		"tx":                   rpcserver.NewRPCFunc(makeTxFunc(c), "hash,prove"),
		"tx_search":            rpcserver.NewRPCFunc(makeTxSearchFunc(c), "query,prove,page,per_page,order_by"),
		"block_search":         rpcserver.NewRPCFunc(makeBlockSearchFunc(c), "query,page,per_page,order_by"),
		"validators":           rpcserver.NewRPCFunc(makeValidatorsFunc(c), "height,page,per_page"),
		"dump_consensus_state": rpcserver.NewRPCFunc(makeDumpConsensusStateFunc(c), ""),
		"consensus_state":      rpcserver.NewRPCFunc(makeConsensusStateFunc(c), ""),
		"consensus_params":     rpcserver.NewRPCFunc(makeConsensusParamsFunc(c), "height"),
		"unconfirmed_txs":      rpcserver.NewRPCFunc(makeUnconfirmedTxsFunc(c), "limit"),
		"num_unconfirmed_txs":  rpcserver.NewRPCFunc(makeNumUnconfirmedTxsFunc(c), ""),

		// tx broadcast API
		"broadcast_tx_commit": rpcserver.NewRPCFunc(makeBroadcastTxCommitFunc(c), "tx"),
		"broadcast_tx_sync":   rpcserver.NewRPCFunc(makeBroadcastTxSyncFunc(c), "tx"),
		"broadcast_tx_async":  rpcserver.NewRPCFunc(makeBroadcastTxAsyncFunc(c), "tx"),

		// abci API
		"abci_query": rpcserver.NewRPCFunc(makeABCIQueryFunc(c), "path,data,height,prove"),
		"abci_info":  rpcserver.NewRPCFunc(makeABCIInfoFunc(c), ""),

		// evidence API
		"broadcast_evidence": rpcserver.NewRPCFunc(makeBroadcastEvidenceFunc(c), "evidence"),
	}
}

func ensureClientStarted(c *lrpc.Client) error {

	// 3) Start a client.
	if !c.IsRunning() {
		if err := c.Start(); err != nil {
			return fmt.Errorf("can't start client: %w", err)
		}
	}
	c.Activate()

	return nil
}

type rpcSubscribeFunc func(ctx *rpctypes.Context, query string) (*ctypes.ResultSubscribe, error)

func makeSubscribeFunc(c *lrpc.Client) rpcSubscribeFunc {
	return func(ctx *rpctypes.Context, query string) (*ctypes.ResultSubscribe, error) {
		if err := ensureClientStarted(c); err != nil {
			return nil, err
		}
		c.DisableAutoReset() //disable auto reset when there are subscribes
		return c.SubscribeWS(ctx, query)
	}
}

type rpcHealthFunc func(ctx *rpctypes.Context) (*ctypes.ResultHealth, error)

func makeHealthFunc(c *lrpc.Client) rpcHealthFunc {
	return func(ctx *rpctypes.Context) (*ctypes.ResultHealth, error) {
		if err := ensureClientStarted(c); err != nil {
			return nil, err
		}
		return c.Health(ctx.Context())
	}
}

type rpcStatusFunc func(ctx *rpctypes.Context) (*ctypes.ResultStatus, error)

// nolint: interfacer
func makeStatusFunc(c *lrpc.Client) rpcStatusFunc {
	return func(ctx *rpctypes.Context) (*ctypes.ResultStatus, error) {
		if err := ensureClientStarted(c); err != nil {
			return nil, err
		}
		return c.Status(ctx.Context())
	}
}

type rpcNetInfoFunc func(ctx *rpctypes.Context, minHeight, maxHeight int64) (*ctypes.ResultNetInfo, error)

func makeNetInfoFunc(c *lrpc.Client) rpcNetInfoFunc {
	return func(ctx *rpctypes.Context, minHeight, maxHeight int64) (*ctypes.ResultNetInfo, error) {
		if err := ensureClientStarted(c); err != nil {
			return nil, err
		}
		return c.NetInfo(ctx.Context())
	}
}

type rpcBlockchainInfoFunc func(ctx *rpctypes.Context, minHeight, maxHeight int64) (*ctypes.ResultBlockchainInfo, error)

func makeBlockchainInfoFunc(c *lrpc.Client) rpcBlockchainInfoFunc {
	return func(ctx *rpctypes.Context, minHeight, maxHeight int64) (*ctypes.ResultBlockchainInfo, error) {
		if err := ensureClientStarted(c); err != nil {
			return nil, err
		}
		return c.BlockchainInfo(ctx.Context(), minHeight, maxHeight)
	}
}

type rpcGenesisFunc func(ctx *rpctypes.Context) (*ctypes.ResultGenesis, error)

func makeGenesisFunc(c *lrpc.Client) rpcGenesisFunc {
	return func(ctx *rpctypes.Context) (*ctypes.ResultGenesis, error) {
		if err := ensureClientStarted(c); err != nil {
			return nil, err
		}
		return c.Genesis(ctx.Context())
	}
}

type rpcGenesisChunkedFunc func(ctx *rpctypes.Context, chunk uint) (*ctypes.ResultGenesisChunk, error)

func makeGenesisChunkedFunc(c *lrpc.Client) rpcGenesisChunkedFunc {
	return func(ctx *rpctypes.Context, chunk uint) (*ctypes.ResultGenesisChunk, error) {
		if err := ensureClientStarted(c); err != nil {
			return nil, err
		}
		return c.GenesisChunked(ctx.Context(), chunk)
	}
}

type rpcBlockFunc func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultBlock, error)

func makeBlockFunc(c *lrpc.Client) rpcBlockFunc {
	return func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultBlock, error) {
		if err := ensureClientStarted(c); err != nil {
			return nil, err
		}
		return c.Block(ctx.Context(), height)
	}
}

type rpcBlockByHashFunc func(ctx *rpctypes.Context, hash []byte) (*ctypes.ResultBlock, error)

func makeBlockByHashFunc(c *lrpc.Client) rpcBlockByHashFunc {
	return func(ctx *rpctypes.Context, hash []byte) (*ctypes.ResultBlock, error) {
		if err := ensureClientStarted(c); err != nil {
			return nil, err
		}
		return c.BlockByHash(ctx.Context(), hash)
	}
}

type rpcBlockResultsFunc func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultBlockResults, error)

func makeBlockResultsFunc(c *lrpc.Client) rpcBlockResultsFunc {
	return func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultBlockResults, error) {
		if err := ensureClientStarted(c); err != nil {
			return nil, err
		}
		return c.BlockResults(ctx.Context(), height)
	}
}

type rpcCommitFunc func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultCommit, error)

func makeCommitFunc(c *lrpc.Client) rpcCommitFunc {
	return func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultCommit, error) {
		if err := ensureClientStarted(c); err != nil {
			return nil, err
		}
		return c.Commit(ctx.Context(), height)
	}
}

type rpcTxFunc func(ctx *rpctypes.Context, hash []byte, prove bool) (*ctypes.ResultTx, error)

func makeTxFunc(c *lrpc.Client) rpcTxFunc {
	return func(ctx *rpctypes.Context, hash []byte, prove bool) (*ctypes.ResultTx, error) {
		if err := ensureClientStarted(c); err != nil {
			return nil, err
		}
		return c.Tx(ctx.Context(), hash, prove)
	}
}

type rpcTxSearchFunc func(
	ctx *rpctypes.Context,
	query string,
	prove bool,
	page, perPage *int,
	orderBy string,
) (*ctypes.ResultTxSearch, error)

func makeTxSearchFunc(c *lrpc.Client) rpcTxSearchFunc {
	return func(
		ctx *rpctypes.Context,
		query string,
		prove bool,
		page, perPage *int,
		orderBy string,
	) (*ctypes.ResultTxSearch, error) {
		if err := ensureClientStarted(c); err != nil {
			return nil, err
		}
		return c.TxSearch(ctx.Context(), query, prove, page, perPage, orderBy)
	}
}

type rpcBlockSearchFunc func(
	ctx *rpctypes.Context,
	query string,
	prove bool,
	page, perPage *int,
	orderBy string,
) (*ctypes.ResultBlockSearch, error)

func makeBlockSearchFunc(c *lrpc.Client) rpcBlockSearchFunc {
	return func(
		ctx *rpctypes.Context,
		query string,
		prove bool,
		page, perPage *int,
		orderBy string,
	) (*ctypes.ResultBlockSearch, error) {
		if err := ensureClientStarted(c); err != nil {
			return nil, err
		}
		return c.BlockSearch(ctx.Context(), query, page, perPage, orderBy)
	}
}

type rpcValidatorsFunc func(ctx *rpctypes.Context, height *int64,
	page, perPage *int) (*ctypes.ResultValidators, error)

func makeValidatorsFunc(c *lrpc.Client) rpcValidatorsFunc {
	return func(ctx *rpctypes.Context, height *int64, page, perPage *int) (*ctypes.ResultValidators, error) {
		if err := ensureClientStarted(c); err != nil {
			return nil, err
		}
		return c.Validators(ctx.Context(), height, page, perPage)
	}
}

type rpcDumpConsensusStateFunc func(ctx *rpctypes.Context) (*ctypes.ResultDumpConsensusState, error)

func makeDumpConsensusStateFunc(c *lrpc.Client) rpcDumpConsensusStateFunc {
	return func(ctx *rpctypes.Context) (*ctypes.ResultDumpConsensusState, error) {
		if err := ensureClientStarted(c); err != nil {
			return nil, err
		}
		return c.DumpConsensusState(ctx.Context())
	}
}

type rpcConsensusStateFunc func(ctx *rpctypes.Context) (*ctypes.ResultConsensusState, error)

func makeConsensusStateFunc(c *lrpc.Client) rpcConsensusStateFunc {
	return func(ctx *rpctypes.Context) (*ctypes.ResultConsensusState, error) {
		if err := ensureClientStarted(c); err != nil {
			return nil, err
		}
		return c.ConsensusState(ctx.Context())
	}
}

type rpcConsensusParamsFunc func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultConsensusParams, error)

func makeConsensusParamsFunc(c *lrpc.Client) rpcConsensusParamsFunc {
	return func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultConsensusParams, error) {
		if err := ensureClientStarted(c); err != nil {
			return nil, err
		}
		return c.ConsensusParams(ctx.Context(), height)
	}
}

type rpcUnconfirmedTxsFunc func(ctx *rpctypes.Context, limit *int) (*ctypes.ResultUnconfirmedTxs, error)

func makeUnconfirmedTxsFunc(c *lrpc.Client) rpcUnconfirmedTxsFunc {
	return func(ctx *rpctypes.Context, limit *int) (*ctypes.ResultUnconfirmedTxs, error) {
		if err := ensureClientStarted(c); err != nil {
			return nil, err
		}
		return c.UnconfirmedTxs(ctx.Context(), limit)
	}
}

type rpcNumUnconfirmedTxsFunc func(ctx *rpctypes.Context) (*ctypes.ResultUnconfirmedTxs, error)

func makeNumUnconfirmedTxsFunc(c *lrpc.Client) rpcNumUnconfirmedTxsFunc {
	return func(ctx *rpctypes.Context) (*ctypes.ResultUnconfirmedTxs, error) {
		if err := ensureClientStarted(c); err != nil {
			return nil, err
		}
		return c.NumUnconfirmedTxs(ctx.Context())
	}
}

type rpcBroadcastTxCommitFunc func(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error)

func makeBroadcastTxCommitFunc(c *lrpc.Client) rpcBroadcastTxCommitFunc {
	return func(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {
		if err := ensureClientStarted(c); err != nil {
			return nil, err
		}
		return c.BroadcastTxCommit(ctx.Context(), tx)
	}
}

type rpcBroadcastTxSyncFunc func(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error)

func makeBroadcastTxSyncFunc(c *lrpc.Client) rpcBroadcastTxSyncFunc {
	return func(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
		if err := ensureClientStarted(c); err != nil {
			return nil, err
		}
		return c.BroadcastTxSync(ctx.Context(), tx)
	}
}

type rpcBroadcastTxAsyncFunc func(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error)

func makeBroadcastTxAsyncFunc(c *lrpc.Client) rpcBroadcastTxAsyncFunc {
	return func(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
		if err := ensureClientStarted(c); err != nil {
			return nil, err
		}
		return c.BroadcastTxAsync(ctx.Context(), tx)
	}
}

type rpcABCIQueryFunc func(ctx *rpctypes.Context, path string,
	data bytes.HexBytes, height int64, prove bool) (*ctypes.ResultABCIQuery, error)

func makeABCIQueryFunc(c *lrpc.Client) rpcABCIQueryFunc {
	return func(ctx *rpctypes.Context, path string, data bytes.HexBytes,
		height int64, prove bool) (*ctypes.ResultABCIQuery, error) {
		if err := ensureClientStarted(c); err != nil {
			return nil, err
		}
		return c.ABCIQueryWithOptions(ctx.Context(), path, data, rpcclient.ABCIQueryOptions{
			Height: height,
			Prove:  prove,
		})
	}
}

type rpcABCIInfoFunc func(ctx *rpctypes.Context) (*ctypes.ResultABCIInfo, error)

func makeABCIInfoFunc(c *lrpc.Client) rpcABCIInfoFunc {
	return func(ctx *rpctypes.Context) (*ctypes.ResultABCIInfo, error) {
		if err := ensureClientStarted(c); err != nil {
			return nil, err
		}
		return c.ABCIInfo(ctx.Context())
	}
}

type rpcBroadcastEvidenceFunc func(ctx *rpctypes.Context, ev types.Evidence) (*ctypes.ResultBroadcastEvidence, error)

// nolint: interfacer
func makeBroadcastEvidenceFunc(c *lrpc.Client) rpcBroadcastEvidenceFunc {
	return func(ctx *rpctypes.Context, ev types.Evidence) (*ctypes.ResultBroadcastEvidence, error) {
		if err := ensureClientStarted(c); err != nil {
			return nil, err
		}
		return c.BroadcastEvidence(ctx.Context(), ev)
	}
}
