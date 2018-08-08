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
	"github.com/InsighterInc/bxmp/consensus"
	"github.com/InsighterInc/bxmp/metrics"
	"github.com/InsighterInc/bxmp/p2p"
)

var (
	propTxnInPacketsMeter     = metrics.NewMeter("bxm/prop/txns/in/packets")
	propTxnInTrafficMeter     = metrics.NewMeter("bxm/prop/txns/in/traffic")
	propTxnOutPacketsMeter    = metrics.NewMeter("bxm/prop/txns/out/packets")
	propTxnOutTrafficMeter    = metrics.NewMeter("bxm/prop/txns/out/traffic")
	propHashInPacketsMeter    = metrics.NewMeter("bxm/prop/hashes/in/packets")
	propHashInTrafficMeter    = metrics.NewMeter("bxm/prop/hashes/in/traffic")
	propHashOutPacketsMeter   = metrics.NewMeter("bxm/prop/hashes/out/packets")
	propHashOutTrafficMeter   = metrics.NewMeter("bxm/prop/hashes/out/traffic")
	propBlockInPacketsMeter   = metrics.NewMeter("bxm/prop/blocks/in/packets")
	propBlockInTrafficMeter   = metrics.NewMeter("bxm/prop/blocks/in/traffic")
	propBlockOutPacketsMeter  = metrics.NewMeter("bxm/prop/blocks/out/packets")
	propBlockOutTrafficMeter  = metrics.NewMeter("bxm/prop/blocks/out/traffic")
	reqHeaderInPacketsMeter   = metrics.NewMeter("bxm/req/headers/in/packets")
	reqHeaderInTrafficMeter   = metrics.NewMeter("bxm/req/headers/in/traffic")
	reqHeaderOutPacketsMeter  = metrics.NewMeter("bxm/req/headers/out/packets")
	reqHeaderOutTrafficMeter  = metrics.NewMeter("bxm/req/headers/out/traffic")
	reqBodyInPacketsMeter     = metrics.NewMeter("bxm/req/bodies/in/packets")
	reqBodyInTrafficMeter     = metrics.NewMeter("bxm/req/bodies/in/traffic")
	reqBodyOutPacketsMeter    = metrics.NewMeter("bxm/req/bodies/out/packets")
	reqBodyOutTrafficMeter    = metrics.NewMeter("bxm/req/bodies/out/traffic")
	reqStateInPacketsMeter    = metrics.NewMeter("bxm/req/states/in/packets")
	reqStateInTrafficMeter    = metrics.NewMeter("bxm/req/states/in/traffic")
	reqStateOutPacketsMeter   = metrics.NewMeter("bxm/req/states/out/packets")
	reqStateOutTrafficMeter   = metrics.NewMeter("bxm/req/states/out/traffic")
	reqReceiptInPacketsMeter  = metrics.NewMeter("bxm/req/receipts/in/packets")
	reqReceiptInTrafficMeter  = metrics.NewMeter("bxm/req/receipts/in/traffic")
	reqReceiptOutPacketsMeter = metrics.NewMeter("bxm/req/receipts/out/packets")
	reqReceiptOutTrafficMeter = metrics.NewMeter("bxm/req/receipts/out/traffic")
	miscInPacketsMeter        = metrics.NewMeter("bxm/misc/in/packets")
	miscInTrafficMeter        = metrics.NewMeter("bxm/misc/in/traffic")
	miscOutPacketsMeter       = metrics.NewMeter("bxm/misc/out/packets")
	miscOutTrafficMeter       = metrics.NewMeter("bxm/misc/out/traffic")
)

// meteredMsgReadWriter is a wrapper around a p2p.MsgReadWriter, capable of
// accumulating the above defined metrics based on the data stream contents.
type meteredMsgReadWriter struct {
	p2p.MsgReadWriter     // Wrapped message stream to meter
	version           int // Protocol version to select correct meters
}

// newMeteredMsgWriter wraps a p2p MsgReadWriter with metering support. If the
// metrics system is disabled, this function returns the original object.
func newMeteredMsgWriter(rw p2p.MsgReadWriter) p2p.MsgReadWriter {
	if !metrics.Enabled {
		return rw
	}
	return &meteredMsgReadWriter{MsgReadWriter: rw}
}

// Init sets the protocol version used by the stream to know which meters to
// increment in case of overlapping message ids between protocol versions.
func (rw *meteredMsgReadWriter) Init(version int) {
	rw.version = version
}

func (rw *meteredMsgReadWriter) ReadMsg() (p2p.Msg, error) {
	// Read the message and short circuit in case of an error
	msg, err := rw.MsgReadWriter.ReadMsg()
	if err != nil {
		return msg, err
	}
	// Account for the data traffic
	packets, traffic := miscInPacketsMeter, miscInTrafficMeter
	switch {
	case msg.Code == BlockHeadersMsg:
		packets, traffic = reqHeaderInPacketsMeter, reqHeaderInTrafficMeter
	case msg.Code == BlockBodiesMsg:
		packets, traffic = reqBodyInPacketsMeter, reqBodyInTrafficMeter

	case rw.version >= consensus.Bxm63 && msg.Code == NodeDataMsg:
		packets, traffic = reqStateInPacketsMeter, reqStateInTrafficMeter
	case rw.version >= consensus.Bxm63 && msg.Code == ReceiptsMsg:
		packets, traffic = reqReceiptInPacketsMeter, reqReceiptInTrafficMeter

	case msg.Code == NewBlockHashesMsg:
		packets, traffic = propHashInPacketsMeter, propHashInTrafficMeter
	case msg.Code == NewBlockMsg:
		packets, traffic = propBlockInPacketsMeter, propBlockInTrafficMeter
	case msg.Code == TxMsg:
		packets, traffic = propTxnInPacketsMeter, propTxnInTrafficMeter
	}
	packets.Mark(1)
	traffic.Mark(int64(msg.Size))

	return msg, err
}

func (rw *meteredMsgReadWriter) WriteMsg(msg p2p.Msg) error {
	// Account for the data traffic
	packets, traffic := miscOutPacketsMeter, miscOutTrafficMeter
	switch {
	case msg.Code == BlockHeadersMsg:
		packets, traffic = reqHeaderOutPacketsMeter, reqHeaderOutTrafficMeter
	case msg.Code == BlockBodiesMsg:
		packets, traffic = reqBodyOutPacketsMeter, reqBodyOutTrafficMeter

	case rw.version >= consensus.Bxm63 && msg.Code == NodeDataMsg:
		packets, traffic = reqStateOutPacketsMeter, reqStateOutTrafficMeter
	case rw.version >= consensus.Bxm63 && msg.Code == ReceiptsMsg:
		packets, traffic = reqReceiptOutPacketsMeter, reqReceiptOutTrafficMeter

	case msg.Code == NewBlockHashesMsg:
		packets, traffic = propHashOutPacketsMeter, propHashOutTrafficMeter
	case msg.Code == NewBlockMsg:
		packets, traffic = propBlockOutPacketsMeter, propBlockOutTrafficMeter
	case msg.Code == TxMsg:
		packets, traffic = propTxnOutPacketsMeter, propTxnOutTrafficMeter
	}
	packets.Mark(1)
	traffic.Mark(int64(msg.Size))

	// Send the packet to the p2p layer
	return rw.MsgReadWriter.WriteMsg(msg)
}
