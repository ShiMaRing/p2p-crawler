package crawler

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/discover/v4wire"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/netutil"
	"github.com/ethereum/go-ethereum/params"
	"io"
	"log"
	"net"
	"testing"
	"time"
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

//PASSED
//start with random node,
func TestBootStartWithRandomNodes(t *testing.T) {
	nodes := make([]*enode.Node, 0)
	var err error
	//start from the MainBootNodes
	s := params.MainnetBootnodes
	for _, record := range s {
		node, err := parseNode(record)
		if err != nil {
			t.Fatal(err)
		}
		nodes = append(nodes, node)
	}
	db, _ := enode.OpenDB("")

	ld, cfg := makeDiscoveryConfig(db, nodes)
	conn := listen(ld, "")
	v4, err := discover.ListenV4(conn, ld, cfg)
	if err != nil {
		t.Fatal(err)
	}
	key, _ := crypto.GenerateKey()
	pbkey := &key.PublicKey
	fmt.Printf("start node with id: %s\n", pbkey)
	nodes = v4.LookupPubkey(pbkey)
	fmt.Printf("found nodes: %d\n", len(nodes))
	for i := range nodes {
		fmt.Printf("node found: %s\n", nodes[i])
	}
}

var MyBootNodes = []string{
	// Ethereum Foundation Go Bootnodes
	"enode://103858bdb88756c71f15e9b5e09b56dc1be52f0a5021d46301dbbfb7e130029cc9d0d6f73f693bc29b665770fff7da4d34f3c6379fe12721b5d7a0bcb5ca1fc1@191.234.162.198:30303", // bootnode-aws-ap-southeast-1-001
	"enode://5d6d7cd20d6da4bb83a1d28cadb5d409b64edf314c0335df658c1a54e32c7c4a7ab7823d57c39b6a757556e68ff1df17c748b698544a55cb488b52479a92b60f@104.42.217.25:30303",
	"enode://8499da03c47d637b20eee24eec3c356c9a2e6148d6fe25ca195c7949ab8ec2c03e3556126b0d7ed644675e78c4318b08691b7b57de10e5f0d40d05b09238fa0a@52.187.207.27:30303",
	"enode://4aeb4ab6c14b23e2c4cfdce879c04b0748a20d8e9b59e25ded2a08143e265c6c25936e74cbc8e641e3312ca288673d91f2f93f8e277de3cfa444ecdaaf982052@157.90.35.166:30303",
	"enode://715171f50508aba88aecd1250af392a45a330af91d7b90701c436b618c86aaa1589c9184561907bebbb56439b8f8787bc01f49a7c77276c58c1b09822d75e8e8@52.231.165.108:30303",
	"enode://2b252ab6a1d0f971d9722cb839a42cb81db019ba44c08754628ab4a823487071b5695317c8ccd085219c3a03af063495b2f1da8d18218da2d6a82981b45e6ffc@65.108.70.101:30303",
	"enode://22a8232c3abc76a16ae9d6c3b164f98775fe226f0917b0ca871128a74a8e9630b458460865bab457221f1d448dd9791d24c4e5d88786180ac185df813a68d4de@3.209.45.79:30303",
	"enode://d860a01f9722d78051619d1e2351aba3f43f943f6f00718d1b9baa4101932a1f5011f16bb2b1bb35db20d6fe28fa0bf09636d26a87d31de9ec6203eeedb1f666@18.138.108.67:30303",
}

type encPubkey [64]byte

const expiration = 20 * time.Second
const maxPacketSize = 1200

func TestFindNode(t *testing.T) {
	var err error
	//open db in memory
	db, _ := enode.OpenDB("")
	prk, _ := crypto.GenerateKey()
	ld := enode.NewLocalNode(db, prk)
	conn := listen(ld, "")

	targetNode, _ := parseNode("enode://5d6d7cd20d6da4bb83a1d28cadb5d409b64edf314c0335df658c1a54e32c7c4a7ab7823d57c39b6a757556e68ff1df17c748b698544a55cb488b52479a92b60f@104.42.217.25:30303")
	//make packet and send it to the target node
	node, _ := parseNode("enode://0694c08573902a5daa723b255fe30bb47a74398c1837e9550f4aa1868116c2ee416df026fb9f1533891c1a2d6fbe3b49a6a11f533e200d26a668809f40e565e6@150.136.69.241:5050")

	fmt.Printf("target node ID: %s\n", targetNode.ID())
	if err != nil {
		t.Fatal(err)
	}
	err = MyPing(conn, ld, node, prk)
	if err != nil {
		t.Fatal(err)
	}
	_, err = handleResponse(conn)
	if err != nil {
		t.Fatal(err)
	}
	//send a find node request to the target node
	//get the public key of the target node
	pbkey := targetNode.Pubkey()
	var e encPubkey
	math.ReadBits(pbkey.X, e[:len(e)/2])
	math.ReadBits(pbkey.Y, e[len(e)/2:])
	ekey := v4wire.Pubkey(e)
	//make a find node packet
	req := &v4wire.Findnode{
		Target:     ekey,
		Expiration: uint64(time.Now().Add(expiration).Unix()),
	}
	packet, _, err := v4wire.Encode(prk, req)
	if err != nil {
		t.Fatal(err)
	}
	_, err = conn.WriteToUDP(packet, &net.UDPAddr{IP: node.IP(), Port: node.UDP()})
	if err != nil {
		t.Fatal(err)
	}

	neighbor, err := handleResponse(conn)
	if err != nil {
		t.Fatal(err)
	}
	nodes := neighbor.(*v4wire.Neighbors).Nodes
	for _, n := range nodes {
		fmt.Println(n.ID)
	}
}

//send a ping request to the target node
func MyPing(conn UDPConn, ld *enode.LocalNode, node *enode.Node, prk *ecdsa.PrivateKey) error {
	toaddr := &net.UDPAddr{IP: node.IP(), Port: node.UDP()}
	n := ld.Node()
	a := &net.UDPAddr{IP: n.IP(), Port: n.UDP()}
	endPoint := v4wire.NewEndpoint(a, uint16(n.TCP()))
	//make a ping packet
	req := &v4wire.Ping{
		Version:    4,
		From:       endPoint,
		To:         v4wire.NewEndpoint(toaddr, 0),
		Expiration: uint64(time.Now().Add(expiration).Unix()),
		ENRSeq:     ld.Seq(),
	}
	packet, _, err := v4wire.Encode(prk, req)
	if err != nil {
		log.Fatalln("encode error", err)
		return err
	}
	ld.UDPContact(toaddr)
	_, err = conn.WriteToUDP(packet, &net.UDPAddr{IP: node.IP(), Port: node.UDP()})
	if err != nil {
		log.Fatalln("encode error", err)
		return err
	}
	return nil
}

//Passed
func TestPing(t *testing.T) {
	//open db in memory
	db, _ := enode.OpenDB("")
	prk, _ := crypto.GenerateKey()
	ld := enode.NewLocalNode(db, prk)
	conn := listen(ld, "")
	//target node
	node, _ := parseNode("enode://0694c08573902a5daa723b255fe30bb47a74398c1837e9550f4aa1868116c2ee416df026fb9f1533891c1a2d6fbe3b49a6a11f533e200d26a668809f40e565e6@150.136.69.241:5050")
	toaddr := &net.UDPAddr{IP: node.IP(), Port: node.UDP()}
	n := ld.Node()
	a := &net.UDPAddr{IP: n.IP(), Port: n.UDP()}
	endPoint := v4wire.NewEndpoint(a, uint16(n.TCP()))
	//make a ping packet
	req := &v4wire.Ping{
		Version:    4,
		From:       endPoint,
		To:         v4wire.NewEndpoint(toaddr, 0),
		Expiration: uint64(time.Now().Add(expiration).Unix()),
		ENRSeq:     ld.Seq(),
	}
	packet, _, err := v4wire.Encode(prk, req)
	if err != nil {
		t.Fatal("encode error", err)
	}
	ld.UDPContact(toaddr)
	_, err = conn.WriteToUDP(packet, &net.UDPAddr{IP: node.IP(), Port: node.UDP()})
	if err != nil {
		t.Fatal("write to udp error", err)
	}

}

// handle the incoming packets
func handleResponse(conn UDPConn) (v4wire.Packet, error) {
	//make a ping packet for the node
	buf := make([]byte, maxPacketSize)
	nbytes, _, err := conn.ReadFromUDP(buf)
	if netutil.IsTemporaryError(err) {
		// Ignore temporary read errors.
		fmt.Printf("temporary read error: %v", err)
		return nil, err
	} else if err != nil {
		// Shut down the loop for permanent errors.
		if !errors.Is(err, io.EOF) {
			fmt.Printf("permanent read error: %v", err)
		}
		return nil, err
	}
	fmt.Printf("read %d bytes:%s \n", nbytes, string(buf[:nbytes]))
	//handle the packet
	rawpacket, _, _, err := v4wire.Decode(buf[:nbytes])
	if err != nil {
		fmt.Printf("decode error: %v", err)
		return nil, err
	}
	//handle the packet may be pong or find node response
	switch t := rawpacket.(type) {
	case *v4wire.Pong:
		fmt.Printf("pong received: %v", rawpacket.(*v4wire.Pong).ReplyTok)
		return rawpacket.(*v4wire.Pong), nil
	case *v4wire.Neighbors:
		fmt.Printf("neighbors received: %v", rawpacket.(*v4wire.Neighbors).Nodes)
		return rawpacket.(*v4wire.Neighbors), nil
	default:
		name := t.Name()
		kind := t.Kind()
		fmt.Printf("unknown packet type: %s and kind is %s", name, string(kind))
	}
	return nil, err
}

//passed
func TestOriginPing(t *testing.T) {
	var err error
	target, err := parseNode("enode://0694c08573902a5daa723b255fe30bb47a74398c1837e9550f4aa1868116c2ee416df026fb9f1533891c1a2d6fbe3b49a6a11f533e200d26a668809f40e565e6@150.136.69.241:5050")
	if err != nil {
		t.Fatal(err)
	}
	nodes := make([]*enode.Node, 0)
	//start from the MainBootNodes
	s := params.MainnetBootnodes
	for _, record := range s {
		node, err := parseNode(record)
		if err != nil {
			t.Fatal(err)
		}
		nodes = append(nodes, node)
	}
	db, _ := enode.OpenDB("")
	ld, cfg := makeDiscoveryConfig(db, nodes)
	conn := listen(ld, "")
	v4, err := discover.ListenV4(conn, ld, cfg)
	if err != nil {
		t.Fatal(err)
	}
	err = v4.Ping(target)
	if err != nil {
		t.Fatal(err)
	}
}
