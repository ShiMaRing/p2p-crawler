package crawler

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/forkid"
	"github.com/ethereum/go-ethereum/p2p"
	"math/big"
)

type clientInfo struct {
	ClientType      string    // ClientType is the name of the client software
	SoftwareVersion uint64    // SoftwareVersion is the version of the client software
	Capabilities    []p2p.Cap // Capabilities is the list of p2p protocol versions supported by the client
	NetworkID       uint64    // NetworkID is the network ID of the chain
	ForkID          forkid.ID // ForkID is the fork ID of the chain
	Blockheight     string
	TotalDifficulty *big.Int
	HeadHash        common.Hash
}
