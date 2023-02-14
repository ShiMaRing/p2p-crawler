package crawler

import (
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"net"
	"reflect"
	"time"
)

// Node represents a node in the network.
type Node struct {
	ID          enode.ID  `json:"ID,omitempty"`          // The node's public key
	Seq         uint64    `json:"seq,omitempty"`         // The node's sequence number,tracks the number of times the node has been updated
	AccessTime  time.Time `json:"accessTime"`            // The time of last successful contact
	Address     net.IP    `json:"address,omitempty"`     // The IP address of the node
	ConnectAble bool      `json:"connectAble,omitempty"` // The node is ConnectAble
	n           *enode.Node
}

// GetEnodeV4 get all nodes from this node
func (node *Node) GetEnodeV4(ch chan *Node, db *enode.DB, bootNodes []*enode.Node) []*Node {
	//make a local node for crawl
	//generate a local node
	ln, cfg := makeDiscoveryConfig(db, bootNodes)
	conn := listen(ln, "")
	//set up udp connection for this node
	disc, err := discover.ListenV4(conn, ln, cfg)
	if err != nil {
		panic(err)
	}
	defer disc.Close()
	//crawl the node ,send it to the channel and return result

	//return the nodes
	return nil
}

func makeDiscoveryConfig(db *enode.DB, bootNodes []*enode.Node) (*enode.LocalNode, discover.Config) {
	var cfg discover.Config
	var err error
	cfg.PrivateKey, _ = crypto.GenerateKey()
	cfg.Bootnodes = bootNodes
	if err != nil {
		panic(err)
	}
	return enode.NewLocalNode(db, cfg.PrivateKey), cfg
}

func listen(ln *enode.LocalNode, addr string) *net.UDPConn {
	if addr == "" {
		addr = "0.0.0.0:0"
	}
	socket, err := net.ListenPacket("udp4", addr)
	if err != nil {
		panic(err)
	}
	usocket := socket.(*net.UDPConn)
	uaddr := socket.LocalAddr().(*net.UDPAddr)
	if uaddr.IP.IsUnspecified() {
		ln.SetFallbackIP(net.IP{127, 0, 0, 1})
	} else {
		ln.SetFallbackIP(uaddr.IP)
	}
	ln.SetFallbackUDP(uaddr.Port)
	return usocket
}

func modifyID(n *enode.LocalNode, id enode.ID) {
	v := reflect.ValueOf(n)
	v.Elem().FieldByName("id").Set(reflect.ValueOf(id))
}
