// Copyright 2015 The BXMP Authors
// This file is part of the BXMP library.
//
// The BXMP library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The BXMP library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the BXMP library. If not, see <http://www.gnu.org/licenses/>.

package bxm

import (
	"context"
	"math/big"

	"github.com/InsighterInc/bxmp/accounts"
	"github.com/InsighterInc/bxmp/common"
	"github.com/InsighterInc/bxmp/common/math"
	"github.com/InsighterInc/bxmp/core"
	"github.com/InsighterInc/bxmp/core/bloombits"
	"github.com/InsighterInc/bxmp/core/state"
	"github.com/InsighterInc/bxmp/core/types"
	"github.com/InsighterInc/bxmp/core/vm"
	"github.com/InsighterInc/bxmp/bxm/downloader"
	"github.com/InsighterInc/bxmp/bxm/gasprice"
	"github.com/InsighterInc/bxmp/bxmdb"
	"github.com/InsighterInc/bxmp/event"
	"github.com/InsighterInc/bxmp/params"
	"github.com/InsighterInc/bxmp/rpc"
)

// BxmApiBackend implements bxmapi.Backend for full nodes
type BxmApiBackend struct {
	bxm *BitMED
	gpo *gasprice.Oracle
}

func (b *BxmApiBackend) ChainConfig() *params.ChainConfig {
	return b.bxm.chainConfig
}

func (b *BxmApiBackend) CurrentBlock() *types.Block {
	return b.bxm.blockchain.CurrentBlock()
}

func (b *BxmApiBackend) SetHead(number uint64) {
	b.bxm.protocolManager.downloader.Cancel()
	b.bxm.blockchain.SetHead(number)
}

func (b *BxmApiBackend) HeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Header, error) {
	// Pending block is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		block := b.bxm.miner.PendingBlock()
		return block.Header(), nil
	}
	// Otherwise resolve and return the block
	if blockNr == rpc.LatestBlockNumber {
		return b.bxm.blockchain.CurrentBlock().Header(), nil
	}
	return b.bxm.blockchain.GetHeaderByNumber(uint64(blockNr)), nil
}

func (b *BxmApiBackend) BlockByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Block, error) {
	// Pending block is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		if b.bxm.protocolManager.raftMode {
			// Use latest instead.
			return b.bxm.blockchain.CurrentBlock(), nil
		}
		block := b.bxm.miner.PendingBlock()
		return block, nil
	}
	// Otherwise resolve and return the block
	if blockNr == rpc.LatestBlockNumber {
		return b.bxm.blockchain.CurrentBlock(), nil
	}
	return b.bxm.blockchain.GetBlockByNumber(uint64(blockNr)), nil
}

func (b *BxmApiBackend) StateAndHeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (vm.MinimalApiState, *types.Header, error) {
	// Pending state is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		if b.bxm.protocolManager.raftMode {
			// Use latest instead.
			header, err := b.HeaderByNumber(ctx, rpc.LatestBlockNumber)
			if header == nil || err != nil {
				return nil, nil, err
			}
			publicState, privateState, err := b.bxm.BlockChain().StateAt(header.Root)
			return BxmApiState{publicState, privateState}, header, err
		}
		block, publicState, privateState := b.bxm.miner.Pending()
		return BxmApiState{publicState, privateState}, block.Header(), nil
	}
	// Otherwise resolve the block number and return its state
	header, err := b.HeaderByNumber(ctx, blockNr)
	if header == nil || err != nil {
		return nil, nil, err
	}
	stateDb, privateState, err := b.bxm.BlockChain().StateAt(header.Root)
	return BxmApiState{stateDb, privateState}, header, err
}

func (b *BxmApiBackend) GetBlock(ctx context.Context, blockHash common.Hash) (*types.Block, error) {
	return b.bxm.blockchain.GetBlockByHash(blockHash), nil
}

func (b *BxmApiBackend) GetReceipts(ctx context.Context, blockHash common.Hash) (types.Receipts, error) {
	return core.GetBlockReceipts(b.bxm.chainDb, blockHash, core.GetBlockNumber(b.bxm.chainDb, blockHash)), nil
}

func (b *BxmApiBackend) GetTd(blockHash common.Hash) *big.Int {
	return b.bxm.blockchain.GetTdByHash(blockHash)
}

func (b *BxmApiBackend) GetEVM(ctx context.Context, msg core.Message, state vm.MinimalApiState, header *types.Header, vmCfg vm.Config) (*vm.EVM, func() error, error) {
	statedb := state.(BxmApiState)
	from := statedb.state.GetOrNewStateObject(msg.From())
	from.SetBalance(math.MaxBig256)
	vmError := func() error { return nil }

	context := core.NewEVMContext(msg, header, b.bxm.BlockChain(), nil)
	return vm.NewEVM(context, statedb.state, statedb.privateState, b.bxm.chainConfig, vmCfg), vmError, nil
}

func (b *BxmApiBackend) SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription {
	return b.bxm.BlockChain().SubscribeRemovedLogsEvent(ch)
}

func (b *BxmApiBackend) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
	return b.bxm.BlockChain().SubscribeChainEvent(ch)
}

func (b *BxmApiBackend) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return b.bxm.BlockChain().SubscribeChainHeadEvent(ch)
}

func (b *BxmApiBackend) SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription {
	return b.bxm.BlockChain().SubscribeChainSideEvent(ch)
}

func (b *BxmApiBackend) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return b.bxm.BlockChain().SubscribeLogsEvent(ch)
}

func (b *BxmApiBackend) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	return b.bxm.txPool.AddLocal(signedTx)
}

func (b *BxmApiBackend) GetPoolTransactions() (types.Transactions, error) {
	pending, err := b.bxm.txPool.Pending()
	if err != nil {
		return nil, err
	}
	var txs types.Transactions
	for _, batch := range pending {
		txs = append(txs, batch...)
	}
	return txs, nil
}

func (b *BxmApiBackend) GetPoolTransaction(hash common.Hash) *types.Transaction {
	return b.bxm.txPool.Get(hash)
}

func (b *BxmApiBackend) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return b.bxm.txPool.State().GetNonce(addr), nil
}

func (b *BxmApiBackend) Stats() (pending int, queued int) {
	return b.bxm.txPool.Stats()
}

func (b *BxmApiBackend) TxPoolContent() (map[common.Address]types.Transactions, map[common.Address]types.Transactions) {
	return b.bxm.TxPool().Content()
}

func (b *BxmApiBackend) SubscribeTxPreEvent(ch chan<- core.TxPreEvent) event.Subscription {
	return b.bxm.TxPool().SubscribeTxPreEvent(ch)
}

func (b *BxmApiBackend) Downloader() *downloader.Downloader {
	return b.bxm.Downloader()
}

func (b *BxmApiBackend) ProtocolVersion() int {
	return b.bxm.BxmVersion()
}

func (b *BxmApiBackend) SuggestPrice(ctx context.Context) (*big.Int, error) {
	if b.ChainConfig().IsBitmed {
		return big.NewInt(0), nil
	} else {
		return b.gpo.SuggestPrice(ctx)
	}
}

func (b *BxmApiBackend) ChainDb() bxmdb.Database {
	return b.bxm.ChainDb()
}

func (b *BxmApiBackend) EventMux() *event.TypeMux {
	return b.bxm.EventMux()
}

func (b *BxmApiBackend) AccountManager() *accounts.Manager {
	return b.bxm.AccountManager()
}

func (b *BxmApiBackend) BloomStatus() (uint64, uint64) {
	sections, _, _ := b.bxm.bloomIndexer.Sections()
	return params.BloomBitsBlocks, sections
}

func (b *BxmApiBackend) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
	for i := 0; i < bloomFilterThreads; i++ {
		go session.Multiplex(bloomRetrievalBatch, bloomRetrievalWait, b.bxm.bloomRequests)
	}
}

type BxmApiState struct {
	state, privateState *state.StateDB
}

func (s BxmApiState) GetBalance(addr common.Address) *big.Int {
	if s.privateState.Exist(addr) {
		return s.privateState.GetBalance(addr)
	}
	return s.state.GetBalance(addr)
}

func (s BxmApiState) GetCode(addr common.Address) []byte {
	if s.privateState.Exist(addr) {
		return s.privateState.GetCode(addr)
	}
	return s.state.GetCode(addr)
}

func (s BxmApiState) GetState(a common.Address, b common.Hash) common.Hash {
	if s.privateState.Exist(a) {
		return s.privateState.GetState(a, b)
	}
	return s.state.GetState(a, b)
}

func (s BxmApiState) GetNonce(addr common.Address) uint64 {
	if s.privateState.Exist(addr) {
		return s.privateState.GetNonce(addr)
	}
	return s.state.GetNonce(addr)
}
