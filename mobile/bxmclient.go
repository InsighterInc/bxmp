// Copyright 2016 The BXMP Authors
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

// Contains a wrapper for the BitMED client.

package geth

import (
	"math/big"

	"github.com/InsighterInc/bxmp/core/types"
	"github.com/InsighterInc/bxmp/bxmclient"
)

// BitmedClient provides access to the BitMED APIs.
type BitmedClient struct {
	client *bxmclient.Client
}

// NewBitmedClient connects a client to the given URL.
func NewBitmedClient(rawurl string) (client *BitmedClient, _ error) {
	rawClient, err := bxmclient.Dial(rawurl)
	return &BitmedClient{rawClient}, err
}

// GetBlockByHash returns the given full block.
func (ec *BitmedClient) GetBlockByHash(ctx *Context, hash *Hash) (block *Block, _ error) {
	rawBlock, err := ec.client.BlockByHash(ctx.context, hash.hash)
	return &Block{rawBlock}, err
}

// GetBlockByNumber returns a block from the current canonical chain. If number is <0, the
// latest known block is returned.
func (ec *BitmedClient) GetBlockByNumber(ctx *Context, number int64) (block *Block, _ error) {
	if number < 0 {
		rawBlock, err := ec.client.BlockByNumber(ctx.context, nil)
		return &Block{rawBlock}, err
	}
	rawBlock, err := ec.client.BlockByNumber(ctx.context, big.NewInt(number))
	return &Block{rawBlock}, err
}

// GetHeaderByHash returns the block header with the given hash.
func (ec *BitmedClient) GetHeaderByHash(ctx *Context, hash *Hash) (header *Header, _ error) {
	rawHeader, err := ec.client.HeaderByHash(ctx.context, hash.hash)
	return &Header{rawHeader}, err
}

// GetHeaderByNumber returns a block header from the current canonical chain. If number is <0,
// the latest known header is returned.
func (ec *BitmedClient) GetHeaderByNumber(ctx *Context, number int64) (header *Header, _ error) {
	if number < 0 {
		rawHeader, err := ec.client.HeaderByNumber(ctx.context, nil)
		return &Header{rawHeader}, err
	}
	rawHeader, err := ec.client.HeaderByNumber(ctx.context, big.NewInt(number))
	return &Header{rawHeader}, err
}

// GetTransactionByHash returns the transaction with the given hash.
func (ec *BitmedClient) GetTransactionByHash(ctx *Context, hash *Hash) (tx *Transaction, _ error) {
	// TODO(karalabe): handle isPending
	rawTx, _, err := ec.client.TransactionByHash(ctx.context, hash.hash)
	return &Transaction{rawTx}, err
}

// GetTransactionSender returns the sender address of a transaction. The transaction must
// be included in blockchain at the given block and index.
func (ec *BitmedClient) GetTransactionSender(ctx *Context, tx *Transaction, blockhash *Hash, index int) (sender *Address, _ error) {
	addr, err := ec.client.TransactionSender(ctx.context, tx.tx, blockhash.hash, uint(index))
	return &Address{addr}, err
}

// GetTransactionCount returns the total number of transactions in the given block.
func (ec *BitmedClient) GetTransactionCount(ctx *Context, hash *Hash) (count int, _ error) {
	rawCount, err := ec.client.TransactionCount(ctx.context, hash.hash)
	return int(rawCount), err
}

// GetTransactionInBlock returns a single transaction at index in the given block.
func (ec *BitmedClient) GetTransactionInBlock(ctx *Context, hash *Hash, index int) (tx *Transaction, _ error) {
	rawTx, err := ec.client.TransactionInBlock(ctx.context, hash.hash, uint(index))
	return &Transaction{rawTx}, err

}

// GetTransactionReceipt returns the receipt of a transaction by transaction hash.
// Note that the receipt is not available for pending transactions.
func (ec *BitmedClient) GetTransactionReceipt(ctx *Context, hash *Hash) (receipt *Receipt, _ error) {
	rawReceipt, err := ec.client.TransactionReceipt(ctx.context, hash.hash)
	return &Receipt{rawReceipt}, err
}

// SyncProgress retrieves the current progress of the sync algorithm. If there's
// no sync currently running, it returns nil.
func (ec *BitmedClient) SyncProgress(ctx *Context) (progress *SyncProgress, _ error) {
	rawProgress, err := ec.client.SyncProgress(ctx.context)
	if rawProgress == nil {
		return nil, err
	}
	return &SyncProgress{*rawProgress}, err
}

// NewHeadHandler is a client-side subscription callback to invoke on events and
// subscription failure.
type NewHeadHandler interface {
	OnNewHead(header *Header)
	OnError(failure string)
}

// SubscribeNewHead subscribes to notifications about the current blockchain head
// on the given channel.
func (ec *BitmedClient) SubscribeNewHead(ctx *Context, handler NewHeadHandler, buffer int) (sub *Subscription, _ error) {
	// Subscribe to the event internally
	ch := make(chan *types.Header, buffer)
	rawSub, err := ec.client.SubscribeNewHead(ctx.context, ch)
	if err != nil {
		return nil, err
	}
	// Start up a dispatcher to feed into the callback
	go func() {
		for {
			select {
			case header := <-ch:
				handler.OnNewHead(&Header{header})

			case err := <-rawSub.Err():
				handler.OnError(err.Error())
				return
			}
		}
	}()
	return &Subscription{rawSub}, nil
}

// State Access

// GetBalanceAt returns the wei balance of the given account.
// The block number can be <0, in which case the balance is taken from the latest known block.
func (ec *BitmedClient) GetBalanceAt(ctx *Context, account *Address, number int64) (balance *BigInt, _ error) {
	if number < 0 {
		rawBalance, err := ec.client.BalanceAt(ctx.context, account.address, nil)
		return &BigInt{rawBalance}, err
	}
	rawBalance, err := ec.client.BalanceAt(ctx.context, account.address, big.NewInt(number))
	return &BigInt{rawBalance}, err
}

// GetStorageAt returns the value of key in the contract storage of the given account.
// The block number can be <0, in which case the value is taken from the latest known block.
func (ec *BitmedClient) GetStorageAt(ctx *Context, account *Address, key *Hash, number int64) (storage []byte, _ error) {
	if number < 0 {
		return ec.client.StorageAt(ctx.context, account.address, key.hash, nil)
	}
	return ec.client.StorageAt(ctx.context, account.address, key.hash, big.NewInt(number))
}

// GetCodeAt returns the contract code of the given account.
// The block number can be <0, in which case the code is taken from the latest known block.
func (ec *BitmedClient) GetCodeAt(ctx *Context, account *Address, number int64) (code []byte, _ error) {
	if number < 0 {
		return ec.client.CodeAt(ctx.context, account.address, nil)
	}
	return ec.client.CodeAt(ctx.context, account.address, big.NewInt(number))
}

// GetNonceAt returns the account nonce of the given account.
// The block number can be <0, in which case the nonce is taken from the latest known block.
func (ec *BitmedClient) GetNonceAt(ctx *Context, account *Address, number int64) (nonce int64, _ error) {
	if number < 0 {
		rawNonce, err := ec.client.NonceAt(ctx.context, account.address, nil)
		return int64(rawNonce), err
	}
	rawNonce, err := ec.client.NonceAt(ctx.context, account.address, big.NewInt(number))
	return int64(rawNonce), err
}

// Filters

// FilterLogs executes a filter query.
func (ec *BitmedClient) FilterLogs(ctx *Context, query *FilterQuery) (logs *Logs, _ error) {
	rawLogs, err := ec.client.FilterLogs(ctx.context, query.query)
	if err != nil {
		return nil, err
	}
	// Temp hack due to vm.Logs being []*vm.Log
	res := make([]*types.Log, len(rawLogs))
	for i, log := range rawLogs {
		res[i] = &log
	}
	return &Logs{res}, nil
}

// FilterLogsHandler is a client-side subscription callback to invoke on events and
// subscription failure.
type FilterLogsHandler interface {
	OnFilterLogs(log *Log)
	OnError(failure string)
}

// SubscribeFilterLogs subscribes to the results of a streaming filter query.
func (ec *BitmedClient) SubscribeFilterLogs(ctx *Context, query *FilterQuery, handler FilterLogsHandler, buffer int) (sub *Subscription, _ error) {
	// Subscribe to the event internally
	ch := make(chan types.Log, buffer)
	rawSub, err := ec.client.SubscribeFilterLogs(ctx.context, query.query, ch)
	if err != nil {
		return nil, err
	}
	// Start up a dispatcher to feed into the callback
	go func() {
		for {
			select {
			case log := <-ch:
				handler.OnFilterLogs(&Log{&log})

			case err := <-rawSub.Err():
				handler.OnError(err.Error())
				return
			}
		}
	}()
	return &Subscription{rawSub}, nil
}

// Pending State

// GetPendingBalanceAt returns the wei balance of the given account in the pending state.
func (ec *BitmedClient) GetPendingBalanceAt(ctx *Context, account *Address) (balance *BigInt, _ error) {
	rawBalance, err := ec.client.PendingBalanceAt(ctx.context, account.address)
	return &BigInt{rawBalance}, err
}

// GetPendingStorageAt returns the value of key in the contract storage of the given account in the pending state.
func (ec *BitmedClient) GetPendingStorageAt(ctx *Context, account *Address, key *Hash) (storage []byte, _ error) {
	return ec.client.PendingStorageAt(ctx.context, account.address, key.hash)
}

// GetPendingCodeAt returns the contract code of the given account in the pending state.
func (ec *BitmedClient) GetPendingCodeAt(ctx *Context, account *Address) (code []byte, _ error) {
	return ec.client.PendingCodeAt(ctx.context, account.address)
}

// GetPendingNonceAt returns the account nonce of the given account in the pending state.
// This is the nonce that should be used for the next transaction.
func (ec *BitmedClient) GetPendingNonceAt(ctx *Context, account *Address) (nonce int64, _ error) {
	rawNonce, err := ec.client.PendingNonceAt(ctx.context, account.address)
	return int64(rawNonce), err
}

// GetPendingTransactionCount returns the total number of transactions in the pending state.
func (ec *BitmedClient) GetPendingTransactionCount(ctx *Context) (count int, _ error) {
	rawCount, err := ec.client.PendingTransactionCount(ctx.context)
	return int(rawCount), err
}

// Contract Calling

// CallContract executes a message call transaction, which is directly executed in the VM
// of the node, but never mined into the blockchain.
//
// blockNumber selects the block height at which the call runs. It can be <0, in which
// case the code is taken from the latest known block. Note that state from very old
// blocks might not be available.
func (ec *BitmedClient) CallContract(ctx *Context, msg *CallMsg, number int64) (output []byte, _ error) {
	if number < 0 {
		return ec.client.CallContract(ctx.context, msg.msg, nil)
	}
	return ec.client.CallContract(ctx.context, msg.msg, big.NewInt(number))
}

// PendingCallContract executes a message call transaction using the EVM.
// The state seen by the contract call is the pending state.
func (ec *BitmedClient) PendingCallContract(ctx *Context, msg *CallMsg) (output []byte, _ error) {
	return ec.client.PendingCallContract(ctx.context, msg.msg)
}

// SuggestGasPrice retrieves the currently suggested gas price to allow a timely
// execution of a transaction.
func (ec *BitmedClient) SuggestGasPrice(ctx *Context) (price *BigInt, _ error) {
	rawPrice, err := ec.client.SuggestGasPrice(ctx.context)
	return &BigInt{rawPrice}, err
}

// EstimateGas tries to estimate the gas needed to execute a specific transaction based on
// the current pending state of the backend blockchain. There is no guarantee that this is
// the true gas limit requirement as other transactions may be added or removed by miners,
// but it should provide a basis for setting a reasonable default.
func (ec *BitmedClient) EstimateGas(ctx *Context, msg *CallMsg) (gas *BigInt, _ error) {
	rawGas, err := ec.client.EstimateGas(ctx.context, msg.msg)
	return &BigInt{rawGas}, err
}

// SendTransaction injects a signed transaction into the pending pool for execution.
//
// If the transaction was a contract creation use the TransactionReceipt method to get the
// contract address after the transaction has been mined.
func (ec *BitmedClient) SendTransaction(ctx *Context, tx *Transaction) error {
	return ec.client.SendTransaction(ctx.context, tx.tx)
}
