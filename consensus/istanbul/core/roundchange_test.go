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

package core

import (
	"math/big"
	"testing"

	"github.com/InsighterInc/bxmp/common"
	"github.com/InsighterInc/bxmp/consensus/istanbul"
	"github.com/InsighterInc/bxmp/consensus/istanbul/validator"
)

func TestRoundChangeSet(t *testing.T) {
	vset := validator.NewSet(generateValidators(4), istanbul.RoundRobin)
	rc := newRoundChangeSet(vset)

	view := &istanbul.View{
		Sequence: big.NewInt(1),
		Round:    big.NewInt(1),
	}
	r := &roundChange{
		Round:    view.Round,
		Sequence: view.Sequence,
		Digest:   common.Hash{},
	}
	m, _ := Encode(r)

	// Test Add()
	// Add message from all validators
	for i, v := range vset.List() {
		msg := &message{
			Code:    msgRoundChange,
			Msg:     m,
			Address: v.Address(),
		}
		rc.Add(view.Round, msg)
		if rc.roundChanges[view.Round.Uint64()].Size() != i+1 {
			t.Errorf("the size of round change messages mismatch: have %v, want %v", rc.roundChanges[view.Round.Uint64()].Size(), i+1)
		}
	}

	// Add message again from all validators, but the size should be the same
	for _, v := range vset.List() {
		msg := &message{
			Code:    msgRoundChange,
			Msg:     m,
			Address: v.Address(),
		}
		rc.Add(view.Round, msg)
		if rc.roundChanges[view.Round.Uint64()].Size() != vset.Size() {
			t.Errorf("the size of round change messages mismatch: have %v, want %v", rc.roundChanges[view.Round.Uint64()].Size(), vset.Size())
		}
	}

	// Test MaxRound()
	for i := 0; i < 10; i++ {
		maxRound := rc.MaxRound(i)
		if i <= vset.Size() {
			if maxRound == nil || maxRound.Cmp(view.Round) != 0 {
				t.Errorf("max round mismatch: have %v, want %v", maxRound, view.Round)
			}
		} else if maxRound != nil {
			t.Errorf("max round mismatch: have %v, want nil", maxRound)
		}
	}

	// Test Clear()
	for i := int64(0); i < 2; i++ {
		rc.Clear(big.NewInt(i))
		if rc.roundChanges[view.Round.Uint64()].Size() != vset.Size() {
			t.Errorf("the size of round change messages mismatch: have %v, want %v", rc.roundChanges[view.Round.Uint64()].Size(), vset.Size())
		}
	}
	rc.Clear(big.NewInt(2))
	if rc.roundChanges[view.Round.Uint64()] != nil {
		t.Errorf("the change messages mismatch: have %v, want nil", rc.roundChanges[view.Round.Uint64()])
	}
}
