package crawler

import (
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/enode"
	_ "unsafe"
)

const MAX_LENGTH = 255
const lookupRequestLimit int = 3

var ALL_ZERO enode.ID
var ALL_ONES enode.ID

func init() {
	for i := 0; i < 32; i++ {
		ALL_ZERO[i] = 0
		ALL_ONES[i] = 255
	}
}

//store the from and the end of enode id
type pair struct {
	from enode.ID
	end  enode.ID
}

func newPair(from enode.ID, end enode.ID) *pair {
	return &pair{from: from, end: end}
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

//if a>b return 1 else if a<b return -1 else return 0
func compareId(a, b enode.ID) int {
	for i := 0; i < len(a); i++ {
		if a[i] > b[i] {
			return 1
		} else if a[i] < b[i] {
			return -1
		}
	}
	return 0
}

//0~255
//return c and the length of c ,the length of c is 0~255
//we need to get the common prefix of a and b
//we will return common prefix of a and b and the length of common prefix
//the length of common prefix is 0~255
func getCommonPrefix(a, b enode.ID) int {
	var length int
	for i := 0; i < 32; i++ {
		if a[i] == b[i] {
			length += 8
			continue
		}
		var at = a[i]
		var bt = b[i]
		for j := 7; j >= 0; j-- {
			if (at>>j)&1 == (bt>>j)&1 {
				length++
			} else {
				break
			}
		}
		break
	}
	return length
}

//length bits start with 0 and other bits are 1
func mergeWith01(a enode.ID, length int) enode.ID {
	if length == MAX_LENGTH {
		return a
	}
	//set the length+1 bit with after to 1 ,and the length bit to 0
	//we need to get the length/8 byte
	var byteIndex = length / 8
	//we need to get the length%8 bit
	var bitIndex = length % 8
	var count = a[byteIndex]
	//set the count
	count = count & ^(1 << (7 - bitIndex))
	//set the length+ 1 bit and other to 1
	mask := (uint8(1) << (7 - bitIndex)) - uint8(1)
	count = count | mask
	for i := byteIndex + 1; i < len(a); i++ {
		a[i] = 255
	}
	var b = enode.ID{}
	copy(b[:], a[:])
	return b
}

func mergeWith10(a enode.ID, length int) enode.ID {
	if length == MAX_LENGTH {
		return a
	}
	//set the length+1 bit with after to 0 ,and the length bit to 1
	//we need to get the length/8 byte
	var byteIndex = length / 8
	//we need to get the length%8 bit
	var bitIndex = length % 8
	var count = a[byteIndex]
	//set the count
	count = count | (uint8(1) << (7 - bitIndex))
	//set the length+ 1 bit and other to 1
	mask := ^((uint8(1) << (7 - bitIndex)) - uint8(1))
	count = count & mask
	for i := byteIndex + 1; i < len(a); i++ {
		a[i] = 0
	}
	return a
}

func (c *Crawler) crawlZeus(node *enode.Node) ([]*enode.Node, error) {
	fmt.Println("crawlZeus start")
	conn, err := c.discv5Pool.Get()
	if err != nil {
		return nil, err
	}
	defer func() {
		fmt.Println("crawlZeus end")
		conn.Close()
	}()
	//we will implement the zeus algorithm here
	err = conn.Conn.Ping(node)
	if err != nil {
		return nil, err
	}
	q := newQueue()
	l := set(make(map[enode.ID]*enode.Node))
	m, err := requestL(conn.Conn, node, ALL_ZERO)
	if err != nil {
		return nil, err
	}

	l.merge(m)
	k_first := getClosetKey(m, ALL_ZERO)
	m, err = requestL(conn.Conn, node, ALL_ONES)
	if err != nil {
		return nil, err
	}
	l.merge(m)
	k_last := getClosetKey(m, ALL_ONES)

	if (k_first == nil) || (k_last == nil) {
		return nil, errors.New("k_first or k_last is nil")
	}
	if compareId(k_first.ID(), k_last.ID()) != 0 {
		q.push(newPair(k_first.ID(), k_last.ID()))
	}
	for !q.empty() {
		p := q.pop()
		k1 := p.from
		k2 := p.end
		length := getCommonPrefix(k1, k2)
		s1 := mergeWith01(k1, length)
		s2 := mergeWith10(k2, length)
		if compareId(k1, s1) < 0 {
			m, err = requestL(conn.Conn, node, s1)
			if err != nil {
				return nil, err
			}
			l.merge(m)
			x := getClosetKey(m, s1)
			if compareId(x.ID(), k1) != 0 {
				q.push(newPair(k1, x.ID()))
			}
		}
		if compareId(k2, s2) > 0 {
			m, err = requestL(conn.Conn, node, s2)
			if err != nil {
				return nil, err
			}
			l.merge(m)
			y := getClosetKey(m, s2)
			if compareId(y.ID(), k2) != 0 {
				q.push(newPair(y.ID(), k2))
			}
		}
	}
	var result []*enode.Node
	for _, v := range l {
		result = append(result, v)
	}
	return result, nil
}

func requestL(dis *discover.UDPv5, destNode *enode.Node, target enode.ID) ([]*enode.Node, error) {
	dists := lookupDistances(target, destNode.ID())
	return findnode(dis, destNode, dists)
}

//go:linkname findnode github.com/ethereum/go-ethereum/p2p/discover.(*UDPv5).findnode
func findnode(_ *discover.UDPv5, n *enode.Node, distances []uint) ([]*enode.Node, error)

func lookupDistances(target, dest enode.ID) (dists []uint) {
	td := enode.LogDist(target, dest)
	dists = append(dists, uint(td))
	for i := 1; len(dists) < lookupRequestLimit; i++ {
		if td+i < 256 {
			dists = append(dists, uint(td+i))
		}
		if td-i > 0 {
			dists = append(dists, uint(td-i))
		}
	}
	return dists
}
