package crawler

import (
	"github.com/ethereum/go-ethereum/core"
	"sync"
)

var geneis *core.Genesis
var once = &sync.Once{}

func makeGenesis() *core.Genesis {
	once.Do(func() {
		geneis = core.DefaultGenesisBlock()
	})
	return geneis
}
