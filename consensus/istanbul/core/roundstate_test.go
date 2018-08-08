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
	"sync"
	"testing"

	"github.com/InsighterInc/bxmp/common"
	"github.com/InsighterInc/bxmp/consensus/istanbul"
)

func newTestRoundState(view *istanbul.View, validatorSet istanbul.ValidatorSet) *roundState {
	return &roundState{
		round:      view.Round,
		sequence:   view.Sequence,
		Preprepare: newTestPreprepare(view),
		Prepares:   newMessageSet(validatorSet),
		Commits:    newMessageSet(validatorSet),
		mu:         new(sync.RWMutex),
	}
}

func TestLockHash(t *testing.T) {
	sys := NewTestSystemWithBackend(1, 0)
	rs := newTestRoundState(
		&istanbul.View{
			Round:    big.NewInt(0),
			Sequence: big.NewInt(0),
		},
		sys.backends[0].peers,
	)
	if !common.EmptyHash(rs.GetLockedHash()) {
		t.Errorf("error mismatch: have %v, want empty", rs.GetLockedHash())
	}
	if rs.IsHashLocked() {
		t.Error("IsHashLocked should return false")
	}

	// Lock
	expected := rs.Proposal().Hash()
	rs.LockHash()
	if expected != rs.GetLockedHash() {
		t.Errorf("error mismatch: have %v, want %v", rs.GetLockedHash(), expected)
	}
	if !rs.IsHashLocked() {
		t.Error("IsHashLocked should return true")
	}

	// Unlock
	rs.UnlockHash()
	if !common.EmptyHash(rs.GetLockedHash()) {
		t.Errorf("error mismatch: have %v, want empty", rs.GetLockedHash())
	}
	if rs.IsHashLocked() {
		t.Error("IsHashLocked should return false")
	}
}
