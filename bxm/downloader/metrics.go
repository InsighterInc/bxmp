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

// Contains the metrics collected by the downloader.

package downloader

import (
	"github.com/InsighterInc/bxmp/metrics"
)

var (
	headerInMeter      = metrics.NewMeter("bxm/downloader/headers/in")
	headerReqTimer     = metrics.NewTimer("bxm/downloader/headers/req")
	headerDropMeter    = metrics.NewMeter("bxm/downloader/headers/drop")
	headerTimeoutMeter = metrics.NewMeter("bxm/downloader/headers/timeout")

	bodyInMeter      = metrics.NewMeter("bxm/downloader/bodies/in")
	bodyReqTimer     = metrics.NewTimer("bxm/downloader/bodies/req")
	bodyDropMeter    = metrics.NewMeter("bxm/downloader/bodies/drop")
	bodyTimeoutMeter = metrics.NewMeter("bxm/downloader/bodies/timeout")

	receiptInMeter      = metrics.NewMeter("bxm/downloader/receipts/in")
	receiptReqTimer     = metrics.NewTimer("bxm/downloader/receipts/req")
	receiptDropMeter    = metrics.NewMeter("bxm/downloader/receipts/drop")
	receiptTimeoutMeter = metrics.NewMeter("bxm/downloader/receipts/timeout")

	stateInMeter   = metrics.NewMeter("bxm/downloader/states/in")
	stateDropMeter = metrics.NewMeter("bxm/downloader/states/drop")
)
