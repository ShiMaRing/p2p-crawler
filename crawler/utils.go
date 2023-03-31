package crawler

import (
	"github.com/ethereum/go-ethereum/core"
)

func makeGenesis() *core.Genesis {
	return core.DefaultGenesisBlock()
}
