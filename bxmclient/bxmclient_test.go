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

package bxmclient

import "github.com/InsighterInc/bxmp"

// Verify that Client implements the bitmed interfaces.
var (
	_ = bitmed.ChainReader(&Client{})
	_ = bitmed.TransactionReader(&Client{})
	_ = bitmed.ChainStateReader(&Client{})
	_ = bitmed.ChainSyncReader(&Client{})
	_ = bitmed.ContractCaller(&Client{})
	_ = bitmed.GasEstimator(&Client{})
	_ = bitmed.GasPricer(&Client{})
	_ = bitmed.LogFilterer(&Client{})
	_ = bitmed.PendingStateReader(&Client{})
	// _ = bitmed.PendingStateEventer(&Client{})
	_ = bitmed.PendingContractCaller(&Client{})
)
