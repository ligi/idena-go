package node

import (
	"fmt"
	"github.com/idena-network/idena-go/api"
	"github.com/idena-network/idena-go/blockchain"
	"github.com/idena-network/idena-go/blockchain/validation"
	"github.com/idena-network/idena-go/common/eventbus"
	util "github.com/idena-network/idena-go/common/ulimit"
	"github.com/idena-network/idena-go/config"
	"github.com/idena-network/idena-go/consensus"
	"github.com/idena-network/idena-go/core/appstate"
	"github.com/idena-network/idena-go/core/ceremony"
	"github.com/idena-network/idena-go/core/flip"
	"github.com/idena-network/idena-go/core/mempool"
	"github.com/idena-network/idena-go/core/profile"
	"github.com/idena-network/idena-go/core/state"
	"github.com/idena-network/idena-go/core/upgrade"
	"github.com/idena-network/idena-go/crypto"
	"github.com/idena-network/idena-go/deferredtx"
	"github.com/idena-network/idena-go/events"
	"github.com/idena-network/idena-go/ipfs"
	"github.com/idena-network/idena-go/keystore"
	"github.com/idena-network/idena-go/log"
	"github.com/idena-network/idena-go/pengings"
	"github.com/idena-network/idena-go/protocol"
	"github.com/idena-network/idena-go/rpc"
	"github.com/idena-network/idena-go/secstore"
	state2 "github.com/idena-network/idena-go/state"
	"github.com/idena-network/idena-go/stats/collector"
	"github.com/idena-network/idena-go/subscriptions"
	"github.com/idena-network/idena-go/vm"
	"github.com/pkg/errors"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/tendermint/tm-db"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Node struct {
	config          *config.Config
	blockchain      *blockchain.Blockchain
	appState        *appstate.AppState
	secStore        *secstore.SecStore
	pm              *protocol.IdenaGossipHandler
	stop            chan struct{}
	proposals       *pengings.Proposals
	votes           *pengings.Votes
	consensusEngine *consensus.Engine
	txpool          *mempool.TxPool
	flipKeyPool     *mempool.KeysPool
	rpcAPIs         []rpc.API
	httpListener    net.Listener // HTTP RPC listener socket to server API requests
	httpHandler     *rpc.Server  // HTTP RPC request handler to process the API requests
	httpServer      *http.Server
	log             log.Logger
	keyStore        *keystore.KeyStore
	fp              *flip.Flipper
	ipfsProxy       ipfs.Proxy
	bus             eventbus.Bus
	ceremony        *ceremony.ValidationCeremony
	downloader      *protocol.Downloader
	offlineDetector *blockchain.OfflineDetector
	appVersion      string
	profileManager  *profile.Manager
	deferJob        *deferredtx.Job
	subManager      *subscriptions.Manager
	upgrader        *upgrade.Upgrader
	nodeState       *state2.NodeState
}

type NodeCtx struct {
	Node            *Node
	AppState        *appstate.AppState
	Ceremony        *ceremony.ValidationCeremony
	Blockchain      *blockchain.Blockchain
	Flipper         *flip.Flipper
	KeysPool        *mempool.KeysPool
	OfflineDetector *blockchain.OfflineDetector
	PendingProofs   *sync.Map
	ProposerByRound pengings.ProposerByRound
	Upgrader        *upgrade.Upgrader
}

type ceremonyChecker struct {
	appState *appstate.AppState
	chain    *blockchain.Blockchain
}

func (checker *ceremonyChecker) IsRunning() bool {
	appState, _ := checker.appState.Readonly(checker.chain.Head.Height())
	if appState != nil {
		return appState.State.ValidationPeriod() >= state.FlipLotteryPeriod
	}
	return false
}

func StartMobileNode(path string, cfg string) string {
	fileHandler, _ := log.FileHandler(filepath.Join(path, "output.log"), log.TerminalFormat(false))
	log.Root().SetHandler(log.LvlFilterHandler(log.LvlInfo, log.MultiHandler(log.StreamHandler(os.Stdout, log.LogfmtFormat()), fileHandler)))

	c, err := config.MakeMobileConfig(path, cfg)

	if err != nil {
		return err.Error()
	}

	n, err := NewNode(c, "mobile")

	if err != nil {
		return err.Error()
	}

	n.Start()

	return "started"
}

func ProvideMobileKey(path string, cfg string, key string, password string) string {
	c, err := config.MakeMobileConfig(path, cfg)

	if err != nil {
		return err.Error()
	}

	if err := c.ProvideNodeKey(key, password, false); err != nil {
		return err.Error()
	}
	return "done"
}

func NewNode(config *config.Config, appVersion string) (*Node, error) {
	nodeCtx, err := NewNodeWithInjections(config, eventbus.New(), collector.NewStatsCollector(), appVersion)
	if err != nil {
		return nil, err
	}
	return nodeCtx.Node, err
}

func NewNodeWithInjections(config *config.Config, bus eventbus.Bus, statsCollector collector.StatsCollector, appVersion string) (*NodeCtx, error) {

	logger := log.New()
	nodeState := state2.NewNodeState(bus)
	httpListener, httpHandler, httpServer, err := startInitialRPC(config, nodeState)
	if err != nil {
		logger.Error("Cannot start initial RPC endpoint", "error", err.Error())
	}

	bus.Publish(&events.DatabaseInitEvent{})
	db, err := OpenDatabase(config.DataDir, "idenachain", 16, 16, true)
	bus.Publish(&events.DatabaseInitCompletedEvent{})

	if err != nil {
		return nil, err
	}

	keyStoreDir, err := config.KeyStoreDataDir()
	if err != nil {
		return nil, err
	}

	err = config.SetApiKey()
	if err != nil {
		return nil, errors.Wrap(err, "cannot set API key")
	}

	ipfsProxy, err := ipfs.NewIpfsProxy(config.IpfsConf, bus)
	if err != nil {
		return nil, err
	}
	validation.SetAppConfig(config)
	keyStore := keystore.NewKeyStore(keyStoreDir, keystore.StandardScryptN, keystore.StandardScryptP)
	secStore := secstore.NewSecStore()

	appState, err := appstate.NewAppState(db, bus)
	if err != nil {
		return nil, err
	}

	offlineDetector := blockchain.NewOfflineDetector(config, db, appState, secStore, bus)

	upgrader := upgrade.NewUpgrader(config, appState, db)

	votes := pengings.NewVotes(appState, bus, offlineDetector, upgrader)

	txpool := mempool.NewTxPool(appState, bus, config, statsCollector)
	flipKeyPool := mempool.NewKeysPool(db, appState, bus, secStore)

	subManager, err := subscriptions.NewManager(config.DataDir)
	if err != nil {
		return nil, err
	}

	chain := blockchain.NewBlockchain(config, db, txpool, appState, ipfsProxy, secStore, bus, offlineDetector, keyStore, subManager, upgrader)
	proposals, pendingProofs := pengings.NewProposals(chain, appState, offlineDetector, upgrader, statsCollector)
	flipper := flip.NewFlipper(db, ipfsProxy, flipKeyPool, txpool, secStore, appState, bus)
	pm := protocol.NewIdenaGossipHandler(ipfsProxy.Host(), ipfsProxy.PubSub(), config.P2P, chain, proposals, votes, txpool, flipper, bus, flipKeyPool, appVersion, &ceremonyChecker{
		appState: appState,
		chain:    chain,
	})
	sm := state.NewSnapshotManager(db, appState.State, bus, ipfsProxy, config)
	downloader := protocol.NewDownloader(pm, config, chain, ipfsProxy, appState, sm, bus, secStore, statsCollector, subManager, keyStore, upgrader)
	consensusEngine := consensus.NewEngine(chain, pm, proposals, config, appState, votes, txpool, secStore,
		downloader, offlineDetector, upgrader, ipfsProxy, bus, statsCollector)
	ceremony := ceremony.NewValidationCeremony(appState, bus, flipper, secStore, db, txpool, chain, downloader, flipKeyPool, config)
	profileManager := profile.NewProfileManager(ipfsProxy)

	deferJob, err := deferredtx.NewJob(bus, config.DataDir, appState, chain, txpool, keyStore, secStore, vm.NewVmImpl)
	if err != nil {
		return nil, err
	}

	node := &Node{
		config:          config,
		blockchain:      chain,
		pm:              pm,
		proposals:       proposals,
		appState:        appState,
		consensusEngine: consensusEngine,
		txpool:          txpool,
		log:             logger,
		keyStore:        keyStore,
		fp:              flipper,
		ipfsProxy:       ipfsProxy,
		secStore:        secStore,
		bus:             bus,
		flipKeyPool:     flipKeyPool,
		ceremony:        ceremony,
		downloader:      downloader,
		offlineDetector: offlineDetector,
		votes:           votes,
		appVersion:      appVersion,
		profileManager:  profileManager,
		deferJob:        deferJob,
		subManager:      subManager,
		upgrader:        upgrader,
		nodeState:       nodeState,
		httpListener:    httpListener,
		httpHandler:     httpHandler,
		httpServer:      httpServer,
	}
	return &NodeCtx{
		Node:            node,
		AppState:        appState,
		Ceremony:        ceremony,
		Blockchain:      chain,
		Flipper:         flipper,
		KeysPool:        flipKeyPool,
		OfflineDetector: offlineDetector,
		PendingProofs:   pendingProofs,
		ProposerByRound: proposals.ProposerByRound,
		Upgrader:        upgrader,
	}, nil
}

func (node *Node) Start() {
	node.StartWithHeight(0)
}

func (node *Node) StartWithHeight(height uint64) {
	if privateKey, err := node.config.NodeKey(); err != nil {
		node.log.Crit("Cannot initialize node key", "error", err.Error())
	} else {
		node.secStore.AddKey(crypto.FromECDSA(privateKey))
	}

	if changed, value, err := util.ManageFdLimit(); changed {
		node.log.Info("Set new fd limit", "value", value)
	} else if err != nil {
		node.log.Warn("Failed to set new fd limit", "err", err)
	}

	if err := node.blockchain.InitializeChain(); err != nil {
		node.log.Error("Cannot initialize blockchain", "error", err.Error())
		return
	}

	if err := node.appState.Initialize(node.blockchain.Head.Height()); err != nil {
		if err := node.appState.Initialize(0); err != nil {
			node.log.Error("Cannot initialize state", "error", err.Error())
		}
	}

	if err := node.blockchain.EnsureIntegrity(); err != nil {
		node.log.Error("Failed to recover blockchain", "err", err)
		return
	}

	if height > 0 && node.blockchain.Head.Height() > height {
		if _, err := node.blockchain.ResetTo(height); err != nil {
			node.log.Error(fmt.Sprintf("Cannot reset blockchain to %d", height), "error", err.Error())
			return
		}
	}

	node.blockchain.ApplyHotfixToState()

	node.txpool.Initialize(node.blockchain.Head, node.secStore.GetAddress(), true)
	node.flipKeyPool.Initialize(node.blockchain.Head)
	node.votes.Initialize(node.blockchain.Head)
	node.fp.Initialize()
	node.ceremony.Initialize(node.blockchain.GetBlock(node.blockchain.Head.Hash()))
	node.blockchain.ProvideApplyNewEpochFunc(node.ceremony.ApplyNewEpoch)
	node.offlineDetector.Start(node.blockchain.Head)
	node.consensusEngine.Start()
	node.pm.Start()
	node.upgrader.Start()

	node.stopInitialRPC()
	// Configure RPC
	if err := node.startRPC(); err != nil {
		node.log.Error("Cannot start RPC endpoint", "error", err.Error())
	}
}

func (node *Node) WaitForStop() {
	<-node.stop
	node.secStore.Destroy()
}

func startInitialRPC(nodeConfig *config.Config, nodeState *state2.NodeState) (net.Listener, *rpc.Server, *http.Server, error) {
	apis := initialApis(nodeState)
	listener, handler, httpServer, err := startInitialHTTP(nodeConfig.RPC.HTTPEndpoint(), apis, nodeConfig.RPC.HTTPModules, nodeConfig.RPC.HTTPCors, nodeConfig.RPC.HTTPVirtualHosts, nodeConfig.RPC.HTTPTimeouts, nodeConfig.RPC.APIKey)
	if err != nil {
		return nil, nil, nil, err
	}
	return listener, handler, httpServer, nil
}

func initialApis(nodeState *state2.NodeState) []rpc.API {
	return []rpc.API{
		{
			Namespace: "bcn",
			Version:   "1.0",
			Service:   api.NewBlockchainInitialApi(nodeState),
			Public:    true,
		},
	}
}

func startInitialHTTP(endpoint string, apis []rpc.API, modules []string, cors []string, vhosts []string, timeouts rpc.HTTPTimeouts, apiKey string) (net.Listener, *rpc.Server, *http.Server, error) {
	if endpoint == "" {
		return nil, nil, nil, nil
	}
	listener, handler, httpServer, err := rpc.StartHTTPEndpoint(endpoint, apis, modules, cors, vhosts, timeouts, apiKey)
	if err != nil {
		return nil, nil, nil, err
	}
	log.Info("initial HTTP endpoint opened", "url", fmt.Sprintf("http://%s", endpoint), "cors", strings.Join(cors, ","), "vhosts", strings.Join(vhosts, ","))

	return listener, handler, httpServer, err
}

// startRPC is a helper method to start all the various RPC endpoint during node
// startup. It's not meant to be called at any time afterwards as it makes certain
// assumptions about the state of the node.
func (node *Node) startRPC() error {
	// Gather all the possible APIs to surface
	apis := node.apis()

	if err := node.startHTTP(node.config.RPC.HTTPEndpoint(), apis, node.config.RPC.HTTPModules, node.config.RPC.HTTPCors, node.config.RPC.HTTPVirtualHosts, node.config.RPC.HTTPTimeouts, node.config.RPC.APIKey); err != nil {
		return err
	}

	node.rpcAPIs = apis
	return nil
}

// startHTTP initializes and starts the HTTP RPC endpoint.
func (node *Node) startHTTP(endpoint string, apis []rpc.API, modules []string, cors []string, vhosts []string, timeouts rpc.HTTPTimeouts, apiKey string) error {
	// Short circuit if the HTTP endpoint isn't being exposed
	if endpoint == "" {
		return nil
	}
	listener, handler, httpServer, err := rpc.StartHTTPEndpoint(endpoint, apis, modules, cors, vhosts, timeouts, apiKey)
	if err != nil {
		return err
	}
	node.log.Info("HTTP endpoint opened", "url", fmt.Sprintf("http://%s", endpoint), "cors", strings.Join(cors, ","), "vhosts", strings.Join(vhosts, ","))

	node.httpListener = listener
	node.httpHandler = handler
	node.httpServer = httpServer

	return nil
}

func (node *Node) stopInitialRPC() {
	node.stopHTTP()
}

// stopHTTP terminates the HTTP RPC endpoint.
func (node *Node) stopHTTP() {
	if node.httpListener != nil {
		node.httpListener.Close()
		node.httpListener = nil

		node.log.Info("HTTP endpoint closed", "url", fmt.Sprintf("http://%s", node.config.RPC.HTTPEndpoint()))
	}
	if node.httpHandler != nil {
		node.httpHandler.Stop()
		node.httpHandler = nil
	}
	if node.httpServer != nil {
		node.httpServer.Close()
		node.httpServer = nil
	}
}

func OpenDatabase(datadir string, name string, cache int, handles int, compact bool) (db.DB, error) {
	res, err := db.NewGoLevelDBWithOpts(name, datadir, &opt.Options{
		OpenFilesCacheCapacity: handles,
		BlockCacheCapacity:     cache / 2 * opt.MiB,
		WriteBuffer:            cache / 4 * opt.MiB,
		Filter:                 filter.NewBloomFilter(10),
	})
	if err != nil {
		return nil, err
	}
	if compact {
		if err := compactDb(res); err != nil {
			res.Close()
			return nil, err
		}
	}
	return res, nil
}

func compactDb(goLevelDB *db.GoLevelDB) error {
	start := time.Now()
	logTimeout := time.After(time.Second)
	completed := make(chan struct{})
	go func() {
		select {
		case <-completed:
		case <-logTimeout:
			log.Info("Start compacting DB")
			<-completed
			log.Info("DB compacted", "d", time.Since(start))
		}
	}()
	err := goLevelDB.ForceCompact(nil, nil)
	completed <- struct{}{}
	return err
}

// apis returns the collection of RPC descriptors this node offers.
func (node *Node) apis() []rpc.API {

	baseApi := api.NewBaseApi(node.consensusEngine, node.txpool, node.keyStore, node.secStore, node.ipfsProxy)

	return []rpc.API{
		{
			Namespace: "net",
			Version:   "1.0",
			Service:   api.NewNetApi(node.pm, node.ipfsProxy),
			Public:    true,
		},
		{
			Namespace: "dna",
			Version:   "1.0",
			Service:   api.NewDnaApi(baseApi, node.blockchain, node.ceremony, node.appVersion, node.profileManager),
			Public:    true,
		},
		{
			Namespace: "account",
			Version:   "1.0",
			Service:   api.NewAccountApi(baseApi),
			Public:    true,
		},
		{
			Namespace: "flip",
			Version:   "1.0",
			Service:   api.NewFlipApi(baseApi, node.fp, node.ipfsProxy, node.ceremony),
			Public:    true,
		},
		{
			Namespace: "bcn",
			Version:   "1.0",
			Service:   api.NewBlockchainApi(baseApi, node.blockchain, node.ipfsProxy, node.txpool, node.downloader, node.pm, node.nodeState),
			Public:    true,
		},
		{
			Namespace: "ipfs",
			Version:   "1.0",
			Service:   api.NewIpfsApi(node.ipfsProxy),
			Public:    true,
		},
		{
			Namespace: "contract",
			Version:   "1.0",
			Service:   api.NewContractApi(baseApi, node.blockchain, node.deferJob, node.subManager),
			Public:    true,
		},
	}
}
