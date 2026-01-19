package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

// BlockStorage handles block data operations
type BlockStorage struct {
	client *PikaClient
}

// NewBlockStorage creates a new block storage
func NewBlockStorage(client *PikaClient) *BlockStorage {
	return &BlockStorage{
		client: client,
	}
}

// Block key formats:
// Header: blk:hdr:{blockNumber}
// Body: blk:body:{blockNumber}
// Transactions: blk:txs:{blockNumber}

func (b *BlockStorage) headerKey(blockNum uint64) string {
	return fmt.Sprintf("blk:hdr:%d", blockNum)
}

func (b *BlockStorage) bodyKey(blockNum uint64) string {
	return fmt.Sprintf("blk:body:%d", blockNum)
}

func (b *BlockStorage) txsKey(blockNum uint64) string {
	return fmt.Sprintf("blk:txs:%d", blockNum)
}

// GetBlock retrieves a complete block
func (b *BlockStorage) GetBlock(ctx context.Context, blockNum uint64) (*types.Block, error) {
	// Get header
	headerData, err := b.client.GetBytes(ctx, b.headerKey(blockNum))
	if err != nil {
		return nil, fmt.Errorf("failed to get header: %w", err)
	}

	var header types.Header
	if err := rlp.DecodeBytes(headerData, &header); err != nil {
		return nil, fmt.Errorf("failed to decode header: %w", err)
	}

	// Get transactions
	txsData, err := b.client.GetBytes(ctx, b.txsKey(blockNum))
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions: %w", err)
	}

	var txs []*types.Transaction
	if err := rlp.DecodeBytes(txsData, &txs); err != nil {
		return nil, fmt.Errorf("failed to decode transactions: %w", err)
	}

	// Construct block (without uncles for simplicity)
	return types.NewBlock(&header, txs, nil, nil, nil), nil
}

// GetHeader retrieves a block header
func (b *BlockStorage) GetHeader(ctx context.Context, blockNum uint64) (*types.Header, error) {
	headerData, err := b.client.GetBytes(ctx, b.headerKey(blockNum))
	if err != nil {
		return nil, err
	}

	var header types.Header
	if err := rlp.DecodeBytes(headerData, &header); err != nil {
		return nil, fmt.Errorf("failed to decode header: %w", err)
	}

	return &header, nil
}

// ReceiptStorage handles receipt operations
type ReceiptStorage struct {
	client *PikaClient
}

// NewReceiptStorage creates a new receipt storage
func NewReceiptStorage(client *PikaClient) *ReceiptStorage {
	return &ReceiptStorage{
		client: client,
	}
}

// Receipt key format: blk:rcpt:{blockNumber}
func (r *ReceiptStorage) receiptKey(blockNum uint64) string {
	return fmt.Sprintf("blk:rcpt:%d", blockNum)
}

// StoreReceipts stores receipts for a block
func (r *ReceiptStorage) StoreReceipts(ctx context.Context, blockNum uint64, receipts types.Receipts) error {
	data, err := rlp.EncodeToBytes(receipts)
	if err != nil {
		return fmt.Errorf("failed to encode receipts: %w", err)
	}

	key := r.receiptKey(blockNum)
	return r.client.SetBytes(ctx, key, data, 0) // Permanent storage
}

// GetReceipts retrieves receipts for a block
func (r *ReceiptStorage) GetReceipts(ctx context.Context, blockNum uint64) (types.Receipts, error) {
	key := r.receiptKey(blockNum)
	data, err := r.client.GetBytes(ctx, key)
	if err != nil {
		return nil, err
	}

	var receipts types.Receipts
	if err := rlp.DecodeBytes(data, &receipts); err != nil {
		return nil, fmt.Errorf("failed to decode receipts: %w", err)
	}

	return receipts, nil
}

// ProgressStorage handles execution progress tracking
type ProgressStorage struct {
	client *PikaClient
}

// NewProgressStorage creates a new progress storage
func NewProgressStorage(client *PikaClient) *ProgressStorage {
	return &ProgressStorage{
		client: client,
	}
}

// Progress represents execution progress
type Progress struct {
	LatestExecutedBlock uint64 `json:"latestExecutedBlock"`
	LastExecutionTime   int64  `json:"lastExecutionTime"`
	TotalTxsExecuted    uint64 `json:"totalTxsExecuted"`
}

func (p *ProgressStorage) syncProgressKey() string {
	return "idx:sync:progress"
}

func (p *ProgressStorage) execProgressKey() string {
	return "idx:exec:progress"
}

// GetSyncProgress retrieves the sync progress
func (p *ProgressStorage) GetSyncProgress(ctx context.Context) (uint64, error) {
	data, err := p.client.Get(ctx, p.syncProgressKey())
	if err != nil {
		return 0, err
	}

	var progress struct {
		LatestSyncedBlock uint64 `json:"latestSyncedBlock"`
	}
	if err := json.Unmarshal([]byte(data), &progress); err != nil {
		return 0, fmt.Errorf("failed to unmarshal sync progress: %w", err)
	}

	return progress.LatestSyncedBlock, nil
}

// GetExecProgress retrieves the execution progress
func (p *ProgressStorage) GetExecProgress(ctx context.Context) (*Progress, error) {
	data, err := p.client.Get(ctx, p.execProgressKey())
	if err != nil {
		// Return default progress if not found
		return &Progress{}, nil
	}

	var progress Progress
	if err := json.Unmarshal([]byte(data), &progress); err != nil {
		return nil, fmt.Errorf("failed to unmarshal exec progress: %w", err)
	}

	return &progress, nil
}

// UpdateExecProgress updates the execution progress
func (p *ProgressStorage) UpdateExecProgress(ctx context.Context, progress *Progress) error {
	data, err := json.Marshal(progress)
	if err != nil {
		return fmt.Errorf("failed to marshal progress: %w", err)
	}

	return p.client.Set(ctx, p.execProgressKey(), string(data), 0)
}

// TxHash key format: blk:txhash:{txHash}
func (b *BlockStorage) txHashKey(txHash common.Hash) string {
	return fmt.Sprintf("blk:txhash:%s", txHash.Hex())
}

// GetBlockNumberByTxHash retrieves block number by transaction hash
func (b *BlockStorage) GetBlockNumberByTxHash(ctx context.Context, txHash common.Hash) (uint64, error) {
	key := b.txHashKey(txHash)
	data, err := b.client.Get(ctx, key)
	if err != nil {
		return 0, err
	}

	var blockNum uint64
	if _, err := fmt.Sscanf(data, "%d", &blockNum); err != nil {
		return 0, fmt.Errorf("failed to parse block number: %w", err)
	}

	return blockNum, nil
}
