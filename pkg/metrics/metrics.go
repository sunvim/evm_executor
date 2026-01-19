package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Execution metrics
	ExecutionHeight = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "evm_executor_execution_height",
		Help: "Current execution block height",
	})

	ExecutionLag = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "evm_executor_execution_lag",
		Help: "Execution lag (sync height - exec height)",
	})

	ExecutionSpeed = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "evm_executor_execution_speed_bps",
		Help: "Execution speed in blocks per second",
	})

	TxsExecutedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "evm_executor_txs_executed_total",
		Help: "Total number of transactions executed",
	})

	TxsFailedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "evm_executor_txs_failed_total",
		Help: "Total number of failed transactions",
	})

	GasUsedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "evm_executor_gas_used_total",
		Help: "Total gas used",
	})

	EVMExecutionLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "evm_executor_evm_execution_latency_seconds",
		Help:    "EVM execution latency in seconds",
		Buckets: prometheus.DefBuckets,
	})

	// State metrics
	StateAccountsCached = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "evm_executor_state_accounts_cached",
		Help: "Number of accounts in cache",
	})

	StateCacheHitRate = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "evm_executor_state_cache_hit_rate",
		Help: "State cache hit rate",
	})

	StateWriteBatchSize = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "evm_executor_state_write_batch_size",
		Help: "State write batch size",
		Buckets: []float64{10, 50, 100, 500, 1000, 5000, 10000},
	})

	StateCommitLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "evm_executor_state_commit_latency_seconds",
		Help:    "State commit latency in seconds",
		Buckets: prometheus.DefBuckets,
	})

	// Worker pool metrics
	WorkerPoolActiveWorkers = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "evm_executor_worker_pool_active_workers",
		Help: "Number of active workers in the pool",
	}, []string{"pool"})

	WorkerPoolQueueLength = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "evm_executor_worker_pool_queue_length",
		Help: "Length of the worker pool task queue",
	}, []string{"pool"})

	WorkerPoolTasksCompletedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "evm_executor_worker_pool_tasks_completed_total",
		Help: "Total number of tasks completed by the pool",
	}, []string{"pool"})

	WorkerPoolTasksFailedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "evm_executor_worker_pool_tasks_failed_total",
		Help: "Total number of failed tasks in the pool",
	}, []string{"pool"})
)
