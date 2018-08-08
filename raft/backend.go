package raft

import (
	"sync"
	"time"

	"github.com/InsighterInc/bxmp/accounts"
	"github.com/InsighterInc/bxmp/core"
	"github.com/InsighterInc/bxmp/bxm"
	"github.com/InsighterInc/bxmp/bxm/downloader"
	"github.com/InsighterInc/bxmp/bxmdb"
	"github.com/InsighterInc/bxmp/event"
	"github.com/InsighterInc/bxmp/log"
	"github.com/InsighterInc/bxmp/node"
	"github.com/InsighterInc/bxmp/p2p"
	"github.com/InsighterInc/bxmp/p2p/discover"
	"github.com/InsighterInc/bxmp/params"
	"github.com/InsighterInc/bxmp/rpc"
)

type RaftService struct {
	blockchain     *core.BlockChain
	chainDb        bxmdb.Database // Block chain database
	txMu           sync.Mutex
	txPool         *core.TxPool
	accountManager *accounts.Manager
	downloader     *downloader.Downloader

	raftProtocolManager *ProtocolManager
	startPeers          []*discover.Node

	// we need an event mux to instantiate the blockchain
	eventMux *event.TypeMux
	minter   *minter
}

func New(ctx *node.ServiceContext, chainConfig *params.ChainConfig, raftId, raftPort uint16, joinExisting bool, blockTime time.Duration, e *bxm.BitMED, startPeers []*discover.Node, datadir string) (*RaftService, error) {
	service := &RaftService{
		eventMux:       ctx.EventMux,
		chainDb:        e.ChainDb(),
		blockchain:     e.BlockChain(),
		txPool:         e.TxPool(),
		accountManager: e.AccountManager(),
		downloader:     e.Downloader(),
		startPeers:     startPeers,
	}

	service.minter = newMinter(chainConfig, service, blockTime)

	var err error
	if service.raftProtocolManager, err = NewProtocolManager(raftId, raftPort, service.blockchain, service.eventMux, startPeers, joinExisting, datadir, service.minter, service.downloader); err != nil {
		return nil, err
	}

	return service, nil
}

// Backend interface methods:

func (service *RaftService) AccountManager() *accounts.Manager { return service.accountManager }
func (service *RaftService) BlockChain() *core.BlockChain      { return service.blockchain }
func (service *RaftService) ChainDb() bxmdb.Database           { return service.chainDb }
func (service *RaftService) DappDb() bxmdb.Database            { return nil }
func (service *RaftService) EventMux() *event.TypeMux          { return service.eventMux }
func (service *RaftService) TxPool() *core.TxPool              { return service.txPool }

// node.Service interface methods:

func (service *RaftService) Protocols() []p2p.Protocol { return []p2p.Protocol{} }
func (service *RaftService) APIs() []rpc.API {
	return []rpc.API{
		{
			Namespace: "raft",
			Version:   "1.0",
			Service:   NewPublicRaftAPI(service),
			Public:    true,
		},
	}
}

// Start implements node.Service, starting the background data propagation thread
// of the protocol.
func (service *RaftService) Start(p2pServer *p2p.Server) error {
	service.raftProtocolManager.Start(p2pServer)
	return nil
}

// Stop implements node.Service, stopping the background data propagation thread
// of the protocol.
func (service *RaftService) Stop() error {
	service.blockchain.Stop()
	service.raftProtocolManager.Stop()
	service.minter.stop()
	service.eventMux.Stop()

	service.chainDb.Close()

	log.Info("Raft stopped")
	return nil
}
