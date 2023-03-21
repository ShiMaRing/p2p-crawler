package crawler

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/p2p/discover/v4wire"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/netutil"
	"net"
	"time"
)

const expiration = 20 * time.Second
const maxPacketSize = 1200

type encPubkey [64]byte

// Ping send a ping to the node
func (c *Crawler) Ping(conn UDPConn, ld *enode.LocalNode, node *enode.Node, prk *ecdsa.PrivateKey) error {
	toaddr := &net.UDPAddr{IP: node.IP(), Port: node.UDP()}
	n := ld.Node()
	a := &net.UDPAddr{IP: n.IP(), Port: n.UDP()}
	endPoint := v4wire.NewEndpoint(a, uint16(n.TCP()))
	req := &v4wire.Ping{
		Version:    4,
		From:       endPoint,
		To:         v4wire.NewEndpoint(toaddr, 0),
		Expiration: uint64(time.Now().Add(expiration).Unix()),
		ENRSeq:     ld.Seq(),
	}
	packet, _, err := v4wire.Encode(prk, req)
	if err != nil {
		return err
	}
	ld.UDPContact(toaddr)
	nbytes, err := conn.WriteToUDP(packet, &net.UDPAddr{IP: node.IP(), Port: node.UDP()})
	if err == nil {
		c.counter.AddDataSizeSent(uint64(nbytes))
		c.counter.AddSendNum()
	}
	time.Sleep(respTimeout)
	return err
}

// Pong send a pong back
func (c *Crawler) Pong(conn UDPConn, ld *enode.LocalNode, prk *ecdsa.PrivateKey,
	tmp v4wire.Packet, mac []byte, from *net.UDPAddr) error {
	// Reply.
	req, success := tmp.(*v4wire.Ping)
	if !success {
		return errors.New("wrong packet type")
	}
	//make a ping packet
	pongPacket := &v4wire.Pong{
		To:         v4wire.NewEndpoint(from, req.From.TCP),
		ReplyTok:   mac,
		Expiration: uint64(time.Now().Add(expiration).Unix()),
		ENRSeq:     ld.Node().Seq(),
	}
	packet, _, err := v4wire.Encode(prk, pongPacket)
	if err != nil {
		return err
	}
	nbytes, err := conn.WriteToUDP(packet, from)
	if err != nil {
		c.counter.AddDataSizeSent(uint64(nbytes))
		c.counter.AddSendNum()
	}
	time.Sleep(respTimeout)
	return err
}

// handle the incoming packets
func (c *Crawler) handleResponse(conn UDPConn) (*net.UDPAddr, v4wire.Packet, v4wire.Pubkey, []byte, error) {
	//make a ping packet for the node
	buf := make([]byte, maxPacketSize)
	nbytes, from, err := conn.ReadFromUDP(buf)
	if netutil.IsTemporaryError(err) {
		return nil, nil, [64]byte{}, nil, err
	} else if err != nil {
		return nil, nil, [64]byte{}, nil, err
	}

	//count the number of packets received
	c.counter.AddDataSizeReceived(uint64(nbytes))
	c.counter.AddRecvNum()

	rawpacket, pubkey, hash, err := v4wire.Decode(buf[:nbytes])
	if err != nil {
		return nil, nil, [64]byte{}, nil, err
	}
	//handle the packet may be pong or find node response
	switch t := rawpacket.(type) {
	case *v4wire.Pong:
		return from, rawpacket.(*v4wire.Pong), pubkey, hash, nil
	case *v4wire.Neighbors:
		return from, rawpacket.(*v4wire.Neighbors), pubkey, hash, nil
	case *v4wire.Ping:
		return from, rawpacket.(*v4wire.Ping), pubkey, hash, nil
	case *v4wire.ENRRequest:
		return from, rawpacket.(*v4wire.ENRRequest), pubkey, hash, nil
	default:
		name := t.Name()
		kind := t.Kind()
		return nil, nil, [64]byte{}, nil, errors.New(fmt.Sprintf("unknown packet type %s %d", name, kind))
	}
}

func (c *Crawler) findNode(conn UDPConn, node *enode.Node, prk *ecdsa.PrivateKey, targetNode *enode.Node) error {
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
		return errors.New(fmt.Sprintf("encode find node packet error %s", err))
	}
	nbytes, err := conn.WriteToUDP(packet, &net.UDPAddr{IP: node.IP(), Port: node.UDP()})
	if err != nil {
		return errors.New(fmt.Sprintf("send find node packet error %s", err))
	}
	c.counter.AddDataSizeSent(uint64(nbytes))
	c.counter.AddSendNum()
	time.Sleep(respTimeout)
	return nil
}
