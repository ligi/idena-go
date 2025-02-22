package blockchain

import (
	"crypto/ecdsa"
	"github.com/idena-network/idena-go/blockchain/types"
	"github.com/idena-network/idena-go/blockchain/validation"
	"github.com/idena-network/idena-go/common"
	"github.com/idena-network/idena-go/common/eventbus"
	"github.com/idena-network/idena-go/config"
	"github.com/idena-network/idena-go/core/appstate"
	"github.com/idena-network/idena-go/core/mempool"
	"github.com/idena-network/idena-go/core/state"
	"github.com/idena-network/idena-go/core/upgrade"
	"github.com/idena-network/idena-go/crypto"
	"github.com/idena-network/idena-go/ipfs"
	"github.com/idena-network/idena-go/keystore"
	"github.com/idena-network/idena-go/secstore"
	"github.com/idena-network/idena-go/stats/collector"
	"github.com/idena-network/idena-go/subscriptions"
	"github.com/shopspring/decimal"
	"github.com/tendermint/tm-db"
	"math/big"
)

func NewTestBlockchainWithConfig(withIdentity bool, conf *config.ConsensusConf, valConf *config.ValidationConfig, alloc map[common.Address]config.GenesisAllocation, queueSlots int, executableSlots int, executableLimit int, queueLimit int) (*TestBlockchain, *appstate.AppState, *mempool.TxPool, *ecdsa.PrivateKey) {
	if alloc == nil {
		alloc = make(map[common.Address]config.GenesisAllocation)
	}
	key, _ := crypto.GenerateKey()
	secStore := secstore.NewSecStore()

	secStore.AddKey(crypto.FromECDSA(key))
	cfg := &config.Config{
		Network:   0x99,
		Consensus: conf,
		GenesisConf: &config.GenesisConf{
			Alloc:      alloc,
			GodAddress: secStore.GetAddress(),
		},
		Validation:       valConf,
		Blockchain:       &config.BlockchainConfig{},
		OfflineDetection: config.GetDefaultOfflineDetectionConfig(),
		Mempool: &config.Mempool{
			TxPoolQueueSlots:          queueSlots,
			TxPoolExecutableSlots:     executableSlots,
			TxPoolAddrExecutableLimit: executableLimit,
			TxPoolAddrQueueLimit:      queueLimit,
		},
	}

	db := db.NewMemDB()
	bus := eventbus.New()
	appState, _ := appstate.NewAppState(db, bus)

	if withIdentity {
		addr := crypto.PubkeyToAddress(key.PublicKey)
		cfg.GenesisConf.Alloc[addr] = config.GenesisAllocation{
			State: uint8(state.Verified),
		}
	}

	txPool := mempool.NewTxPool(appState, bus, cfg, collector.NewStatsCollector())
	offline := NewOfflineDetector(cfg, db, appState, secStore, bus)
	keyStore := keystore.NewKeyStore("./testdata", keystore.StandardScryptN, keystore.StandardScryptP)
	subManager, _ := subscriptions.NewManager("./testdata2")
	upgrader := upgrade.NewUpgrader(cfg, appState, db)
	chain := NewBlockchain(cfg, db, txPool, appState, ipfs.NewMemoryIpfsProxy(), secStore, bus, offline, keyStore, subManager, upgrader)

	chain.InitializeChain()
	appState.Initialize(chain.Head.Height())
	txPool.Initialize(chain.Head, secStore.GetAddress(), false)

	return &TestBlockchain{db, chain, 0}, appState, txPool, key
}

func NewTestBlockchain(withIdentity bool, alloc map[common.Address]config.GenesisAllocation) (*TestBlockchain, *appstate.AppState, *mempool.TxPool, *ecdsa.PrivateKey) {
	cfg := GetDefaultConsensusConfig()
	cfg.Automine = true
	return NewTestBlockchainWithConfig(withIdentity, cfg, &config.ValidationConfig{}, alloc, -1, -1, 0, 0)
}

func NewTestBlockchainWithBlocks(blocksCount int, emptyBlocksCount int) (*TestBlockchain, *appstate.AppState) {
	key, _ := crypto.GenerateKey()
	return NewCustomTestBlockchain(blocksCount, emptyBlocksCount, key)
}

func NewCustomTestBlockchain(blocksCount int, emptyBlocksCount int, key *ecdsa.PrivateKey) (*TestBlockchain, *appstate.AppState) {
	addr := crypto.PubkeyToAddress(key.PublicKey)
	consensusCfg := GetDefaultConsensusConfig()
	consensusCfg.Automine = true
	cfg := &config.Config{
		Network:   0x99,
		Consensus: consensusCfg,
		GenesisConf: &config.GenesisConf{
			Alloc: map[common.Address]config.GenesisAllocation{
				addr: {Balance: big.NewInt(0).Mul(big.NewInt(100), common.DnaBase)},
			},
			GodAddress:        addr,
			FirstCeremonyTime: 4070908800, //01.01.2099
		},
		Validation: &config.ValidationConfig{},
		Blockchain: &config.BlockchainConfig{},
	}
	return NewCustomTestBlockchainWithConfig(blocksCount, emptyBlocksCount, key, cfg)
}

func NewCustomTestBlockchainWithConfig(blocksCount int, emptyBlocksCount int, key *ecdsa.PrivateKey, cfg *config.Config) (*TestBlockchain, *appstate.AppState) {
	db := db.NewMemDB()
	bus := eventbus.New()
	appState, _ := appstate.NewAppState(db, bus)
	secStore := secstore.NewSecStore()
	secStore.AddKey(crypto.FromECDSA(key))
	if cfg.OfflineDetection == nil {
		cfg.OfflineDetection = config.GetDefaultOfflineDetectionConfig()
	}
	if cfg.Mempool == nil {
		cfg.Mempool = config.GetDefaultMempoolConfig()
	}
	txPool := mempool.NewTxPool(appState, bus, cfg, collector.NewStatsCollector())
	offline := NewOfflineDetector(cfg, db, appState, secStore, bus)
	keyStore := keystore.NewKeyStore("./testdata", keystore.StandardScryptN, keystore.StandardScryptP)
	subManager, _ := subscriptions.NewManager("./testdata2")
	upgrader := upgrade.NewUpgrader(cfg, appState, db)
	chain := NewBlockchain(cfg, db, txPool, appState, ipfs.NewMemoryIpfsProxy(), secStore, bus, offline, keyStore, subManager, upgrader)
	chain.InitializeChain()
	appState.Initialize(chain.Head.Height())

	result := &TestBlockchain{db, chain, 0}
	result.GenerateBlocks(blocksCount, 0).GenerateEmptyBlocks(emptyBlocksCount)
	txPool.Initialize(chain.Head, secStore.GetAddress(), false)
	return result, appState
}

type TestBlockchain struct {
	db db.DB
	*Blockchain
	coinbaseTxNonce uint32
}

func (chain *TestBlockchain) AddTx(tx *types.Transaction) error {
	return chain.txpool.AddExternalTxs(validation.InboundTx, tx)
}

func (chain *TestBlockchain) Copy() (*TestBlockchain, *appstate.AppState) {
	db := db.NewMemDB()
	bus := eventbus.New()

	it, _ := chain.db.Iterator(nil, nil)
	defer it.Close()
	for ; it.Valid(); it.Next() {
		db.Set(it.Key(), it.Value())
	}
	appState, _ := appstate.NewAppState(db, bus)
	consensusCfg := GetDefaultConsensusConfig()
	consensusCfg.Automine = true
	cfg := &config.Config{
		Network:   0x99,
		Consensus: consensusCfg,
		GenesisConf: &config.GenesisConf{
			Alloc:             nil,
			GodAddress:        chain.secStore.GetAddress(),
			FirstCeremonyTime: 4070908800, //01.01.2099
		},
		Validation:       &config.ValidationConfig{},
		Blockchain:       &config.BlockchainConfig{},
		OfflineDetection: config.GetDefaultOfflineDetectionConfig(),
		Mempool:          config.GetDefaultMempoolConfig(),
	}
	txPool := mempool.NewTxPool(appState, bus, cfg, collector.NewStatsCollector())
	offline := NewOfflineDetector(cfg, db, appState, chain.secStore, bus)
	keyStore := keystore.NewKeyStore("./testdata", keystore.StandardScryptN, keystore.StandardScryptP)
	subManager, _ := subscriptions.NewManager("./testdata2")
	upgrader := upgrade.NewUpgrader(cfg, appState, db)
	copy := NewBlockchain(cfg, db, txPool, appState, ipfs.NewMemoryIpfsProxy(), chain.secStore, bus, offline, keyStore, subManager, upgrader)
	copy.InitializeChain()
	appState.Initialize(copy.Head.Height())
	txPool.Initialize(chain.Head, chain.secStore.GetAddress(), false)
	return &TestBlockchain{db, copy, appState.State.GetNonce(chain.secStore.GetAddress())}, appState
}

func (chain *TestBlockchain) addCert(block *types.Block) {
	vote := &types.Vote{
		Header: &types.VoteHeader{
			Round:       block.Height(),
			Step:        1,
			ParentHash:  block.Header.ParentHash(),
			VotedHash:   block.Header.Hash(),
			TurnOffline: false,
		},
	}
	hash := crypto.SignatureHash(vote)
	vote.Signature = chain.secStore.Sign(hash[:])
	cert := types.FullBlockCert{Votes: []*types.Vote{vote}}
	chain.WriteCertificate(block.Header.Hash(), cert.Compress(), true)
}

func (chain *TestBlockchain) GenerateBlocks(count int, txsInBlock int) *TestBlockchain {
	for i := 0; i < count; i++ {
		for j := 0; j < txsInBlock; j++ {
			tx := BuildTx(chain.appState, chain.coinBaseAddress, &chain.coinBaseAddress, types.SendTx, decimal.Zero,
				decimal.New(20, 0), decimal.Zero, chain.coinbaseTxNonce, 0, nil)
			tx, err := chain.secStore.SignTx(tx)
			if err != nil {
				panic(err)
			}
			if err = chain.txpool.AddExternalTxs(validation.InboundTx, tx); err != nil {
				panic(err)
			}
		}

		block := chain.ProposeBlock([]byte{})
		block.Block.Header.ProposedHeader.Time = chain.Head.Time() + 20
		err := chain.AddBlock(block.Block, nil, collector.NewStatsCollector())
		if err != nil {
			panic(err)
		}
		chain.addCert(block.Block)
	}
	return chain
}

func (chain *TestBlockchain) GenerateEmptyBlocks(count int) *TestBlockchain {
	for i := 0; i < count; i++ {
		block := chain.GenerateEmptyBlock()
		err := chain.AddBlock(block, nil, collector.NewStatsCollector())
		if err != nil {
			panic(err)
		}
		chain.addCert(block)
	}
	return chain
}

func (chain *TestBlockchain) CommitState() *TestBlockchain {
	if chain.Head.EmptyBlockHeader != nil {
		chain.Head.EmptyBlockHeader.Height++
	} else {
		chain.Head.ProposedHeader.Height++
	}
	block := chain.GenerateEmptyBlock()
	err := chain.AddBlock(block, nil, collector.NewStatsCollector())
	if err != nil {
		panic(err)
	}
	chain.addCert(block)
	return chain
}

func (chain *TestBlockchain) SecStore() *secstore.SecStore {
	return chain.secStore
}

func (chain *TestBlockchain) Bus() eventbus.Bus {
	return chain.bus
}

func GetDefaultConsensusConfig() *config.ConsensusConf {
	base := config.GetDefaultConsensusConfig()
	res := *base
	return &res
}
