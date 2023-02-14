package crawler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"sort"
	"time"
)

type nodeSet map[enode.ID]Node

//ParseEnNode parse the enode.Node to Node. after successfully connected to the node.
func ParseEnNode(n *enode.Node, connected bool) Node {
	return Node{
		ID:          n.ID(),
		Seq:         n.Seq(),
		AccessTime:  time.Now(),
		Address:     n.IP(),
		ConnectAble: connected,
		n:           n,
	}
}

// Node2Json MarshalJSON implements the json.Marshaler interface.
func (n *Node) Node2Json() ([]byte, error) {
	return json.Marshal(n)
}

// AddNode add a node to the nodeSet.
func (s nodeSet) AddNode(n *enode.Node, connected bool) {
	s[n.ID()] = ParseEnNode(n, connected)
}

// GetNode get a node from the nodeSet.
func (s nodeSet) GetNode(id enode.ID) *Node {
	if n, ok := s[id]; ok {
		return &n
	}
	return nil
}

// Contain check if the nodeSet contains the node.
func (s nodeSet) Contain(n *enode.Node) bool {
	if _, ok := s[n.ID()]; ok {
		return true
	}
	return false
}

//RemoveNode Remove a node from the nodeSet.
func (s nodeSet) RemoveNode(n *enode.Node) {
	delete(s, n.ID())
}

//OutputNodes output the nodeSet to a slice.
func (s nodeSet) OutputNodes() []*enode.Node {
	var nodes []*enode.Node
	for _, v := range s {
		nodes = append(nodes, v.n)
	}
	sort.Slice(nodes, func(i, j int) bool {
		return bytes.Compare(nodes[i].ID().Bytes(), nodes[j].ID().Bytes()) < 0
	})
	return nodes
}

//PrintNodes print the nodeSet.
func (s nodeSet) PrintNodes() {
	for _, v := range s {
		fmt.Println(v.n.String())
	}
}
