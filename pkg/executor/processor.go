package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"

	"github.com/sunvim/evm_executor/pkg/logger"
	"github.com/sunvim/evm_executor/pkg/state"
	"github.com/sunvim/evm_executor/pkg/storage"
)

// Processor handles block processing and transaction execution
type Processor struct {
	storage      *storage.StateStorage
	blockStorage *storage.BlockStorage
	receiptStore *storage.ReceiptStorage
	chainConfig  *params.ChainConfig
	vmConfig     vm.Config
	cacheConfig  CacheConfig
}

// CacheConfig holds cache configuration
type CacheConfig struct {
	StateCacheSize   int
	StorageCacheSize int
	CodeCacheSize    int
}

// NewProcessor creates a new block processor
func NewProcessor(
	stateStorage *storage.StateStorage,
	blockStorage *storage.BlockStorage,
	receiptStore *storage.ReceiptStorage,
	chainConfig *params.ChainConfig,
	vmConfig vm.Config,
	cacheConfig CacheConfig,
) *Processor {
	return &Processor{
		storage:      stateStorage,
		blockStorage: blockStorage,
		receiptStore: receiptStore,
		chainConfig:  chainConfig,
		vmConfig:     vmConfig,
		cacheConfig:  cacheConfig,
	}
}

// ProcessBlock processes a single block and returns receipts
func (p *Processor) ProcessBlock(ctx context.Context, blockNum uint64) (types.Receipts, uint64, error) {
	startTime := time.Now()

	// Get block
	block, err := p.blockStorage.GetBlock(ctx, blockNum)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get block %d: %w", blockNum, err)
	}

	// Create state DB based on parent block state
	parentBlockNum := blockNum - 1
	stateDB, err := state.NewPikaStateDB(
		ctx,
		p.storage,
		parentBlockNum,
		p.cacheConfig.StateCacheSize,
		p.cacheConfig.StorageCacheSize,
		p.cacheConfig.CodeCacheSize,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create state DB: %w", err)
	}

	// Create chain context
	chainContext := &blockChainContext{
		blockStorage: p.blockStorage,
		ctx:          ctx,
	}

	// Process all transactions
	receipts, gasUsed, err := p.processTransactions(ctx, chainContext, stateDB, block)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to process transactions: %w", err)
	}

	// Commit state changes
	if err := stateDB.Commit(); err != nil {
		return nil, 0, fmt.Errorf("failed to commit state: %w", err)
	}

	// Store receipts
	if err := p.receiptStore.StoreReceipts(ctx, blockNum, receipts); err != nil {
		return nil, 0, fmt.Errorf("failed to store receipts: %w", err)
	}

	elapsed := time.Since(startTime)
	logger.Infof("Block %d executed: %d txs, %d gas, took %s",
		blockNum, len(receipts), gasUsed, elapsed)

	return receipts, gasUsed, nil
}

// processTransactions executes all transactions in a block
func (p *Processor) processTransactions(
	ctx context.Context,
	chainContext ChainContext,
	stateDB *state.PikaStateDB,
	block *types.Block,
) (types.Receipts, uint64, error) {
	var (
		receipts    = make([]*types.Receipt, 0, len(block.Transactions()))
		usedGas     = uint64(0)
		header      = block.Header()
		gp          = new(core.GasPool).AddGas(block.GasLimit())
		blockHash   = block.Hash()
		blockNumber = block.NumberU64()
	)

	// Process each transaction
	for i, tx := range block.Transactions() {
		stateDB.SetTxContext(tx.Hash(), i)

		receipt, err := ApplyTransaction(
			p.chainConfig,
			chainContext,
			&header.Coinbase,
			gp,
			stateDB,
			header,
			tx,
			&usedGas,
			p.vmConfig,
		)

		if err != nil {
			logger.Warnf("Transaction %s failed: %v", tx.Hash().Hex(), err)
			// Even if transaction fails, we create a failed receipt
			if receipt == nil {
				receipt = &types.Receipt{
					Type:              tx.Type(),
					Status:            types.ReceiptStatusFailed,
					CumulativeGasUsed: usedGas,
					TxHash:            tx.Hash(),
					GasUsed:           0,
					BlockHash:         blockHash,
					BlockNumber:       header.Number,
					TransactionIndex:  uint(i),
				}
			}
		}

		// Ensure receipt has block info
		receipt.BlockHash = blockHash
		receipt.BlockNumber = header.Number
		receipt.TransactionIndex = uint(i)

		receipts = append(receipts, receipt)
	}

	// Validate gas used matches block header
	if usedGas != header.GasUsed {
		logger.Warnf("Block %d gas mismatch: calculated %d, header %d",
			blockNumber, usedGas, header.GasUsed)
	}

	return receipts, usedGas, nil
}

// blockChainContext implements ChainContext
type blockChainContext struct {
	blockStorage *storage.BlockStorage
	ctx          context.Context
}

func (b *blockChainContext) GetHeader(hash common.Hash, number uint64) *types.Header {
	header, err := b.blockStorage.GetHeader(b.ctx, number)
	if err != nil {
		return nil
	}
	// Verify hash matches
	if header.Hash() != hash {
		return nil
	}
	return header
}
