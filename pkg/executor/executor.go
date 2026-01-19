package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/sunvim/evm_executor/pkg/chain"
	"github.com/sunvim/evm_executor/pkg/config"
	"github.com/sunvim/evm_executor/pkg/logger"
	"github.com/sunvim/evm_executor/pkg/storage"
	"github.com/sunvim/evm_executor/pkg/worker"
)

// Executor is the main block execution engine
type Executor struct {
	config          *config.Config
	pikaClient      *storage.PikaClient
	stateStorage    *storage.StateStorage
	blockStorage    *storage.BlockStorage
	receiptStorage  *storage.ReceiptStorage
	progressStorage *storage.ProgressStorage
	processor       *Processor
	workerPool      *worker.Pool
	chainAdapter    chain.ChainAdapter
	stopChan        chan struct{}
}

// NewExecutor creates a new executor
func NewExecutor(cfg *config.Config) (*Executor, error) {
	// Create Pika client
	pikaClient, err := storage.NewPikaClient(
		cfg.Storage.Pika.Addr,
		cfg.Storage.Pika.Password,
		cfg.Storage.Pika.DB,
		cfg.Storage.Pika.MaxConnections,
		cfg.Storage.Pika.PipelineSize,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Pika client: %w", err)
	}

	// Create storage layers
	stateStorage := storage.NewStateStorage(pikaClient)
	blockStorage := storage.NewBlockStorage(pikaClient)
	receiptStorage := storage.NewReceiptStorage(pikaClient)
	progressStorage := storage.NewProgressStorage(pikaClient)

	// Create chain adapter
	chainAdapter, err := chain.NewChainAdapter(cfg.Chain.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to create chain adapter: %w", err)
	}

	// Create VM config
	vmConfig := vm.Config{
		EnablePreimageRecording: false,
	}

	// Create cache config
	cacheConfig := CacheConfig{
		StateCacheSize:   cfg.Execution.StateCacheSize,
		StorageCacheSize: cfg.Execution.StorageCacheSize,
		CodeCacheSize:    cfg.Execution.CodeCacheSize,
	}

	// Create processor
	processor := NewProcessor(
		stateStorage,
		blockStorage,
		receiptStorage,
		chainAdapter.GetChainConfig(),
		vmConfig,
		cacheConfig,
	)

	// Create worker pool for block execution
	workerPool := worker.NewPool(
		"tx-executor",
		cfg.Workers.TxExecutor.WorkerCount,
		cfg.Workers.TxExecutor.QueueSize,
	)

	return &Executor{
		config:          cfg,
		pikaClient:      pikaClient,
		stateStorage:    stateStorage,
		blockStorage:    blockStorage,
		receiptStorage:  receiptStorage,
		progressStorage: progressStorage,
		processor:       processor,
		workerPool:      workerPool,
		chainAdapter:    chainAdapter,
		stopChan:        make(chan struct{}),
	}, nil
}

// Start starts the executor
func (e *Executor) Start(ctx context.Context) error {
	logger.Info("Starting EVM executor")

	// Start worker pool
	e.workerPool.Start()

	// Start main execution loop
	go e.run(ctx)

	return nil
}

// run is the main execution loop
func (e *Executor) run(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Executor context cancelled, stopping...")
			return

		case <-e.stopChan:
			logger.Info("Executor stop signal received")
			return

		case <-ticker.C:
			if err := e.executeNextBlocks(ctx); err != nil {
				logger.Errorf("Failed to execute blocks: %v", err)
			}
		}
	}
}

// executeNextBlocks checks for and executes pending blocks
func (e *Executor) executeNextBlocks(ctx context.Context) error {
	// Get sync progress
	syncHeight, err := e.progressStorage.GetSyncProgress(ctx)
	if err != nil {
		return fmt.Errorf("failed to get sync progress: %w", err)
	}

	// Get execution progress
	execProgress, err := e.progressStorage.GetExecProgress(ctx)
	if err != nil {
		return fmt.Errorf("failed to get exec progress: %w", err)
	}

	execHeight := execProgress.LatestExecutedBlock

	// Check if there are blocks to execute
	if syncHeight <= execHeight {
		return nil // All caught up
	}

	// Execute next batch of blocks
	batchSize := uint64(e.config.Execution.BatchSize)
	endBlock := execHeight + batchSize
	if endBlock > syncHeight {
		endBlock = syncHeight
	}

	logger.Infof("Executing blocks %d to %d (sync height: %d)",
		execHeight+1, endBlock, syncHeight)

	// Execute blocks sequentially for now
	for blockNum := execHeight + 1; blockNum <= endBlock; blockNum++ {
		if err := e.executeBlock(ctx, blockNum); err != nil {
			return fmt.Errorf("failed to execute block %d: %w", blockNum, err)
		}

		// Update progress
		execProgress.LatestExecutedBlock = blockNum
		execProgress.LastExecutionTime = time.Now().Unix()
		
		if err := e.progressStorage.UpdateExecProgress(ctx, execProgress); err != nil {
			return fmt.Errorf("failed to update progress: %w", err)
		}
	}

	return nil
}

// executeBlock executes a single block
func (e *Executor) executeBlock(ctx context.Context, blockNum uint64) error {
	startTime := time.Now()

	// Process block
	receipts, gasUsed, err := e.processor.ProcessBlock(ctx, blockNum)
	if err != nil {
		return err
	}

	elapsed := time.Since(startTime)
	logger.Infof("Block %d executed: %d txs, %d gas, took %s",
		blockNum, len(receipts), gasUsed, elapsed)

	return nil
}

// Stop gracefully stops the executor
func (e *Executor) Stop() error {
	logger.Info("Stopping executor...")

	// Signal stop
	close(e.stopChan)

	// Shutdown worker pool
	e.workerPool.Shutdown()

	// Close Pika client
	if err := e.pikaClient.Close(); err != nil {
		return fmt.Errorf("failed to close Pika client: %w", err)
	}

	logger.Info("Executor stopped")
	return nil
}

// GetProgress returns current execution progress
func (e *Executor) GetProgress(ctx context.Context) (*storage.Progress, error) {
	return e.progressStorage.GetExecProgress(ctx)
}

// GetSyncHeight returns the current sync height
func (e *Executor) GetSyncHeight(ctx context.Context) (uint64, error) {
	return e.progressStorage.GetSyncProgress(ctx)
}
