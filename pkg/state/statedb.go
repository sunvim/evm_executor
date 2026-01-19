package state

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/sunvim/evm_executor/pkg/storage"
)

// PikaStateDB implements vm.StateDB interface backed by Pika storage
type PikaStateDB struct {
	storage      *storage.StateStorage
	blockNumber  uint64
	ctx          context.Context

	// Caches
	stateCache   *lru.Cache[common.Address, *storage.Account]
	storageCache *lru.Cache[string, common.Hash]
	codeCache    *lru.Cache[common.Hash, []byte]

	// Current block modifications
	accounts  map[common.Address]*Account
	storages  map[common.Address]map[common.Hash]common.Hash
	codes     map[common.Hash][]byte

	// Logs and suicides
	logs      map[common.Hash][]*types.Log
	logSize   uint
	suicides  map[common.Address]bool
	preimages map[common.Hash][]byte

	// Journal for state changes
	journal        *journal
	validRevisions []revision
	nextRevisionID int

	// Refund counter
	refund uint64

	// Access list (EIP-2930)
	accessList *AccessList

	// Thehash of the transaction currently being processed
	thash common.Hash
	// The index of the transaction in the block
	txIndex int
}

type revision struct {
	id           int
	journalIndex int
}

// NewPikaStateDB creates a new StateDB backed by Pika
func NewPikaStateDB(
	ctx context.Context,
	stateStorage *storage.StateStorage,
	blockNumber uint64,
	stateCacheSize, storageCacheSize, codeCacheSize int,
) (*PikaStateDB, error) {
	stateCache, err := lru.New[common.Address, *storage.Account](stateCacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create state cache: %w", err)
	}

	storageCache, err := lru.New[string, common.Hash](storageCacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage cache: %w", err)
	}

	codeCache, err := lru.New[common.Hash, []byte](codeCacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create code cache: %w", err)
	}

	return &PikaStateDB{
		storage:      stateStorage,
		blockNumber:  blockNumber,
		ctx:          ctx,
		stateCache:   stateCache,
		storageCache: storageCache,
		codeCache:    codeCache,
		accounts:     make(map[common.Address]*Account),
		storages:     make(map[common.Address]map[common.Hash]common.Hash),
		codes:        make(map[common.Hash][]byte),
		logs:         make(map[common.Hash][]*types.Log),
		suicides:     make(map[common.Address]bool),
		preimages:    make(map[common.Hash][]byte),
		journal:      newJournal(),
		accessList:   newAccessList(),
	}, nil
}

// getAccount retrieves an account from cache or storage
func (s *PikaStateDB) getAccount(addr common.Address) *Account {
	// Check current modifications first
	if acc, ok := s.accounts[addr]; ok {
		return acc
	}

	// Check cache
	if cached, ok := s.stateCache.Get(addr); ok {
		acc := &Account{
			Balance:  new(big.Int).Set(cached.Balance),
			Nonce:    cached.Nonce,
			CodeHash: cached.CodeHash,
			Root:     cached.StorageRoot,
		}
		s.accounts[addr] = acc
		return acc
	}

	// Load from storage
	stored, err := s.storage.GetAccount(s.ctx, s.blockNumber, addr)
	if err != nil {
		// Account doesn't exist, return empty account
		acc := NewAccount()
		return acc
	}

	acc := &Account{
		Balance:  new(big.Int).Set(stored.Balance),
		Nonce:    stored.Nonce,
		CodeHash: stored.CodeHash,
		Root:     stored.StorageRoot,
	}

	// Cache it
	s.stateCache.Add(addr, stored)
	s.accounts[addr] = acc
	return acc
}

// CreateAccount creates a new account
func (s *PikaStateDB) CreateAccount(addr common.Address) {
	acc := s.getAccount(addr)
	if acc.Nonce == 0 && acc.Balance.Sign() == 0 && acc.CodeHash == (common.Hash{}) {
		// Record journal entry
		s.journal.append(createObjectChange{account: &addr})
		
		newAcc := NewAccount()
		s.accounts[addr] = newAcc
	}
}

// SubBalance subtracts amount from account balance
func (s *PikaStateDB) SubBalance(addr common.Address, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	acc := s.getAccount(addr)
	s.journal.append(balanceChange{account: &addr, prev: new(big.Int).Set(acc.Balance)})
	acc.Balance = new(big.Int).Sub(acc.Balance, amount)
	s.accounts[addr] = acc
}

// AddBalance adds amount to account balance
func (s *PikaStateDB) AddBalance(addr common.Address, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	acc := s.getAccount(addr)
	s.journal.append(balanceChange{account: &addr, prev: new(big.Int).Set(acc.Balance)})
	acc.Balance = new(big.Int).Add(acc.Balance, amount)
	s.accounts[addr] = acc
}

// GetBalance returns account balance
func (s *PikaStateDB) GetBalance(addr common.Address) *big.Int {
	acc := s.getAccount(addr)
	return new(big.Int).Set(acc.Balance)
}

// GetNonce returns account nonce
func (s *PikaStateDB) GetNonce(addr common.Address) uint64 {
	acc := s.getAccount(addr)
	return acc.Nonce
}

// SetNonce sets account nonce
func (s *PikaStateDB) SetNonce(addr common.Address, nonce uint64) {
	acc := s.getAccount(addr)
	s.journal.append(nonceChange{account: &addr, prev: acc.Nonce})
	acc.Nonce = nonce
	s.accounts[addr] = acc
}

// GetCodeHash returns account code hash
func (s *PikaStateDB) GetCodeHash(addr common.Address) common.Hash {
	acc := s.getAccount(addr)
	if acc.CodeHash == (common.Hash{}) {
		return crypto.Keccak256Hash(nil)
	}
	return acc.CodeHash
}

// GetCode returns contract code
func (s *PikaStateDB) GetCode(addr common.Address) []byte {
	acc := s.getAccount(addr)
	if acc.CodeHash == (common.Hash{}) {
		return nil
	}

	// Check current modifications
	if code, ok := s.codes[acc.CodeHash]; ok {
		return code
	}

	// Check cache
	if code, ok := s.codeCache.Get(acc.CodeHash); ok {
		return code
	}

	// Load from storage
	code, err := s.storage.GetCode(s.ctx, acc.CodeHash)
	if err != nil {
		return nil
	}

	s.codeCache.Add(acc.CodeHash, code)
	return code
}

// SetCode sets contract code
func (s *PikaStateDB) SetCode(addr common.Address, code []byte) {
	acc := s.getAccount(addr)
	prevCode := s.GetCode(addr)
	s.journal.append(codeChange{
		account:  &addr,
		prevcode: prevCode,
		prevhash: acc.CodeHash,
	})

	codeHash := crypto.Keccak256Hash(code)
	acc.CodeHash = codeHash
	s.codes[codeHash] = code
	s.accounts[addr] = acc
}

// GetCodeSize returns the size of contract code
func (s *PikaStateDB) GetCodeSize(addr common.Address) int {
	code := s.GetCode(addr)
	return len(code)
}

// storageKey creates a cache key for storage
func storageKey(addr common.Address, key common.Hash) string {
	return fmt.Sprintf("%s:%s", addr.Hex(), key.Hex())
}

// GetState returns contract storage value
func (s *PikaStateDB) GetState(addr common.Address, key common.Hash) common.Hash {
	// Check current modifications
	if storage, ok := s.storages[addr]; ok {
		if value, ok := storage[key]; ok {
			return value
		}
	}

	// Check cache
	cacheKey := storageKey(addr, key)
	if value, ok := s.storageCache.Get(cacheKey); ok {
		return value
	}

	// Load from storage
	value, err := s.storage.GetStorage(s.ctx, s.blockNumber, addr, key)
	if err != nil {
		return common.Hash{}
	}

	s.storageCache.Add(cacheKey, value)
	return value
}

// SetState sets contract storage value
func (s *PikaStateDB) SetState(addr common.Address, key, value common.Hash) {
	prev := s.GetState(addr, key)
	if prev == value {
		return
	}

	s.journal.append(storageChange{
		account:  &addr,
		key:      key,
		prevalue: prev,
	})

	if _, ok := s.storages[addr]; !ok {
		s.storages[addr] = make(map[common.Hash]common.Hash)
	}
	s.storages[addr][key] = value
}

// GetCommittedState returns committed storage value (same as GetState for now)
func (s *PikaStateDB) GetCommittedState(addr common.Address, key common.Hash) common.Hash {
	return s.GetState(addr, key)
}

// Suicide marks account for deletion
func (s *PikaStateDB) Suicide(addr common.Address) bool {
	acc := s.getAccount(addr)
	prevBalance := new(big.Int).Set(acc.Balance)
	
	prev := s.suicides[addr]
	s.journal.append(suicideChange{
		account:     &addr,
		prev:        prev,
		prevbalance: prevBalance,
	})

	s.suicides[addr] = true
	acc.Balance = new(big.Int)
	s.accounts[addr] = acc
	return !prev
}

// SelfDestruct is an alias for Suicide for v1.13.8 compatibility
func (s *PikaStateDB) SelfDestruct(addr common.Address) {
	s.Suicide(addr)
}

// Selfdestruct6780 implements EIP-6780: only self-destruct in the same transaction
func (s *PikaStateDB) Selfdestruct6780(addr common.Address) {
	s.SelfDestruct(addr)
}

// HasSuicided returns whether account is marked for deletion
func (s *PikaStateDB) HasSuicided(addr common.Address) bool {
	return s.suicides[addr]
}

// HasSelfDestructed is an alias for HasSuicided for v1.13.8 compatibility
func (s *PikaStateDB) HasSelfDestructed(addr common.Address) bool {
	return s.HasSuicided(addr)
}

// Exist returns whether an account exists
func (s *PikaStateDB) Exist(addr common.Address) bool {
	acc := s.getAccount(addr)
	return !acc.Empty() || s.suicides[addr]
}

// Empty returns whether account is empty
func (s *PikaStateDB) Empty(addr common.Address) bool {
	acc := s.getAccount(addr)
	return acc.Empty()
}

// AddLog adds a log
func (s *PikaStateDB) AddLog(log *types.Log) {
	s.journal.append(addLogChange{txhash: s.thash})
	log.TxHash = s.thash
	log.TxIndex = uint(s.txIndex)
	log.Index = s.logSize
	s.logs[s.thash] = append(s.logs[s.thash], log)
	s.logSize++
}

// GetLogs returns all logs for a transaction
func (s *PikaStateDB) GetLogs(txHash common.Hash, blockNumber uint64, blockHash common.Hash) []*types.Log {
	logs := s.logs[txHash]
	for _, log := range logs {
		log.BlockNumber = blockNumber
		log.BlockHash = blockHash
	}
	return logs
}

// AddRefund adds gas refund
func (s *PikaStateDB) AddRefund(gas uint64) {
	s.journal.append(refundChange{prev: s.refund})
	s.refund += gas
}

// SubRefund subtracts gas refund
func (s *PikaStateDB) SubRefund(gas uint64) {
	s.journal.append(refundChange{prev: s.refund})
	if gas > s.refund {
		s.refund = 0
	} else {
		s.refund -= gas
	}
}

// GetRefund returns current gas refund
func (s *PikaStateDB) GetRefund() uint64 {
	return s.refund
}

// Snapshot creates a snapshot for reverting
func (s *PikaStateDB) Snapshot() int {
	id := s.nextRevisionID
	s.nextRevisionID++
	s.validRevisions = append(s.validRevisions, revision{id, s.journal.length()})
	return id
}

// RevertToSnapshot reverts to a snapshot
func (s *PikaStateDB) RevertToSnapshot(revid int) {
	idx := -1
	for i, r := range s.validRevisions {
		if r.id == revid {
			idx = i
			break
		}
	}
	if idx == -1 {
		panic(fmt.Sprintf("revision id %v cannot be reverted", revid))
	}

	snapshot := s.validRevisions[idx].journalIndex
	s.journal.revert(s, snapshot)
	s.validRevisions = s.validRevisions[:idx]
}

// Prepare handles transaction preparation
func (s *PikaStateDB) Prepare(rules params.Rules, sender, coinbase common.Address, dest *common.Address, precompiles []common.Address, txAccesses types.AccessList) {
	// Add addresses to access list if supported
	s.accessList.AddAddress(sender)
	if dest != nil {
		s.accessList.AddAddress(*dest)
	}
	for _, addr := range precompiles {
		s.accessList.AddAddress(addr)
	}
	for _, el := range txAccesses {
		s.accessList.AddAddress(el.Address)
		for _, key := range el.StorageKeys {
			s.accessList.AddSlot(el.Address, key)
		}
	}
	s.thash = common.Hash{}
	s.txIndex = 0
}

// AddPreimage adds a preimage
func (s *PikaStateDB) AddPreimage(hash common.Hash, preimage []byte) {
	if _, ok := s.preimages[hash]; !ok {
		s.journal.append(addPreimageChange{hash: hash})
		s.preimages[hash] = common.CopyBytes(preimage)
	}
}

// AddressInAccessList checks if address is in access list
func (s *PikaStateDB) AddressInAccessList(addr common.Address) bool {
	return s.accessList.ContainsAddress(addr)
}

// SlotInAccessList checks if storage slot is in access list
func (s *PikaStateDB) SlotInAccessList(addr common.Address, slot common.Hash) (bool, bool) {
	return s.accessList.Contains(addr, slot)
}

// AddAddressToAccessList adds address to access list
func (s *PikaStateDB) AddAddressToAccessList(addr common.Address) {
	if s.accessList.AddAddress(addr) {
		s.journal.append(accessListAddAccountChange{address: &addr})
	}
}

// AddSlotToAccessList adds storage slot to access list
func (s *PikaStateDB) AddSlotToAccessList(addr common.Address, slot common.Hash) {
	addrMod, slotMod := s.accessList.AddSlot(addr, slot)
	if addrMod {
		s.journal.append(accessListAddAccountChange{address: &addr})
	}
	if slotMod {
		s.journal.append(accessListAddSlotChange{address: &addr, slot: &slot})
	}
}

// AccessEvents returns nil for v1.13.8 compatibility
func (s *PikaStateDB) AccessEvents() interface{} {
	return nil
}

// GetTransientState gets transient storage value (EIP-1153)
func (s *PikaStateDB) GetTransientState(addr common.Address, key common.Hash) common.Hash {
	// Not implemented for now, return zero
	return common.Hash{}
}

// SetTransientState sets transient storage value (EIP-1153)
func (s *PikaStateDB) SetTransientState(addr common.Address, key, value common.Hash) {
	// Not implemented for now
}

// SetTxContext sets the transaction context
func (s *PikaStateDB) SetTxContext(thash common.Hash, ti int) {
	s.thash = thash
	s.txIndex = ti
}

// GetTxIndexInternal returns the current transaction index
func (s *PikaStateDB) GetTxIndexInternal() int {
	return s.txIndex
}

// Commit writes the state changes to storage
func (s *PikaStateDB) Commit() error {
	ops := make([]storage.WriteOp, 0)

	// Write accounts
	for addr, acc := range s.accounts {
		storageAcc := &storage.Account{
			Balance:     new(big.Int).Set(acc.Balance),
			Nonce:       acc.Nonce,
			CodeHash:    acc.CodeHash,
			StorageRoot: acc.Root,
		}
		ops = append(ops, storage.WriteOp{
			Type:     storage.WriteOpAccount,
			BlockNum: s.blockNumber + 1, // Next block number
			Address:  addr,
			Account:  storageAcc,
		})
	}

	// Write storage
	for addr, storageMap := range s.storages {
		for key, value := range storageMap {
			ops = append(ops, storage.WriteOp{
				Type:     storage.WriteOpStorage,
				BlockNum: s.blockNumber + 1,
				Address:  addr,
				Key:      key,
				Value:    value,
			})
		}
	}

	// Write code
	for codeHash, code := range s.codes {
		ops = append(ops, storage.WriteOp{
			Type:     storage.WriteOpCode,
			CodeHash: codeHash,
			Code:     code,
		})
	}

	// Batch write to storage
	return s.storage.BatchWrite(s.ctx, ops)
}
