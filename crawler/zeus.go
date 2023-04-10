package crawler

import (
	"github.com/ethereum/go-ethereum/p2p/enode"
	_ "unsafe"
)

//store the from and the end of enode id
type pair struct {
	from enode.ID
	end  enode.ID
}

type queue struct {
	nums int
	data []*pair
}

func newQueue() *queue {
	return &queue{
		nums: 0,
		data: make([]*pair, 0),
	}
}

func (q *queue) push(p *pair) {
	q.data = append(q.data, p)
	q.nums++
}

func (q *queue) pop() *pair {
	if q.nums == 0 {
		return nil
	}
	p := q.data[0]
	q.data = q.data[1:]
	q.nums--
	return p
}

//whether the queue is empty
func (q *queue) empty() bool {
	return q.nums == 0
}

type set map[enode.ID]*enode.Node

func (s set) merge(arr []*enode.Node) {
	for _, n := range arr {
		s[n.ID()] = n
	}
}

//getClosetKey returns the closet key with the given key
func getClosetKey(arr []*enode.Node, target enode.ID) *enode.Node {
	//calculate the distance between the node and the target
	if len(arr) == 0 {
		return nil
	}
	if len(arr) == 1 {
		return arr[0]
	}

	var closet = arr[0]
	for i := 1; i < len(arr); i++ {
		if enode.DistCmp(target, arr[i].ID(), closet.ID()) < 0 {
			closet = arr[i]
		}
	}
	return closet
}

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
