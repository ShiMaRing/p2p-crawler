package crawler

import (
	"github.com/ethereum/go-ethereum/p2p/enode"
	_ "unsafe"
)

//TODO: implementation of the zeus algorithm
func (c *Crawler) crawlZeus(node *enode.Node) ([]*enode.Node, error) {
	conn, err := c.discv5Pool.Get()
	if err != nil {
		return nil, err
	}
	defer func() {
		conn.Close()
	}()
	//we will implement the zeus algorithm here
	err = conn.Conn.Ping(node)
	if err != nil {
		return nil, err
	}
	//get the new node record
	*node = *conn.Conn.Resolve(node)

	//256 bits

	return nil, nil
}
