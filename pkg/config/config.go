package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Chain     ChainConfig     `mapstructure:"chain"`
	Storage   StorageConfig   `mapstructure:"storage"`
	Execution ExecutionConfig `mapstructure:"execution"`
	EVM       EVMConfig       `mapstructure:"evm"`
	Workers   WorkersConfig   `mapstructure:"worker_pools"`
	Metrics   MetricsConfig   `mapstructure:"metrics"`
	Logging   LoggingConfig   `mapstructure:"logging"`
}

type ChainConfig struct {
	Name      string `mapstructure:"name"`
	NetworkID uint64 `mapstructure:"network_id"`
}

type StorageConfig struct {
	Pika PikaConfig `mapstructure:"pika"`
}

type PikaConfig struct {
	Addr           string `mapstructure:"addr"`
	Password       string `mapstructure:"password"`
	DB             int    `mapstructure:"db"`
	MaxConnections int    `mapstructure:"max_connections"`
	PipelineSize   int    `mapstructure:"pipeline_size"`
}

type ExecutionConfig struct {
	ConcurrentWorkers int `mapstructure:"concurrent_workers"`
	BatchSize         int `mapstructure:"batch_size"`
	StateWindowSize   int `mapstructure:"state_window_size"`
	StateCacheSize    int `mapstructure:"state_cache_size"`
	StorageCacheSize  int `mapstructure:"storage_cache_size"`
	CodeCacheSize     int `mapstructure:"code_cache_size"`
}

type EVMConfig struct {
	EnableDebug bool          `mapstructure:"enable_debug"`
	VMTimeout   time.Duration `mapstructure:"vm_timeout"`
	GasLimit    uint64        `mapstructure:"gas_limit"`
}

type WorkersConfig struct {
	TxExecutor     WorkerPoolConfig `mapstructure:"tx_executor"`
	StateCommitter WorkerPoolConfig `mapstructure:"state_committer"`
	ReceiptWriter  WorkerPoolConfig `mapstructure:"receipt_writer"`
}

type WorkerPoolConfig struct {
	WorkerCount int `mapstructure:"worker_count"`
	QueueSize   int `mapstructure:"queue_size"`
}

type MetricsConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	ListenAddr string `mapstructure:"listen_addr"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// Load loads configuration from file and environment variables
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Read config file if provided
	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Read from environment variables
	v.SetEnvPrefix("EVM_EXECUTOR")
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	// Chain defaults
	v.SetDefault("chain.name", "bsc")
	v.SetDefault("chain.network_id", 56)

	// Storage defaults
	v.SetDefault("storage.pika.addr", "127.0.0.1:9221")
	v.SetDefault("storage.pika.password", "")
	v.SetDefault("storage.pika.db", 0)
	v.SetDefault("storage.pika.max_connections", 200)
	v.SetDefault("storage.pika.pipeline_size", 1000)

	// Execution defaults
	v.SetDefault("execution.concurrent_workers", 4)
	v.SetDefault("execution.batch_size", 10)
	v.SetDefault("execution.state_window_size", 1024)
	v.SetDefault("execution.state_cache_size", 10000)
	v.SetDefault("execution.storage_cache_size", 50000)
	v.SetDefault("execution.code_cache_size", 1000)

	// EVM defaults
	v.SetDefault("evm.enable_debug", false)
	v.SetDefault("evm.vm_timeout", "30s")
	v.SetDefault("evm.gas_limit", 30000000)

	// Worker pools defaults
	v.SetDefault("worker_pools.tx_executor.worker_count", 4)
	v.SetDefault("worker_pools.tx_executor.queue_size", 100)
	v.SetDefault("worker_pools.state_committer.worker_count", 2)
	v.SetDefault("worker_pools.state_committer.queue_size", 50)
	v.SetDefault("worker_pools.receipt_writer.worker_count", 4)
	v.SetDefault("worker_pools.receipt_writer.queue_size", 100)

	// Metrics defaults
	v.SetDefault("metrics.enabled", true)
	v.SetDefault("metrics.listen_addr", "0.0.0.0:9091")

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
}
