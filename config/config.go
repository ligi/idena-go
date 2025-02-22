package config

import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/idena-network/idena-go/common"
	"github.com/idena-network/idena-go/crypto"
	"github.com/idena-network/idena-go/log"
	"github.com/idena-network/idena-go/rpc"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	datadirPrivateKey = "nodekey" // Path within the datadir to the node's private key
	apiKeyFileName    = "api.key"
	LowPowerProfile   = "lowpower"
	SharedNodeProfile = "shared"
	DefaultProfile    = "default"
)

type Config struct {
	DataDir          string
	Network          uint32
	AutoOnline       bool
	Consensus        *ConsensusConf
	P2P              P2P
	RPC              *rpc.Config
	GenesisConf      *GenesisConf
	IpfsConf         *IpfsConfig
	Validation       *ValidationConfig
	Sync             *SyncConfig
	OfflineDetection *OfflineDetectionConfig
	Blockchain       *BlockchainConfig
	Mempool          *Mempool
}

func (c *Config) ProvideNodeKey(key string, password string, withBackup bool) error {
	instanceDir := filepath.Join(c.DataDir, "keystore")
	if err := os.MkdirAll(instanceDir, 0700); err != nil {
		return err
	}

	keyfile := filepath.Join(instanceDir, datadirPrivateKey)

	currentKey, err := crypto.LoadECDSA(keyfile)

	if !withBackup && err == nil {
		return errors.New("key already exists")
	}

	keyBytes, err := hex.DecodeString(key)
	if err != nil {
		return errors.Errorf("error while decoding key, err: %v", err.Error())
	}

	decrypted, err := crypto.Decrypt(keyBytes, password)
	if err != nil {
		return errors.Errorf("error while decrypting key, err: %v", err.Error())
	}

	ecdsaKey, err := crypto.ToECDSA(decrypted)
	if err != nil {
		return errors.Errorf("key is not valid ECDSA key, err: %v", err.Error())
	}

	if withBackup && currentKey != nil {
		backupFile := filepath.Join(instanceDir, fmt.Sprintf("backup-%v", time.Now().Unix()))
		if err := crypto.SaveECDSA(backupFile, currentKey); err != nil {
			return errors.Errorf("failed to backup key, err: %v", err.Error())
		}
	}

	if err := crypto.SaveECDSA(keyfile, ecdsaKey); err != nil {
		return errors.Errorf("failed to persist key, err: %v", err.Error())
	}
	return nil
}

func (c *Config) NodeKey() (*ecdsa.PrivateKey, error) {
	// Generate ephemeral key if no datadir is being used.
	if c.DataDir == "" {
		key, err := crypto.GenerateKey()
		return key, errors.Wrap(err, "failed to generate ephemeral node key")
	}

	instanceDir := filepath.Join(c.DataDir, "keystore")
	if err := os.MkdirAll(instanceDir, 0700); err != nil {
		return nil, errors.Wrap(err, "failed to persist node key")
	}

	keyfile := filepath.Join(instanceDir, datadirPrivateKey)

	if _, err := os.Stat(keyfile); os.IsNotExist(err) {
		// No persistent key found, generate and store a new one.
		key, err := crypto.GenerateKey()
		if err != nil {
			return nil, errors.Wrap(err, "failed to generate node key")
		}
		if err := crypto.SaveECDSA(keyfile, key); err != nil {
			return nil, errors.Wrap(err, "failed to persist node key")
		}
		return key, nil
	} else if err != nil {
		return nil, errors.Wrap(err, "failed to check node key file")
	}

	key, err := crypto.LoadECDSA(keyfile)
	return key, errors.Wrap(err, "failed to load node key")
}

// NodeDB returns the path to the discovery node database.
func (c *Config) NodeDB() string {
	if c.DataDir == "" {
		return "" // ephemeral
	}
	return filepath.Join(c.DataDir, "nodes")
}

func (c *Config) KeyStoreDataDir() (string, error) {
	instanceDir := filepath.Join(c.DataDir, "keystore")
	if err := os.MkdirAll(instanceDir, 0700); err != nil {
		log.Error(fmt.Sprintf("Failed to create keystore datadir: %v", err))
		return "", err
	}
	return instanceDir, nil
}

func (c *Config) SetApiKey() error {
	shouldSaveKey := true
	if c.RPC.APIKey == "" {
		apiKeyFile := filepath.Join(c.DataDir, apiKeyFileName)
		data, _ := ioutil.ReadFile(apiKeyFile)
		key := strings.TrimSpace(string(data))
		if key == "" {
			randomKey, _ := crypto.GenerateKey()
			key = hex.EncodeToString(crypto.FromECDSA(randomKey)[:16])
		} else {
			shouldSaveKey = false
		}
		c.RPC.APIKey = key
	}

	if shouldSaveKey {
		f, err := os.OpenFile(filepath.Join(c.DataDir, apiKeyFileName), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.WriteString(c.RPC.APIKey)
		return err
	}
	return nil
}

func MakeMobileConfig(path string, cfg string) (*Config, error) {
	conf := getDefaultConfig(filepath.Join(path, DefaultDataDir))

	if cfg != "" {
		log.Info("using custom configuration")
		bytes := []byte(cfg)
		err := json.Unmarshal(bytes, &conf)
		if err != nil {
			return nil, errors.Errorf("Cannot parse JSON config")
		}
	} else {
		log.Info("using default config")
	}

	return conf, nil
}

func MakeConfig(ctx *cli.Context, cfgTransform func(cfg *Config)) (*Config, error) {
	cfg, err := MakeConfigFromFile(ctx.String(CfgFileFlag.Name))
	if err != nil {
		return nil, err
	}
	if ctx.IsSet(DataDirFlag.Name) {
		cfg.DataDir = ctx.String(DataDirFlag.Name)
	}
	cfgTransform(cfg)
	applyFlags(ctx, cfg)
	return cfg, nil
}

func applyProfile(ctx *cli.Context, cfg *Config) {
	if ctx.IsSet(ProfileFlag.Name) {
		switch ctx.String(ProfileFlag.Name) {
		case LowPowerProfile:
			applyLowPowerProfile(cfg)
		case SharedNodeProfile:
			applySharedNodeProfile(cfg)
		case DefaultProfile:
			applyDefaultProfile(cfg)
		default:
			println("unknown node profile")
		}
	} else {
		applyDefaultProfile(cfg)
	}
	if cfg.IpfsConf.GracePeriod == "" {
		cfg.IpfsConf.GracePeriod = "40s"
	}
	if cfg.IpfsConf.ReproviderInterval == "" {
		cfg.IpfsConf.ReproviderInterval = "12h"
	}
	if cfg.IpfsConf.Routing == "" {
		cfg.IpfsConf.Routing = "dht"
	}
}

func MakeConfigFromFile(file string) (*Config, error) {
	cfg := getDefaultConfig(DefaultDataDir)
	if file != "" {
		if err := loadConfig(file, cfg); err != nil {
			log.Error(err.Error())
			return nil, err
		}
	}
	return cfg, nil
}

func getDefaultConfig(dataDir string) *Config {

	ipfsConfig := GetDefaultIpfsConfig()
	ipfsConfig.DataDir = filepath.Join(dataDir, DefaultIpfsDataDir)
	ipfsConfig.IpfsPort = DefaultIpfsPort
	ipfsConfig.BootNodes = DefaultIpfsBootstrapNodes
	ipfsConfig.SwarmKey = DefaultSwarmKey

	return &Config{
		DataDir: dataDir,
		Network: 0x1, // testnet
		P2P: P2P{
			MaxInboundPeers:          DefaultMaxInboundNotOwnShardPeers,
			MaxOutboundPeers:         DefaultMaxOutboundNotOwnShardPeers,
			MaxInboundOwnShardPeers:  DefaultMaxInboundOwnShardPeers,
			MaxOutboundOwnShardPeers: DefaultMaxOutboundOwnShardPeers,
			DisableMetrics:           false,
		},
		Consensus: GetDefaultConsensusConfig(),
		RPC:       rpc.GetDefaultRPCConfig(DefaultRpcHost, DefaultRpcPort),
		GenesisConf: &GenesisConf{
			FirstCeremonyTime: DefaultCeremonyTime,
			GodAddress:        common.HexToAddress(DefaultGodAddress),
		},
		IpfsConf:   ipfsConfig,
		Validation: &ValidationConfig{},
		Sync: &SyncConfig{
			FastSync:            true,
			ForceFullSync:       DefaultForceFullSync,
			AllFlipsLoadingTime: time.Hour * 2,
		},
		OfflineDetection: GetDefaultOfflineDetectionConfig(),
		Blockchain: &BlockchainConfig{
			StoreCertRange: DefaultStoreCertRange,
			BurnTxRange:    DefaultBurntTxRange,
		},
		Mempool: GetDefaultMempoolConfig(),
	}
}

func applyFlags(ctx *cli.Context, cfg *Config) {
	applyProfile(ctx, cfg)
	applyCommonFlags(ctx, cfg)
	applyP2PFlags(ctx, cfg)
	applyConsensusFlags(ctx, cfg)
	applyRpcFlags(ctx, cfg)
	applyGenesisFlags(ctx, cfg)
	applyIpfsFlags(ctx, cfg)
	applyValidationFlags(ctx, cfg)
	applySyncFlags(ctx, cfg)
}

func applyCommonFlags(ctx *cli.Context, cfg *Config) {
	if ctx.IsSet(AutoOnline.Name) {
		cfg.AutoOnline = ctx.Bool(AutoOnline.Name)
	}
}

func applySyncFlags(ctx *cli.Context, cfg *Config) {
	if ctx.IsSet(FastSyncFlag.Name) {
		cfg.Sync.FastSync = ctx.Bool(FastSyncFlag.Name)
	}
	if ctx.IsSet(ForceFullSyncFlag.Name) {
		cfg.Sync.ForceFullSync = ctx.Uint64(ForceFullSyncFlag.Name)
	}
}

func applyP2PFlags(ctx *cli.Context, cfg *Config) {
	if ctx.IsSet(MaxNetworkDelayFlag.Name) {
		cfg.P2P.MaxDelay = ctx.Int(MaxNetworkDelayFlag.Name)
	}
}

func applyConsensusFlags(ctx *cli.Context, cfg *Config) {
	if ctx.IsSet(AutomineFlag.Name) {
		cfg.Consensus.Automine = ctx.Bool(AutomineFlag.Name)
	}
}

func applyRpcFlags(ctx *cli.Context, cfg *Config) {
	if ctx.IsSet(RpcHostFlag.Name) {
		cfg.RPC.HTTPHost = ctx.String(RpcHostFlag.Name)
	}
	if ctx.IsSet(RpcPortFlag.Name) {
		cfg.RPC.HTTPPort = ctx.Int(RpcPortFlag.Name)
	}
	if ctx.IsSet(ApiKeyFlag.Name) {
		cfg.RPC.APIKey = ctx.String(ApiKeyFlag.Name)
	}
}

func applyGenesisFlags(ctx *cli.Context, cfg *Config) {
	if ctx.IsSet(GodAddressFlag.Name) {
		cfg.GenesisConf.GodAddress = common.HexToAddress(ctx.String(GodAddressFlag.Name))
	}
	if ctx.IsSet(CeremonyTimeFlag.Name) {
		cfg.GenesisConf.FirstCeremonyTime = ctx.Int64(CeremonyTimeFlag.Name)
	}
}

func applyIpfsFlags(ctx *cli.Context, cfg *Config) {
	cfg.IpfsConf.DataDir = filepath.Join(cfg.DataDir, DefaultIpfsDataDir)

	if ctx.IsSet(IpfsPortFlag.Name) {
		cfg.IpfsConf.IpfsPort = ctx.Int(IpfsPortFlag.Name)
	}
	if ctx.IsSet(IpfsPortStaticFlag.Name) {
		cfg.IpfsConf.StaticPort = ctx.Bool(IpfsPortStaticFlag.Name)
	}
	if ctx.IsSet(IpfsBootNodeFlag.Name) {
		cfg.IpfsConf.BootNodes = []string{ctx.String(IpfsBootNodeFlag.Name)}
	}
}

func applyValidationFlags(ctx *cli.Context, cfg *Config) {

}

func loadConfig(configPath string, conf *Config) error {
	if _, err := os.Stat(configPath); err != nil {
		return errors.Errorf("Config file cannot be found, path: %v", configPath)
	}

	if jsonFile, err := os.Open(configPath); err != nil {
		return errors.Errorf("Config file cannot be opened, path: %v", configPath)
	} else {
		byteValue, _ := ioutil.ReadAll(jsonFile)
		err := json.Unmarshal(byteValue, &conf)
		if err != nil {
			return errors.Wrap(err, errors.Errorf("Cannot parse JSON config, path: %v", configPath).Error())
		}
		return nil
	}
}
