package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

const (
	// TTL for historical states (60 minutes for 1024 blocks sliding window)
	StateHistoryTTL = 60 * time.Minute
)

// Account represents an Ethereum account
type Account struct {
	Balance     *big.Int    `json:"balance"`
	Nonce       uint64      `json:"nonce"`
	CodeHash    common.Hash `json:"codeHash"`
	StorageRoot common.Hash `json:"storageRoot"`
}

// StateStorage handles state persistence in Pika
type StateStorage struct {
	client *PikaClient
}

// NewStateStorage creates a new state storage
func NewStateStorage(client *PikaClient) *StateStorage {
	return &StateStorage{
		client: client,
	}
}

// Account key formats:
// Historical: st:{blockNum}:acc:{address}
// Latest: st:latest:acc:{address}
func (s *StateStorage) accountKey(blockNum uint64, addr common.Address) string {
	if blockNum == 0 {
		return fmt.Sprintf("st:latest:acc:%s", addr.Hex())
	}
	return fmt.Sprintf("st:%d:acc:%s", blockNum, addr.Hex())
}

// Storage key formats:
// Historical: st:{blockNum}:stor:{address}:{key}
// Latest: st:latest:stor:{address}:{key}
func (s *StateStorage) storageKey(blockNum uint64, addr common.Address, key common.Hash) string {
	if blockNum == 0 {
		return fmt.Sprintf("st:latest:stor:%s:%s", addr.Hex(), key.Hex())
	}
	return fmt.Sprintf("st:%d:stor:%s:%s", blockNum, addr.Hex(), key.Hex())
}

// Code key format: st:code:{codeHash}
func (s *StateStorage) codeKey(codeHash common.Hash) string {
	return fmt.Sprintf("st:code:%s", codeHash.Hex())
}

// State root key format: st:root:{blockNum}
func (s *StateStorage) stateRootKey(blockNum uint64) string {
	return fmt.Sprintf("st:root:%d", blockNum)
}

// GetAccount retrieves an account from storage
func (s *StateStorage) GetAccount(ctx context.Context, blockNum uint64, addr common.Address) (*Account, error) {
	key := s.accountKey(blockNum, addr)
	data, err := s.client.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	var acc Account
	if err := json.Unmarshal([]byte(data), &acc); err != nil {
		return nil, fmt.Errorf("failed to unmarshal account: %w", err)
	}

	return &acc, nil
}

// SetAccount stores an account
func (s *StateStorage) SetAccount(ctx context.Context, blockNum uint64, addr common.Address, acc *Account) error {
	data, err := json.Marshal(acc)
	if err != nil {
		return fmt.Errorf("failed to marshal account: %w", err)
	}

	// Write to historical state (with TTL)
	if blockNum > 0 {
		histKey := s.accountKey(blockNum, addr)
		if err := s.client.Set(ctx, histKey, string(data), StateHistoryTTL); err != nil {
			return err
		}
	}

	// Always write to latest state (no TTL)
	latestKey := s.accountKey(0, addr)
	return s.client.Set(ctx, latestKey, string(data), 0)
}

// GetStorage retrieves a storage value
func (s *StateStorage) GetStorage(ctx context.Context, blockNum uint64, addr common.Address, key common.Hash) (common.Hash, error) {
	storKey := s.storageKey(blockNum, addr, key)
	data, err := s.client.Get(ctx, storKey)
	if err != nil {
		return common.Hash{}, err
	}

	return common.HexToHash(data), nil
}

// SetStorage stores a storage value
func (s *StateStorage) SetStorage(ctx context.Context, blockNum uint64, addr common.Address, key, value common.Hash) error {
	// Write to historical state (with TTL)
	if blockNum > 0 {
		histKey := s.storageKey(blockNum, addr, key)
		if err := s.client.Set(ctx, histKey, value.Hex(), StateHistoryTTL); err != nil {
			return err
		}
	}

	// Always write to latest state (no TTL)
	latestKey := s.storageKey(0, addr, key)
	return s.client.Set(ctx, latestKey, value.Hex(), 0)
}

// GetCode retrieves contract code
func (s *StateStorage) GetCode(ctx context.Context, codeHash common.Hash) ([]byte, error) {
	key := s.codeKey(codeHash)
	return s.client.GetBytes(ctx, key)
}

// SetCode stores contract code (permanent)
func (s *StateStorage) SetCode(ctx context.Context, codeHash common.Hash, code []byte) error {
	key := s.codeKey(codeHash)
	return s.client.SetBytes(ctx, key, code, 0) // No TTL for code
}

// GetStateRoot retrieves the state root for a block
func (s *StateStorage) GetStateRoot(ctx context.Context, blockNum uint64) (common.Hash, error) {
	key := s.stateRootKey(blockNum)
	data, err := s.client.Get(ctx, key)
	if err != nil {
		return common.Hash{}, err
	}
	return common.HexToHash(data), nil
}

// SetStateRoot stores the state root for a block
func (s *StateStorage) SetStateRoot(ctx context.Context, blockNum uint64, root common.Hash) error {
	key := s.stateRootKey(blockNum)
	return s.client.Set(ctx, key, root.Hex(), StateHistoryTTL)
}

// BatchWrite performs batch write operations using pipeline
func (s *StateStorage) BatchWrite(ctx context.Context, ops []WriteOp) error {
	pipe := s.client.Pipeline()

	for _, op := range ops {
		switch op.Type {
		case WriteOpAccount:
			data, err := json.Marshal(op.Account)
			if err != nil {
				return fmt.Errorf("failed to marshal account: %w", err)
			}
			if op.BlockNum > 0 {
				histKey := s.accountKey(op.BlockNum, op.Address)
				pipe.Set(ctx, histKey, data, StateHistoryTTL)
			}
			latestKey := s.accountKey(0, op.Address)
			pipe.Set(ctx, latestKey, data, 0)

		case WriteOpStorage:
			if op.BlockNum > 0 {
				histKey := s.storageKey(op.BlockNum, op.Address, op.Key)
				pipe.Set(ctx, histKey, op.Value.Hex(), StateHistoryTTL)
			}
			latestKey := s.storageKey(0, op.Address, op.Key)
			pipe.Set(ctx, latestKey, op.Value.Hex(), 0)

		case WriteOpCode:
			codeKey := s.codeKey(op.CodeHash)
			pipe.Set(ctx, codeKey, op.Code, 0)
		}
	}

	return s.client.ExecutePipeline(ctx, pipe)
}

// WriteOpType defines the type of write operation
type WriteOpType int

const (
	WriteOpAccount WriteOpType = iota
	WriteOpStorage
	WriteOpCode
)

// WriteOp represents a batch write operation
type WriteOp struct {
	Type     WriteOpType
	BlockNum uint64
	Address  common.Address
	Account  *Account
	Key      common.Hash
	Value    common.Hash
	CodeHash common.Hash
	Code     []byte
}
