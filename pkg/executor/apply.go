package executor

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"

	"github.com/sunvim/evm_executor/pkg/state"
)

// ChainContext supports retrieving chain data
type ChainContext interface {
	// GetHeader returns the hash corresponding to their hash
	GetHeader(common.Hash, uint64) *types.Header
}

// ApplyTransaction attempts to apply a transaction to the given state database
// and uses the input parameters for its environment. It returns the receipt
// for the transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
func ApplyTransaction(
	config *params.ChainConfig,
	bc ChainContext,
	author *common.Address,
	gp *core.GasPool,
	stateDB *state.PikaStateDB,
	header *types.Header,
	tx *types.Transaction,
	usedGas *uint64,
	cfg vm.Config,
) (*types.Receipt, error) {
	// Create message from transaction
	msg, err := core.TransactionToMessage(tx, types.MakeSigner(config, header.Number, header.Time), header.BaseFee)
	if err != nil {
		return nil, err
	}

	// Create a new context to be used in the EVM environment
	blockContext := NewEVMBlockContext(header, bc, author)
	txContext := core.NewEVMTxContext(msg)
	vmenv := vm.NewEVM(blockContext, txContext, stateDB, config, cfg)

	return applyTransaction(msg, config, gp, stateDB, header, tx, usedGas, vmenv)
}

func applyTransaction(
	msg *core.Message,
	config *params.ChainConfig,
	gp *core.GasPool,
	stateDB *state.PikaStateDB,
	header *types.Header,
	tx *types.Transaction,
	usedGas *uint64,
	evm *vm.EVM,
) (*types.Receipt, error) {
	// Set transaction context
	stateDB.SetTxContext(tx.Hash(), len(stateDB.GetLogs(tx.Hash(), header.Number.Uint64(), header.Hash())))

	// Apply the transaction to the current state
	result, err := core.ApplyMessage(evm, msg, gp)
	if err != nil {
		return nil, err
	}

	// Update the state with pending changes
	var root []byte
	if config.IsByzantium(header.Number) {
		// For post-Byzantium, we don't calculate state root
		root = nil
	}

	*usedGas += result.UsedGas

	// Create a new receipt for the transaction
	receipt := &types.Receipt{
		Type:              tx.Type(),
		PostState:         root,
		CumulativeGasUsed: *usedGas,
	}

	if result.Failed() {
		receipt.Status = types.ReceiptStatusFailed
	} else {
		receipt.Status = types.ReceiptStatusSuccessful
	}

	receipt.TxHash = tx.Hash()
	receipt.GasUsed = result.UsedGas

	// If the transaction created a contract, store the creation address in the receipt
	if msg.To == nil {
		receipt.ContractAddress = crypto.CreateAddress(evm.TxContext.Origin, tx.Nonce())
	}

	// Set the receipt logs and create the bloom filter
	receipt.Logs = stateDB.GetLogs(tx.Hash(), header.Number.Uint64(), header.Hash())
	receipt.BlockHash = header.Hash()
	receipt.BlockNumber = header.Number
	receipt.TransactionIndex = uint(stateDB.GetTxIndexInternal())

	return receipt, nil
}

// NewEVMBlockContext creates a new context for use in the EVM
func NewEVMBlockContext(header *types.Header, chain ChainContext, author *common.Address) vm.BlockContext {
	var beneficiary common.Address
	if author != nil {
		beneficiary = *author
	} else {
		beneficiary = header.Coinbase
	}

	return vm.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		GetHash:     GetHashFn(header, chain),
		Coinbase:    beneficiary,
		BlockNumber: new(big.Int).Set(header.Number),
		Time:        header.Time,
		Difficulty:  new(big.Int).Set(header.Difficulty),
		BaseFee:     header.BaseFee,
		GasLimit:    header.GasLimit,
	}
}

// GetHashFn returns a GetHashFunc which retrieves block hashes by number
func GetHashFn(header *types.Header, chain ChainContext) vm.GetHashFunc {
	// Cache will initially contain [refHash.parent],
	// Then refHash.parent.parent, and so on
	var cache = make(map[uint64]common.Hash)

	return func(n uint64) common.Hash {
		// If there's no hash cache yet, use the chain context
		if hash, ok := cache[n]; ok {
			return hash
		}

		// Try to get from chain
		if header := chain.GetHeader(header.ParentHash, n); header != nil {
			hash := header.Hash()
			cache[n] = hash
			return hash
		}

		return common.Hash{}
	}
}
