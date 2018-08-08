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

// Package les implements the Light BitMED Subprotocol.
package les

import (
	"fmt"
	"sync"
	"time"

	"github.com/InsighterInc/bxmp/accounts"
	"github.com/InsighterInc/bxmp/common"
	"github.com/InsighterInc/bxmp/common/hexutil"
	"github.com/InsighterInc/bxmp/consensus"
	"github.com/InsighterInc/bxmp/core"
	"github.com/InsighterInc/bxmp/core/types"
	"github.com/InsighterInc/bxmp/bxm"
	"github.com/InsighterInc/bxmp/bxm/downloader"
	"github.com/InsighterInc/bxmp/bxm/filters"
	"github.com/InsighterInc/bxmp/bxm/gasprice"
	"github.com/InsighterInc/bxmp/bxmdb"
	"github.com/InsighterInc/bxmp/event"
	"github.com/InsighterInc/bxmp/internal/bxmapi"
	"github.com/InsighterInc/bxmp/light"
	"github.com/InsighterInc/bxmp/log"
	"github.com/InsighterInc/bxmp/node"
	"github.com/InsighterInc/bxmp/p2p"
	"github.com/InsighterInc/bxmp/p2p/discv5"
	"github.com/InsighterInc/bxmp/params"
	rpc "github.com/InsighterInc/bxmp/rpc"
)

type LightBitmed struct {
	odr         *LesOdr
	relay       *LesTxRelay
	chainConfig *params.ChainConfig
	// Channel for shutting down the service
	shutdownChan chan bool
	// Handlers
	peers           *peerSet
	txPool          *light.TxPool
	blockchain      *light.LightChain
	protocolManager *ProtocolManager
	serverPool      *serverPool
	reqDist         *requestDistributor
	retriever       *retrieveManager
	// DB interfaces
	chainDb bxmdb.Database // Block chain database

	ApiBackend *LesApiBackend

	eventMux       *event.TypeMux
	engine         consensus.Engine
	accountManager *accounts.Manager

	networkId     uint64
	netRPCService *bxmapi.PublicNetAPI

	wg sync.WaitGroup
}

func New(ctx *node.ServiceContext, config *bxm.Config) (*LightBitmed, error) {
	chainDb, err := bxm.CreateDB(ctx, config, "lightchaindata")
	if err != nil {
		return nil, err
	}
	chainConfig, genesisHash, genesisErr := core.SetupGenesisBlock(chainDb, config.Genesis)
	if _, isCompat := genesisErr.(*params.ConfigCompatError); genesisErr != nil && !isCompat {
		return nil, genesisErr
	}
	log.Info("Initialised chain configuration", "config", chainConfig)

	peers := newPeerSet()
	quitSync := make(chan struct{})

	bxm := &LightBitmed{
		chainConfig:    chainConfig,
		chainDb:        chainDb,
		eventMux:       ctx.EventMux,
		peers:          peers,
		reqDist:        newRequestDistributor(peers, quitSync),
		accountManager: ctx.AccountManager,
		engine:         bxm.CreateConsensusEngine(ctx, config, chainConfig, chainDb),
		shutdownChan:   make(chan bool),
		networkId:      config.NetworkId,
	}

	bxm.relay = NewLesTxRelay(peers, bxm.reqDist)
	bxm.serverPool = newServerPool(chainDb, quitSync, &bxm.wg)
	bxm.retriever = newRetrieveManager(peers, bxm.reqDist, bxm.serverPool)
	bxm.odr = NewLesOdr(chainDb, bxm.retriever)
	if bxm.blockchain, err = light.NewLightChain(bxm.odr, bxm.chainConfig, bxm.engine); err != nil {
		return nil, err
	}
	// Rewind the chain in case of an incompatible config upgrade.
	if compat, ok := genesisErr.(*params.ConfigCompatError); ok {
		log.Warn("Rewinding chain to upgrade configuration", "err", compat)
		bxm.blockchain.SetHead(compat.RewindTo)
		core.WriteChainConfig(chainDb, genesisHash, chainConfig)
	}

	bxm.txPool = light.NewTxPool(bxm.chainConfig, bxm.blockchain, bxm.relay)
	if bxm.protocolManager, err = NewProtocolManager(bxm.chainConfig, true, config.NetworkId, bxm.eventMux, bxm.engine, bxm.peers, bxm.blockchain, nil, chainDb, bxm.odr, bxm.relay, quitSync, &bxm.wg); err != nil {
		return nil, err
	}
	bxm.ApiBackend = &LesApiBackend{bxm, nil}
	gpoParams := config.GPO
	if gpoParams.Default == nil {
		gpoParams.Default = config.GasPrice
	}
	bxm.ApiBackend.gpo = gasprice.NewOracle(bxm.ApiBackend, gpoParams)
	return bxm, nil
}

func lesTopic(genesisHash common.Hash) discv5.Topic {
	return discv5.Topic("LES@" + common.Bytes2Hex(genesisHash.Bytes()[0:8]))
}

type LightDummyAPI struct{}

// Bxmbase is the address that mining rewards will be send to
func (s *LightDummyAPI) Bxmbase() (common.Address, error) {
	return common.Address{}, fmt.Errorf("not supported")
}

// Coinbase is the address that mining rewards will be send to (alias for Bxmbase)
func (s *LightDummyAPI) Coinbase() (common.Address, error) {
	return common.Address{}, fmt.Errorf("not supported")
}

// Hashrate returns the POW hashrate
func (s *LightDummyAPI) Hashrate() hexutil.Uint {
	return 0
}

// Mining returns an indication if this node is currently mining.
func (s *LightDummyAPI) Mining() bool {
	return false
}

// APIs returns the collection of RPC services the bitmed package offers.
// NOTE, some of these services probably need to be moved to somewhere else.
func (s *LightBitmed) APIs() []rpc.API {
	return append(bxmapi.GetAPIs(s.ApiBackend), []rpc.API{
		{
			Namespace: "bxm",
			Version:   "1.0",
			Service:   &LightDummyAPI{},
			Public:    true,
		}, {
			Namespace: "bxm",
			Version:   "1.0",
			Service:   downloader.NewPublicDownloaderAPI(s.protocolManager.downloader, s.eventMux),
			Public:    true,
		}, {
			Namespace: "bxm",
			Version:   "1.0",
			Service:   filters.NewPublicFilterAPI(s.ApiBackend, true),
			Public:    true,
		}, {
			Namespace: "net",
			Version:   "1.0",
			Service:   s.netRPCService,
			Public:    true,
		},
	}...)
}

func (s *LightBitmed) ResetWithGenesisBlock(gb *types.Block) {
	s.blockchain.ResetWithGenesisBlock(gb)
}

func (s *LightBitmed) BlockChain() *light.LightChain      { return s.blockchain }
func (s *LightBitmed) TxPool() *light.TxPool              { return s.txPool }
func (s *LightBitmed) Engine() consensus.Engine           { return s.engine }
func (s *LightBitmed) LesVersion() int                    { return int(s.protocolManager.SubProtocols[0].Version) }
func (s *LightBitmed) Downloader() *downloader.Downloader { return s.protocolManager.downloader }
func (s *LightBitmed) EventMux() *event.TypeMux           { return s.eventMux }

// Protocols implements node.Service, returning all the currently configured
// network protocols to start.
func (s *LightBitmed) Protocols() []p2p.Protocol {
	return s.protocolManager.SubProtocols
}

// Start implements node.Service, starting all internal goroutines needed by the
// BitMED protocol implementation.
func (s *LightBitmed) Start(srvr *p2p.Server) error {
	log.Warn("Light client mode is an experimental feature")
	s.netRPCService = bxmapi.NewPublicNetAPI(srvr, s.networkId)
	s.serverPool.start(srvr, lesTopic(s.blockchain.Genesis().Hash()))
	s.protocolManager.Start()
	return nil
}

// Stop implements node.Service, terminating all internal goroutines used by the
// BitMED protocol.
func (s *LightBitmed) Stop() error {
	s.odr.Stop()
	s.blockchain.Stop()
	s.protocolManager.Stop()
	s.txPool.Stop()

	s.eventMux.Stop()

	time.Sleep(time.Millisecond * 200)
	s.chainDb.Close()
	close(s.shutdownChan)

	return nil
}
