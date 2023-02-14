package crawler

import (
	"fmt"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"testing"
)

func TestGetUDPV4(t *testing.T) {
	var n = &enode.Node{}
	var node = &Node{n: n}
	db, err := enode.OpenDB("")
	if err != nil {
		t.Fatal(err)
	}
	_ = node.GetEnodeV4(make(chan *Node), db, nil)
}

func TestModifyID(t *testing.T) {

	db, _ := enode.OpenDB("")

	key, _ := crypto.GenerateKey()
	ln := enode.NewLocalNode(db, key)
	fmt.Println(ln.ID())

}
