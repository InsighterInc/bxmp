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

// Package consensus implements different BitMED consensus engines.
package consensus

import (
	"github.com/InsighterInc/bxmp/common"
	"github.com/InsighterInc/bxmp/core/types"
)

// Constants to match up protocol versions and messages
//TODO: We'll need to change this version before getting developer onboard to avoid confusion
const (
		Bxm62 = 62
    	Bxm63 = 63
)

var (
	BxmProtocol = Protocol{
		Name:     "bxm",
		Versions: []uint{Bxm62, Bxm63},
		Lengths:  []uint64{17, 8},
	}
)

// Protocol defines the protocol of the consensus
type Protocol struct {
	// Official short name of the protocol used during capability negotiation.
	Name string
	// Supported versions of the bxm protocol (first is primary).
	Versions []uint
	// Number of implemented message corresponding to different protocol versions.
	Lengths []uint64
}

// Broadcaster defines the interface to broadcast blocks and find peer
type Broadcaster interface {
	// BroadcastBlock broadcasts blocks to peers
	BroadcastBlock(block *types.Block, propagate bool)
	// FindPeers retrives peers by addresses
	FindPeers(map[common.Address]bool) map[common.Address]Peer
}

// Peer defines the interface to communicate with peer
type Peer interface {
	// Send sends the message to this peer
	Send(msgcode uint64, data interface{}) error
}
