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

// Contains the metrics collected by the fetcher.

package fetcher

import (
	"github.com/InsighterInc/bxmp/metrics"
)

var (
	propAnnounceInMeter   = metrics.NewMeter("bxm/fetcher/prop/announces/in")
	propAnnounceOutTimer  = metrics.NewTimer("bxm/fetcher/prop/announces/out")
	propAnnounceDropMeter = metrics.NewMeter("bxm/fetcher/prop/announces/drop")
	propAnnounceDOSMeter  = metrics.NewMeter("bxm/fetcher/prop/announces/dos")

	propBroadcastInMeter   = metrics.NewMeter("bxm/fetcher/prop/broadcasts/in")
	propBroadcastOutTimer  = metrics.NewTimer("bxm/fetcher/prop/broadcasts/out")
	propBroadcastDropMeter = metrics.NewMeter("bxm/fetcher/prop/broadcasts/drop")
	propBroadcastDOSMeter  = metrics.NewMeter("bxm/fetcher/prop/broadcasts/dos")

	headerFetchMeter = metrics.NewMeter("bxm/fetcher/fetch/headers")
	bodyFetchMeter   = metrics.NewMeter("bxm/fetcher/fetch/bodies")

	headerFilterInMeter  = metrics.NewMeter("bxm/fetcher/filter/headers/in")
	headerFilterOutMeter = metrics.NewMeter("bxm/fetcher/filter/headers/out")
	bodyFilterInMeter    = metrics.NewMeter("bxm/fetcher/filter/bodies/in")
	bodyFilterOutMeter   = metrics.NewMeter("bxm/fetcher/filter/bodies/out")
)
