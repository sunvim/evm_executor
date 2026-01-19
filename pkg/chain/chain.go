package chain

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/params"
)

// ChainAdapter provides chain-specific configuration
type ChainAdapter interface {
	GetChainConfig() *params.ChainConfig
	GetChainID() *big.Int
	GetName() string
}

// BSCAdapter provides BSC mainnet configuration
type BSCAdapter struct{}

func NewBSCAdapter() *BSCAdapter {
	return &BSCAdapter{}
}

func (b *BSCAdapter) GetName() string {
	return "bsc"
}

func (b *BSCAdapter) GetChainID() *big.Int {
	return big.NewInt(56)
}

func (b *BSCAdapter) GetChainConfig() *params.ChainConfig {
	return &params.ChainConfig{
		ChainID:             big.NewInt(56),
		HomesteadBlock:      big.NewInt(0),
		EIP150Block:         big.NewInt(0),
		EIP155Block:         big.NewInt(0),
		EIP158Block:         big.NewInt(0),
		ByzantiumBlock:      big.NewInt(0),
		ConstantinopleBlock: big.NewInt(0),
		PetersburgBlock:     big.NewInt(0),
		IstanbulBlock:       big.NewInt(0),
		MuirGlacierBlock:    big.NewInt(0),
		BerlinBlock:         big.NewInt(0),
		LondonBlock:         big.NewInt(0),
		ArrowGlacierBlock:   nil,
		MergeNetsplitBlock:  nil,
	}
}

// NewChainAdapter creates a chain adapter based on chain name
func NewChainAdapter(chainName string) (ChainAdapter, error) {
	switch chainName {
	case "bsc":
		return NewBSCAdapter(), nil
	default:
		return nil, fmt.Errorf("unsupported chain: %s", chainName)
	}
}
