package state

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// Account represents an Ethereum account state
type Account struct {
	Balance  *big.Int
	Nonce    uint64
	CodeHash common.Hash
	Root     common.Hash // Storage root
}

// NewAccount creates a new account with zero balance
func NewAccount() *Account {
	return &Account{
		Balance:  new(big.Int),
		Nonce:    0,
		CodeHash: common.Hash{},
		Root:     common.Hash{},
	}
}

// Copy creates a deep copy of the account
func (a *Account) Copy() *Account {
	return &Account{
		Balance:  new(big.Int).Set(a.Balance),
		Nonce:    a.Nonce,
		CodeHash: a.CodeHash,
		Root:     a.Root,
	}
}

// Empty returns whether the account is empty (no balance, nonce=0, no code)
func (a *Account) Empty() bool {
	return a.Nonce == 0 && a.Balance.Sign() == 0 && a.CodeHash == common.Hash{}
}

// journalEntry is an interface for state change journal entries
type journalEntry interface {
	revert(*PikaStateDB)
	dirtied() *common.Address
}

// Journal tracks state changes for reverting
type journal struct {
	entries []journalEntry
	dirties map[common.Address]int // Dirty accounts and the index in the journal
}

func newJournal() *journal {
	return &journal{
		entries: make([]journalEntry, 0),
		dirties: make(map[common.Address]int),
	}
}

func (j *journal) append(entry journalEntry) {
	j.entries = append(j.entries, entry)
	if addr := entry.dirtied(); addr != nil {
		j.dirties[*addr] = len(j.entries) - 1
	}
}

func (j *journal) revert(statedb *PikaStateDB, snapshot int) {
	for i := len(j.entries) - 1; i >= snapshot; i-- {
		j.entries[i].revert(statedb)
	}
	j.entries = j.entries[:snapshot]

	// Rebuild dirties map
	j.dirties = make(map[common.Address]int)
	for i, entry := range j.entries {
		if addr := entry.dirtied(); addr != nil {
			j.dirties[*addr] = i
		}
	}
}

func (j *journal) length() int {
	return len(j.entries)
}

func (j *journal) dirty(addr common.Address) {
	if _, exist := j.dirties[addr]; !exist {
		j.dirties[addr] = len(j.entries)
	}
}

// Journal entry types
type (
	createObjectChange struct {
		account *common.Address
	}

	resetObjectChange struct {
		prev *Account
		addr common.Address
	}

	suicideChange struct {
		account     *common.Address
		prev        bool
		prevbalance *big.Int
	}

	balanceChange struct {
		account *common.Address
		prev    *big.Int
	}

	nonceChange struct {
		account *common.Address
		prev    uint64
	}

	storageChange struct {
		account  *common.Address
		key      common.Hash
		prevalue common.Hash
	}

	codeChange struct {
		account  *common.Address
		prevcode []byte
		prevhash common.Hash
	}

	refundChange struct {
		prev uint64
	}

	addLogChange struct {
		txhash common.Hash
	}

	addPreimageChange struct {
		hash common.Hash
	}

	touchChange struct {
		account *common.Address
	}

	accessListAddAccountChange struct {
		address *common.Address
	}

	accessListAddSlotChange struct {
		address *common.Address
		slot    *common.Hash
	}
)

func (ch createObjectChange) revert(s *PikaStateDB) {
	delete(s.accounts, *ch.account)
}

func (ch createObjectChange) dirtied() *common.Address {
	return ch.account
}

func (ch resetObjectChange) revert(s *PikaStateDB) {
	s.accounts[ch.addr] = ch.prev
}

func (ch resetObjectChange) dirtied() *common.Address {
	return &ch.addr
}

func (ch suicideChange) revert(s *PikaStateDB) {
	s.suicides[*ch.account] = ch.prev
	if !ch.prev {
		delete(s.suicides, *ch.account)
	}
	if ch.prevbalance != nil {
		if acc, ok := s.accounts[*ch.account]; ok {
			acc.Balance = new(big.Int).Set(ch.prevbalance)
		}
	}
}

func (ch suicideChange) dirtied() *common.Address {
	return ch.account
}

func (ch touchChange) revert(s *PikaStateDB) {
}

func (ch touchChange) dirtied() *common.Address {
	return ch.account
}

func (ch balanceChange) revert(s *PikaStateDB) {
	if acc, ok := s.accounts[*ch.account]; ok {
		acc.Balance = ch.prev
	}
}

func (ch balanceChange) dirtied() *common.Address {
	return ch.account
}

func (ch nonceChange) revert(s *PikaStateDB) {
	if acc, ok := s.accounts[*ch.account]; ok {
		acc.Nonce = ch.prev
	}
}

func (ch nonceChange) dirtied() *common.Address {
	return ch.account
}

func (ch codeChange) revert(s *PikaStateDB) {
	if acc, ok := s.accounts[*ch.account]; ok {
		acc.CodeHash = ch.prevhash
	}
	s.codes[ch.prevhash] = ch.prevcode
}

func (ch codeChange) dirtied() *common.Address {
	return ch.account
}

func (ch storageChange) revert(s *PikaStateDB) {
	if _, ok := s.storages[*ch.account]; !ok {
		s.storages[*ch.account] = make(map[common.Hash]common.Hash)
	}
	s.storages[*ch.account][ch.key] = ch.prevalue
}

func (ch storageChange) dirtied() *common.Address {
	return ch.account
}

func (ch refundChange) revert(s *PikaStateDB) {
	s.refund = ch.prev
}

func (ch refundChange) dirtied() *common.Address {
	return nil
}

func (ch addLogChange) revert(s *PikaStateDB) {
	logs := s.logs[ch.txhash]
	if len(logs) == 1 {
		delete(s.logs, ch.txhash)
	} else {
		s.logs[ch.txhash] = logs[:len(logs)-1]
	}
	s.logSize--
}

func (ch addLogChange) dirtied() *common.Address {
	return nil
}

func (ch addPreimageChange) revert(s *PikaStateDB) {
	delete(s.preimages, ch.hash)
}

func (ch addPreimageChange) dirtied() *common.Address {
	return nil
}

func (ch accessListAddAccountChange) revert(s *PikaStateDB) {
	s.accessList.DeleteAddress(*ch.address)
}

func (ch accessListAddAccountChange) dirtied() *common.Address {
	return nil
}

func (ch accessListAddSlotChange) revert(s *PikaStateDB) {
	s.accessList.DeleteSlot(*ch.address, *ch.slot)
}

func (ch accessListAddSlotChange) dirtied() *common.Address {
	return nil
}

// AccessList is the access list implementation
type AccessList struct {
	addresses map[common.Address]int
	slots     []map[common.Hash]struct{}
}

func newAccessList() *AccessList {
	return &AccessList{
		addresses: make(map[common.Address]int),
		slots:     make([]map[common.Hash]struct{}, 0),
	}
}

func (a *AccessList) Copy() *AccessList {
	cp := newAccessList()
	for addr, idx := range a.addresses {
		cp.addresses[addr] = idx
		cp.slots = append(cp.slots, make(map[common.Hash]struct{}))
		for slot := range a.slots[idx] {
			cp.slots[idx][slot] = struct{}{}
		}
	}
	return cp
}

func (a *AccessList) AddAddress(address common.Address) bool {
	if _, present := a.addresses[address]; present {
		return false
	}
	a.addresses[address] = len(a.slots)
	a.slots = append(a.slots, make(map[common.Hash]struct{}))
	return true
}

func (a *AccessList) AddSlot(address common.Address, slot common.Hash) (bool, bool) {
	addrPresent := true
	idx, ok := a.addresses[address]
	if !ok {
		addrPresent = false
		idx = len(a.slots)
		a.addresses[address] = idx
		a.slots = append(a.slots, make(map[common.Hash]struct{}))
	}
	if _, present := a.slots[idx][slot]; present {
		return addrPresent, false
	}
	a.slots[idx][slot] = struct{}{}
	return addrPresent, true
}

func (a *AccessList) ContainsAddress(address common.Address) bool {
	_, ok := a.addresses[address]
	return ok
}

func (a *AccessList) Contains(address common.Address, slot common.Hash) (bool, bool) {
	idx, ok := a.addresses[address]
	if !ok {
		return false, false
	}
	_, slotPresent := a.slots[idx][slot]
	return true, slotPresent
}

func (a *AccessList) DeleteAddress(address common.Address) {
	delete(a.addresses, address)
}

func (a *AccessList) DeleteSlot(address common.Address, slot common.Hash) {
	idx, ok := a.addresses[address]
	if !ok {
		return
	}
	delete(a.slots[idx], slot)
}

// Convert to types.AccessList
func (a *AccessList) ToAccessList() types.AccessList {
	var accessList types.AccessList
	for addr, idx := range a.addresses {
		entry := types.AccessTuple{
			Address: addr,
		}
		for slot := range a.slots[idx] {
			entry.StorageKeys = append(entry.StorageKeys, slot)
		}
		accessList = append(accessList, entry)
	}
	return accessList
}
