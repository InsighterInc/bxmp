// Copyright 2014 The BXMP Authors
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

package core

import (
	"errors"
	"math/big"

	"github.com/InsighterInc/bxmp/common"
	"github.com/InsighterInc/bxmp/common/math"
	"github.com/InsighterInc/bxmp/core/vm"
	"github.com/InsighterInc/bxmp/log"
	"github.com/InsighterInc/bxmp/params"
	"github.com/InsighterInc/bxmp/private"
)

var (
	Big0                         = big.NewInt(0)
	errInsufficientBalanceForGas = errors.New("insufficient balance to pay for gas")
)

/*
The State Transitioning Model

A state transition is a change made when a transaction is applied to the current world state
The state transitioning model does all all the necessary work to work out a valid new state root.

1) Nonce handling
2) Pre pay gas
3) Create a new state object if the recipient is \0*32
4) Value transfer
== If contract creation ==
  4a) Attempt to run transaction data
  4b) If valid, use result as code for the new state object
== end ==
5) Run Script section
6) Derive new state root
*/
type StateTransition struct {
	gp         *GasPool
	msg        Message
	gas        uint64
	gasPrice   *big.Int
	initialGas *big.Int
	value      *big.Int
	data       []byte
	state      vm.StateDB
	evm        *vm.EVM
}

// Message represents a message sent to a contract.
type Message interface {
	From() common.Address
	//FromFrontier() (common.Address, error)
	To() *common.Address

	GasPrice() *big.Int
	Gas() *big.Int
	Value() *big.Int

	Nonce() uint64
	CheckNonce() bool
	Data() []byte
}

// PrivateMessage implements a private message
type PrivateMessage interface {
	Message
	IsPrivate() bool
}

// IntrinsicGas computes the 'intrinsic gas' for a message
// with the given data.
//
// TODO convert to uint64
func IntrinsicGas(data []byte, contractCreation, homestead bool) *big.Int {
	igas := new(big.Int)
	if contractCreation && homestead {
		igas.SetUint64(params.TxGasContractCreation)
	} else {
		igas.SetUint64(params.TxGas)
	}
	if len(data) > 0 {
		var nz int64
		for _, byt := range data {
			if byt != 0 {
				nz++
			}
		}
		m := big.NewInt(nz)
		m.Mul(m, new(big.Int).SetUint64(params.TxDataNonZeroGas))
		igas.Add(igas, m)
		m.SetInt64(int64(len(data)) - nz)
		m.Mul(m, new(big.Int).SetUint64(params.TxDataZeroGas))
		igas.Add(igas, m)
	}
	return igas
}

// NewStateTransition initialises and returns a new state transition object.
func NewStateTransition(evm *vm.EVM, msg Message, gp *GasPool) *StateTransition {
	return &StateTransition{
		gp:         gp,
		evm:        evm,
		msg:        msg,
		gasPrice:   msg.GasPrice(),
		initialGas: new(big.Int),
		value:      msg.Value(),
		data:       msg.Data(),
		state:      evm.PublicState(),
	}
}

// ApplyMessage computes the new state by applying the given message
// against the old state within the environment.
//
// ApplyMessage returns the bytes returned by any EVM execution (if it took place),
// the gas used (which includes gas refunds) and an error if it failed. An error always
// indicates a core error meaning that the message would always fail for that particular
// state and would never be accepted within a block.
func ApplyMessage(evm *vm.EVM, msg Message, gp *GasPool) ([]byte, *big.Int, bool, error) {
	st := NewStateTransition(evm, msg, gp)

	ret, _, gasUsed, failed, err := st.TransitionDb()
	return ret, gasUsed, failed, err
}

func (st *StateTransition) from() vm.AccountRef {
	f := st.msg.From()
	if !st.state.Exist(f) {
		st.state.CreateAccount(f)
	}
	return vm.AccountRef(f)
}

func (st *StateTransition) to() vm.AccountRef {
	if st.msg == nil {
		return vm.AccountRef{}
	}
	to := st.msg.To()
	if to == nil {
		return vm.AccountRef{} // contract creation
	}

	reference := vm.AccountRef(*to)
	if !st.state.Exist(*to) {
		st.state.CreateAccount(*to)
	}
	return reference
}

func (st *StateTransition) useGas(amount uint64) error {
	if st.gas < amount {
		return vm.ErrOutOfGas
	}
	st.gas -= amount

	return nil
}

func (st *StateTransition) buyGas() error {
	mgas := st.msg.Gas()
	if mgas.BitLen() > 64 {
		return vm.ErrOutOfGas
	}

	mgval := new(big.Int).Mul(mgas, st.gasPrice)

	var (
		state  = st.state
		sender = st.from()
	)
	if state.GetBalance(sender.Address()).Cmp(mgval) < 0 {
		return errInsufficientBalanceForGas
	}
	if err := st.gp.SubGas(mgas); err != nil {
		return err
	}
	st.gas += mgas.Uint64()

	st.initialGas.Set(mgas)
	state.SubBalance(sender.Address(), mgval)
	return nil
}

func (st *StateTransition) preCheck() error {
	msg := st.msg
	sender := st.from()

	// Make sure this transaction's nonce is correct
	if msg.CheckNonce() {
		nonce := st.state.GetNonce(sender.Address())
		if nonce < msg.Nonce() {
			return ErrNonceTooHigh
		} else if nonce > msg.Nonce() {
			return ErrNonceTooLow
		}
	}
	return st.buyGas()
}

// TransitionDb will transition the state by applying the current message and returning the result
// including the required gas for the operation as well as the used gas. It returns an error if it
// failed. An error indicates a consensus issue.
func (st *StateTransition) TransitionDb() (ret []byte, requiredGas, usedGas *big.Int, failed bool, err error) {
	if err = st.preCheck(); err != nil {
		return
	}
	msg := st.msg
	sender := st.from() // err checked in preCheck

	homestead := st.evm.ChainConfig().IsHomestead(st.evm.BlockNumber)
	contractCreation := msg.To() == nil
	isBitmed := st.evm.ChainConfig().IsBitmed

	var data []byte
	isPrivate := false
	publicState := st.state
	if msg, ok := msg.(PrivateMessage); ok && isBitmed && msg.IsPrivate() {
		isPrivate = true
		data, err = private.P.Receive(st.data)
		// Increment the public account nonce if:
		// 1. Tx is private and *not* a participant of the group and either call or create
		// 2. Tx is private we are part of the group and is a call
		if err != nil || !contractCreation {
			publicState.SetNonce(sender.Address(), publicState.GetNonce(sender.Address())+1)
		}

		if err != nil {
			return nil, new(big.Int), new(big.Int), false, nil
		}
	} else {
		data = st.data
	}

	// Pay intrinsic gas
	// TODO convert to uint64
	intrinsicGas := IntrinsicGas(data, contractCreation, homestead)
	if intrinsicGas.BitLen() > 64 {
		return nil, nil, nil, false, vm.ErrOutOfGas
	}
	if err = st.useGas(intrinsicGas.Uint64()); err != nil {
		return nil, nil, nil, false, err
	}

	var (
		evm = st.evm
		// vm errors do not effect consensus and are therefor
		// not assigned to err, except for insufficient balance
		// error.
		vmerr error
	)
	if contractCreation {
		ret, _, st.gas, vmerr = evm.Create(sender, data, st.gas, st.value)
	} else {
		// Increment the account nonce only if the transaction isn't private.
		// If the transaction is private it has already been incremented on
		// the public state.
		if !isPrivate {
			publicState.SetNonce(sender.Address(), publicState.GetNonce(sender.Address())+1)
		}
		var to common.Address
		if isBitmed {
			to = *st.msg.To()
		} else {
			to = st.to().Address()
		}
		ret, st.gas, vmerr = evm.Call(sender, to, data, st.gas, st.value)
	}
	if vmerr != nil {
		log.Debug("VM returned with error", "err", vmerr)
		// The only possible consensus-error would be if there wasn't
		// sufficient balance to make the transfer happen. The first
		// balance transfer may never fail.
		if vmerr == vm.ErrInsufficientBalance {
			return nil, nil, nil, false, vmerr
		}
	}
	requiredGas = new(big.Int).Set(st.gasUsed())

	st.refundGas()
	st.state.AddBalance(st.evm.Coinbase, new(big.Int).Mul(st.gasUsed(), st.gasPrice))

	if isPrivate {
		return ret, new(big.Int), new(big.Int), vmerr != nil, err
	}
	return ret, requiredGas, st.gasUsed(), vmerr != nil, err
}

func (st *StateTransition) refundGas() {
	// Return bxm for remaining gas to the sender account,
	// exchanged at the original rate.
	sender := st.from() // err already checked
	remaining := new(big.Int).Mul(new(big.Int).SetUint64(st.gas), st.gasPrice)
	st.state.AddBalance(sender.Address(), remaining)

	// Apply refund counter, capped to half of the used gas.
	uhalf := remaining.Div(st.gasUsed(), common.Big2)
	refund := math.BigMin(uhalf, st.state.GetRefund())
	st.gas += refund.Uint64()

	st.state.AddBalance(sender.Address(), refund.Mul(refund, st.gasPrice))

	// Also return remaining gas to the block gas counter so it is
	// available for the next transaction.
	st.gp.AddGas(new(big.Int).SetUint64(st.gas))
}

func (st *StateTransition) gasUsed() *big.Int {
	return new(big.Int).Sub(st.initialGas, new(big.Int).SetUint64(st.gas))
}
