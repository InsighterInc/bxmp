// Copyright 2014 The BXMP Authors
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

// Package bxm implements the BitMED protocol.
package bxm

import (
	"errors"
	"fmt"
	"math/big"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/InsighterInc/bxmp/accounts"
	"github.com/InsighterInc/bxmp/common"
	"github.com/InsighterInc/bxmp/common/hexutil"
	"github.com/InsighterInc/bxmp/consensus"
	"github.com/InsighterInc/bxmp/consensus/clique"
	"github.com/InsighterInc/bxmp/consensus/ethash"
	"github.com/InsighterInc/bxmp/consensus/istanbul"
	istanbulBackend "github.com/InsighterInc/bxmp/consensus/istanbul/backend"
	"github.com/InsighterInc/bxmp/core"
	"github.com/InsighterInc/bxmp/core/bloombits"
	"github.com/InsighterInc/bxmp/core/types"
	"github.com/InsighterInc/bxmp/core/vm"
	"github.com/InsighterInc/bxmp/crypto"
	"github.com/InsighterInc/bxmp/bxm/downloader"
	"github.com/InsighterInc/bxmp/bxm/filters"
	"github.com/InsighterInc/bxmp/bxm/gasprice"
	"github.com/InsighterInc/bxmp/bxmdb"
	"github.com/InsighterInc/bxmp/event"
	"github.com/InsighterInc/bxmp/internal/bxmapi"
	"github.com/InsighterInc/bxmp/log"
	"github.com/InsighterInc/bxmp/miner"
	"github.com/InsighterInc/bxmp/node"
	"github.com/InsighterInc/bxmp/p2p"
	"github.com/InsighterInc/bxmp/params"
	"github.com/InsighterInc/bxmp/rlp"
	"github.com/InsighterInc/bxmp/rpc"
)

type LesServer interface {
	Start(srvr *p2p.Server)
	Stop()
	Protocols() []p2p.Protocol
}

// BitMED implements the BitMED full node service.
type BitMED struct {
	config      *Config
	chainConfig *params.ChainConfig

	// Channel for shutting down the service
	shutdownChan  chan bool    // Channel for shutting down the bitmed
	stopDbUpgrade func() error // stop chain db sequential key upgrade

	// Handlers
	txPool          *core.TxPool
	blockchain      *core.BlockChain
	protocolManager *ProtocolManager
	lesServer       LesServer

	// DB interfaces
	chainDb bxmdb.Database // Block chain database

	eventMux       *event.TypeMux
	engine         consensus.Engine
	accountManager *accounts.Manager

	bloomRequests chan chan *bloombits.Retrieval // Channel receiving bloom data retrieval requests
	bloomIndexer  *core.ChainIndexer             // Bloom indexer operating during block imports

	ApiBackend *BxmApiBackend

	miner     *miner.Miner
	gasPrice  *big.Int
	bxmbase common.Address

	networkId     uint64
	netRPCService *bxmapi.PublicNetAPI

	lock sync.RWMutex // Protects the variadic fields (e.g. gas price and bxmbase)
}

// HACK(joel) this was added just to make the bxm chain config visible to RegisterRaftService
func (s *BitMED) ChainConfig() *params.ChainConfig {
	return s.chainConfig
}

func (s *BitMED) AddLesServer(ls LesServer) {
	s.lesServer = ls
}

// New creates a new BitMED object (including the
// initialisation of the common BitMED object)
func New(ctx *node.ServiceContext, config *Config) (*BitMED, error) {
	if config.SyncMode == downloader.LightSync {
		return nil, errors.New("can't run bxm.BitMED in light sync mode, use les.LightBitmed")
	}
	if !config.SyncMode.IsValid() {
		return nil, fmt.Errorf("invalid sync mode %d", config.SyncMode)
	}
	chainDb, err := CreateDB(ctx, config, "chaindata")
	if err != nil {
		return nil, err
	}
	stopDbUpgrade := upgradeDeduplicateData(chainDb)
	chainConfig, genesisHash, genesisErr := core.SetupGenesisBlock(chainDb, config.Genesis)
	if _, ok := genesisErr.(*params.ConfigCompatError); genesisErr != nil && !ok {
		return nil, genesisErr
	}
	log.Info("Initialised chain configuration", "config", chainConfig)

	bxm := &BitMED{
		config:         config,
		chainDb:        chainDb,
		chainConfig:    chainConfig,
		eventMux:       ctx.EventMux,
		accountManager: ctx.AccountManager,
		engine:         CreateConsensusEngine(ctx, config, chainConfig, chainDb),
		shutdownChan:   make(chan bool),
		stopDbUpgrade:  stopDbUpgrade,
		networkId:      config.NetworkId,
		gasPrice:       config.GasPrice,
		bxmbase:      config.Bxmbase,
		bloomRequests:  make(chan chan *bloombits.Retrieval),
		bloomIndexer:   NewBloomIndexer(chainDb, params.BloomBitsBlocks),
	}

	// force to set the istanbul bxmbase to node key address
	if chainConfig.Istanbul != nil {
		bxm.bxmbase = crypto.PubkeyToAddress(ctx.NodeKey().PublicKey)
	}

	log.Info("Initialising BitMED protocol", "versions", ProtocolVersions, "network", config.NetworkId)

	if !config.SkipBcVersionCheck {
		bcVersion := core.GetBlockChainVersion(chainDb)
		if bcVersion != core.BlockChainVersion && bcVersion != 0 {
			return nil, fmt.Errorf("Blockchain DB version mismatch (%d / %d). Run geth upgradedb.\n", bcVersion, core.BlockChainVersion)
		}
		core.WriteBlockChainVersion(chainDb, core.BlockChainVersion)
	}

	vmConfig := vm.Config{EnablePreimageRecording: config.EnablePreimageRecording}
	bxm.blockchain, err = core.NewBlockChain(chainDb, bxm.chainConfig, bxm.engine, vmConfig)
	if err != nil {
		return nil, err
	}
	// Rewind the chain in case of an incompatible config upgrade.
	if compat, ok := genesisErr.(*params.ConfigCompatError); ok {
		log.Warn("Rewinding chain to upgrade configuration", "err", compat)
		bxm.blockchain.SetHead(compat.RewindTo)
		core.WriteChainConfig(chainDb, genesisHash, chainConfig)
	}
	bxm.bloomIndexer.Start(bxm.blockchain.CurrentHeader(), bxm.blockchain.SubscribeChainEvent)

	if config.TxPool.Journal != "" {
		config.TxPool.Journal = ctx.ResolvePath(config.TxPool.Journal)
	}
	bxm.txPool = core.NewTxPool(config.TxPool, bxm.chainConfig, bxm.blockchain)

	if bxm.protocolManager, err = NewProtocolManager(bxm.chainConfig, config.SyncMode, config.NetworkId, bxm.eventMux, bxm.txPool, bxm.engine, bxm.blockchain, chainDb, config.RaftMode); err != nil {
		return nil, err
	}
	bxm.miner = miner.New(bxm, bxm.chainConfig, bxm.EventMux(), bxm.engine)
	bxm.miner.SetExtra(makeExtraData(config.ExtraData, bxm.chainConfig.IsBitmed))

	bxm.ApiBackend = &BxmApiBackend{bxm, nil}
	gpoParams := config.GPO
	if gpoParams.Default == nil {
		gpoParams.Default = config.GasPrice
	}
	bxm.ApiBackend.gpo = gasprice.NewOracle(bxm.ApiBackend, gpoParams)

	return bxm, nil
}

func makeExtraData(extra []byte, isBitmed bool) []byte {
	if len(extra) == 0 {
		// create default extradata
		extra, _ = rlp.EncodeToBytes([]interface{}{
			uint(params.VersionMajor<<16 | params.VersionMinor<<8 | params.VersionPatch),
			"geth",
			runtime.Version(),
			runtime.GOOS,
		})
	}
	if uint64(len(extra)) > params.GetMaximumExtraDataSize(isBitmed) {
		log.Warn("Miner extra data exceed limit", "extra", hexutil.Bytes(extra), "limit", params.GetMaximumExtraDataSize(isBitmed))
		extra = nil
	}
	return extra
}

// CreateDB creates the chain database.
func CreateDB(ctx *node.ServiceContext, config *Config, name string) (bxmdb.Database, error) {
	db, err := ctx.OpenDatabase(name, config.DatabaseCache, config.DatabaseHandles)
	if err != nil {
		return nil, err
	}
	if db, ok := db.(*bxmdb.LDBDatabase); ok {
		db.Meter("bxm/db/chaindata/")
	}
	return db, nil
}

// CreateConsensusEngine creates the required type of consensus engine instance for an BitMED service
func CreateConsensusEngine(ctx *node.ServiceContext, config *Config, chainConfig *params.ChainConfig, db bxmdb.Database) consensus.Engine {
	// If proof-of-authority is requested, set it up
	if chainConfig.Clique != nil {
		return clique.New(chainConfig.Clique, db)
	}
	// If Istanbul is requested, set it up
	if chainConfig.Istanbul != nil {
		if chainConfig.Istanbul.Epoch != 0 {
			config.Istanbul.Epoch = chainConfig.Istanbul.Epoch
		}
		config.Istanbul.ProposerPolicy = istanbul.ProposerPolicy(chainConfig.Istanbul.ProposerPolicy)
		return istanbulBackend.New(&config.Istanbul, ctx.NodeKey(), db)
	}

	// Otherwise assume proof-of-work
	switch {
	case config.PowFake:
		log.Warn("Ethash used in fake mode")
		return ethash.NewFaker()
	case config.PowTest:
		log.Warn("Ethash used in test mode")
		return ethash.NewTester()
	case config.PowShared:
		log.Warn("Ethash used in shared mode")
		return ethash.NewShared()
	default:
		engine := ethash.New(ctx.ResolvePath(config.EthashCacheDir), config.EthashCachesInMem, config.EthashCachesOnDisk,
			config.EthashDatasetDir, config.EthashDatasetsInMem, config.EthashDatasetsOnDisk)
		engine.SetThreads(-1) // Disable CPU mining
		return engine
	}
}

// APIs returns the collection of RPC services the bitmed package offers.
// NOTE, some of these services probably need to be moved to somewhere else.
func (s *BitMED) APIs() []rpc.API {
	apis := bxmapi.GetAPIs(s.ApiBackend)

	// Append any APIs exposed explicitly by the consensus engine
	apis = append(apis, s.engine.APIs(s.BlockChain())...)

	// Append all the local APIs and return
	return append(apis, []rpc.API{
		{
			Namespace: "bxm",
			Version:   "1.0",
			Service:   NewPublicBitmedAPI(s),
			Public:    true,
		}, {
			Namespace: "bxm",
			Version:   "1.0",
			Service:   NewPublicMinerAPI(s),
			Public:    true,
		}, {
			Namespace: "bxm",
			Version:   "1.0",
			Service:   downloader.NewPublicDownloaderAPI(s.protocolManager.downloader, s.eventMux),
			Public:    true,
		}, {
			Namespace: "miner",
			Version:   "1.0",
			Service:   NewPrivateMinerAPI(s),
			Public:    false,
		}, {
			Namespace: "bxm",
			Version:   "1.0",
			Service:   filters.NewPublicFilterAPI(s.ApiBackend, false),
			Public:    true,
		}, {
			Namespace: "admin",
			Version:   "1.0",
			Service:   NewPrivateAdminAPI(s),
		}, {
			Namespace: "debug",
			Version:   "1.0",
			Service:   NewPublicDebugAPI(s),
			Public:    true,
		}, {
			Namespace: "debug",
			Version:   "1.0",
			Service:   NewPrivateDebugAPI(s.chainConfig, s),
		}, {
			Namespace: "net",
			Version:   "1.0",
			Service:   s.netRPCService,
			Public:    true,
		},
	}...)
}

func (s *BitMED) ResetWithGenesisBlock(gb *types.Block) {
	s.blockchain.ResetWithGenesisBlock(gb)
}

func (s *BitMED) Bxmbase() (eb common.Address, err error) {
	s.lock.RLock()
	bxmbase := s.bxmbase
	s.lock.RUnlock()

	if bxmbase != (common.Address{}) {
		return bxmbase, nil
	}
	if wallets := s.AccountManager().Wallets(); len(wallets) > 0 {
		if accounts := wallets[0].Accounts(); len(accounts) > 0 {
			return accounts[0].Address, nil
		}
	}
	return common.Address{}, fmt.Errorf("bxmbase address must be explicitly specified")
}

// set in js console via admin interface or wrapper from cli flags
func (self *BitMED) SetBxmbase(bxmbase common.Address) {
	self.lock.Lock()
	if _, ok := self.engine.(consensus.Istanbul); ok {
		log.Error("Cannot set bxmbase in Istanbul consensus")
		return
	}
	self.bxmbase = bxmbase
	self.lock.Unlock()

	self.miner.SetBxmbase(bxmbase)
}

func (s *BitMED) StartMining(local bool) error {
	eb, err := s.Bxmbase()
	if err != nil {
		log.Error("Cannot start mining without bxmbase", "err", err)
		return fmt.Errorf("bxmbase missing: %v", err)
	}
	if clique, ok := s.engine.(*clique.Clique); ok {
		wallet, err := s.accountManager.Find(accounts.Account{Address: eb})
		if wallet == nil || err != nil {
			log.Error("Bxmbase account unavailable locally", "err", err)
			return fmt.Errorf("signer missing: %v", err)
		}
		clique.Authorize(eb, wallet.SignHash)
	} else if istanbul, ok := s.engine.(consensus.Istanbul); ok {
		istanbul.Start(s.blockchain, s.blockchain.InsertChain)
	}
	if local {
		// If local (CPU) mining is started, we can disable the transaction rejection
		// mechanism introduced to speed sync times. CPU mining on mainnet is ludicrous
		// so noone will ever hit this path, whereas marking sync done on CPU mining
		// will ensure that private networks work in single miner mode too.
		atomic.StoreUint32(&s.protocolManager.acceptTxs, 1)
	}
	go s.miner.Start(eb)
	return nil
}

func (s *BitMED) StopMining() {
	s.miner.Stop()
	if istanbul, ok := s.engine.(consensus.Istanbul); ok {
		istanbul.Stop()
	}
}
func (s *BitMED) IsMining() bool      { return s.miner.Mining() }
func (s *BitMED) Miner() *miner.Miner { return s.miner }

func (s *BitMED) AccountManager() *accounts.Manager  { return s.accountManager }
func (s *BitMED) BlockChain() *core.BlockChain       { return s.blockchain }
func (s *BitMED) TxPool() *core.TxPool               { return s.txPool }
func (s *BitMED) EventMux() *event.TypeMux           { return s.eventMux }
func (s *BitMED) Engine() consensus.Engine           { return s.engine }
func (s *BitMED) ChainDb() bxmdb.Database            { return s.chainDb }
func (s *BitMED) IsListening() bool                  { return true } // Always listening
func (s *BitMED) BxmVersion() int                    { return int(s.protocolManager.SubProtocols[0].Version) }
func (s *BitMED) NetVersion() uint64                 { return s.networkId }
func (s *BitMED) Downloader() *downloader.Downloader { return s.protocolManager.downloader }

// Protocols implements node.Service, returning all the currently configured
// network protocols to start.
func (s *BitMED) Protocols() []p2p.Protocol {
	if s.lesServer == nil {
		return s.protocolManager.SubProtocols
	}
	return append(s.protocolManager.SubProtocols, s.lesServer.Protocols()...)
}

// Start implements node.Service, starting all internal goroutines needed by the
// BitMED protocol implementation.
func (s *BitMED) Start(srvr *p2p.Server) error {
	// Start the bloom bits servicing goroutines
	s.startBloomHandlers()

	// Start the RPC service
	s.netRPCService = bxmapi.NewPublicNetAPI(srvr, s.NetVersion())

	// Figure out a max peers count based on the server limits
	maxPeers := srvr.MaxPeers
	if s.config.LightServ > 0 {
		maxPeers -= s.config.LightPeers
		if maxPeers < srvr.MaxPeers/2 {
			maxPeers = srvr.MaxPeers / 2
		}
	}
	// Start the networking layer and the light server if requested
	s.protocolManager.Start(maxPeers)
	if s.lesServer != nil {
		s.lesServer.Start(srvr)
	}
	return nil
}

// Stop implements node.Service, terminating all internal goroutines used by the
// BitMED protocol.
func (s *BitMED) Stop() error {
	if s.stopDbUpgrade != nil {
		s.stopDbUpgrade()
	}
	s.bloomIndexer.Close()
	s.blockchain.Stop()
	s.protocolManager.Stop()
	if s.lesServer != nil {
		s.lesServer.Stop()
	}
	s.txPool.Stop()
	s.miner.Stop()
	s.eventMux.Stop()

	s.chainDb.Close()
	close(s.shutdownChan)

	return nil
}
