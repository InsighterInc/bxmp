// Copyright 2017 The BXMP Authors
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

package backend

import (
	"bytes"
	"crypto/ecdsa"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/InsighterInc/bxmp/common"
	"github.com/InsighterInc/bxmp/consensus/istanbul"
	"github.com/InsighterInc/bxmp/consensus/istanbul/validator"
	"github.com/InsighterInc/bxmp/core/types"
	"github.com/InsighterInc/bxmp/crypto"
	"github.com/InsighterInc/bxmp/log"
)

func TestSign(t *testing.T) {
	b, _, _ := newBackend()
	data := []byte("Here is a string....")
	sig, err := b.Sign(data)
	if err != nil {
		t.Errorf("error mismatch: have %v, want nil", err)
	}
	//Check signature recover
	hashData := crypto.Keccak256([]byte(data))
	pubkey, _ := crypto.Ecrecover(hashData, sig)
	var signer common.Address
	copy(signer[:], crypto.Keccak256(pubkey[1:])[12:])
	if signer != getAddress() {
		t.Errorf("address mismatch: have %v, want %s", signer.Hex(), getAddress().Hex())
	}
}

func TestCheckSignature(t *testing.T) {
	key, _ := generatePrivateKey()
	data := []byte("Here is a string....")
	hashData := crypto.Keccak256([]byte(data))
	sig, _ := crypto.Sign(hashData, key)
	b, _, _ := newBackend()
	a := getAddress()
	err := b.CheckSignature(data, a, sig)
	if err != nil {
		t.Errorf("error mismatch: have %v, want nil", err)
	}
	a = getInvalidAddress()
	err = b.CheckSignature(data, a, sig)
	if err != errInvalidSignature {
		t.Errorf("error mismatch: have %v, want %v", err, errInvalidSignature)
	}
}

func TestCheckValidatorSignature(t *testing.T) {
	_, keys, vset := newBackend()

	// 1. Positive test: sign with validator's key should succeed
	data := []byte("dummy data")
	hashData := crypto.Keccak256([]byte(data))
	for i, k := range keys {
		// Sign
		sig, err := crypto.Sign(hashData, k)
		if err != nil {
			t.Errorf("error mismatch: have %v, want nil", err)
		}
		// CheckValidatorSignature should succeed
		addr, err := istanbul.CheckValidatorSignature(vset, data, sig)
		if err != nil {
			t.Errorf("error mismatch: have %v, want nil", err)
		}
		validator := vset.GetByIndex(uint64(i))
		if addr != validator.Address() {
			t.Errorf("validator address mismatch: have %v, want %v", addr, validator.Address())
		}
	}

	// 2. Negative test: sign with any key other than validator's key should return error
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Errorf("error mismatch: have %v, want nil", err)
	}
	// Sign
	sig, err := crypto.Sign(hashData, key)
	if err != nil {
		t.Errorf("error mismatch: have %v, want nil", err)
	}

	// CheckValidatorSignature should return ErrUnauthorizedAddress
	addr, err := istanbul.CheckValidatorSignature(vset, data, sig)
	if err != istanbul.ErrUnauthorizedAddress {
		t.Errorf("error mismatch: have %v, want %v", err, istanbul.ErrUnauthorizedAddress)
	}
	emptyAddr := common.Address{}
	if addr != emptyAddr {
		t.Errorf("address mismatch: have %v, want %v", addr, emptyAddr)
	}
}

func TestCommit(t *testing.T) {
	backend, _, _ := newBackend()

	commitCh := make(chan *types.Block)
	// Case: it's a proposer, so the backend.commit will receive channel result from backend.Commit function
	testCases := []struct {
		expectedErr       error
		expectedSignature [][]byte
		expectedBlock     func() *types.Block
	}{
		{
			// normal case
			nil,
			[][]byte{append([]byte{1}, bytes.Repeat([]byte{0x00}, types.IstanbulExtraSeal-1)...)},
			func() *types.Block {
				chain, engine := newBlockChain(1)
				block := makeBlockWithoutSeal(chain, engine, chain.Genesis())
				expectedBlock, _ := engine.updateBlock(engine.chain.GetHeader(block.ParentHash(), block.NumberU64()-1), block)
				return expectedBlock
			},
		},
		{
			// invalid signature
			errInvalidCommittedSeals,
			nil,
			func() *types.Block {
				chain, engine := newBlockChain(1)
				block := makeBlockWithoutSeal(chain, engine, chain.Genesis())
				expectedBlock, _ := engine.updateBlock(engine.chain.GetHeader(block.ParentHash(), block.NumberU64()-1), block)
				return expectedBlock
			},
		},
	}

	for _, test := range testCases {
		expBlock := test.expectedBlock()
		go func() {
			select {
			case result := <-backend.commitCh:
				commitCh <- result
				return
			}
		}()

		backend.proposedBlockHash = expBlock.Hash()
		if err := backend.Commit(expBlock, test.expectedSignature); err != nil {
			if err != test.expectedErr {
				t.Errorf("error mismatch: have %v, want %v", err, test.expectedErr)
			}
		}

		if test.expectedErr == nil {
			// to avoid race condition is occurred by goroutine
			select {
			case result := <-commitCh:
				if result.Hash() != expBlock.Hash() {
					t.Errorf("hash mismatch: have %v, want %v", result.Hash(), expBlock.Hash())
				}
			case <-time.After(10 * time.Second):
				t.Fatal("timeout")
			}
		}
	}
}

func TestGetProposer(t *testing.T) {
	chain, engine := newBlockChain(1)
	block := makeBlock(chain, engine, chain.Genesis())
	chain.InsertChain(types.Blocks{block})
	expected := engine.GetProposer(1)
	actual := engine.Address()
	if actual != expected {
		t.Errorf("proposer mismatch: have %v, want %v", actual.Hex(), expected.Hex())
	}
}

/**
 * SimpleBackend
 * Private key: bb047e5940b6d83354d9432db7c449ac8fca2248008aaa7271369880f9f11cc1
 * Public key: 04a2bfb0f7da9e1b9c0c64e14f87e8fb82eb0144e97c25fe3a977a921041a50976984d18257d2495e7bfd3d4b280220217f429287d25ecdf2b0d7c0f7aae9aa624
 * Address: 0x70524d664ffe731100208a0154e556f9bb679ae6
 */
func getAddress() common.Address {
	return common.HexToAddress("0x70524d664ffe731100208a0154e556f9bb679ae6")
}

func getInvalidAddress() common.Address {
	return common.HexToAddress("0x9535b2e7faaba5288511d89341d94a38063a349b")
}

func generatePrivateKey() (*ecdsa.PrivateKey, error) {
	key := "bb047e5940b6d83354d9432db7c449ac8fca2248008aaa7271369880f9f11cc1"
	return crypto.HexToECDSA(key)
}

func newTestValidatorSet(n int) (istanbul.ValidatorSet, []*ecdsa.PrivateKey) {
	// generate validators
	keys := make(Keys, n)
	addrs := make([]common.Address, n)
	for i := 0; i < n; i++ {
		privateKey, _ := crypto.GenerateKey()
		keys[i] = privateKey
		addrs[i] = crypto.PubkeyToAddress(privateKey.PublicKey)
	}
	vset := validator.NewSet(addrs, istanbul.RoundRobin)
	sort.Sort(keys) //Keys need to be sorted by its public key address
	return vset, keys
}

type Keys []*ecdsa.PrivateKey

func (slice Keys) Len() int {
	return len(slice)
}

func (slice Keys) Less(i, j int) bool {
	return strings.Compare(crypto.PubkeyToAddress(slice[i].PublicKey).String(), crypto.PubkeyToAddress(slice[j].PublicKey).String()) < 0
}

func (slice Keys) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

func newBackend() (b *backend, validatorKeys Keys, validatorSet istanbul.ValidatorSet) {
	key, _ := generatePrivateKey()
	validatorSet, validatorKeys = newTestValidatorSet(5)
	b = &backend{
		privateKey: key,
		logger:     log.New("backend", "simple"),
		commitCh:   make(chan *types.Block, 1),
	}
	return
}
